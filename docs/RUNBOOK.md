# SignalBox Production Runbook

This runbook describes how to deploy, operate, back up and recover SignalBox in a small production or beta-production environment.

## Status

SignalBox is suitable for beta-production usage when deployed behind HTTPS with secure secrets and PostgreSQL backups.

## Minimum production requirements

- Linux VPS or container host
- Docker Engine and Docker Compose plugin
- Domain name pointed to the server
- HTTPS reverse proxy such as Nginx, Caddy or Traefik
- Persistent PostgreSQL volume or managed PostgreSQL
- Regular PostgreSQL backups
- Long random `ADMIN_API_KEY`
- Private source webhook tokens

## First deploy from GHCR image

Create the deployment directory:

```bash
mkdir -p /opt/signalbox
cd /opt/signalbox
```

Download deployment files:

```bash
curl -fsSLO https://raw.githubusercontent.com/DizzyZ7/SignalBox/main/docker-compose.prod.yml
curl -fsSLO https://raw.githubusercontent.com/DizzyZ7/SignalBox/main/.env.production.example
cp .env.production.example .env.production
```

Edit `.env.production`:

```bash
nano .env.production
```

Start services:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml up -d
```

Check containers:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml ps
```

Check API from the server:

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
```

## Reverse proxy checklist

Expose SignalBox through HTTPS only.

Minimum proxy requirements:

- terminate TLS;
- proxy to `http://127.0.0.1:8080`;
- forward `Host`;
- forward `X-Real-IP`;
- forward `X-Forwarded-For`;
- forward `X-Forwarded-Proto`;
- limit request body size close to `MAX_BODY_BYTES`.

Example Nginx server block:

```nginx
server {
    listen 80;
    server_name signalbox.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name signalbox.example.com;

    client_max_body_size 1m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Smoke test

Create a source:

```bash
curl -X POST https://signalbox.example.com/v1/sources \
  -H "X-API-Key: $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"Production smoke test"}'
```

Send a webhook event with the returned source token:

```bash
curl -X POST https://signalbox.example.com/v1/hooks/<SOURCE_TOKEN> \
  -H "Content-Type: application/json" \
  -d '{"type":"smoke.test","source":"runbook","message":"hello"}'
```

List events:

```bash
curl https://signalbox.example.com/v1/events?limit=10 \
  -H "X-API-Key: $ADMIN_API_KEY"
```

Get stats:

```bash
curl https://signalbox.example.com/v1/stats \
  -H "X-API-Key: $ADMIN_API_KEY"
```

## Operations

View logs:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml logs -f api
```

Restart API:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml restart api
```

Pull a new image and restart:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml pull api
docker compose --env-file .env.production -f docker-compose.prod.yml up -d api
```

## Delivery troubleshooting

List failed Telegram deliveries:

```bash
curl "https://signalbox.example.com/v1/deliveries?status=failed&channel=telegram" \
  -H "X-API-Key: $ADMIN_API_KEY"
```

Retry one failed delivery job:

```bash
curl -X POST https://signalbox.example.com/v1/deliveries/<DELIVERY_ID>/retry \
  -H "X-API-Key: $ADMIN_API_KEY"
```

Replay an event into the delivery queue:

```bash
curl -X POST https://signalbox.example.com/v1/events/<EVENT_ID>/replay \
  -H "X-API-Key: $ADMIN_API_KEY"
```

Use delivery retry when a specific delivery job failed. Use event replay when you want to enqueue delivery again from the stored event.

## PostgreSQL backup

Create a backup:

```bash
mkdir -p backups
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" > backups/signalbox-$(date +%F-%H%M%S).sql
```

Verify that the backup file is not empty:

```bash
ls -lh backups/
```

Recommended backup policy for a beta deployment:

- daily backup;
- keep at least 7 daily backups;
- copy backups outside the VPS;
- test restore before relying on backups.

## PostgreSQL restore

Stop API to avoid writes during restore:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml stop api
```

Restore from backup:

```bash
cat backups/signalbox-YYYY-MM-DD-HHMMSS.sql | docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  psql -U "$POSTGRES_USER" "$POSTGRES_DB"
```

Start API again:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml start api
```

## Rollback

Set previous image version in `.env.production`:

```env
SIGNALBOX_VERSION=v0.9.1
```

Then redeploy:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml pull api
docker compose --env-file .env.production -f docker-compose.prod.yml up -d api
```

Rollback is safest when migrations are backward-compatible.

## Release v1.0.0

From local repository:

```bash
git pull origin main
git tag v1.0.0
git push origin v1.0.0
```

GitHub Actions will publish:

```text
ghcr.io/dizzyz7/signalbox:v1.0.0
ghcr.io/dizzyz7/signalbox:latest
```

After the Docker workflow is green, update production `.env.production`:

```env
SIGNALBOX_VERSION=v1.0.0
```

Then pull and restart API.

## Incident checklist

If webhooks are not accepted:

- check `/readyz`;
- check PostgreSQL container health;
- check API logs;
- check reverse proxy body size;
- check source token;
- check rate limit response and `Retry-After`.

If Telegram messages are not delivered:

- check `TELEGRAM_BOT_TOKEN`;
- check `TELEGRAM_DEFAULT_CHAT_ID` or source-specific `telegram_chat_id`;
- check failed deliveries;
- retry failed delivery;
- replay event if needed.

If database is unavailable:

- stop public traffic if needed;
- check disk space;
- check PostgreSQL logs;
- restore from backup if the volume is corrupted.
