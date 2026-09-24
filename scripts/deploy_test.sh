#!/usr/bin/env bash

set -euo pipefail

project_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
temporary_dir=$(mktemp -d)
trap 'rm -rf "$temporary_dir"' EXIT

mkdir -p "$temporary_dir/project/scripts" "$temporary_dir/bin"
cp "$project_dir/scripts/deploy.sh" "$temporary_dir/project/scripts/deploy.sh"
mkdir -p "$temporary_dir/project/observability/grafana/dashboards"
cp "$project_dir/observability/grafana/dashboards/hooklook.json" "$temporary_dir/project/observability/grafana/dashboards/hooklook.json"
cp "$project_dir/scripts/check-storage-headroom.sh" "$temporary_dir/project/scripts/check-storage-headroom.sh"
cp "$project_dir/scripts/telemetry-smoke-test.sh" "$temporary_dir/project/scripts/telemetry-smoke-test.sh"
touch "$temporary_dir/project/compose.yaml"

cat >"$temporary_dir/bin/go" <<'EOF'
#!/usr/bin/env bash
printf 'go %s\n' "$*" >>"$DEPLOY_TEST_LOG"
exit 0
EOF
chmod +x "$temporary_dir/bin/go"

cat >"$temporary_dir/bin/ssh" <<'EOF'
#!/usr/bin/env bash
touch "$DEPLOY_TEST_SSH_LOG"
printf '%s\n' "$*" >>"$DEPLOY_TEST_SSH_LOG"
exit 0
EOF
chmod +x "$temporary_dir/bin/ssh"

cat >"$temporary_dir/bin/rsync" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$DEPLOY_TEST_RSYNC_LOG"
EOF
chmod +x "$temporary_dir/bin/rsync"

assert_contains() {
	local expected=$1
	local actual=$2
	[[ $actual == *"$expected"* ]] || {
		printf 'expected output to contain %q, got:\n%s\n' "$expected" "$actual" >&2
		exit 1
	}
}

run_missing_environment_test() {
	local output status
	set +e
	output=$(cd "$temporary_dir/project" && \
		PATH="$temporary_dir/bin:$PATH" DEPLOY_TEST_LOG="$temporary_dir/go.log" \
		bash scripts/deploy.sh 2>&1)
	status=$?
	set -e

	[[ $status -ne 0 ]] || {
		printf '%s\n' 'deploy script unexpectedly accepted missing environment variables.' >&2
		exit 1
	}
	assert_contains 'DEPLOY_HOST is required.' "$output"
	[[ $(cat "$temporary_dir/go.log") == 'go test -count=1 ./...' ]] || {
		printf '%s\n' 'local Go test was not the first deployment action.' >&2
		exit 1
	}
}

run_invalid_origin_test() {
	local output status
	rm -f "$temporary_dir/go.log" "$temporary_dir/ssh.log"
	set +e
	output=$(cd "$temporary_dir/project" && \
		PATH="$temporary_dir/bin:$PATH" \
		DEPLOY_TEST_LOG="$temporary_dir/go.log" \
		DEPLOY_TEST_SSH_LOG="$temporary_dir/ssh.log" \
		DEPLOY_HOST=example.invalid \
		PUBLIC_BASE_URL=http://hooklook.app \
		ADMIN_TOKEN=abcdefghijklmnopqrstuvwxyz123456 \
		bash scripts/deploy.sh 2>&1)
	status=$?
	set -e

	[[ $status -ne 0 ]] || {
		printf '%s\n' 'deploy script unexpectedly accepted an insecure public origin.' >&2
		exit 1
	}
	assert_contains 'PUBLIC_BASE_URL must be an HTTPS origin without a path.' "$output"
	[[ $(cat "$temporary_dir/go.log") == 'go test -count=1 ./...' ]] || {
		printf '%s\n' 'local Go test did not precede public-origin validation.' >&2
		exit 1
	}
	[[ ! -e $temporary_dir/ssh.log ]] || {
		printf '%s\n' 'invalid public-origin validation attempted SSH.' >&2
		exit 1
	}
}

run_dashboard_only_test() {
	local output
	rm -f "$temporary_dir/go.log" "$temporary_dir/ssh.log" "$temporary_dir/rsync.log"
	output=$(cd "$temporary_dir/project" && \
		PATH="$temporary_dir/bin:$PATH" \
		DEPLOY_TEST_LOG="$temporary_dir/go.log" \
		DEPLOY_TEST_SSH_LOG="$temporary_dir/ssh.log" \
		DEPLOY_TEST_RSYNC_LOG="$temporary_dir/rsync.log" \
		DEPLOY_HOST=example.invalid DEPLOY_PROFILE=dashboard \
		bash scripts/deploy.sh)
	assert_contains 'Hooklook dashboard updated and verified.' "$output"
	assert_contains 'hooklook.json' "$(cat "$temporary_dir/rsync.log")"
	assert_contains 'telemetry-smoke-test.sh dashboard' "$(cat "$temporary_dir/ssh.log")"
	[[ $(wc -l <"$temporary_dir/rsync.log") -eq 1 ]] || {
		printf '%s\n' 'Dashboard mode synced more than one file.' >&2
		exit 1
	}
	[[ $(cat "$temporary_dir/ssh.log") != *'docker compose build hooklook'* ]] || {
		printf '%s\n' 'Dashboard mode attempted an application build.' >&2
		exit 1
	}
}

run_invalid_dashboard_test() {
	local output status
	printf '%s\n' '{"uid":"wrong","panels":[]}' >"$temporary_dir/project/observability/grafana/dashboards/hooklook.json"
	rm -f "$temporary_dir/ssh.log"
	set +e
	output=$(cd "$temporary_dir/project" && \
		PATH="$temporary_dir/bin:$PATH" \
		DEPLOY_TEST_LOG="$temporary_dir/go.log" \
		DEPLOY_TEST_SSH_LOG="$temporary_dir/ssh.log" \
		DEPLOY_HOST=example.invalid DEPLOY_PROFILE=dashboard \
		bash scripts/deploy.sh 2>&1)
	status=$?
	set -e
	[[ $status -ne 0 ]] || {
		printf '%s\n' 'Dashboard mode accepted an invalid UID.' >&2
		exit 1
	}
	assert_contains 'classic Grafana dashboard with UID hooklook-operator' "$output"
	[[ ! -e $temporary_dir/ssh.log ]] || {
		printf '%s\n' 'Invalid dashboard attempted SSH.' >&2
		exit 1
	}
	cp "$project_dir/observability/grafana/dashboards/hooklook.json" "$temporary_dir/project/observability/grafana/dashboards/hooklook.json"
}

run_observability_only_test() {
	local output
	rm -f "$temporary_dir/go.log" "$temporary_dir/ssh.log" "$temporary_dir/rsync.log"
	output=$(cd "$temporary_dir/project" && \
		PATH="$temporary_dir/bin:$PATH" \
		DEPLOY_TEST_LOG="$temporary_dir/go.log" \
		DEPLOY_TEST_SSH_LOG="$temporary_dir/ssh.log" \
		DEPLOY_TEST_RSYNC_LOG="$temporary_dir/rsync.log" \
		DEPLOY_HOST=example.invalid DEPLOY_PROFILE=observability \
		bash scripts/deploy.sh)
	assert_contains 'Hooklook observability services updated and verified.' "$output"
	assert_contains 'observability/' "$(cat "$temporary_dir/rsync.log")"
	assert_contains 'compose.yaml' "$(cat "$temporary_dir/rsync.log")"
	assert_contains 'telemetry-smoke-test.sh' "$(cat "$temporary_dir/ssh.log")"
	assert_contains 'up -d --no-deps --no-build hooklook' "$(cat "$temporary_dir/ssh.log")"
	[[ $(cat "$temporary_dir/ssh.log") != *'docker compose build hooklook'* ]] || {
		printf '%s\n' 'Observability mode attempted an application build.' >&2
		exit 1
	}
}

run_full_deploy_test() {
	local output
	rm -f "$temporary_dir/go.log" "$temporary_dir/ssh.log" "$temporary_dir/rsync.log"
	output=$(cd "$temporary_dir/project" && \
		PATH="$temporary_dir/bin:$PATH" \
		DEPLOY_TEST_LOG="$temporary_dir/go.log" \
		DEPLOY_TEST_SSH_LOG="$temporary_dir/ssh.log" \
		DEPLOY_TEST_RSYNC_LOG="$temporary_dir/rsync.log" \
		DEPLOY_HOST=example.invalid DEPLOY_PROFILE=full \
		PUBLIC_BASE_URL=https://hooklook.example \
		ADMIN_TOKEN=abcdefghijklmnopqrstuvwxyz123456 \
		GRAFANA_ADMIN_USER=operator \
		GRAFANA_ADMIN_PASSWORD=abcdefghijklmnopqrstuvwxyz123456 \
		bash scripts/deploy.sh)
	assert_contains 'Hooklook deployed successfully' "$output"
	assert_contains 'docker compose build hooklook' "$(cat "$temporary_dir/ssh.log")"
	assert_contains './scripts/telemetry-smoke-test.sh' "$(cat "$temporary_dir/ssh.log")"
}

run_missing_environment_test
run_invalid_origin_test
run_dashboard_only_test
run_invalid_dashboard_test
run_observability_only_test
run_full_deploy_test
printf '%s\n' 'deploy script tests passed'
