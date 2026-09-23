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
	[[ -n ${PUBLIC_BASE_URL:-} ]] || die 'PUBLIC_BASE_URL is required.'
	[[ -n ${ADMIN_TOKEN:-} ]] || die 'ADMIN_TOKEN is required.'

	if [[ ! $PUBLIC_BASE_URL =~ ^https://[^[:space:]/]+$ ]]; then
		die 'PUBLIC_BASE_URL must be an HTTPS origin without a path.'
	fi
	if [[ ! $ADMIN_TOKEN =~ ^[A-Za-z0-9_-]{32,}$ ]]; then
		die 'ADMIN_TOKEN must be at least 32 URL-safe characters.'
	fi
	if [[ ! ${MAX_STORE:-5000000000} =~ ^[1-9][0-9]*$ ]]; then
		die 'MAX_STORE must be a positive decimal byte count.'
	fi
}

require_environment

deploy_user=${DEPLOY_USER:-root}
deploy_target="${deploy_user}@${DEPLOY_HOST}"
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
