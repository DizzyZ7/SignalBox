# Admin Audit Log

SignalBox includes an admin audit log foundation for enterprise operations and incident response.

The audit log is designed to answer questions such as:

```text
Who rotated a source token?
Who disabled a source?
Who retried a failed delivery?
Who replayed an event?
Who sent a source test event?
```

## Storage model

Audit events are stored in PostgreSQL in `admin_audit_events`.

Fields:

| Field | Description |
| --- | --- |
| `id` | Public audit event ID |
| `request_id` | Request correlation ID |
| `action` | Normalized admin action |
| `method` | HTTP method |
| `path` | Admin API path |
| `target_type` | Source, event, delivery or other resource type |
| `target_id` | Public target ID when available |
| `status_code` | HTTP response status code |
| `ip` | Admin client IP |
| `user_agent` | Admin client user agent |
| `metadata` | Small JSON metadata object |
| `created_at` | Audit event timestamp |

## Recorded actions

The audit middleware records mutating admin requests. `GET` requests are intentionally skipped to avoid noisy logs.

Examples:

```text
sources:create
sources:update
sources:delete
sources:rotate-token
sources:test-event
events:replay
deliveries:retry
templates:create
```

## API

List audit events:

```http
GET /v1/audit?limit=50&offset=0
X-API-Key: <ADMIN_API_KEY>
```

Filter by action:

```http
GET /v1/audit?action=sources:rotate-token
X-API-Key: <ADMIN_API_KEY>
```

Filter by target:

```http
GET /v1/audit?target_type=source&target_id=<SOURCE_ID>
X-API-Key: <ADMIN_API_KEY>
```

Response:

```json
{
  "items": [
    {
      "id": "audit-public-id",
      "request_id": "request-id",
      "action": "sources:rotate-token",
      "method": "POST",
      "path": "/v1/sources/source-id/rotate-token",
      "target_type": "source",
      "target_id": "source-id",
      "status_code": 200,
      "ip": "203.0.113.10",
      "user_agent": "curl/8.0",
      "metadata": {},
      "created_at": "2026-06-12T00:00:00Z"
    }
  ],
  "limit": 50,
  "offset": 0
}
```

## Security notes

The audit log must not store secrets.

Current audit metadata intentionally excludes request bodies to avoid storing:

- source tokens;
- admin API keys;
- HMAC keys;
- webhook payload secrets;
- customer PII.

Only path, target, status, request ID, IP and user agent are stored.

## Operational usage

During incident response:

1. Find the affected source/event/delivery.
2. Filter audit events by `target_type` and `target_id`.
3. Check whether a token was rotated, a source was disabled, or a delivery was manually retried.
4. Correlate `request_id` with JSON logs and reverse-proxy logs.

## Implementation status

The domain, storage and HTTP handler are implemented. The final activation step is wiring the route and middleware in `internal/httpapi/server.go`:

```text
GET /v1/audit
record mutating admin requests after successful/failed handler execution
```
