#!/usr/bin/env bash

set -euo pipefail

project_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
temporary_dir=$(mktemp -d)
trap 'rm -rf "$temporary_dir"' EXIT

mkdir -p "$temporary_dir/project/scripts" "$temporary_dir/bin"
cp "$project_dir/scripts/deploy.sh" "$temporary_dir/project/scripts/deploy.sh"

cat >"$temporary_dir/bin/go" <<'EOF'
#!/usr/bin/env bash
printf 'go %s\n' "$*" >>"$DEPLOY_TEST_LOG"
exit 0
EOF
chmod +x "$temporary_dir/bin/go"

cat >"$temporary_dir/bin/ssh" <<'EOF'
#!/usr/bin/env bash
touch "$DEPLOY_TEST_SSH_LOG"
exit 0
EOF
chmod +x "$temporary_dir/bin/ssh"

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

run_missing_environment_test
run_invalid_origin_test
printf '%s\n' 'deploy script tests passed'
