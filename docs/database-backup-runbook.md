# Database backup runbook

Hooklook stores captured payloads, request metadata, and owner-secret digests
in the SQLite database on the `hooklook-data` Docker volume. Treat backups as
sensitive production data. Snapshot validity is established on the production
host by the SQLite-aware backup command and an integrity check of the completed
snapshot. The off-host upload is a separate storage operation.

## Backup storage

For resilience against loss of the production VM, the intended destination is
a private Cloudflare R2 bucket. Its configuration and the automated uploader
are deliberately deferred to the final post-observability milestone; the
current script creates a local staging pair only. Do not treat that staging
directory as durable backup storage.

## Create and transfer a backup

On the VM, choose an existing staging directory outside Docker's
`hooklook-data` volume. It must have enough free space for a full SQLite
snapshot.

```bash
cd /opt/hooklook
BACKUP_DIR=/var/backups/hooklook ./scripts/backup.sh
```

The script finds the running service's `/data` volume and refuses a staging
directory inside it. It starts a short-lived root one-off container solely to
write to the operator-selected bind mount; the long-running Hooklook service
continues as its non-root user. The application executes SQLite `VACUUM INTO`,
so the snapshot is consistent while Hooklook remains online. The script writes
these sibling files with mode `0600` and never overwrites either destination:

```text
hooklook-YYYYmmddTHHMMSSZ.db
hooklook-YYYYmmddTHHMMSSZ.db.sha256
```

Before uploading, run the integrity check on the completed snapshot from the
same production host. Replace `backup_file` with the path printed by the
backup script:

```bash
cd /opt/hooklook
backup_file=/var/backups/hooklook/hooklook-YYYYmmddTHHMMSSZ.db
./scripts/compose.sh run --rm --no-deps --user 0 \
  --volume "$(dirname "$backup_file"):/backups" \
  hooklook integrity-check "/backups/$(basename "$backup_file")"
```

It must print `ok`. The later R2 automation will upload this database and its
already-created SHA-256 sidecar under the same timestamped object prefix,
record a successful upload of both objects at the source, and only then remove
the local staging pair. Never copy the live `hooklook.db` file directly;
SQLite journals can make that copy inconsistent.

## Isolated restore drill

This is separate from normal backup automation. Run it only when deliberately
testing restore compatibility. It creates an alternate local container; it
does not start Caddy, touch the production container, or mount
`hooklook-data`.

On the VM, first verify the backup pair and create a separate temporary data
directory.

```bash
cd /opt/hooklook
backup_file=/var/backups/hooklook/hooklook-YYYYmmddTHHMMSSZ.db

cd "$(dirname "$backup_file")"
sha256sum --check "$(basename "$backup_file").sha256"

restore_dir=$(mktemp -d /var/tmp/hooklook-restore.XXXXXX)
install -m 600 "$backup_file" "$restore_dir/hooklook.db"
chown 10001:10001 "$restore_dir" "$restore_dir/hooklook.db"

set -a
source /opt/hooklook/.env
source /opt/hooklook/.env.image
set +a
```

Run the application-provided SQLite integrity check against the isolated copy:

```bash
docker run --rm --user 0 --network none \
  --read-only --tmpfs /tmp \
  --volume "$restore_dir:/data" \
  "$HOOKLOOK_IMAGE" integrity-check /data/hooklook.db
```

It must print `ok`. Then start the restored database on an alternate loopback
port, with no Caddy network attachment:

```bash
docker run --detach --rm \
  --name hooklook-restore-test \
  --read-only --tmpfs /tmp:rw,noexec,nosuid,nodev,size=64m \
  --env PUBLIC_BASE_URL=http://127.0.0.1:18081 \
  --env ADMIN_TOKEN="$ADMIN_TOKEN" \
  --env MAX_STORE="${MAX_STORE:-5000000000}" \
  --env LISTEN_ADDRESS=0.0.0.0:8080 \
  --publish 127.0.0.1:18081:8080 \
  --volume "$restore_dir:/data" \
  "$HOOKLOOK_IMAGE"

curl --fail http://127.0.0.1:18081/health
curl --fail -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://127.0.0.1:18081/admin/storage
```

The health and protected-storage checks are compatibility smoke checks for the
restored application image; they are not a record-level backup test. Record
the backup timestamp, checksum result, integrity result, startup result, and
operator. When the drill has been recorded, remove only the temporary test
container and directory:

```bash
docker stop hooklook-restore-test
rm -rf "$restore_dir"
```

If any check fails, retain the isolated copy for investigation and do not
replace production data. A production restore is an incident response action:
stop, preserve a rollback copy of the live volume, verify the candidate backup
again, and only then plan a separately reviewed restoration.
