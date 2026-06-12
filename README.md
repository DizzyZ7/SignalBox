# SignalBox

SignalBox is a production-oriented Go service for receiving webhooks, storing events in PostgreSQL, detecting duplicate payloads, forwarding new events to Telegram or HTTP endpoints through a durable delivery queue, and managing webhook sources through an admin API.

It is built for teams that need a compact self-hosted webhook gateway without pulling in a heavy event platform, Redis stack or separate frontend build.

## Why SignalBox

Modern products receive webhooks from GitHub, GitLab, Stripe, payment providers, CRMs, monitoring systems, landing pages and internal services. In many teams this turns into repeated glue code: verify a token, store the payload, deduplicate, retry delivery, notify Telegram, inspect failures and keep enough audit trail for incidents.

SignalBox packages that operational layer into one small Go service:

```text
Incoming webhook -> Rate limit -> Token hash lookup -> Deduplication -> PostgreSQL event log -> Delivery queue -> Telegram / HTTP forwarding
```

Use SignalBox when you need:

- a lightweight Go-native webhook inbox;
- durable event storage and replay;
- deduplication without losing duplicate audit records;
- Telegram alerts with per-source templates;
- HTTP forwarding with HMAC signatures;
- retry/backoff queue semantics;
- an embedded admin console without Node, npm or a second UI container;
- a service that can be deployed on a small VPS or inside a larger production stack.

## Features

- Public webhook URL per source
- Admin API protected by `X-API-Key`
- Source tokens stored only as SHA-256 hashes
- Source update, disable and token rotation endpoints
- PostgreSQL event storage
- Duplicate detection without losing duplicate audit records
- Event filters by source, type, origin, duplicate flag and time range
- Cursor-based event pagination with legacy offset support
- Manual event replay back into the delivery queue
- Aggregated stats endpoint with delivery queue counters
- Prometheus-compatible `/metrics` endpoint
- Embedded `/admin` UI
- Editable source test events from the Admin UI
- Optional Telegram notifications
- Per-source Telegram message templates
- Queued HTTP forwarding to external webhook URLs
- HMAC-SHA256 signatures for HTTP forwarding
- SSRF guard for HTTP forwarding destinations
- Postgres-backed delivery queue with retry/backoff
- Delivery filters by status, channel, source and event
- Manual retry endpoint for failed delivery jobs
- OpenAPI 3.0 specification
- Public webhook rate limiting
- Admin API rate limiting
- Webhook token redaction in access logs
- Security headers middleware
- Backup and restore helper scripts
- Dependabot dependency updates
- CodeQL security scanning
- Trivy filesystem and Docker image scanning
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

Check API:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

Open Admin UI:

```text
http://localhost:8080/admin
```

Create source:

```bash
curl -X POST http://localhost:8080/v1/sources \
  -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"name":"Main landing","forward_url":"https://example.com/webhooks/signalbox"}'
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

Telegram and HTTP forwarding delivery are queued and retried by a background worker. Defaults:

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

Replay an event into the delivery queue:

```bash
curl -X POST http://localhost:8080/v1/events/<EVENT_ID>/replay \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

Send a source test event without exposing the public webhook token:

```bash
curl -X POST http://localhost:8080/v1/sources/<SOURCE_ID>/test-event \
  -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"payload":{"type":"signalbox.test","source":"admin","external_id":"test-1"}}'
```

List failed deliveries:

```bash
curl "http://localhost:8080/v1/deliveries?status=failed" \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

Get stats:

```bash
curl http://localhost:8080/v1/stats \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

Get Prometheus metrics:

```bash
curl http://localhost:8080/metrics
```

Rotate source token:

```bash
curl -X POST http://localhost:8080/v1/sources/<SOURCE_ID>/rotate-token \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

## Production deploy

Use the prebuilt GHCR image or the production Compose file from this repository.

```bash
git clone https://github.com/DizzyZ7/SignalBox.git
cd SignalBox
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
internal/security   token/id/hash/forward URL safety helpers
internal/ratelimit  in-memory webhook rate limiter
internal/storage    PostgreSQL queries, migrations and delivery queue
internal/delivery   Telegram and HTTP delivery queue worker
internal/httpapi    HTTP routing, handlers, embedded UI and metrics wrapper
internal/metrics    lightweight Prometheus text metrics registry
scripts             backup and restore helpers
.github             CI, Dependabot, CodeQL and Trivy workflows
```

## Development

```bash
make fmt
make test
make vet
make build
```

## Security automation

SignalBox uses Dependabot for Go modules and GitHub Actions updates, CodeQL for Go security analysis, Trivy for filesystem and Docker image CVE scanning, ShellCheck for recovery scripts and a security policy for vulnerability reporting.

## Observability

SignalBox exposes Prometheus-compatible metrics at `/metrics`. See [`docs/METRICS.md`](docs/METRICS.md) for metric names, scrape examples and alert ideas.

## Telegram templates

SignalBox supports per-source Telegram message templates using Go `text/template`. See [`docs/TELEGRAM_TEMPLATES.md`](docs/TELEGRAM_TEMPLATES.md) for available variables and examples.

## HTTP forwarding

SignalBox can forward unique accepted events to external HTTP endpoints through the same durable delivery queue as Telegram notifications. Forwarding includes HMAC signatures and SSRF protection for local/private destinations. See [`docs/HTTP_FORWARDING.md`](docs/HTTP_FORWARDING.md).

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
- [Metrics](docs/METRICS.md)
- [Telegram templates](docs/TELEGRAM_TEMPLATES.md)
- [HTTP forwarding](docs/HTTP_FORWARDING.md)
- [Enterprise readiness audit](docs/ENTERPRISE_AUDIT.md)
