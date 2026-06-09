# SignalBox

Go webhook event relay service with PostgreSQL storage, source-based webhook URLs, duplicate detection, optional Telegram notifications, Docker setup, and CI.

## Quick start

```bash
cp .env.example .env
docker compose --env-file .env up --build
```

See `docs/API.md` for API examples.
