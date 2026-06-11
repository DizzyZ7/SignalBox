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
