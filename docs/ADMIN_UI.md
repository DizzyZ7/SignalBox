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
- Telegram chat id configuration
- Telegram message template configuration
- HTTP forward URL configuration
- HTTP forward HMAC key configuration
- recent event list
- event replay
- delivery list
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
