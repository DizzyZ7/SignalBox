# SignalBox

SignalBox is a production-oriented Go service for receiving webhooks, storing events in PostgreSQL, detecting duplicate payloads, forwarding new events to Telegram, and managing webhook sources through an admin API.

## Features

- Public webhook URL per source
- Admin API protected by `X-API-Key`
- Source tokens stored only as SHA-256 hashes
- Source update, disable and token rotation endpoints
- PostgreSQL event storage
- Duplicate detection without losing audit records
- Event filters by source, type, origin, duplicate flag and time range
- Aggregated stats endpoint
- Optional Telegram notifications
- Health and readiness probes
- Docker and Docker Compose setup
- JSON structured logs
- Graceful shutdown
- GitHub Actions CI
- Layered internal architecture

## Stack

- Go 1.23
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

List events:

```bash
curl "http://localhost:8080/v1/events?type=lead.created&duplicate=false" \
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

## Architecture

```text
cmd/api          app bootstrap
internal/config  environment loading
internal/domain  domain models
internal/security token/id/hash helpers
internal/storage PostgreSQL queries and migrations
internal/delivery Telegram notification delivery
internal/httpapi HTTP routing and handlers
```

## Development

```bash
make fmt
make test
make vet
make build
```

## Documentation

- [API](docs/API.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Deployment](docs/DEPLOY.md)

## Production notes

Use HTTPS before public exposure, rotate tokens if leaked, keep PostgreSQL backups enabled, and use a long random `ADMIN_API_KEY`.
