# Self-Hosting SignalBox

SignalBox is designed to be self-hosted with one Go API container and PostgreSQL.

The recommended production setup is:

```text
Internet -> HTTPS reverse proxy -> SignalBox API -> PostgreSQL
```

## Deployment options

### Simple VPS

Good for early production, agencies, internal tools and demos.

Use:

- VPS from Hetzner, DigitalOcean, Scaleway, AWS Lightsail or similar;
- Docker Compose;
- Caddy or Nginx for HTTPS;
- PostgreSQL volume backups;
- firewall allowing only ports 80, 443 and SSH.

### Private infrastructure

Good for teams that already run services internally.

Use:

- internal PostgreSQL;
- container registry image from GHCR;
- reverse proxy or ingress;
- Prometheus-compatible metrics scraping;
- centralized logs.

### Kubernetes

Possible, but not required for the core product.

Use Kubernetes only if the team already has it.

## Production compose

Clone the repository:

```bash
git clone https://github.com/DizzyZ7/SignalBox.git
cd SignalBox
```

Create production env:

```bash
cp .env.production.example .env.production
```

Edit required values:

```env
ADMIN_API_KEY=long-random-admin-key
POSTGRES_PASSWORD=long-random-postgres-password
```

Start:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml up -d
```

Check:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml ps
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
```

## Reverse proxy

Run SignalBox behind HTTPS.

Caddy example:

```caddyfile
signalbox.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

Nginx example:

```nginx
server {
    listen 443 ssl http2;
    server_name signalbox.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Admin UI access

Open:

```text
https://signalbox.example.com/admin
```

Use:

```text
API base URL: https://signalbox.example.com
X-API-Key: ADMIN_API_KEY
```

For stronger security, restrict `/admin` by:

- VPN;
- IP allowlist;
- basic auth;
- SSO in reverse proxy;
- private network access.

The backend still requires `X-API-Key` for admin API calls.

## Backups

Create backup:

```bash
./scripts/backup-postgres.sh
```

Restore requires explicit confirmation:

```bash
CONFIRM_RESTORE=YES ./scripts/restore-postgres.sh backups/signalbox-YYYYMMDDTHHMMSSZ.sql.gz
```

Store backups outside the server as well.

Recommended backup policy:

```text
daily local backup
weekly off-server backup
monthly restore test
```

## Monitoring

Health endpoints:

```text
/healthz
/readyz
```

Metrics endpoint:

```text
/metrics
```

Alert on:

- API down;
- readiness failing;
- PostgreSQL unavailable;
- delivery failures increasing;
- pending deliveries growing;
- disk usage;
- backup failures.

## Security checklist

Before exposing SignalBox publicly:

- set a long random `ADMIN_API_KEY`;
- use HTTPS;
- keep `/admin` protected;
- keep PostgreSQL private;
- do not expose Docker socket;
- enable firewall;
- monitor logs;
- configure backups;
- rotate source tokens if leaked;
- keep images updated.

## Upgrade flow

Pull new image or code:

```bash
git pull
docker compose --env-file .env.production -f docker-compose.prod.yml pull
docker compose --env-file .env.production -f docker-compose.prod.yml up -d
```

Then check:

```bash
curl https://signalbox.example.com/healthz
curl https://signalbox.example.com/readyz
```

## Troubleshooting

Check logs:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml logs -f api
```

Check database:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml logs -f postgres
```

Check failed deliveries in Admin UI:

```text
/admin -> Deliveries -> status failed
```

Use `Source incident snapshot` to copy diagnostics for a specific source.
