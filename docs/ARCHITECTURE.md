# Architecture

SignalBox v0.4 is split into internal layers instead of keeping all logic in `cmd/api/main.go`.

## Structure

```text
cmd/api
  main.go              application bootstrap

internal/config
  config.go            environment loading and validation

internal/domain
  models.go            shared domain models and filters

internal/security
  security.go          token generation, hashing, id helpers

internal/ratelimit
  limiter.go           in-memory fixed-window webhook limiter

internal/storage
  postgres.go          PostgreSQL connection, migrations, queries

internal/delivery
  telegram.go          Telegram notification delivery

internal/httpapi
  server.go            HTTP routing, handlers, middleware
```

## Request flow

```text
client/webhook
  -> internal/httpapi
  -> internal/ratelimit
  -> internal/storage
  -> PostgreSQL
  -> optional internal/delivery Telegram notification
```

## Design decisions

- `cmd/api/main.go` only wires dependencies and starts the HTTP server.
- Business-facing models live in `internal/domain`.
- PostgreSQL-specific logic is isolated inside `internal/storage`.
- Telegram delivery is isolated behind a notifier interface.
- HTTP handlers do validation and translate storage errors into API responses.
- Public webhook requests are rate-limited by client IP and source token.
- Source tokens are never stored in plain text, only SHA-256 hashes.
- Source token is returned only on source creation and token rotation.

## Current limitations

- Telegram delivery is async and lightweight, but not a durable queue.
- Rate limiting is in-memory, so limits are per application replica.
- Pagination is offset-based, not cursor-based.
- Migrations are embedded in Go code.
- There is no web admin UI yet.

## Next production upgrades

- Durable delivery worker with retry/backoff.
- Cursor pagination for events.
- OpenAPI specification.
- Redis-backed distributed rate limit for multi-replica deployments.
- Separate migration files or migration CLI.
