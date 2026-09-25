#!/usr/bin/env bash

set -euo pipefail

die() {
	printf '%s\n' "$*" >&2
	exit 1
}

project_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
backup_dir=${BACKUP_DIR:-}

[[ -n $backup_dir ]] || die 'Set BACKUP_DIR to an existing directory outside the Hooklook data volume.'
[[ -d $backup_dir ]] || die "Backup directory does not exist: $backup_dir"
backup_dir=$(cd -- "$backup_dir" && pwd -P)

cd "$project_dir"
container_id=$(scripts/compose.sh ps -q hooklook)
[[ -n $container_id ]] || die 'Hooklook is not running; cannot locate its live data volume.'
data_dir=$(docker inspect --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Source}}{{end}}{{end}}' "$container_id")
[[ -n $data_dir && -d $data_dir ]] || die 'Could not locate Hooklook data volume on this host.'
data_dir=$(cd -- "$data_dir" && pwd -P)

case "$backup_dir/" in
"$data_dir/"*) die 'BACKUP_DIR must be outside the Hooklook data volume.' ;;
esac

backup_name="hooklook-$(date -u +%Y%m%dT%H%M%SZ).db"
backup_path="$backup_dir/$backup_name"
checksum_path="$backup_path.sha256"

[[ ! -e $backup_path ]] || die "Backup destination already exists: $backup_path"
[[ ! -e $checksum_path ]] || die "Backup checksum destination already exists: $checksum_path"

# The temporary root user is limited to the one-off container so it can write
# to an operator-selected host directory. The long-running service stays
# non-root. The application's backup command uses SQLite VACUUM INTO.
scripts/compose.sh run --rm --no-deps --user 0 \
	--volume "$backup_dir:/backups" \
	hooklook backup "/backups/$backup_name"

(
	cd "$backup_dir"
	sha256sum "$backup_name" >"$backup_name.sha256"
)
chmod 600 "$backup_path" "$checksum_path"
printf 'Backup complete: %s\n' "$backup_path"
