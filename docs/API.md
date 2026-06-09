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
GET /v1/sources?active=true
X-API-Key: <ADMIN_API_KEY>
```

Query params:

| Param | Type | Description |
| --- | --- | --- |
| `active` | boolean | Optional active/inactive filter |

## Update source

```http
PATCH /v1/sources/<SOURCE_ID>
Content-Type: application/json
X-API-Key: <ADMIN_API_KEY>
```

Body supports partial update:

```json
{
  "name": "Main landing v2",
  "telegram_chat_id": "987654321",
  "is_active": true
}
```

## Disable source

```http
DELETE /v1/sources/<SOURCE_ID>
X-API-Key: <ADMIN_API_KEY>
```

This is a soft delete. The source becomes inactive, existing events stay in storage.

## Rotate source token

```http
POST /v1/sources/<SOURCE_ID>/rotate-token
X-API-Key: <ADMIN_API_KEY>
```

Returns a new `token` once. The old webhook token stops working immediately.

## Receive webhook

```http
POST /v1/hooks/<SOURCE_TOKEN>
Content-Type: application/json
```

Body must be a single non-empty JSON object.

The public webhook endpoint is rate-limited by client IP and source token. Defaults:

```text
WEBHOOK_RATE_LIMIT_REQUESTS=120
WEBHOOK_RATE_LIMIT_WINDOW=1m
```

Set `WEBHOOK_RATE_LIMIT_REQUESTS=0` to disable webhook rate limiting.

When the limit is exceeded, API returns:

```http
429 Too Many Requests
Retry-After: <seconds>
```

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
GET /v1/events?limit=50&offset=0&source=<SOURCE_ID>&type=lead.created&origin=landing&duplicate=false&from=2026-06-09T00:00:00Z&to=2026-06-10T00:00:00Z
X-API-Key: <ADMIN_API_KEY>
```

Query params:

| Param | Type | Description |
| --- | --- | --- |
| `limit` | integer | Page size, capped at 200 |
| `offset` | integer | Offset pagination |
| `source` | string | Source public ID |
| `type` | string | Event type alias |
| `event_type` | string | Event type |
| `origin` | string | Origin/source field from payload |
| `duplicate` | boolean | Duplicate flag |
| `from` | RFC3339 | Created at lower bound |
| `to` | RFC3339 | Created at upper bound |

## Get event

```http
GET /v1/events/<EVENT_ID>
X-API-Key: <ADMIN_API_KEY>
```

## Stats

```http
GET /v1/stats
X-API-Key: <ADMIN_API_KEY>
```

Response:

```json
{
  "total_events": 100,
  "unique_events": 91,
  "duplicate_events": 9,
  "events_24h": 12,
  "sources": 3,
  "active_sources": 2,
  "by_type": [{"key":"lead.created","count":80}],
  "by_origin": [{"key":"landing","count":70}]
}
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
- Keep webhook rate limits enabled for public deployments.
- Use external PostgreSQL backups.
- Set `AUTO_MIGRATE=false` when migrations are managed by deployment tooling.
