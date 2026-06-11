#!/usr/bin/env bash
set -Eeuo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"
ENV_FILE="${ENV_FILE:-.env.production}"
BACKUP_DIR="${BACKUP_DIR:-backups}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"

if [[ ! -f "$COMPOSE_FILE" ]]; then
  echo "compose file not found: $COMPOSE_FILE" >&2
  exit 1
fi

if [[ ! -f "$ENV_FILE" ]]; then
  echo "env file not found: $ENV_FILE" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

: "${POSTGRES_DB:?POSTGRES_DB is required}"
: "${POSTGRES_USER:?POSTGRES_USER is required}"

mkdir -p "$BACKUP_DIR"

TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
BACKUP_FILE="$BACKUP_DIR/signalbox-$TIMESTAMP.sql.gz"
TMP_FILE="$BACKUP_FILE.tmp"

cleanup() {
  rm -f "$TMP_FILE"
}
trap cleanup EXIT

echo "creating PostgreSQL backup: $BACKUP_FILE"

docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" exec -T postgres \
  pg_dump \
    -U "$POSTGRES_USER" \
    -d "$POSTGRES_DB" \
    --clean \
    --if-exists \
    --no-owner \
    --no-privileges \
  | gzip -9 > "$TMP_FILE"

if [[ ! -s "$TMP_FILE" ]]; then
  echo "backup failed: empty output" >&2
  exit 1
fi

gzip -t "$TMP_FILE"
mv "$TMP_FILE" "$BACKUP_FILE"
sha256sum "$BACKUP_FILE" > "$BACKUP_FILE.sha256"

if [[ "$RETENTION_DAYS" =~ ^[0-9]+$ ]] && [[ "$RETENTION_DAYS" -gt 0 ]]; then
  find "$BACKUP_DIR" -type f -name 'signalbox-*.sql.gz' -mtime +"$RETENTION_DAYS" -delete
  find "$BACKUP_DIR" -type f -name 'signalbox-*.sql.gz.sha256' -mtime +"$RETENTION_DAYS" -delete
fi

echo "backup completed: $BACKUP_FILE"
echo "checksum: $BACKUP_FILE.sha256"
