# Architecture

SignalBox is split into internal layers instead of keeping all logic in `cmd/api/main.go`.

## Structure

```text
cmd/api
  main.go              application bootstrap, HTTP server and worker startup

internal/config
  config.go            environment loading and validation

internal/domain
  models.go            shared source/event/stat models and filters
  delivery.go          delivery job model

internal/security
  security.go          token generation, hashing, id helpers

internal/ratelimit
  limiter.go           in-memory fixed-window webhook limiter

internal/storage
  postgres.go          PostgreSQL connection, migrations, event queries
  events_cursor.go     cursor-based event listing query
  delivery_jobs.go     Postgres-backed delivery queue

internal/delivery
  telegram.go          Telegram delivery producer and retry worker

internal/httpapi
  server.go            HTTP routing, handlers, middleware
  events_cursor.go     cursor pagination handler
  events_replay.go     event replay handler
  deliveries.go        delivery job admin handlers

scripts
  backup-postgres.sh   compressed PostgreSQL backup with checksum
  restore-postgres.sh  guarded PostgreSQL restore from backup
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

## Event replay flow

```text
admin
  -> POST /v1/events/{id}/replay
  -> load event with source
  -> validate active source and notifier readiness
  -> enqueue new delivery_jobs row
  -> delivery worker processes the job
```

Replay does not create a new event row and does not change deduplication state. It only reuses the stored event and puts it back into the delivery pipeline.

## Design decisions

- `cmd/api/main.go` only wires dependencies, starts the HTTP server and starts the delivery worker.
- Business-facing models live in `internal/domain`.
- PostgreSQL-specific logic is isolated inside `internal/storage`.
- Telegram delivery is isolated behind a notifier interface and backed by a durable queue.
- HTTP handlers do validation and translate storage errors into API responses.
- Public webhook requests are rate-limited by client IP and source token.
- Admin API requests are separately rate-limited before API key validation.
- Source tokens are never stored in plain text, only SHA-256 hashes.
- Source token is returned only on source creation and token rotation.
- Webhook source tokens are redacted from access logs.
- Event replay is admin-only and validates notifier readiness before queueing.

## Delivery reliability

Telegram delivery uses the `delivery_jobs` table:

- new unique events enqueue a Telegram job;
- admin replay can enqueue another job for an existing event;
- worker claims jobs with `FOR UPDATE SKIP LOCKED`;
- jobs are locked with `locked_until` to avoid double processing;
- failed jobs return to `pending` with exponential backoff;
- jobs become terminal `failed` after `DELIVERY_MAX_ATTEMPTS`;
- sent jobs are marked as `sent` with `sent_at`.

## Data-loss prevention

- PostgreSQL stores sources, events, deduplication keys, delivery jobs and delivery attempts.
- `scripts/backup-postgres.sh` creates compressed backups and SHA-256 checksums.
- `scripts/restore-postgres.sh` requires `CONFIRM_RESTORE=YES`, verifies backup integrity and stops API writes before restore.
- CI checks shell scripts with `bash -n` and ShellCheck so backup tooling cannot break silently.

## Current limitations

- Rate limiting is in-memory, so limits are per application replica.
- Delivery queue is Postgres-backed; for very high throughput it can later move to Redis, NATS or RabbitMQ.
- Migrations are embedded in Go code.
- There is no web admin UI yet.

## Next production upgrades

- Redis-backed distributed rate limit for multi-replica deployments.
- Separate migration files or migration CLI.
- Web admin UI for sources, events and deliveries.
- Delivery provider abstraction for email, Slack, Discord or webhooks.
