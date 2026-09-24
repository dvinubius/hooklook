#!/usr/bin/env bash

set -euo pipefail

project_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
readonly project_dir

die() {
	printf '%s\n' "$*" >&2
	exit 1
}

# This must remain the first deployment action. In particular, do not validate
# environment variables, open SSH, or sync files before the local test suite
# has passed.
cd "$project_dir"
go test -count=1 ./...

require_environment() {
	[[ -n ${DEPLOY_HOST:-} ]] || die 'DEPLOY_HOST is required.'
	case ${DEPLOY_PROFILE:-core} in
		core|full|observability|dashboard) ;;
		*) die 'DEPLOY_PROFILE must be core, full, observability, or dashboard.' ;;
	esac
	if [[ ${DEPLOY_PROFILE:-core} == observability || ${DEPLOY_PROFILE:-core} == dashboard ]]; then
		return
	fi
	[[ -n ${PUBLIC_BASE_URL:-} ]] || die 'PUBLIC_BASE_URL is required.'
	[[ -n ${ADMIN_TOKEN:-} ]] || die 'ADMIN_TOKEN is required.'

	if [[ ! $PUBLIC_BASE_URL =~ ^https://[^[:space:]/]+$ ]]; then
		die 'PUBLIC_BASE_URL must be an HTTPS origin without a path.'
	fi
	if [[ ! $ADMIN_TOKEN =~ ^[A-Za-z0-9_-]{32,}$ ]]; then
		die 'ADMIN_TOKEN must be at least 32 URL-safe characters.'
	fi
	if [[ ${DEPLOY_PROFILE:-core} == full ]]; then
		[[ -n ${GRAFANA_ADMIN_USER:-} && -n ${GRAFANA_ADMIN_PASSWORD:-} ]] || die 'Full profile requires Hooklook Grafana credentials.'
		[[ $GRAFANA_ADMIN_USER =~ ^[A-Za-z0-9_-]+$ && $GRAFANA_ADMIN_PASSWORD =~ ^[A-Za-z0-9_-]{24,}$ ]] || die 'Grafana credentials must be single-line URL-safe values; password at least 24 characters.'
	fi
	if [[ ! ${MAX_STORE:-5000000000} =~ ^[1-9][0-9]*$ ]]; then
		die 'MAX_STORE must be a positive decimal byte count.'
	fi
}

require_environment

deploy_user=${DEPLOY_USER:-root}
deploy_target="${deploy_user}@${DEPLOY_HOST}"
deploy_profile=${DEPLOY_PROFILE:-core}
max_store=${MAX_STORE:-5000000000}
remote_dir=/opt/hooklook
stamp=$(date -u +%Y%m%dT%H%M%SZ)

ssh_options=(-o BatchMode=yes -o ConnectTimeout=15)
if [[ -n ${DEPLOY_SSH_KEY:-} ]]; then
	ssh_options+=(-i "$DEPLOY_SSH_KEY")
fi

ssh_run() {
	ssh "${ssh_options[@]}" "$deploy_target" "$@"
}

validate_dashboard() {
	python3 - "$project_dir/observability/grafana/dashboards/hooklook.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as source:
    dashboard = json.load(source)
if dashboard.get("uid") != "hooklook-operator" or not isinstance(dashboard.get("panels"), list) or "apiVersion" in dashboard:
    raise SystemExit("hooklook.json must be a classic Grafana dashboard with UID hooklook-operator")
PY
}

# Dashboard-only deployment is deliberately narrow: validate locally, sync one
# JSON file, and let Grafana's file provider reload it. No Compose up or app
# image build is needed.
if [[ $deploy_profile == dashboard ]]; then
	validate_dashboard
	ssh_run "test -f '$remote_dir/.env.observability' && cd '$remote_dir' && docker compose --env-file .env.observability --profile observability ps --status running -q grafana | grep -q ." || die 'Hooklook Grafana must already be running for dashboard-only deployment.'
	ssh_run "install -d -m 0755 '$remote_dir/observability/grafana/dashboards'"
	rsync -a --chmod=Du=rwx,Dgo=rx,Fu=rw,Fgo=r \
		-e "ssh ${ssh_options[*]}" \
		"$project_dir/observability/grafana/dashboards/hooklook.json" \
		"$deploy_target:$remote_dir/observability/grafana/dashboards/"
	ssh_run "touch '$remote_dir/observability/grafana/dashboards/hooklook.json' && cd '$remote_dir' && ./scripts/telemetry-smoke-test.sh dashboard"
	printf '%s\n' 'Hooklook dashboard updated and verified.'
	exit 0
fi

# This mode updates telemetry configuration and services. Compose also checks
# the app's network attachment: a network rename can recreate the container
# once, using its existing image and data volume, without rebuilding it.
if [[ $deploy_profile == observability ]]; then
	validate_dashboard
	ssh_run "test -f '$remote_dir/.env.observability' && cd '$remote_dir' && docker compose --env-file .env.observability ps --status running -q hooklook | grep -q ." || die 'Hooklook and its observability environment must already be deployed.'
	ssh_run "install -d -m 0755 '$remote_dir/observability' '$remote_dir/scripts'"
	rsync -a --delete --chmod=Du=rwx,Dgo=rx,Fu=rw,Fgo=r \
		-e "ssh ${ssh_options[*]}" \
		"$project_dir/observability/" "$deploy_target:$remote_dir/observability/"
	rsync -a -e "ssh ${ssh_options[*]}" \
		"$project_dir/compose.yaml" "$deploy_target:$remote_dir/compose.yaml"
	rsync -a -e "ssh ${ssh_options[*]}" \
		"$project_dir/scripts/telemetry-smoke-test.sh" \
		"$deploy_target:$remote_dir/scripts/telemetry-smoke-test.sh"
	ssh_run "touch '$remote_dir/observability/grafana/dashboards/hooklook.json' && cd '$remote_dir' && docker compose --env-file .env.observability --profile observability config --quiet && docker compose --env-file .env.observability up -d --no-deps --no-build hooklook && docker compose --env-file .env.observability --profile observability up -d --no-deps prometheus alloy loki grafana && ./scripts/telemetry-smoke-test.sh"
	printf '%s\n' 'Hooklook observability services updated and verified.'
	exit 0
fi

printf 'Checking %s before deployment...\n' "$deploy_target"
ssh_run 'command -v docker >/dev/null && command -v curl >/dev/null && docker compose version >/dev/null && docker network inspect hooklook-edge >/dev/null'

# Run the same storage check that operators can invoke directly, before any
# remote source or container state changes.
ssh "${ssh_options[@]}" "$deploy_target" "MAX_STORE=$max_store bash -s" <scripts/check-storage-headroom.sh

printf 'Syncing source to %s...\n' "$remote_dir"
rsync -a --delete \
	--exclude '.git/' \
	--exclude '.agents/' \
	--exclude '.env' \
	--exclude '.env.*' \
	--exclude '*.db' \
	--exclude '*.db-*' \
	--exclude 'hooklook' \
	--exclude 'frontend/node_modules/' \
	--exclude 'frontend/dist/' \
	--exclude 'coverage/' \
	-e "ssh ${ssh_options[*]}" \
	"$project_dir/" "$deploy_target:$remote_dir/"

# The values have already been restricted to single-line forms. Stream them to
# SSH rather than adding them to a remote command or local process arguments.
{
	printf 'PUBLIC_BASE_URL=%s\n' "$PUBLIC_BASE_URL"
	printf 'ADMIN_TOKEN=%s\n' "$ADMIN_TOKEN"
	printf 'MAX_STORE=%s\n' "$max_store"
} | ssh "${ssh_options[@]}" "$deploy_target" "umask 077 && mkdir -p '$remote_dir' && cat >'$remote_dir/.env.tmp' && chmod 600 '$remote_dir/.env.tmp' && mv '$remote_dir/.env.tmp' '$remote_dir/.env'"

if [[ ${DEPLOY_PROFILE:-core} == full ]]; then
	{
		printf 'PUBLIC_BASE_URL=%s\n' "$PUBLIC_BASE_URL"
		printf 'ADMIN_TOKEN=%s\n' "$ADMIN_TOKEN"
		printf 'MAX_STORE=%s\n' "$max_store"
		printf 'GRAFANA_ADMIN_USER=%s\n' "$GRAFANA_ADMIN_USER"
		printf 'GRAFANA_ADMIN_PASSWORD=%s\n' "$GRAFANA_ADMIN_PASSWORD"
	} | ssh "${ssh_options[@]}" "$deploy_target" "umask 077 && cat >'$remote_dir/.env.observability.tmp' && chmod 600 '$remote_dir/.env.observability.tmp' && mv '$remote_dir/.env.observability.tmp' '$remote_dir/.env.observability'"
fi

printf 'Building candidate image on %s...\n' "$deploy_target"
ssh_run "
	set -eu
	cd '$remote_dir'
	if current_id=\$(docker compose ps -q hooklook) && [ -n \"\$current_id\" ]; then
		previous_image=\$(docker inspect --format '{{.Image}}' \"\$current_id\")
		docker image tag \"\$previous_image\" 'hooklook:rollback-$stamp'
		printf '%s\\n' 'hooklook:rollback-$stamp' > .previous-image
	else
		rm -f .previous-image
	fi
	docker compose build hooklook
"

printf 'Starting Hooklook candidate...\n'
if ! ssh_run "
	set -eu
	cd '$remote_dir'
	docker compose up -d --no-deps --force-recreate hooklook
	for attempt in \$(seq 1 30); do
		if curl --fail --silent --show-error http://127.0.0.1:8081/health >/dev/null; then
			break
		fi
		if [ \"\$attempt\" -eq 30 ]; then
			exit 1
		fi
		sleep 2
	done
	docker compose ps --status running -q hooklook | grep -q .
	curl --fail --silent --show-error '$PUBLIC_BASE_URL/health' >/dev/null
"; then
	printf '%s\n' 'Hooklook candidate failed; collecting diagnostics and restoring the prior image if available.' >&2
	ssh_run "
		cd '$remote_dir'
		docker compose ps || true
		docker compose logs --tail=100 hooklook || true
		if [ -f .previous-image ]; then
			previous_image=\$(cat .previous-image)
			docker image tag \"\$previous_image\" hooklook:latest
			docker compose up -d --no-deps --force-recreate hooklook || true
		fi
	" || true
	die 'Hooklook deployment failed; the prior image was restored when available.'
fi

printf 'Hooklook deployed successfully to %s.\n' "$PUBLIC_BASE_URL"

if [[ ${DEPLOY_PROFILE:-core} == full ]]; then
	printf 'Starting Hooklook telemetry services...\n'
	ssh_run "
		set -eu
		cd '$remote_dir'
		docker compose --env-file .env.observability --profile observability up -d --no-deps prometheus alloy loki grafana
		./scripts/telemetry-smoke-test.sh
	"
fi
