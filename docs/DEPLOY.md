# Deployment

## Recommended production deployment

For a public VPS/beta-production deployment, use the prebuilt GHCR image and `docker-compose.prod.yml`.

```bash
mkdir -p /opt/signalbox
cd /opt/signalbox
curl -fsSLO https://raw.githubusercontent.com/DizzyZ7/SignalBox/main/docker-compose.prod.yml
curl -fsSLO https://raw.githubusercontent.com/DizzyZ7/SignalBox/main/.env.production.example
cp .env.production.example .env.production
nano .env.production
docker compose --env-file .env.production -f docker-compose.prod.yml up -d
```

The production compose file binds the API to localhost by default:

```text
127.0.0.1:8080 -> api:8080
```

Expose it through an HTTPS reverse proxy.

For operating procedures, backups, restore, rollback and incident handling, see [`docs/RUNBOOK.md`](RUNBOOK.md).

## VPS deployment from source

1. Install Docker and Docker Compose plugin.
2. Clone repository.
3. Create `.env` from `.env.example`.
4. Set a long random `ADMIN_API_KEY`.
5. Review webhook rate limit and delivery worker settings.
6. Start services:

```bash
docker compose --env-file .env up -d --build
```

## GHCR image deployment

Released images are published to GitHub Container Registry:

```bash
docker pull ghcr.io/dizzyz7/signalbox:latest
```

Versioned releases use semver tags:

```bash
docker pull ghcr.io/dizzyz7/signalbox:v1.0.0
```

To publish a new image, create and push a version tag:

```bash
git tag v1.0.0
git push origin v1.0.0
```

The `Docker image` GitHub Actions workflow builds the image and pushes both the version tag and `latest` to GHCR.

## Reverse proxy

Run SignalBox behind HTTPS. Recommended external path:

```text
https://signalbox.example.com
```

Minimum proxy rules:

- forward `Host`;
- forward `X-Forwarded-For`;
- forward `X-Real-IP`;
- forward `X-Forwarded-Proto`;
- limit request body close to `MAX_BODY_BYTES`;
- enable gzip only for responses, not request bodies.

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

PostgreSQL volume is named `signalbox_pgdata`. For production, use external backups or managed PostgreSQL. Backups protect sources, events, deduplication records, delivery attempts and pending delivery jobs.

See [`docs/RUNBOOK.md`](RUNBOOK.md) for backup and restore commands.

## Scaling

Multiple API replicas can run behind a load balancer. Deduplication is safe because PostgreSQL enforces the `(source_id, payload_hash)` primary key in `event_dedup_keys`.

Delivery workers claim jobs with row locks and `FOR UPDATE SKIP LOCKED`, so several replicas can process the queue without taking the same job at the same time.

For very high delivery throughput, replace the Postgres-backed queue with Redis, NATS or RabbitMQ.
