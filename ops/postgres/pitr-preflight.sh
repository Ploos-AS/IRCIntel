#!/bin/sh
set -eu

: "${IRCINTEL_DATABASE_URL:?IRCINTEL_DATABASE_URL is required}"

psql_scalar() {
  psql "$IRCINTEL_DATABASE_URL" -X -A -t -v ON_ERROR_STOP=1 -c "$1" | tr -d '\r' | tail -n 1
}

server_version_num="$(psql_scalar "SHOW server_version_num")"
wal_level="$(psql_scalar "SHOW wal_level")"
archive_mode="$(psql_scalar "SHOW archive_mode")"
archive_command="$(psql_scalar "SHOW archive_command")"
archive_timeout="$(psql_scalar "SHOW archive_timeout")"

case "$server_version_num" in
  ''|*[!0-9]*)
    echo "PITR preflight: invalid PostgreSQL server_version_num: $server_version_num" >&2
    exit 1
    ;;
esac

major=$((server_version_num / 10000))
if [ "$major" -ne 17 ]; then
  echo "PITR preflight: PostgreSQL 17 required, found major $major" >&2
  exit 1
fi

case "$wal_level" in
  replica|logical) ;;
  *)
    echo "PITR preflight: wal_level must be replica or logical, found '$wal_level'" >&2
    exit 1
    ;;
esac

case "$archive_mode" in
  on|always) ;;
  *)
    echo "PITR preflight: archive_mode must be on or always, found '$archive_mode'" >&2
    exit 1
    ;;
esac

if [ -z "$archive_command" ] || [ "$archive_command" = '(disabled)' ] || [ "$archive_command" = 'true' ] || [ "$archive_command" = '/bin/true' ]; then
  echo "PITR preflight: archive_command must perform durable WAL archival, found '$archive_command'" >&2
  exit 1
fi

printf '%s\n' \
  "pitr_ready=true" \
  "postgres_major=$major" \
  "wal_level=$wal_level" \
  "archive_mode=$archive_mode" \
  "archive_timeout=$archive_timeout"
