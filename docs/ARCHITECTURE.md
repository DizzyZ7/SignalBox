# Architecture

SignalBox v0.5 is split into internal layers instead of keeping all logic in `cmd/api/main.go`.

## Structure

```text
cmd/api
  main.go              application bootstrap and worker startup

internal/config
  config.go            environment loading and validation

internal/domain
  models.go            shared domain models and filters
  delivery.go          delivery job model

internal/security
  security.go          token generation, hashing, id helpers

internal/ratelimit
  limiter.go           in-memory fixed-window webhook limiter

internal/storage
  postgres.go          PostgreSQL connection, migrations, event queries
  delivery_jobs.go     Postgres-backed delivery queue

internal/delivery
  telegram.go          Telegram delivery producer and retry worker

internal/httpapi
  server.go            HTTP routing, handlers, middleware
```

## Request flow

```text
client/webhook
  -> internal/httpapi
  -> internal/ratelimit
  -> internal/storage
  -> PostgreSQL events
  -> internal/delivery enqueue delivery_jobs row
  -> background worker claims delivery_jobs row
  -> Telegram API
  -> sent or retry with backoff
```

## Design decisions

- `cmd/api/main.go` only wires dependencies, starts the HTTP server and starts the delivery worker.
- Business-facing models live in `internal/domain`.
- PostgreSQL-specific logic is isolated inside `internal/storage`.
- Telegram delivery is isolated behind a notifier interface and backed by a durable queue.
- HTTP handlers do validation and translate storage errors into API responses.
- Public webhook requests are rate-limited by client IP and source token.
- Source tokens are never stored in plain text, only SHA-256 hashes.
- Source token is returned only on source creation and token rotation.

## Delivery reliability

Telegram delivery uses the `delivery_jobs` table:

- new unique events enqueue a Telegram job;
- worker claims jobs with `FOR UPDATE SKIP LOCKED`;
- jobs are locked with `locked_until` to avoid double processing;
- failed jobs return to `pending` with exponential backoff;
- jobs become terminal `failed` after `DELIVERY_MAX_ATTEMPTS`;
- sent jobs are marked as `sent` with `sent_at`.

## Current limitations

- Rate limiting is in-memory, so limits are per application replica.
- Delivery queue is Postgres-backed; for very high throughput it can later move to Redis, NATS or RabbitMQ.
- Pagination is offset-based, not cursor-based.
- Migrations are embedded in Go code.
- There is no web admin UI yet.

## Next production upgrades

- Cursor pagination for events.
- Delivery jobs admin endpoint.
- OpenAPI specification.
- Redis-backed distributed rate limit for multi-replica deployments.
- Separate migration files or migration CLI.
