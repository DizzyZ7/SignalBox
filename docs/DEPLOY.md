# Deployment

## VPS deployment

1. Install Docker and Docker Compose plugin.
2. Clone repository.
3. Create `.env` from `.env.example`.
4. Set a long random `ADMIN_API_KEY`.
5. Review webhook rate limit and delivery worker settings.
6. Start services:

```bash
docker compose --env-file .env up -d --build
```

## Reverse proxy

Run SignalBox behind HTTPS. Recommended external path:

```text
https://signalbox.example.com
```

Minimum proxy rules:

- forward `X-Forwarded-For`
- forward `X-Real-IP`
- limit request body close to `MAX_BODY_BYTES`
- enable gzip only for responses, not request bodies

## Webhook rate limiting

Public webhook requests are limited by client IP and source token.

Default local values:

```env
WEBHOOK_RATE_LIMIT_REQUESTS=120
WEBHOOK_RATE_LIMIT_WINDOW=1m
```

Set `WEBHOOK_RATE_LIMIT_REQUESTS=0` only for trusted internal deployments.

For multi-replica deployments, the built-in limiter is per replica. Use sticky sessions or replace it with Redis-backed distributed rate limiting when strict global limits are required.

## Delivery worker

Telegram notifications are stored in `delivery_jobs` and processed by a background worker.

Default values:

```env
DELIVERY_WORKER_ENABLED=true
DELIVERY_WORKER_INTERVAL=5s
DELIVERY_WORKER_BATCH_SIZE=10
DELIVERY_WORKER_LOCK_DURATION=1m
DELIVERY_MAX_ATTEMPTS=8
```

If Telegram is down, jobs stay in Postgres and are retried with backoff until they are sent or reach `DELIVERY_MAX_ATTEMPTS`.

## Backups

PostgreSQL volume is named `signalbox_pgdata`. For production, use external backups or managed PostgreSQL. Backups now protect both events and pending delivery jobs.

## Scaling

Multiple API replicas can run behind a load balancer. Deduplication is safe because PostgreSQL enforces the `(source_id, payload_hash)` primary key in `event_dedup_keys`.

Delivery workers claim jobs with row locks and `FOR UPDATE SKIP LOCKED`, so several replicas can process the queue without taking the same job at the same time. For very high delivery throughput, replace the Postgres-backed queue with Redis, NATS or RabbitMQ.
