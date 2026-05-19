#!/usr/bin/env bash
# backup-rotate.sh — Standalone rotation / listing utility
# Usage: ./scripts/backup-rotate.sh [--keep N] [--list] [--dir DIR]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

BACKUP_DIR="${BACKUP_DIR:-$ROOT_DIR/backups}"
KEEP=7
LIST_ONLY=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --keep) KEEP="$2"; shift 2 ;;
    --dir)  BACKUP_DIR="$2"; shift 2 ;;
    --list) LIST_ONLY=true; shift ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

echo "[rotate] backup dir: $BACKUP_DIR"
echo "[rotate] keep last: $KEEP days"

if $LIST_ONLY; then
  ls -lhtr "$BACKUP_DIR"/invobill_*.sql.gz 2>/dev/null || echo "  (no backups found)"
  exit 0
fi

echo "[rotate] pruning backups older than $KEEP days…"
find "$BACKUP_DIR" -name "invobill_*.sql.gz" -mtime +"$KEEP" -delete

echo "[rotate] remaining:"
ls -lhtr "$BACKUP_DIR"/invobill_*.sql.gz 2>/dev/null || echo "  (none)"
