# SignalBox

SignalBox is a production-oriented Go service for receiving webhooks, storing events in PostgreSQL, detecting duplicate payloads, forwarding new events to Telegram through a durable delivery queue, and managing webhook sources through an admin API.

## Features

- Public webhook URL per source
- Admin API protected by `X-API-Key`
- Source tokens stored only as SHA-256 hashes
- Source update, disable and token rotation endpoints
- PostgreSQL event storage
- Duplicate detection without losing audit records
- Event filters by source, type, origin, duplicate flag and time range
- Cursor-based event pagination with legacy offset support
- Manual event replay back into the delivery queue
- Aggregated stats endpoint with delivery queue counters
- Optional Telegram notifications
- Postgres-backed delivery queue with retry/backoff
- Manual retry endpoint for failed delivery jobs
- OpenAPI 3.0 specification
- Public webhook rate limiting
- Admin API rate limiting
- Webhook token redaction in access logs
- Security headers middleware
- Backup and restore helper scripts
- Health and readiness probes
- Docker and Docker Compose setup
- Production compose file for GHCR deployments
- GHCR image publishing workflow
- JSON structured logs
- Graceful shutdown
- GitHub Actions CI
- Layered internal architecture

## Stack

- Go 1.25
- PostgreSQL 16
- pgx
- Docker

## Quick start

```bash
cp .env.example .env
# edit ADMIN_API_KEY in .env
docker compose --env-file .env up --build
```

Released Docker images are published to GHCR:

```bash
docker pull ghcr.io/dizzyz7/signalbox:latest
```

Check API:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

Create source:

```bash
curl -X POST http://localhost:8080/v1/sources \
  -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"name":"Main landing"}'
```

Save the returned `token`. It is shown only once.

Send event:

```bash
curl -X POST http://localhost:8080/v1/hooks/<SOURCE_TOKEN> \
  -H "Content-Type: application/json" \
  -d '{"type":"lead.created","source":"landing","contact":"@user"}'
```

Public webhooks are rate-limited by IP and source token. Defaults:

```env
WEBHOOK_RATE_LIMIT_REQUESTS=120
WEBHOOK_RATE_LIMIT_WINDOW=1m
```

Telegram delivery is queued and retried by a background worker. Defaults:

```env
DELIVERY_WORKER_ENABLED=true
DELIVERY_WORKER_INTERVAL=5s
DELIVERY_WORKER_BATCH_SIZE=10
DELIVERY_WORKER_LOCK_DURATION=1m
DELIVERY_MAX_ATTEMPTS=8
```

List events:

```bash
curl "http://localhost:8080/v1/events?type=lead.created&duplicate=false&limit=50" \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

Next page with cursor:

```bash
curl "http://localhost:8080/v1/events?limit=50&cursor=<NEXT_CURSOR>" \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

Replay an event into the delivery queue:

```bash
curl -X POST http://localhost:8080/v1/events/<EVENT_ID>/replay \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

List failed deliveries:

```bash
curl "http://localhost:8080/v1/deliveries?status=failed&channel=telegram" \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

Get stats:

```bash
curl http://localhost:8080/v1/stats \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

Rotate source token:

```bash
curl -X POST http://localhost:8080/v1/sources/<SOURCE_ID>/rotate-token \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

## Production deploy

Use the prebuilt image and production compose file for VPS deployments:

```bash
mkdir -p /opt/signalbox
cd /opt/signalbox
curl -fsSLO https://raw.githubusercontent.com/DizzyZ7/SignalBox/main/docker-compose.prod.yml
curl -fsSLO https://raw.githubusercontent.com/DizzyZ7/SignalBox/main/.env.production.example
mkdir -p scripts
curl -fsSLo scripts/backup-postgres.sh https://raw.githubusercontent.com/DizzyZ7/SignalBox/main/scripts/backup-postgres.sh
curl -fsSLo scripts/restore-postgres.sh https://raw.githubusercontent.com/DizzyZ7/SignalBox/main/scripts/restore-postgres.sh
chmod +x scripts/*.sh
cp .env.production.example .env.production
nano .env.production
docker compose --env-file .env.production -f docker-compose.prod.yml up -d
```

Run SignalBox behind HTTPS through Nginx, Caddy or another reverse proxy. The production compose file binds the API to `127.0.0.1` by default.

Create a compressed PostgreSQL backup with checksum:

```bash
./scripts/backup-postgres.sh
```

Restore from backup requires explicit confirmation:

```bash
CONFIRM_RESTORE=YES ./scripts/restore-postgres.sh backups/signalbox-YYYYMMDDTHHMMSSZ.sql.gz
```

See [`docs/RUNBOOK.md`](docs/RUNBOOK.md) for operations, backups, restore, rollback and incident handling.

## Architecture

```text
cmd/api             app bootstrap
internal/config     environment loading
internal/domain     domain models
internal/security   token/id/hash helpers
internal/ratelimit  in-memory webhook rate limiter
internal/storage    PostgreSQL queries, migrations and delivery queue
internal/delivery   Telegram delivery queue producer and worker
internal/httpapi    HTTP routing and handlers
```

## Development

```bash
make fmt
make test
make vet
make build
```

## Releases

Create a version tag to publish a Docker image to GHCR:

```bash
git tag v1.0.0
git push origin v1.0.0
```

The workflow publishes:

```text
ghcr.io/dizzyz7/signalbox:v1.0.0
ghcr.io/dizzyz7/signalbox:latest
```

## Documentation

- [API](docs/API.md)
- [OpenAPI](docs/openapi.yaml)
- [Architecture](docs/ARCHITECTURE.md)
- [Deployment](docs/DEPLOY.md)
- [Runbook](docs/RUNBOOK.md)

## Production notes

Use HTTPS before public exposure, keep webhook rate limits enabled, rotate tokens if leaked, keep PostgreSQL backups enabled, keep the delivery worker enabled for Telegram notifications, and use a long random `ADMIN_API_KEY`.
