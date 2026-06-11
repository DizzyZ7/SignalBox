# SignalBox API

Base URL for local development:

```text
http://localhost:8080
```

Admin endpoints require this header:

```http
X-API-Key: <ADMIN_API_KEY>
```

OpenAPI specification is available in [`docs/openapi.yaml`](openapi.yaml).

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
  "telegram_chat_id": "123456789",
  "forward_url": "https://example.com/webhooks/signalbox",
  "forward_hmac_key": "your-shared-key"
}
```

Optional fields:

- `telegram_chat_id`: overrides `TELEGRAM_DEFAULT_CHAT_ID` for this source;
- `forward_url`: external HTTP endpoint for queued forwarding;
- `forward_hmac_key`: enables HMAC-SHA256 signatures for HTTP forwarding.

Response contains `token` only once. Store it immediately. `forward_hmac_key` is never returned.

```json
{
  "id": "source-public-id",
  "name": "Main landing",
  "token_hint": "abcd...wxyz",
  "telegram_chat_id": "123456789",
  "forward_url": "https://example.com/webhooks/signalbox",
  "forward_hmac_key_set": true,
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
  "forward_url": "https://example.com/new-webhook",
  "forward_hmac_key": "new-shared-key",
  "is_active": true
}
```

Set `telegram_chat_id`, `forward_url` or `forward_hmac_key` to an empty string to clear that field.

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

For unique events, SignalBox enqueues delivery jobs for every configured destination: Telegram and/or HTTP forwarding.

## HTTP forwarding

If `forward_url` is configured on the source, SignalBox forwards the original JSON payload to that URL with retry/backoff.

Outgoing headers include:

```http
X-SignalBox-Event-ID: <event_public_id>
X-SignalBox-Delivery-ID: <delivery_public_id>
X-SignalBox-Source-ID: <source_public_id>
X-SignalBox-Event-Type: <event_type>
X-SignalBox-Timestamp: <unix_timestamp>
X-SignalBox-Signature: sha256=<hex_digest>
```

`X-SignalBox-Signature` is present only when `forward_hmac_key` is configured.

## List events

```http
GET /v1/events?limit=50&source=<SOURCE_ID>&type=lead.created&origin=landing&duplicate=false&from=2026-06-09T00:00:00Z&to=2026-06-10T00:00:00Z
X-API-Key: <ADMIN_API_KEY>
```

Use the returned `next_cursor` for the next page:

```http
GET /v1/events?limit=50&cursor=<NEXT_CURSOR>
X-API-Key: <ADMIN_API_KEY>
```

Query params:

| Param | Type | Description |
| --- | --- | --- |
| `limit` | integer | Page size, capped at 200 |
| `cursor` | string | Cursor returned as `next_cursor` by previous response |
| `offset` | integer | Legacy offset pagination, ignored when cursor is present |
| `source` | string | Source public ID |
| `type` | string | Event type alias |
| `event_type` | string | Event type |
| `origin` | string | Origin/source field from payload |
| `duplicate` | boolean | Duplicate flag |
| `from` | RFC3339 | Created at lower bound |
| `to` | RFC3339 | Created at upper bound |

Response includes `next_cursor`:

```json
{
  "items": [],
  "limit": 50,
  "offset": 0,
  "next_cursor": "opaque-cursor"
}
```

## Get event

```http
GET /v1/events/<EVENT_ID>
X-API-Key: <ADMIN_API_KEY>
```

## Replay event

```http
POST /v1/events/<EVENT_ID>/replay
X-API-Key: <ADMIN_API_KEY>
```

Replays an existing event by putting it back into the delivery queue. This does not create a new event record and does not change deduplication state.

Replay checks that:

- the event exists;
- the source is still active;
- the notifier is configured;
- the notifier has a destination for the source.

Response:

```json
{
  "status": "queued",
  "event": {
    "id": "event-public-id"
  }
}
```

## List delivery jobs

```http
GET /v1/deliveries?status=failed&channel=http&limit=50&offset=0
X-API-Key: <ADMIN_API_KEY>
```

Query params:

| Param | Type | Description |
| --- | --- | --- |
| `limit` | integer | Page size, capped at 200 |
| `offset` | integer | Offset pagination |
| `status` | string | Optional `pending`, `processing`, `sent`, `failed` |
| `channel` | string | Optional delivery channel, for example `telegram` or `http` |

## Get delivery job

```http
GET /v1/deliveries/<DELIVERY_ID>
X-API-Key: <ADMIN_API_KEY>
```

## Retry delivery job

```http
POST /v1/deliveries/<DELIVERY_ID>/retry
X-API-Key: <ADMIN_API_KEY>
```

This returns a `failed` or already `pending` job back to `pending`, clears lock/error fields, and schedules it for immediate retry.

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
  "deliveries": {
    "pending": 3,
    "processing": 1,
    "sent": 120,
    "failed": 2
  },
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
- Keep delivery worker enabled when Telegram or HTTP forwarding is needed.
- Restrict `/admin` and `/metrics` when exposed publicly.
- Use external PostgreSQL backups.
- Set `AUTO_MIGRATE=false` when migrations are managed by deployment tooling.
