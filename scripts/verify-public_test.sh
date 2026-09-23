#!/usr/bin/env bash

set -euo pipefail

project_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
temporary_dir=$(mktemp -d)
trap 'rm -rf "$temporary_dir"' EXIT

mkdir -p "$temporary_dir/project/scripts" "$temporary_dir/bin"
cp "$project_dir/scripts/verify-public.sh" "$temporary_dir/project/scripts/verify-public.sh"

cat >"$temporary_dir/bin/curl" <<'EOF'
#!/usr/bin/env bash
touch "$VERIFY_TEST_CURL_LOG"
EOF
chmod +x "$temporary_dir/bin/curl"

assert_contains() {
	[[ $2 == *"$1"* ]] || {
		printf 'expected output to contain %q, got:\n%s\n' "$1" "$2" >&2
		exit 1
	}
}

run_missing_token_test() {
	local output status
	set +e
	output=$(cd "$temporary_dir/project" && \
		PATH="$temporary_dir/bin:$PATH" VERIFY_TEST_CURL_LOG="$temporary_dir/curl.log" \
		bash scripts/verify-public.sh 2>&1)
	status=$?
	set -e
	[[ $status -ne 0 ]] || {
		printf '%s\n' 'public verifier accepted a missing admin token.' >&2
		exit 1
	}
	assert_contains 'ADMIN_TOKEN is required.' "$output"
	[[ ! -e $temporary_dir/curl.log ]] || {
		printf '%s\n' 'public verifier used curl before token validation.' >&2
		exit 1
	}
}

run_invalid_origin_test() {
	local output status
	set +e
	output=$(cd "$temporary_dir/project" && \
		PATH="$temporary_dir/bin:$PATH" VERIFY_TEST_CURL_LOG="$temporary_dir/curl.log" \
		ADMIN_TOKEN=operator BASE_URL=http://hooklook.app bash scripts/verify-public.sh 2>&1)
	status=$?
	set -e
	[[ $status -ne 0 ]] || {
		printf '%s\n' 'public verifier accepted an insecure base URL.' >&2
		exit 1
	}
	assert_contains 'BASE_URL must be an HTTPS origin without a path.' "$output"
	[[ ! -e $temporary_dir/curl.log ]] || {
		printf '%s\n' 'public verifier used curl before origin validation.' >&2
		exit 1
	}
}

run_missing_token_test
run_invalid_origin_test
printf '%s\n' 'public verifier tests passed'
