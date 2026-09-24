#!/usr/bin/env bash

# Exercises the public Hooklook contract through Caddy. This script creates a
# temporary bin and captures only test data. It never restarts a service or
# alters Caddy. Set VERIFY_RATE_LIMITS=1 to deliberately trigger 429 responses
# for the current public source IP after all ordinary checks have finished.

set -uo pipefail

die() {
	printf '%s\n' "$*" >&2
	exit 1
}

base_url=${BASE_URL:-https://hooklook.app}
admin_token=${ADMIN_TOKEN:-}
verify_rate_limits=${VERIFY_RATE_LIMITS:-0}
sse_idle_seconds=${SSE_IDLE_SECONDS:-65}

[[ $base_url =~ ^https://[^[:space:]/]+$ ]] || die 'BASE_URL must be an HTTPS origin without a path.'
[[ -n $admin_token ]] || die 'ADMIN_TOKEN is required.'
[[ $sse_idle_seconds =~ ^[1-9][0-9]*$ ]] || die 'SSE_IDLE_SECONDS must be a positive whole number.'
command -v curl >/dev/null || die 'curl is required.'
command -v dd >/dev/null || die 'dd is required.'
command -v tr >/dev/null || die 'tr is required.'

work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT
cookie_jar="$work_dir/owner.cookies"
failures=0

pass() { printf '  ok   %s\n' "$1"; }
fail() { printf '  FAIL %s\n' "$1" >&2; failures=$((failures + 1)); }
check() {
	if [[ $2 == "$3" ]]; then
		pass "$1"
	else
		fail "$1 (got $2, want $3)"
	fi
}
status() {
	curl --silent --show-error --output /dev/null --write-out '%{http_code}' "$@"
}
header_value() {
	awk -F ': ' -v expected="$1" 'tolower($1) == tolower(expected) { sub("\\r$", "", $2); print $2; exit }' "$2"
}
count_request_ids() {
	grep -o '"id":"[0-9][0-9]*"' "$1" | wc -l | tr -d ' '
}
capture() {
	local output id
	output=$(curl --silent --show-error --request POST "$base_url/b/$bin_code$1" "${@:2}") || return 1
	id=$(sed -n 's/.*"id":"\([0-9][0-9]*\)".*/\1/p' <<<"$output")
	[[ -n $id ]] || return 1
	printf '%s\n' "$id"
}

echo '== public baseline'
check 'public health' "$(status "$base_url/health")" '200'
check 'invalid admin bearer is refused' \
	"$(status --header 'Authorization: Bearer invalid' "$base_url/admin/storage")" '401'
check 'valid admin bearer works' \
	"$(status --header "Authorization: Bearer $admin_token" "$base_url/admin/storage")" '200'

home_headers="$work_dir/home.headers"
home_status=$(curl --silent --show-error --cookie-jar "$cookie_jar" --dump-header "$home_headers" \
	--output /dev/null --write-out '%{http_code}' "$base_url/")
check 'home redirects to a new bin' "$home_status" '303'
location=$(header_value Location "$home_headers")
case "$location" in
"/bins/"*) bin_code=${location#/bins/} ;;
"$base_url/bins/"*) bin_code=${location#"$base_url/bins/"} ;;
*) die 'home did not return a Hooklook bin location.' ;;
esac
[[ $bin_code =~ ^[A-Za-z0-9-]+$ ]] || die 'home did not return a valid Hooklook bin code.'
grep -q 'hooklook_owner' "$cookie_jar" && pass 'home sets an owner cookie' || fail 'home sets an owner cookie'
check 'repeat home visit keeps the same bin' \
	"$(curl --silent --show-error --cookie "$cookie_jar" --output /dev/null --write-out '%{redirect_url}' "$base_url/")" \
	"$base_url/bins/$bin_code"

page="$work_dir/page.html"
curl --silent --show-error --cookie "$cookie_jar" "$base_url/bins/$bin_code" >"$page"
grep -q '<div id="app">' "$page" && pass 'authorized bin page serves the application' || fail 'authorized bin page serves the application'
asset=$(grep -o '/assets/[^"?]*\.js' "$page" | head -n 1)
[[ -n $asset ]] || die 'bin page did not reference a JavaScript asset.'
check 'hashed frontend asset resolves' "$(status "$base_url$asset")" '200'
asset_headers="$work_dir/asset.headers"
curl --silent --show-error --dump-header "$asset_headers" --output /dev/null "$base_url$asset"
grep -qi 'cache-control:.*immutable' "$asset_headers" && pass 'hashed asset is immutable' || fail 'hashed asset is immutable'

echo '== capture, inspection, ownership, and sharing'
request_id=$(capture '/verify?source=public' --header 'Content-Type: application/json' --data '{"verification":true}') || die 'small public capture failed.'
requests="$work_dir/requests.json"
curl --silent --show-error --cookie "$cookie_jar" "$base_url/api/bins/$bin_code/requests" >"$requests"
grep -q "\"id\":\"$request_id\"" "$requests" && pass 'owner lists captured request' || fail 'owner lists captured request'
grep -q 'rawBody' "$requests" && fail 'request list is body-free' || pass 'request list is body-free'
detail="$work_dir/detail.json"
curl --silent --show-error --cookie "$cookie_jar" "$base_url/api/bins/$bin_code/requests/$request_id" >"$detail"
grep -q '"path":"/verify"' "$detail" && pass 'owner reads request detail' || fail 'owner reads request detail'

invite=$(sed -n 's/.*"inviteId":"\([^"]*\)".*/\1/p' < <(curl --silent --show-error --cookie "$cookie_jar" "$base_url/api/bins/$bin_code"))
[[ -n $invite ]] || die 'owner metadata did not contain an invitation identifier.'
check 'guest is refused while sharing is disabled' \
	"$(status "$base_url/api/bins/$bin_code?invite=$invite")" '404'
check 'owner enables sharing' \
	"$(status --request PUT --cookie "$cookie_jar" --header "Origin: $base_url" \
		--header 'Content-Type: application/json' --data '{"enabled":true}' \
		"$base_url/api/bins/$bin_code/sharing")" '204'
check 'guest reads metadata after sharing is enabled' \
	"$(status "$base_url/api/bins/$bin_code?invite=$invite")" '200'
check 'guest mutation is refused' \
	"$(status --request DELETE --header "Origin: $base_url" \
		"$base_url/api/bins/$bin_code/requests/$request_id?invite=$invite")" '403'
check 'owner disables sharing' \
	"$(status --request PUT --cookie "$cookie_jar" --header "Origin: $base_url" \
		--header 'Content-Type: application/json' --data '{"enabled":false}' \
		"$base_url/api/bins/$bin_code/sharing")" '204'
check 'revoked guest is refused' \
	"$(status "$base_url/api/bins/$bin_code?invite=$invite")" '404'

echo '== SSE'
sse_output="$work_dir/events.txt"
curl --silent --show-error --no-buffer --cookie "$cookie_jar" --max-time "$((sse_idle_seconds + 15))" \
	"$base_url/api/bins/$bin_code/events" >"$sse_output" 2>/dev/null &
sse_pid=$!
sleep "$sse_idle_seconds"
sse_request_id=$(capture '/sse-verify' --header 'Content-Type: application/json' --data '{"event":"after-idle"}') || die 'SSE verification capture failed.'
wait "$sse_pid" || true
grep -q '^: connected' "$sse_output" && pass 'SSE opens immediately' || fail 'SSE opens immediately'
grep -q '^event: request' "$sse_output" && pass 'SSE receives a persisted capture after idle' || fail 'SSE receives a persisted capture after idle'
grep -q "\"id\":\"$sse_request_id\"" "$sse_output" && pass 'SSE event identifies the persisted capture' || fail 'SSE event identifies the persisted capture'

echo '== Caddy body and header limits'
body_10mb="$work_dir/body-10mb"
body_10mb_plus_one="$work_dir/body-10mb-plus-one"
dd if=/dev/zero bs=1000000 count=10 2>/dev/null | tr '\000' x >"$body_10mb"
{ cat "$body_10mb"; printf x; } >"$body_10mb_plus_one"
curl --silent --show-error --cookie "$cookie_jar" "$base_url/api/bins/$bin_code/requests" >"$requests"
before_rejected_count=$(count_request_ids "$requests")
check 'exactly 10 MB (10,000,000 bytes) fixed-length capture is accepted' \
	"$(status --request POST --data-binary "@$body_10mb" "$base_url/b/$bin_code/body-fixed")" '201'
check 'first byte beyond 10 MB fixed-length is rejected' \
	"$(status --request POST --data-binary "@$body_10mb_plus_one" "$base_url/b/$bin_code/body-too-large")" '413'
check 'first byte beyond 10 MB chunked is rejected' \
	"$(cat "$body_10mb_plus_one" | curl --silent --show-error --http1.1 --request POST \
		--header 'Transfer-Encoding: chunked' --data-binary @- --output /dev/null --write-out '%{http_code}' \
		"$base_url/b/$bin_code/body-too-large-chunked")" '413'
curl --silent --show-error --cookie "$cookie_jar" "$base_url/api/bins/$bin_code/requests" >"$requests"
after_rejected_count=$(count_request_ids "$requests")
check 'rejected bodies are not persisted' "$after_rejected_count" "$((before_rejected_count + 1))"

header_under=$(head -c 30720 /dev/zero | tr '\000' x)
header_over=$(head -c 40960 /dev/zero | tr '\000' x)
check 'header block below the effective ~36 KiB limit is accepted' \
	"$(status --http1.1 --header "X-Hooklook-Verify: $header_under" "$base_url/health")" '200'
check 'header block above the effective ~36 KiB limit is rejected' \
	"$(status --http1.1 --header "X-Hooklook-Verify: $header_over" "$base_url/health")" '431'

if [[ $verify_rate_limits == 1 ]]; then
	echo '== intentional rate-limit checks'
	printf '  This deliberately triggers 429 for the current public source IP.\n'
	create_limit_status=''
	# The ordinary checks can run for more than one minute, so do not rely on
	# their earlier GET / being in this sliding window. Make eleven requests to
	# exceed Caddy's explicit ten-per-minute creation allowance on their own.
	for _ in $(seq 1 11); do
		create_headers="$work_dir/create-rate.headers"
		create_limit_status=$(curl --silent --show-error --cookie "$cookie_jar" --dump-header "$create_headers" \
			--output /dev/null --write-out '%{http_code}' "$base_url/")
		[[ $create_limit_status == 429 ]] && break
	done
	check 'bin creation limit returns 429' "$create_limit_status" '429'
	[[ -n $(header_value Retry-After "$create_headers") ]] && pass 'creation limit supplies Retry-After' || fail 'creation limit supplies Retry-After'
	check 'ordinary inspector API is not swept into creation limit' \
		"$(status --cookie "$cookie_jar" "$base_url/api/bins/$bin_code")" '200'

	printf '  Waiting 21 seconds before the capture-burst check.\n'
	sleep 21
	capture_limit_status=''
	for _ in $(seq 1 21); do
		capture_headers="$work_dir/capture-rate.headers"
		capture_limit_status=$(curl --silent --show-error --request POST --data '{"rate":true}' \
			--dump-header "$capture_headers" --output /dev/null --write-out '%{http_code}' \
			"$base_url/b/$bin_code/rate-limit")
		[[ $capture_limit_status == 429 ]] && break
	done
	check 'capture burst limit returns 429' "$capture_limit_status" '429'
	[[ -n $(header_value Retry-After "$capture_headers") ]] && pass 'capture limit supplies Retry-After' || fail 'capture limit supplies Retry-After'
	check 'ordinary inspector API is not swept into capture limit' \
		"$(status --cookie "$cookie_jar" "$base_url/api/bins/$bin_code")" '200'
else
	printf '%s\n' 'Rate-limit checks skipped. Re-run with VERIFY_RATE_LIMITS=1 to exercise the intentional 429 cases.'
fi

echo
if [[ $failures -eq 0 ]]; then
	echo 'ALL PUBLIC CHECKS PASSED'
else
	printf '%d PUBLIC CHECK(S) FAILED\n' "$failures"
fi
exit "$failures"
