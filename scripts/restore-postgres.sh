#!/usr/bin/env bash
set -Eeuo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"
ENV_FILE="${ENV_FILE:-.env.production}"
BACKUP_FILE="${1:-}"

if [[ -z "$BACKUP_FILE" ]]; then
  echo "usage: $0 <backup.sql.gz>" >&2
  exit 1
fi

if [[ ! -f "$BACKUP_FILE" ]]; then
  echo "backup file not found: $BACKUP_FILE" >&2
  exit 1
fi

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

if [[ "${CONFIRM_RESTORE:-}" != "YES" ]]; then
  echo "restore is destructive and may overwrite current database state" >&2
  echo "run with CONFIRM_RESTORE=YES $0 $BACKUP_FILE" >&2
  exit 1
fi

echo "verifying backup archive: $BACKUP_FILE"
gzip -t "$BACKUP_FILE"

if [[ -f "$BACKUP_FILE.sha256" ]]; then
  echo "verifying checksum: $BACKUP_FILE.sha256"
  sha256sum -c "$BACKUP_FILE.sha256"
fi

echo "stopping api container before restore"
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" stop api

restore_api() {
  echo "starting api container"
  docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" start api >/dev/null || true
}
trap restore_api EXIT

echo "restoring PostgreSQL database from: $BACKUP_FILE"
gunzip -c "$BACKUP_FILE" | docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" exec -T postgres \
  psql \
    -U "$POSTGRES_USER" \
    -d "$POSTGRES_DB" \
    -v ON_ERROR_STOP=1

echo "restore completed"
