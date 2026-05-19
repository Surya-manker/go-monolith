#!/usr/bin/env bash
# backup.sh — MySQL backup with rotation
# Usage: ./scripts/backup.sh [--keep N]
#
# Required env vars (or use app.env values):
#   DATABASE_DSN  — full MySQL DSN, or set individual vars below.
#   BACKUP_DIR    — destination directory (default: ./backups)
#   BACKUP_KEEP   — number of daily backups to retain (default: 7)

set -euo pipefail

# ── Load app.env if present ──────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
if [[ -f "$ROOT_DIR/app.env" ]]; then
  # shellcheck disable=SC1090
  set -a; source "$ROOT_DIR/app.env"; set +a
fi

# ── Defaults ─────────────────────────────────────────────────────────────────
BACKUP_DIR="${BACKUP_DIR:-$ROOT_DIR/backups}"
BACKUP_KEEP="${BACKUP_KEEP:-7}"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
FILENAME="invobill_${TIMESTAMP}.sql.gz"

# Parse DATABASE_DSN into components (user:pass@tcp(host:port)/db).
DSN="${DATABASE_DSN:-root:@tcp(127.0.0.1:3306)/invobill?parseTime=true}"
DB_USER="$(echo "$DSN" | sed 's|:.*||')"
DB_PASS="$(echo "$DSN" | sed 's|[^:]*:||; s|@.*||')"
DB_HOST="$(echo "$DSN" | sed 's|.*tcp(||; s|).*||; s|:.*||')"
DB_PORT="$(echo "$DSN" | sed 's|.*tcp([^:]*:||; s|).*||')"
DB_NAME="$(echo "$DSN" | sed 's|.*/||; s|?.*||')"

mkdir -p "$BACKUP_DIR"

echo "[backup] dumping $DB_NAME@$DB_HOST:$DB_PORT → $BACKUP_DIR/$FILENAME"

MYSQL_PWD="$DB_PASS" mysqldump \
  -u "$DB_USER" \
  -h "$DB_HOST" \
  -P "$DB_PORT" \
  --single-transaction \
  --routines \
  --triggers \
  "$DB_NAME" | gzip > "$BACKUP_DIR/$FILENAME"

echo "[backup] done — $(du -sh "$BACKUP_DIR/$FILENAME" | cut -f1)"

# ── Rotation: delete backups older than BACKUP_KEEP days ─────────────────────
echo "[backup] pruning backups older than $BACKUP_KEEP days…"
find "$BACKUP_DIR" -name "invobill_*.sql.gz" -mtime +"$BACKUP_KEEP" -delete
echo "[backup] remaining backups:"
ls -lh "$BACKUP_DIR"/invobill_*.sql.gz 2>/dev/null || echo "  (none)"
