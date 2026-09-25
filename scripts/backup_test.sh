#!/usr/bin/env bash

set -euo pipefail

project_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
temporary_dir=$(mktemp -d)
trap 'rm -rf "$temporary_dir"' EXIT

mkdir -p "$temporary_dir/project/scripts" "$temporary_dir/bin" \
	"$temporary_dir/data-volume" "$temporary_dir/backups"
cp "$project_dir/scripts/backup.sh" "$project_dir/scripts/compose.sh" "$temporary_dir/project/scripts/"
printf '%s\n' 'ADMIN_TOKEN=test' >"$temporary_dir/project/.env"
printf '%s\n' 'HOOKLOOK_IMAGE=ghcr.io/example/hooklook@sha256:0000' >"$temporary_dir/project/.env.image"

cat >"$temporary_dir/bin/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

printf '%s\n' "$*" >>"$BACKUP_TEST_LOG"
if [[ $1 == compose && " $* " == *" ps "* ]]; then
	printf '%s\n' test-container
	elif [[ $1 == inspect ]]; then
	printf '%s\n' "$BACKUP_TEST_DATA_DIR"
	elif [[ $1 == compose && " $* " == *" run "* ]]; then
	for argument in "$@"; do
		if [[ $argument == /backups/*.db ]]; then
			touch "${BACKUP_TEST_BACKUP_DIR}/${argument#/backups/}"
			exit 0
		fi
	done
	printf '%s\n' 'backup destination was not passed to the one-off container' >&2
	exit 1
fi
EOF
chmod +x "$temporary_dir/bin/docker"

cat >"$temporary_dir/bin/date" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' 20260923T190000Z
EOF
chmod +x "$temporary_dir/bin/date"

assert_contains() {
	local expected=$1 actual=$2
	[[ $actual == *"$expected"* ]] || {
		printf 'expected output to contain %q, got:\n%s\n' "$expected" "$actual" >&2
		exit 1
	}
}

run_missing_directory_test() {
	local output status
	set +e
	output=$(cd "$temporary_dir/project" && \
		PATH="$temporary_dir/bin:$PATH" BACKUP_TEST_LOG="$temporary_dir/docker.log" \
		BACKUP_DIR="$temporary_dir/missing" bash scripts/backup.sh 2>&1)
	status=$?
	set -e
	[[ $status -ne 0 ]] || die 'backup script accepted a missing backup directory.'
	assert_contains 'Backup directory does not exist:' "$output"
	[[ ! -e $temporary_dir/docker.log ]] || die 'missing backup directory invoked Docker.'
}

run_data_volume_rejection_test() {
	local output status
	set +e
	output=$(cd "$temporary_dir/project" && \
		PATH="$temporary_dir/bin:$PATH" \
		BACKUP_TEST_LOG="$temporary_dir/docker.log" \
		BACKUP_TEST_DATA_DIR="$temporary_dir/data-volume" \
		BACKUP_DIR="$temporary_dir/data-volume" bash scripts/backup.sh 2>&1)
	status=$?
	set -e
	[[ $status -ne 0 ]] || die 'backup script accepted the live data volume as BACKUP_DIR.'
	assert_contains 'BACKUP_DIR must be outside the Hooklook data volume.' "$output"
	[[ $(grep -c 'compose run' "$temporary_dir/docker.log") -eq 0 ]] || die 'data-volume rejection started a backup container.'
}

run_existing_destination_test() {
	local output status backup_path
	backup_path="$temporary_dir/backups/hooklook-20260923T190000Z.db"
	touch "$backup_path"
	rm -f "$temporary_dir/docker.log"
	set +e
	output=$(cd "$temporary_dir/project" && \
		PATH="$temporary_dir/bin:$PATH" \
		BACKUP_TEST_LOG="$temporary_dir/docker.log" \
		BACKUP_TEST_DATA_DIR="$temporary_dir/data-volume" \
		BACKUP_DIR="$temporary_dir/backups" bash scripts/backup.sh 2>&1)
	status=$?
	set -e
	[[ $status -ne 0 ]] || die 'backup script accepted an existing destination.'
	assert_contains "Backup destination already exists: $(cd "$temporary_dir/backups" && pwd -P)/$(basename "$backup_path")" "$output"
	[[ $(grep -c 'compose run' "$temporary_dir/docker.log") -eq 0 ]] || die 'existing destination started a backup container.'
	rm -f "$backup_path"
}

file_mode() {
	stat -f '%Lp' "$1" 2>/dev/null || stat -c '%a' "$1"
}

run_success_test() {
	rm -f "$temporary_dir/docker.log"
	local output backup_path checksum_path canonical_backup_dir
	canonical_backup_dir=$(cd "$temporary_dir/backups" && pwd -P)
	output=$(cd / && \
		PATH="$temporary_dir/bin:$PATH" \
		BACKUP_TEST_LOG="$temporary_dir/docker.log" \
		BACKUP_TEST_DATA_DIR="$temporary_dir/data-volume" \
		BACKUP_TEST_BACKUP_DIR="$temporary_dir/backups" \
		BACKUP_DIR="$temporary_dir/backups" bash "$temporary_dir/project/scripts/backup.sh")
	backup_path=$(cd "$temporary_dir/backups" && pwd -P)/$(find "$temporary_dir/backups" -name 'hooklook-*.db' -type f -exec basename {} \;)
	checksum_path="$backup_path.sha256"
	[[ -n $backup_path && -f $checksum_path ]] || die 'successful backup did not create database and checksum files.'
	[[ $(file_mode "$backup_path") == 600 ]] || die 'backup file mode is not 0600.'
	[[ $(file_mode "$checksum_path") == 600 ]] || die 'checksum file mode is not 0600.'
	(
		cd "$temporary_dir/backups"
		sha256sum --check "$(basename "$checksum_path")" >/dev/null
	)
	assert_contains "Backup complete: $backup_path" "$output"
	assert_contains 'compose --env-file .env --env-file .env.image run' "$(cat "$temporary_dir/docker.log")"
	assert_contains "--volume $canonical_backup_dir:/backups hooklook backup /backups/$(basename "$backup_path")" "$(cat "$temporary_dir/docker.log")"
}

die() {
	printf '%s\n' "$*" >&2
	exit 1
}

run_missing_directory_test
rm -f "$temporary_dir/docker.log"
run_data_volume_rejection_test
run_existing_destination_test
run_success_test
printf '%s\n' 'backup script tests passed'
