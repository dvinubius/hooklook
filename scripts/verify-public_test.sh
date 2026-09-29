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

# A fake curl that plays Hooklook's home flow: `/` sets the cookie check and
# redirects to `/?cookie-check`, which creates a bin only when the check comes
# back. Every other path is 404, so the verifier stops after the home checks.
write_home_flow_curl() {
	cat >"$temporary_dir/home-bin/curl" <<'EOF'
#!/usr/bin/env bash
origin=https://hooklook.test
read_jar='' write_jar='' headers='' write_out='' url='' invalid_bearer=0
while (($#)); do
	case $1 in
	--cookie) read_jar=$2; shift 2 ;;
	--cookie-jar) write_jar=$2; shift 2 ;;
	--dump-header) headers=$2; shift 2 ;;
	--write-out) write_out=$2; shift 2 ;;
	--header) [[ $2 == *'Bearer invalid'* ]] && invalid_bearer=1; shift 2 ;;
	--output | --request | --data | --data-binary | --max-time) shift 2 ;;
	"$origin"*) url=$1; shift ;;
	*) shift ;;
	esac
done
sent() { [[ -f $read_jar ]] && grep -q "$1" "$read_jar"; }
code=404 location='' error='' set_cookie=''
case ${url#"$origin"} in
/health) code=200 ;;
/admin/storage) code=200; ((invalid_bearer)) && code=401 ;;
/)
	code=303
	if sent hooklook_owner; then location=/bins/test-bin; else location=/?cookie-check set_cookie=hooklook_cookie_check; fi
	;;
/?cookie-check)
	if sent hooklook_cookie_check; then code=303 location=/bins/test-bin set_cookie=hooklook_owner; else code=200 error=cookies_required; fi
	;;
esac
if [[ -n $write_jar ]]; then
	jar=''
	[[ -f $read_jar ]] && jar=$(grep -v hooklook_cookie_check "$read_jar")
	[[ -n $set_cookie ]] && jar+=$'\n'"hooklook.test"$'\tFALSE\t/\tTRUE\t0\t'"$set_cookie"$'\tsecret'
	printf '%s\n' "$jar" >"$write_jar"
fi
if [[ -n $headers ]]; then
	printf 'HTTP/2 %s\r\n' "$code" >"$headers"
	[[ -n $location ]] && printf 'location: %s\r\n' "$location" >>"$headers"
	[[ -n $error ]] && printf 'x-hooklook-error: %s\r\n' "$error" >>"$headers"
fi
redirect_url=''
[[ -n $location ]] && redirect_url=$origin$location
out=${write_out//'%{http_code}'/$code}
printf '%s' "${out//'%{redirect_url}'/$redirect_url}"
EOF
	chmod +x "$temporary_dir/home-bin/curl"
}

run_home_flow_test() {
	local output
	mkdir -p "$temporary_dir/home-bin"
	write_home_flow_curl
	output=$(cd "$temporary_dir/project" && \
		PATH="$temporary_dir/home-bin:$PATH" ADMIN_TOKEN=operator BASE_URL=https://hooklook.test \
		bash scripts/verify-public.sh 2>&1) || true
	for check in \
		'ok   client without cookies gets no bin' \
		'ok   refusal is marked cookies_required' \
		'ok   home checks cookies first' \
		'ok   cookie check redirects to its marked address' \
		'ok   returned cookie check redirects to a new bin' \
		'ok   home sets an owner cookie' \
		'ok   repeat home visit keeps the same bin'; do
		assert_contains "$check" "$output"
	done
	# The fake answers the bin page with 404, so only the home section counts.
	[[ ${output%%repeat home visit*} != *FAIL* ]] || {
		printf 'home flow check failed:\n%s\n' "$output" >&2
		exit 1
	}
}

run_missing_token_test
run_invalid_origin_test
run_home_flow_test
printf '%s\n' 'public verifier tests passed'
