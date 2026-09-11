#!/bin/sh
set -eu

[ "$#" -eq 1 ] || { echo "usage: $0 BACKUP.dump" >&2; exit 2; }
: "${IRCINTEL_RESTORE_DATABASE_URL:?IRCINTEL_RESTORE_DATABASE_URL is required}"
: "${IRCINTEL_RESTORE_CONFIRM:?set IRCINTEL_RESTORE_CONFIRM=restore-into-empty-database}"
[ "$IRCINTEL_RESTORE_CONFIRM" = "restore-into-empty-database" ] || {
  echo "refusing restore: confirmation value is incorrect" >&2
  exit 2
}

backup="$1"
test -s "$backup"
pg_restore --list "$backup" >/dev/null

# This intentionally does not create/drop databases and does not clean an
# existing database. Operators must supply a fresh empty target database.
pg_restore "$backup" \
  --dbname="$IRCINTEL_RESTORE_DATABASE_URL" \
  --no-owner \
  --no-acl \
  --exit-on-error
