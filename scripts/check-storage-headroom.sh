#!/usr/bin/env bash

set -euo pipefail

max_store=${MAX_STORE:-5000000000}
operational_reserve=2000000000

if ! [[ $max_store =~ ^[1-9][0-9]*$ ]]; then
	printf '%s\n' 'MAX_STORE must be a positive decimal byte count.' >&2
	exit 1
fi

docker_root=${DOCKER_DATA_ROOT:-$(docker info --format '{{.DockerRootDir}}')}
if [[ -z $docker_root ]]; then
	printf '%s\n' 'Could not determine Docker data root.' >&2
	exit 1
fi
if [[ ! -d $docker_root ]]; then
	printf 'Docker data root is not accessible from this host: %s\n' "$docker_root" >&2
	exit 1
fi

available_kib=$(df -Pk "$docker_root" | awk 'NR == 2 { print $4 }')
if ! [[ $available_kib =~ ^[0-9]+$ ]]; then
	printf 'Could not determine available space for Docker data root: %s\n' "$docker_root" >&2
	exit 1
fi

available_bytes=$((available_kib * 1024))
required_bytes=$((max_store * 2 + operational_reserve))

if ((available_bytes < required_bytes)); then
	printf 'Insufficient Docker filesystem space: have %d bytes, need at least %d bytes.\n' \
		"$available_bytes" "$required_bytes" >&2
	exit 1
fi

printf 'Docker filesystem headroom: %d bytes available; %d bytes required.\n' \
	"$available_bytes" "$required_bytes"
