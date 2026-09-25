#!/usr/bin/env bash

# Run Docker Compose for the deployed Hooklook project with its VPS-owned
# environment: the runtime secrets (.env.observability when the telemetry stack
# is deployed, otherwise .env) plus .env.image, which pins HOOKLOOK_IMAGE to a
# published image digest. Every operator and script Compose call goes through
# here so a reboot, backup, or manual `up` uses the same image.
#
# Usage: scripts/compose.sh <compose arguments...>

set -euo pipefail

project_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$project_dir"

[[ -f .env.image ]] || {
	printf '%s\n' ".env.image is missing in $project_dir; deploy a published image first." >&2
	exit 1
}

if [[ -f .env.observability ]]; then
	exec docker compose --env-file .env.observability --env-file .env.image --profile observability "$@"
fi
[[ -f .env ]] || {
	printf '%s\n' ".env is missing in $project_dir." >&2
	exit 1
}
exec docker compose --env-file .env --env-file .env.image "$@"
