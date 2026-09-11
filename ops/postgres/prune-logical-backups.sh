#!/bin/sh
set -eu

: "${IRCINTEL_BACKUP_DIR:?IRCINTEL_BACKUP_DIR is required}"
retention_days="${IRCINTEL_BACKUP_RETENTION_DAYS:-30}"

case "$retention_days" in
  ''|*[!0-9]*) echo "IRCINTEL_BACKUP_RETENTION_DAYS must be a positive integer" >&2; exit 2 ;;
esac
[ "$retention_days" -gt 0 ] || { echo "IRCINTEL_BACKUP_RETENTION_DAYS must be > 0" >&2; exit 2; }
[ -d "$IRCINTEL_BACKUP_DIR" ] || exit 0

# Only remove files created by backup-logical.sh. Never recurse and never
# remove unrelated files from the backup directory.
find "$IRCINTEL_BACKUP_DIR" -maxdepth 1 -type f \
  \( -name 'ircintel-*.dump' -o -name 'ircintel-*.dump.sha256' \) \
  -mtime "+$retention_days" -print -delete
