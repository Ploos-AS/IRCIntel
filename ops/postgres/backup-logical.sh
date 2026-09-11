#!/bin/sh
set -eu

: "${IRCINTEL_DATABASE_URL:?IRCINTEL_DATABASE_URL is required}"
: "${IRCINTEL_BACKUP_DIR:?IRCINTEL_BACKUP_DIR is required}"

umask 077
mkdir -p "$IRCINTEL_BACKUP_DIR"

stamp="$(date -u +%Y%m%dT%H%M%SZ)"
final="$IRCINTEL_BACKUP_DIR/ircintel-$stamp.dump"
tmp="$final.tmp"
sha="$final.sha256"

cleanup() { rm -f "$tmp"; }
trap cleanup EXIT HUP INT TERM

pg_dump "$IRCINTEL_DATABASE_URL" \
  --format=custom \
  --no-owner \
  --no-acl \
  --file="$tmp"

test -s "$tmp"
pg_restore --list "$tmp" >/dev/null
mv "$tmp" "$final"
sha256sum "$final" > "$sha"
trap - EXIT HUP INT TERM

printf '%s\n' "$final"
