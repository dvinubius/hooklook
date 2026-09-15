#!/usr/bin/env bash

# Exercises the request shapes captured in Milestone 2.
#
# Start hooklook and create a bin separately, then run:
#   ./scripts/exercise-capture.sh YOUR_BIN_CODE

set -euo pipefail

base_url=http://localhost:8080
bin_code="${1:-}"

if [[ -z "$bin_code" ]]; then
  printf 'Usage: %s BIN_CODE\n' "$0" >&2
  exit 1
fi

bin_url="$base_url/b/$bin_code"

curl --fail-with-body --silent --show-error --output /dev/null \
  --request POST "$bin_url/json?source=smoke&tag=one&tag=two" \
  --header 'Content-Type: application/json' \
  --data '{"event":"push","repository":"hooklook"}'

curl --fail-with-body --silent --show-error --output /dev/null \
  --request PUT "$bin_url/text" \
  --header 'Content-Type: text/plain; charset=utf-8' \
  --data 'Hello from a plain-text webhook.'

curl --fail-with-body --silent --show-error --output /dev/null \
  --request PATCH "$bin_url/xml" \
  --header 'Content-Type: application/xml' \
  --data '<event><name>build.finished</name></event>'

curl --fail-with-body --silent --show-error --output /dev/null \
  --request POST "$bin_url/form" \
  --header 'X-Tag: first' \
  --header 'X-Tag: second' \
  --header 'Authorization: Bearer should-not-be-stored' \
  --header 'Cookie: session=should-not-be-stored' \
  --header 'X-Hub-Signature-256: sha256=retained-for-debugging' \
  --data-urlencode 'project=hooklook' \
  --data-urlencode 'status=passing'

curl --fail-with-body --silent --show-error --output /dev/null \
  --request DELETE "$bin_url/empty"

printf '\\x00\\x01\\x02\\xffhooklook\\x00' | curl --fail-with-body --silent --show-error --output /dev/null \
  --request POST "$bin_url/binary" \
  --header 'Content-Type: application/octet-stream' \
  --data-binary @-

echo 'Capture requests sent.'
