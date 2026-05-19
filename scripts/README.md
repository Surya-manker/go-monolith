# Scripts

Operational scripts for InvoBill production deployments.

## backup.sh

Full MySQL backup with automatic rotation.

```bash
# One-time backup (reads DATABASE_DSN from app.env automatically)
./scripts/backup.sh

# Keep 14 days of backups instead of the default 7
BACKUP_KEEP=14 ./scripts/backup.sh

# Custom backup directory
BACKUP_DIR=/mnt/nas/invobill ./scripts/backup.sh
```

Backup files are named `invobill_YYYYMMDD_HHMMSS.sql.gz` and stored in `./backups/` (or `$BACKUP_DIR`).

### Schedule with cron

```cron
# Daily backup at 2:00 AM
0 2 * * * /path/to/go-monolith/scripts/backup.sh >> /var/log/invobill-backup.log 2>&1
```

## restore.sh

Restore a backup (interactive — prompts before dropping the database).

```bash
./scripts/restore.sh ./backups/invobill_20240115_020000.sql.gz
```

## backup-rotate.sh

List or prune backups without running a new dump.

```bash
# List all backups
./scripts/backup-rotate.sh --list

# Prune backups older than 30 days
./scripts/backup-rotate.sh --keep 30 --dir /mnt/nas/invobill
```

## Docker backup (docker-compose)

Run an on-demand backup via Docker Compose:

```bash
docker-compose --profile backup run --rm backup
```
