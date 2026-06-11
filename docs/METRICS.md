# SignalBox Metrics

SignalBox exposes Prometheus-compatible metrics at:

```text
GET /metrics
```

The endpoint uses the Prometheus text exposition format and does not require an additional SDK or sidecar.

## Security

`/metrics` is intentionally not protected by `X-API-Key`, because Prometheus usually scrapes it inside a private network, Kubernetes cluster, VPN or reverse proxy allowlist.

For public deployments, do not expose `/metrics` to the internet without an additional reverse-proxy rule.

Recommended options:

- allow only Prometheus IP;
- expose only inside private Docker/Kubernetes network;
- put `/metrics` behind VPN;
- add reverse-proxy basic auth if needed.

## Metrics

### Process

```text
signalbox_build_info
signalbox_uptime_seconds
```

### HTTP

```text
signalbox_http_requests_total{method,path,status}
signalbox_http_request_duration_seconds_sum{method,path,status}
signalbox_http_request_duration_seconds_count{method,path,status}
```

### Webhooks

```text
signalbox_webhook_events_total{source,type,duplicate}
```

### Stored events

```text
signalbox_events_total
signalbox_events_unique_total
signalbox_events_duplicate_total
signalbox_events_24h_total
```

### Sources

```text
signalbox_sources_total
signalbox_sources_active_total
```

### Delivery queue

```text
signalbox_delivery_jobs_by_status{status="pending"}
signalbox_delivery_jobs_by_status{status="processing"}
signalbox_delivery_jobs_by_status{status="sent"}
signalbox_delivery_jobs_by_status{status="failed"}
```

## Example Prometheus scrape config

```yaml
scrape_configs:
  - job_name: signalbox
    metrics_path: /metrics
    static_configs:
      - targets:
          - signalbox.example.com
```

If SignalBox is behind HTTPS:

```yaml
scrape_configs:
  - job_name: signalbox
    scheme: https
    metrics_path: /metrics
    static_configs:
      - targets:
          - signalbox.example.com
```

## Useful alert ideas

Failed delivery jobs:

```promql
signalbox_delivery_jobs_by_status{status="failed"} > 0
```

No successful webhook traffic for a source-dependent deployment:

```promql
increase(signalbox_webhook_events_total[15m]) == 0
```

High duplicate ratio:

```promql
signalbox_events_duplicate_total / clamp_min(signalbox_events_total, 1) > 0.5
```

HTTP 5xx responses:

```promql
increase(signalbox_http_requests_total{status=~"5.."}[5m]) > 0
```
