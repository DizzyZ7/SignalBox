# Deployment

## VPS deployment

1. Install Docker and Docker Compose plugin.
2. Clone repository.
3. Create `.env` from `.env.example`.
4. Set a long random `ADMIN_API_KEY`.
5. Start services:

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

## Backups

PostgreSQL volume is named `signalbox_pgdata`. For production, use external backups or managed PostgreSQL.

## Scaling

Multiple API replicas can run behind a load balancer. Deduplication is safe because PostgreSQL enforces the `(source_id, payload_hash)` primary key in `event_dedup_keys`.

Telegram notifications are fire-and-forget in the API process. For guaranteed delivery, move delivery to a background worker with retries.
