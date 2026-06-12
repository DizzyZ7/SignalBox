# SignalBox Admin UI

SignalBox includes a lightweight embedded admin interface served by the Go API.

```text
/admin
```

The UI is embedded into the Go binary with `embed`, so it does not require Node.js, npm, a separate frontend build, CDN assets or another container.

## Features

- API base URL configuration
- `X-API-Key` configuration
- dashboard stats
- source list
- source creation
- source editing
- source active/inactive switching
- source token rotation
- source test event sending with editable JSON payloads
- Telegram chat id configuration
- Telegram message template configuration
- Telegram template preview
- HTTP forward URL configuration
- HTTP forward HMAC key configuration
- recent event list
- event replay
- delivery list
- delivery filters by status, channel, source public ID and event public ID
- failed/pending delivery retry
- local activity log

## Security model

The Admin UI is only a browser interface over the existing Admin API.

- The UI does not bypass backend authorization.
- API calls still require `X-API-Key`.
- Admin API rate limiting still applies.
- No external JavaScript is loaded.
- API key is stored in browser local storage for convenience.
- Use the UI only over HTTPS in production.
- HTTP forward HMAC keys are write-only from the UI perspective: the API returns only whether the key is configured.
- During source editing, leaving the HMAC input blank keeps the current key unchanged.
- Test events are sent through the Admin API only; they do not use or expose public source tokens.

For stricter environments, put `/admin` behind additional reverse-proxy authentication or VPN access.

## Production access

If SignalBox is deployed at:

```text
https://signalbox.example.com
```

open:

```text
https://signalbox.example.com/admin
```

Then enter:

```text
API base URL: https://signalbox.example.com
X-API-Key: <ADMIN_API_KEY>
```

## Creating a source

In the source form, fill:

```text
Source name: GitHub events
Telegram chat id: optional
Telegram template: optional Go text/template message
Forward URL: https://example.com/webhooks/signalbox
Forward HMAC key: optional shared key for outgoing signatures
```

When `Telegram template` is configured, every unique accepted event sent to Telegram uses that custom text.

When `Forward URL` is configured, every unique accepted event is queued for HTTP forwarding.

See [`TELEGRAM_TEMPLATES.md`](TELEGRAM_TEMPLATES.md) for template variables and examples.

## Editing a source

In the sources table, click `Edit`.

The editor supports:

```text
- name update
- active/inactive switch
- Telegram chat id update or clear
- Telegram template update or clear
- Telegram template preview
- HTTP forward URL update or clear
- HMAC key rotation by entering a new value
- source token rotation
- editable test event payloads
- test event sending
```

Important behavior:

```text
Blank Telegram chat id clears the chat override.
Blank Telegram template clears the custom template and returns to default Telegram text.
Blank Forward URL disables HTTP forwarding.
Blank HMAC key in edit mode keeps the current HMAC key unchanged.
```

Token rotation returns the new source token once in the activity log. Save it immediately.

## Sending a test event

In the source editor, edit the `Test event payload` JSON if needed, then click `Send test event`.

The UI calls:

```text
POST /v1/sources/{id}/test-event
```

Request body:

```json
{
  "payload": {
    "type": "signalbox.test",
    "source": "admin-ui",
    "external_id": "test-1760000000000",
    "message": "Test event from SignalBox Admin UI"
  }
}
```

The payload must be a JSON object. The UI rejects invalid JSON before sending the request.

The default editor payload uses:

```json
"external_id": "AUTO"
```

When sending, the UI replaces `AUTO` with a fresh `test-<timestamp>` value. This avoids accidental deduplication when repeatedly testing the same source.

Click `Reset payload` to restore the default sample payload for the currently selected source.

The backend creates a real test event for the selected source and runs it through the same storage, deduplication and delivery queue path as a normal incoming webhook.

After a successful test, the UI refreshes:

```text
- Events
- Deliveries
- Stats
```

This is useful for checking Telegram delivery, HTTP forwarding and templates without using curl or exposing the source token.

## Filtering deliveries

The delivery panel supports filtering by:

```text
- status: failed, pending, processing, sent or all
- channel: telegram, http or any custom channel
- source public ID
- event public ID
```

Press `Enter` inside text filters to reload the list. Status changes reload automatically.

These filters are intended for production incident response. For example, when a customer says one integration stopped receiving events, filter deliveries by the source public ID and failed status, then retry only the affected jobs.

## Recommended reverse proxy rule

For production, expose `/admin` only through HTTPS. For higher security, restrict it by IP, VPN, basic auth or SSO at the reverse proxy level.

Example Nginx idea:

```nginx
location /admin/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

The API endpoints remain under `/v1/...` and continue to require `X-API-Key`.
