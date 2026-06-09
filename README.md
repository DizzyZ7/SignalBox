# SignalBox

Go webhook event relay service with PostgreSQL storage, source-based webhook URLs, duplicate detection, optional Telegram notifications, Docker setup, and CI.

## Quick start

```bash
cp .env.example .env
# edit ADMIN_API_KEY in .env
docker compose --env-file .env up --build
```

Create source:

```bash
curl -X POST http://localhost:8080/v1/sources \
  -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"name":"Main landing"}'
```

Send event:

```bash
curl -X POST http://localhost:8080/v1/hooks/<SOURCE_TOKEN> \
  -H "Content-Type: application/json" \
  -d '{"type":"lead.created","source":"landing","contact":"@user"}'
```

List events:

```bash
curl http://localhost:8080/v1/events -H "X-API-Key: <ADMIN_API_KEY>"
```

See `docs/API.md` for endpoint details.
