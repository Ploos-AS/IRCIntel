#!/usr/bin/env bash
set -euo pipefail

: "${IRCINTEL_BACKUP_S3_ENDPOINT:=http://127.0.0.1:4566}"
: "${IRCINTEL_BASEBACKUP_S3_ENDPOINT:=$IRCINTEL_BACKUP_S3_ENDPOINT}"
: "${IRCINTEL_WAL_S3_ENDPOINT:=$IRCINTEL_BACKUP_S3_ENDPOINT}"
: "${IRCINTEL_BASEBACKUP_S3_BUCKET:=ircintel-drill}"
: "${IRCINTEL_WAL_S3_BUCKET:=$IRCINTEL_BASEBACKUP_S3_BUCKET}"
: "${IRCINTEL_BASEBACKUP_S3_PREFIX:=postgres/basebackup}"
: "${IRCINTEL_WAL_S3_PREFIX:=postgres/wal}"
export IRCINTEL_BACKUP_S3_ENDPOINT IRCINTEL_BASEBACKUP_S3_ENDPOINT IRCINTEL_WAL_S3_ENDPOINT
export IRCINTEL_BASEBACKUP_S3_BUCKET IRCINTEL_WAL_S3_BUCKET
export IRCINTEL_BASEBACKUP_S3_PREFIX IRCINTEL_WAL_S3_PREFIX

cleanup() {
  docker rm -f m434-source m434-restored ircintel-localstack >/dev/null 2>&1 || true
  docker network rm ircintel-remote-pitr >/dev/null 2>&1 || true
}
trap cleanup EXIT

rm -rf /tmp/m434 /tmp/m434-base.tar.gz
mkdir -p /tmp/m434/archive /tmp/m434/base
chmod 0777 /tmp/m434/archive /tmp/m434/base

docker run -d --name ircintel-localstack -p 4566:4566 -e SERVICES=s3 localstack/localstack:4.4.0 >/dev/null
for attempt in $(seq 1 60); do
  if curl -fsS http://127.0.0.1:4566/_localstack/health | grep -q '"s3"'; then
    break
  fi
  if [ "$attempt" -eq 60 ]; then
    docker logs ircintel-localstack
    exit 1
  fi
  sleep 1
done
aws --endpoint-url "$IRCINTEL_BACKUP_S3_ENDPOINT" s3api create-bucket --bucket "$IRCINTEL_BASEBACKUP_S3_BUCKET" >/dev/null

chmod +x ops/postgres/basebackup-s3.sh ops/postgres/wal-archive-s3.sh ops/postgres/pitr-fetch-s3.sh
docker network create ircintel-remote-pitr >/dev/null

docker run -d --name m434-source --network ircintel-remote-pitr \
  -e POSTGRES_USER=ircintel -e POSTGRES_PASSWORD=ircintel -e POSTGRES_DB=ircintel \
  -v /tmp/m434/archive:/archive postgres:17 \
  -c wal_level=replica -c archive_mode=on \
  -c "archive_command=test ! -f /archive/%f && cp %p /archive/%f" >/dev/null

for attempt in $(seq 1 30); do
  if docker exec m434-source psql -U ircintel -d ircintel -Atc 'SELECT 1' 2>/dev/null | grep -qx 1; then
    break
  fi
  if [ "$attempt" -eq 30 ]; then
    docker logs m434-source
    exit 1
  fi
  sleep 1
done

docker exec m434-source psql -U ircintel -d ircintel -v ON_ERROR_STOP=1 -c \
  "CREATE TABLE remote_pitr(id integer PRIMARY KEY, marker text NOT NULL); INSERT INTO remote_pitr VALUES (1,'base');"
docker exec m434-source pg_basebackup -U ircintel -D /tmp/basebackup -Fp -Xs -P
docker cp m434-source:/tmp/basebackup/. /tmp/m434/base/
docker exec m434-source psql -U ircintel -d ircintel -v ON_ERROR_STOP=1 -c \
  "INSERT INTO remote_pitr VALUES (2,'before-target');"
target="$(docker exec m434-source psql -U ircintel -d ircintel -Atc "SELECT clock_timestamp() + interval '2 seconds'")"
printf '%s' "$target" > /tmp/m434/target
sleep 3
docker exec m434-source psql -U ircintel -d ircintel -v ON_ERROR_STOP=1 -c \
  "INSERT INTO remote_pitr VALUES (3,'after-target'); SELECT pg_switch_wal();"
sleep 3
docker stop m434-source >/dev/null

sudo chown -R "$(id -u):$(id -g)" /tmp/m434/base /tmp/m434/archive
tar -C /tmp/m434 -czf /tmp/m434-base.tar.gz base
ops/postgres/basebackup-s3.sh /tmp/m434-base.tar.gz m434-base.tar.gz | tee /tmp/m434/base-upload.txt
grep -q '^basebackup_transfer_result=uploaded$' /tmp/m434/base-upload.txt

wal_count=0
for wal in /tmp/m434/archive/*; do
  [ -f "$wal" ] || continue
  ops/postgres/wal-archive-s3.sh "$wal" "$(basename "$wal")" > /tmp/m434/wal-upload.txt
  grep -Eq '^wal_archive_result=(uploaded|already_present)$' /tmp/m434/wal-upload.txt
  wal_count=$((wal_count + 1))
done
[ "$wal_count" -gt 0 ]

rm -rf /tmp/m434/base /tmp/m434/archive /tmp/m434-base.tar.gz
ops/postgres/pitr-fetch-s3.sh m434-base.tar.gz postgres/wal /tmp/m434/remote | tee /tmp/m434/fetch.txt
grep -q '^pitr_fetch_result=ok$' /tmp/m434/fetch.txt
tar -C /tmp/m434/remote -xzf /tmp/m434/remote/basebackup.tar.gz
test -f /tmp/m434/remote/base/PG_VERSION

target="$(cat /tmp/m434/target)"
cat >> /tmp/m434/remote/base/postgresql.auto.conf <<EOF
restore_command = 'cp /archive/%f %p'
recovery_target_time = '$target'
recovery_target_action = 'promote'
EOF
touch /tmp/m434/remote/base/recovery.signal
chmod 0700 /tmp/m434/remote/base
sudo chown -R 999:999 /tmp/m434/remote/base

docker run -d --name m434-restored --network ircintel-remote-pitr \
  -v /tmp/m434/remote/base:/var/lib/postgresql/data \
  -v /tmp/m434/remote/wal:/archive:ro postgres:17 >/dev/null

for attempt in $(seq 1 60); do
  if docker exec m434-restored pg_isready -U ircintel -d ircintel >/dev/null 2>&1; then
    break
  fi
  if [ "$attempt" -eq 60 ]; then
    docker logs m434-restored
    exit 1
  fi
  sleep 1
done

rows="$(docker exec m434-restored psql -U ircintel -d ircintel -Atc \
  "SELECT string_agg(id || ':' || marker, ',' ORDER BY id) FROM remote_pitr")"
printf 'recovered=<%s> target=<%s>\n' "$rows" "$target"
test "$rows" = '1:base,2:before-target'
docker exec m434-restored psql -U ircintel -d ircintel -Atc 'SELECT pg_is_in_recovery()' | grep -qx f

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  {
    echo '## Remote PITR recovery drill'
    echo
    echo "- trigger: ${GITHUB_EVENT_NAME:-unknown}"
    echo '- PostgreSQL: 17'
    echo '- recovery source: S3-compatible remote storage only'
    echo "- recovered rows: $rows"
    echo "- target: $target"
    echo '- result: PASS'
  } >> "$GITHUB_STEP_SUMMARY"
fi

echo 'remote_pitr_drill_result=PASS'
