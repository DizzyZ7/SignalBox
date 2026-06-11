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
  source_forwarding.go source HTTP forwarding migration

internal/delivery
  telegram.go          Telegram and HTTP delivery producer/worker

internal/httpapi
  server.go            HTTP routing, handlers, middleware
  admin_ui.go          embedded Admin UI handler
  metrics.go           Prometheus-compatible metrics wrapper
  events_cursor.go     cursor pagination handler
  events_replay.go     event replay handler
  deliveries.go        delivery job admin handlers

internal/metrics
  metrics.go           lightweight Prometheus text exposition registry

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
  -> internal/delivery enqueue delivery_jobs rows
  -> background worker claims delivery_jobs rows
  -> Telegram API and/or HTTP forward URL
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

## HTTP forwarding flow

```text
unique event
  -> source.forward_url exists
  -> delivery_jobs(channel=http, destination=forward_url)
  -> worker sends original JSON payload
  -> optional HMAC-SHA256 signature headers
  -> retry/backoff on non-2xx responses
```

HTTP forwarding reuses the same persistent queue as Telegram delivery. The target endpoint can verify `X-SignalBox-Signature` when `forward_hmac_key` is configured.

## Observability flow

```text
Prometheus
  -> GET /metrics
  -> HTTP in-memory counters
  -> PostgreSQL stats snapshot
  -> delivery queue status gauges
```

The metrics layer wraps the HTTP handler without changing business handlers. Queue and event gauges are read from PostgreSQL so process restarts do not erase core operational state.

## Design decisions

- `cmd/api/main.go` only wires dependencies, starts the HTTP server and starts the delivery worker.
- Business-facing models live in `internal/domain`.
- PostgreSQL-specific logic is isolated inside `internal/storage`.
- Telegram and HTTP forwarding use the same durable delivery queue.
- HTTP handlers do validation and translate storage errors into API responses.
- Public webhook requests are rate-limited by client IP and source token.
- Admin API requests are separately rate-limited before API key validation.
- Source tokens are never stored in plain text, only SHA-256 hashes.
- Source token is returned only on source creation and token rotation.
- Webhook source tokens are redacted from access logs.
- Event replay is admin-only and validates notifier readiness before queueing.
- Admin UI is embedded into the Go binary and does not require a separate frontend service.
- Prometheus metrics are exposed in text format without adding a heavy metrics SDK.

## Delivery reliability

Delivery uses the `delivery_jobs` table:

- new unique events enqueue Telegram and/or HTTP jobs depending on source settings;
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

## Horizontal scaling readiness

- Delivery workers use `FOR UPDATE SKIP LOCKED`, so multiple instances can claim jobs safely.
- Event deduplication uses the `(source_id, payload_hash)` primary key in PostgreSQL.
- HTTP metrics are per-process, while stored event and delivery gauges are read from PostgreSQL.
- For strict global rate limits in multi-replica deployments, replace the in-memory limiter with Redis-backed limits.

## Current limitations

- Rate limiting is in-memory, so limits are per application replica.
- Delivery queue is Postgres-backed; for very high throughput it can later move to Redis, NATS or RabbitMQ.
- Migrations are embedded in Go code.
- SQLite/WAL storage backend is not implemented yet.

## Next production upgrades

- Redis-backed distributed rate limit for multi-replica deployments.
- Separate migration files or migration CLI.
- Telegram message templates per source.
- Additional delivery providers for email, Slack or Discord.
- Optional SQLite/WAL storage backend for single-binary deployments.
