#!/usr/bin/env bash
# restore.sh — Restore an InvoBill MySQL backup
# Usage: ./scripts/restore.sh <backup-file.sql.gz>
#
# WARNING: This drops and recreates the target database. Back up first!

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
if [[ -f "$ROOT_DIR/app.env" ]]; then
  set -a; source "$ROOT_DIR/app.env"; set +a
fi

BACKUP_FILE="${1:-}"
if [[ -z "$BACKUP_FILE" ]]; then
  echo "Usage: $0 <backup-file.sql.gz>"
  exit 1
fi
if [[ ! -f "$BACKUP_FILE" ]]; then
  echo "Error: file not found: $BACKUP_FILE"
  exit 1
fi

DSN="${DATABASE_DSN:-root:@tcp(127.0.0.1:3306)/invobill?parseTime=true}"
DB_USER="$(echo "$DSN" | sed 's|:.*||')"
DB_PASS="$(echo "$DSN" | sed 's|[^:]*:||; s|@.*||')"
DB_HOST="$(echo "$DSN" | sed 's|.*tcp(||; s|).*||; s|:.*||')"
DB_PORT="$(echo "$DSN" | sed 's|.*tcp([^:]*:||; s|).*||')"
DB_NAME="$(echo "$DSN" | sed 's|.*/||; s|?.*||')"

echo "[restore] restoring $BACKUP_FILE → $DB_NAME@$DB_HOST:$DB_PORT"
read -rp "This will DROP and recreate $DB_NAME. Continue? [y/N] " confirm
[[ "$confirm" =~ ^[Yy]$ ]] || { echo "Aborted."; exit 1; }

export MYSQL_PWD="$DB_PASS"

mysql -u "$DB_USER" -h "$DB_HOST" -P "$DB_PORT" \
  -e "DROP DATABASE IF EXISTS \`$DB_NAME\`; CREATE DATABASE \`$DB_NAME\` CHARACTER SET utf8mb4;"

zcat "$BACKUP_FILE" | mysql -u "$DB_USER" -h "$DB_HOST" -P "$DB_PORT" "$DB_NAME"

echo "[restore] done."
