# SignalBox API

Base URL for local development:

```text
http://localhost:8080
```

Admin endpoints require this header:

```http
X-API-Key: <ADMIN_API_KEY>
```

## Health

```http
GET /healthz
```

Response:

```json
{"status":"ok"}
```

## Readiness

```http
GET /readyz
```

Checks database connectivity.

## Create source

```http
POST /v1/sources
Content-Type: application/json
X-API-Key: <ADMIN_API_KEY>
```

Body:

```json
{
  "name": "Main landing",
  "telegram_chat_id": "123456789"
}
```

`telegram_chat_id` is optional. If omitted, `TELEGRAM_DEFAULT_CHAT_ID` is used.

Response contains `token` only once. Store it immediately.

```json
{
  "id": "source-public-id",
  "name": "Main landing",
  "token_hint": "abcd...wxyz",
  "telegram_chat_id": "123456789",
  "is_active": true,
  "created_at": "2026-06-09T00:00:00Z",
  "updated_at": "2026-06-09T00:00:00Z",
  "token": "source-secret-token"
}
```

## List sources

```http
GET /v1/sources
X-API-Key: <ADMIN_API_KEY>
```

## Receive webhook

```http
POST /v1/hooks/<SOURCE_TOKEN>
Content-Type: application/json
```

Body must be a non-empty JSON object.

Example:

```json
{
  "type": "lead.created",
  "source": "landing",
  "external_id": "lead-1001",
  "contact": "@user",
  "message": "Need a website"
}
```

Response:

```json
{
  "id": "event-public-id",
  "event_type": "lead.created",
  "origin": "landing",
  "external_id": "lead-1001",
  "payload": {},
  "payload_hash": "sha256",
  "is_duplicate": false,
  "created_at": "2026-06-09T00:00:00Z"
}
```

## List events

```http
GET /v1/events?limit=50&offset=0
X-API-Key: <ADMIN_API_KEY>
```

`limit` is capped at 200.

## Get event

```http
GET /v1/events/<EVENT_ID>
X-API-Key: <ADMIN_API_KEY>
```

## Error response

```json
{
  "error": "message",
  "request_id": "request-id"
}
```

## Production checklist

- Use HTTPS before public exposure.
- Use a long random `ADMIN_API_KEY`.
- Keep source tokens private.
- Put the service behind a reverse proxy with body size limits.
- Use external PostgreSQL backups.
- Set `AUTO_MIGRATE=false` when migrations are managed by deployment tooling.
