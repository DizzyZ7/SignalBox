# Environment Variables

This page describes SignalBox runtime configuration.

Use `.env.example` for local development and `.env.production.example` for self-hosted production deployments.

## HTTP server

| Variable | Default/example | Description |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | Address used by the API server in local/binary mode. |
| `SIGNALBOX_HOST_PORT` | `8080` | Host port used by the production Compose file. |
| `SIGNALBOX_VERSION` | `v1.0.0` | Container image tag used by the production Compose file. |

## PostgreSQL

| Variable | Default/example | Description |
| --- | --- | --- |
| `POSTGRES_DB` | `signalbox` | PostgreSQL database name. |
| `POSTGRES_USER` | `signalbox` | PostgreSQL user. |
| `POSTGRES_PASSWORD` | placeholder | PostgreSQL password. Replace before production. |
| `DATABASE_URL` | local URL | PostgreSQL connection URL for local/binary mode. |
| `AUTO_MIGRATE` | `true` | Runs built-in schema setup on startup. |

## Admin API

| Variable | Default/example | Description |
| --- | --- | --- |
| `ADMIN_API_KEY` | placeholder | Required for Admin API and Admin UI calls. Use a long random value. |

## Telegram

| Variable | Default/example | Description |
| --- | --- | --- |
| `TELEGRAM_BOT_TOKEN` | empty | Optional Telegram bot token. Leave empty to disable Telegram delivery. |
| `TELEGRAM_DEFAULT_CHAT_ID` | empty | Optional default Telegram chat ID. Sources can override it. |

## Webhook ingestion

| Variable | Default/example | Description |
| --- | --- | --- |
| `MAX_BODY_BYTES` | `1048576` | Maximum accepted webhook body size in bytes. |
| `WEBHOOK_RATE_LIMIT_REQUESTS` | `120` | Public webhook rate-limit request count. |
| `WEBHOOK_RATE_LIMIT_WINDOW` | `1m` | Public webhook rate-limit window. |

## Delivery worker

| Variable | Default/example | Description |
| --- | --- | --- |
| `DELIVERY_WORKER_ENABLED` | `true` | Enables background delivery worker. |
| `DELIVERY_WORKER_INTERVAL` | `5s` | Delay between queue polling cycles. |
| `DELIVERY_WORKER_BATCH_SIZE` | `10` | Number of jobs claimed per worker cycle. |
| `DELIVERY_WORKER_LOCK_DURATION` | `1m` | Lock duration for claimed delivery jobs. |
| `DELIVERY_MAX_ATTEMPTS` | `8` | Maximum attempts for queued delivery jobs. |

## Production notes

Recommended production rules:

- use a long random admin key;
- use a long random PostgreSQL password;
- keep production env files out of git;
- put SignalBox behind HTTPS;
- keep PostgreSQL private;
- keep the Admin UI behind trusted access controls;
- rotate source tokens if they are exposed;
- keep backups outside the host.
