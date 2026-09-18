#!/usr/bin/env bash

# Exercises the whole inspection flow against the built binary: first visit,
# capture, list, detail, the live stream, owner mutations, guest invitation,
# revocation and replacement.
#
# It starts its own server over a throwaway SQLite database in a temporary
# directory, so the repository's hooklook.db is left alone. The server binds
# the only address hooklook has, 127.0.0.1:8080, so stop `make dev-go` first.
# Build, then run:
#   make build
#   ./scripts/exercise-inspection.sh
#
# BINARY overrides which binary is exercised:
#   BINARY=/tmp/webhook-inspector ./scripts/exercise-inspection.sh
#
# Deliberately no `set -e`: every check runs and the exit status is the number
# of failures, so one broken behavior does not hide the rest.

set -uo pipefail

binary="${BINARY:-$(cd "$(dirname "$0")/.." && pwd)/webhook-inspector}"
base="http://127.0.0.1:8080"

if [[ ! -x "$binary" ]]; then
  printf 'No binary at %s. Run `make build` first, or set BINARY.\n' "$binary" >&2
  exit 1
fi
if curl --silent --output /dev/null --max-time 1 "$base/health"; then
  printf 'Something is already serving %s — stop `make dev-go` first.\n' "$base" >&2
  exit 1
fi

work="$(mktemp -d)"
cd "$work" || exit 1

PUBLIC_BASE_URL="$base" ADMIN_TOKEN=exercise-token \
  "$binary" > "$work/server.log" 2>&1 &
server=$!
cleanup() {
  kill "$server" 2>/dev/null
  wait "$server" 2>/dev/null
  rm -rf "$work"
}
trap cleanup EXIT

for _ in $(seq 1 50); do
  curl --silent --output /dev/null "$base/health" && break
  sleep 0.1
done
if ! curl --silent --output /dev/null "$base/health"; then
  printf 'The server did not start. Its log:\n' >&2
  cat "$work/server.log" >&2
  exit 1
fi

owner="$work/owner.jar"
failures=0
pass() { printf '  ok   %s\n' "$1"; }
fail() { printf '  FAIL %s\n' "$1"; failures=$((failures + 1)); }
check() {
  if [[ "$2" == "$3" ]]; then pass "$1"; else fail "$1 (got '$2', want '$3')"; fi
}
capture() { curl --silent --request "$1" "$base/b/$code$2" "${@:3}"; }
id_of() { sed 's/.*"id":"\([0-9]*\)".*/\1/'; }

echo "== first visit"
location=$(curl --silent --output /dev/null --cookie-jar "$owner" --write-out '%{redirect_url}' "$base/")
code=${location##*/bins/}
[[ -n "$code" ]] && pass "GET / redirects to /bins/$code" || fail "GET / redirect"
grep -q hooklook_owner "$owner" && pass "owner cookie set" || fail "owner cookie"
again=$(curl --silent --output /dev/null --cookie "$owner" --cookie-jar "$owner" \
  --write-out '%{redirect_url}' "$base/")
check "repeat visit resolves the same bin" "${again##*/bins/}" "$code"

echo "== page shell and assets"
page=$(curl --silent --cookie "$owner" "$base/bins/$code")
grep -q '<div id="app">' <<<"$page" && pass "bin page serves the application" || fail "bin page"
asset=$(grep -o '/assets/[^"]*\.js' <<<"$page" | head -1)
stylesheet=$(grep -o '/assets/[^"]*\.css' <<<"$page" | head -1)
status_of() { curl --silent --output /dev/null --write-out '%{http_code}' "$@"; }
check "hashed JS resolves" "$(status_of "$base$asset")" "200"
check "hashed CSS resolves" "$(status_of "$base$stylesheet")" "200"
check "font resolves" "$(status_of "$base/fonts/AzeretMono-latin.woff2")" "200"
check "immutable caching on hashed assets" \
  "$(curl --silent --dump-header - --output /dev/null "$base$asset" | grep -ci immutable)" "1"

echo "== captures of every body shape the detail view must handle"
printf 'caf\xe9 na\xefve' > "$work/latin1.bin"
printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x01' > "$work/binary.bin"
id_json=$(capture POST '/orders/42?retry=1&mode=live' \
  --header 'Content-Type: application/json' \
  --header 'Authorization: Bearer super-secret-token' \
  --data '{"b":1,"a":["x","<script>alert(1)</script>"]}' | id_of)
id_xml=$(capture PUT '/feed' --header 'Content-Type: application/xml' \
  --data '<order id="7"><item>widget</item></order>' | id_of)
id_empty=$(capture GET '/ping' | id_of)
id_utf8=$(capture POST '/text' --header 'Content-Type: text/plain' --data 'héllo ✓ ↳ world' | id_of)
id_latin=$(capture POST '/latin' --header 'Content-Type: text/plain' \
  --data-binary "@$work/latin1.bin" | id_of)
id_binary=$(capture POST '/upload' --header 'Content-Type: image/png' \
  --data-binary "@$work/binary.bin" | id_of)
id_malformed=$(capture POST '/broken' --header 'Content-Type: application/json' --data '{"a":' | id_of)
printf '  captured ids: json=%s xml=%s empty=%s utf8=%s latin1=%s binary=%s malformed=%s\n' \
  "$id_json" "$id_xml" "$id_empty" "$id_utf8" "$id_latin" "$id_binary" "$id_malformed"

list=$(curl --silent --cookie "$owner" "$base/api/bins/$code/requests")
check "list holds every capture" "$(grep -o '"id":' <<<"$list" | wc -l | tr -d ' ')" "7"
grep -q rawBody <<<"$list" && fail "list must stay body-free" || pass "list stays body-free"

echo "== detail"
detail=$(curl --silent --cookie "$owner" "$base/api/bins/$code/requests/$id_json")
# Go's JSON encoder escapes & as &; the query itself is unchanged.
grep -q '"rawQuery":"retry=1.u0026mode=live"' <<<"$detail" && pass "raw query preserved" || fail "raw query"
grep -q '"path":"/orders/42"' <<<"$detail" && pass "path preserved" || fail "path"
grep -q 'REDACTED' <<<"$detail" && pass "Authorization stored redacted" || fail "redaction"
grep -q 'super-secret-token' <<<"$detail" && fail "credential leaked into detail" || pass "no credential in detail"
raw=$(sed 's/.*"rawBody":"\([^"]*\)".*/\1/' <<<"$detail" | base64 -d)
check "body bytes round-trip exactly" "$raw" '{"b":1,"a":["x","<script>alert(1)</script>"]}'
# An absent body is an empty base64 string, which decodes to zero bytes.
check "empty body carries no bytes" \
  "$(curl --silent --cookie "$owner" "$base/api/bins/$code/requests/$id_empty" | grep -c '"rawBody":""')" "1"
check "unknown request id is 404" \
  "$(status_of --cookie "$owner" "$base/api/bins/$code/requests/999999")" "404"

echo "== direct detail URL serves the same document"
shell_at_bin=$(curl --silent --cookie "$owner" "$base/bins/$code" | shasum | cut -d' ' -f1)
shell_at_request=$(curl --silent --cookie "$owner" "$base/bins/$code/requests/$id_json" | shasum | cut -d' ' -f1)
check "page shell identical at both depths" "$shell_at_bin" "$shell_at_request"

echo "== live stream"
curl --silent --no-buffer --cookie "$owner" --max-time 3 "$base/api/bins/$code/events" > "$work/sse.txt" &
stream=$!
sleep 0.5
capture POST '/streamed' --header 'Content-Type: application/json' --data '{"live":true}' > /dev/null
sleep 0.3
curl --silent --request DELETE --cookie "$owner" --header "Origin: $base" \
  "$base/api/bins/$code/requests/$id_malformed" > /dev/null
wait "$stream"
grep -q '^event: request' "$work/sse.txt" && pass "capture arrives as a request event" || fail "request event"
grep -q '"path":"/streamed"' "$work/sse.txt" && pass "event carries the summary" || fail "event summary"
grep -q '^event: refresh' "$work/sse.txt" && pass "deletion announces a refresh" || fail "refresh event"

echo "== owner mutations"
check "deleted request is gone" \
  "$(status_of --cookie "$owner" "$base/api/bins/$code/requests/$id_malformed")" "404"
bytes_before=$(curl --silent --cookie "$owner" "$base/api/bins/$code" |
  sed 's/.*"totalBodyBytes":\([0-9]*\).*/\1/')
check "mutation without an Origin header is refused" \
  "$(status_of --request DELETE --cookie "$owner" "$base/api/bins/$code/requests/$id_xml")" "403"

echo "== sharing and the guest"
invite=$(curl --silent --cookie "$owner" "$base/api/bins/$code" | sed 's/.*"inviteId":"\([^"]*\)".*/\1/')
[[ -n "$invite" ]] && pass "owner metadata carries the invitation" || fail "invitation"
check "guest is refused while sharing is off" "$(status_of "$base/api/bins/$code?invite=$invite")" "404"
set_sharing() {
  curl --silent --output /dev/null --request PUT --cookie "$owner" --header "Origin: $base" \
    --header 'Content-Type: application/json' --data "{\"enabled\":$1}" "$base/api/bins/$code/sharing"
}
set_sharing true
guest=$(curl --silent "$base/api/bins/$code?invite=$invite")
check "guest reads metadata once sharing is on" "$(grep -c '"owner":false' <<<"$guest")" "1"
grep -q inviteId <<<"$guest" && fail "guest metadata leaks the invitation" || pass "guest metadata has no invitation"
check "guest reads the list" "$(status_of "$base/api/bins/$code/requests?invite=$invite")" "200"
check "guest reads a detail link" "$(status_of "$base/api/bins/$code/requests/$id_json?invite=$invite")" "200"
check "guest page loads at detail depth" "$(status_of "$base/bins/$code/requests/$id_json?invite=$invite")" "200"
check "guest deletion is refused" \
  "$(status_of --request DELETE --header "Origin: $base" "$base/api/bins/$code/requests/$id_json?invite=$invite")" "403"
check "guest clear is refused" \
  "$(status_of --request DELETE --header "Origin: $base" "$base/api/bins/$code/requests?invite=$invite")" "403"
check "guest replace is refused" \
  "$(status_of --request POST --header "Origin: $base" "$base/api/bins/$code/replace?invite=$invite")" "403"
check "guest sharing change is refused" \
  "$(status_of --request PUT --header "Origin: $base" --header 'Content-Type: application/json' \
    --data '{"enabled":false}' "$base/api/bins/$code/sharing?invite=$invite")" "403"
check "a bogus invitation gets its own bin instead" \
  "$(status_of "$base/bins/$code?invite=not-a-real-invitation")" "303"

echo "== revocation, then re-enabling the same link"
set_sharing false
check "revoked guest is refused" "$(status_of "$base/api/bins/$code?invite=$invite")" "404"
set_sharing true
check "the same link works again" "$(status_of "$base/api/bins/$code?invite=$invite")" "200"

echo "== clearing keeps the address, replacement does not"
curl --silent --output /dev/null --request DELETE --cookie "$owner" --header "Origin: $base" \
  "$base/api/bins/$code/requests"
check "clearing empties the list" "$(curl --silent --cookie "$owner" "$base/api/bins/$code/requests")" "[]"
after=$(curl --silent --cookie "$owner" "$base/api/bins/$code")
check "clearing reclaims every byte" "$(sed 's/.*"totalBodyBytes":\([0-9]*\).*/\1/' <<<"$after")" "0"
check "clearing keeps the invitation" "$(sed 's/.*"inviteId":"\([^"]*\)".*/\1/' <<<"$after")" "$invite"
check "clearing keeps the capture URL working" \
  "$(status_of --request POST "$base/b/$code/after-clear")" "201"
printf '  (total body bytes before clearing: %s)\n' "$bytes_before"

new_location=$(curl --silent --output /dev/null --request POST --cookie "$owner" --cookie-jar "$owner" \
  --header "Origin: $base" --write-out '%{redirect_url}' "$base/api/bins/$code/replace")
new_code=${new_location##*/bins/}
[[ -n "$new_code" && "$new_code" != "$code" ]] &&
  pass "replacement gives a new bin $new_code" || fail "replacement"
check "the old capture URL stops working" "$(status_of --request POST "$base/b/$code/gone")" "404"
check "the old invitation link stops working" "$(status_of "$base/api/bins/$code?invite=$invite")" "404"
check "the new bin is reachable with the new cookie" \
  "$(status_of --cookie "$owner" "$base/bins/$new_code")" "200"
check "the new capture URL works" "$(status_of --request POST "$base/b/$new_code/fresh")" "201"

echo "== logs carry no cookie or invitation"
grep -q "$invite" "$work/server.log" && fail "invitation appears in the log" || pass "no invitation in the log"
grep -qi 'hooklook_owner' "$work/server.log" && fail "cookie appears in the log" || pass "no owner cookie in the log"

echo
if [[ "$failures" -eq 0 ]]; then
  echo 'ALL CHECKS PASSED'
else
  printf '%d CHECK(S) FAILED\n' "$failures"
fi
exit "$failures"
