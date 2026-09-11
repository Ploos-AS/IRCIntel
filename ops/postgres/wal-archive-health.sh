#!/bin/sh
set -eu

: "${IRCINTEL_DATABASE_URL:?IRCINTEL_DATABASE_URL is required}"
IRCINTEL_WAL_ARCHIVE_MAX_AGE_SECONDS="${IRCINTEL_WAL_ARCHIVE_MAX_AGE_SECONDS:-300}"

case "$IRCINTEL_WAL_ARCHIVE_MAX_AGE_SECONDS" in
  ''|*[!0-9]*)
    echo "WAL archive health: IRCINTEL_WAL_ARCHIVE_MAX_AGE_SECONDS must be a non-negative integer" >&2
    exit 2
    ;;
esac

psql_row() {
  psql "$IRCINTEL_DATABASE_URL" -X -A -t -F '|' -v ON_ERROR_STOP=1 -c "$1" | tr -d '\r' | tail -n 1
}

row="$(psql_row "SELECT archived_count, failed_count, COALESCE(EXTRACT(EPOCH FROM (clock_timestamp() - last_archived_time))::bigint, -1), COALESCE(last_archived_wal, ''), COALESCE(last_failed_wal, '') FROM pg_stat_archiver")"
IFS='|' read -r archived_count failed_count last_archived_age_seconds last_archived_wal last_failed_wal <<EOF
$row
EOF

case "$archived_count:$failed_count:$last_archived_age_seconds" in
  *[!0-9:-]*)
    echo "WAL archive health: invalid pg_stat_archiver values: $row" >&2
    exit 2
    ;;
esac

healthy=true
reason=ok
if [ "$archived_count" -lt 1 ] || [ "$last_archived_age_seconds" -lt 0 ]; then
  healthy=false
  reason=no_archived_wal
elif [ "$last_archived_age_seconds" -gt "$IRCINTEL_WAL_ARCHIVE_MAX_AGE_SECONDS" ]; then
  healthy=false
  reason=archive_stale
elif [ "$failed_count" -gt 0 ]; then
  healthy=false
  reason=archive_failures
fi

printf '%s\n' \
  "wal_archive_healthy=$healthy" \
  "wal_archive_reason=$reason" \
  "wal_archived_count=$archived_count" \
  "wal_failed_count=$failed_count" \
  "wal_last_archived_age_seconds=$last_archived_age_seconds" \
  "wal_last_archived=$last_archived_wal" \
  "wal_last_failed=$last_failed_wal"

[ "$healthy" = true ]
