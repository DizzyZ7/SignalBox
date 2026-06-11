# HTTP Forwarding

SignalBox can forward accepted unique webhook events to an external HTTP endpoint through the durable `delivery_jobs` queue.

```text
incoming webhook -> dedupe -> PostgreSQL event -> delivery_jobs -> HTTP POST -> retry/backoff
```

This makes SignalBox usable not only as a Telegram notification relay, but also as a classic webhook gateway.

## Source settings

When creating or updating a source, configure:

```json
{
  "name": "GitHub events",
  "forward_url": "https://example.com/webhooks/signalbox",
  "forward_hmac_key": "your-shared-key"
}
```

`forward_url` is optional. If it is not set, no HTTP forwarding job is created.

`forward_hmac_key` is optional. If it is set, SignalBox signs outgoing HTTP forwarding requests.

The key is not returned by the API. Responses only expose whether it is configured:

```json
{
  "forward_hmac_key_set": true
}
```

## Delivery behavior

HTTP forwarding uses the same durable delivery queue as Telegram delivery.

- each unique event creates an `http` delivery job when `forward_url` is configured;
- duplicate events are stored but do not enqueue a new forwarding job;
- failed requests are retried with exponential backoff;
- `Retry-After` is respected when the target endpoint returns it;
- jobs become terminal `failed` after `DELIVERY_MAX_ATTEMPTS`.

## Outgoing request

SignalBox sends the original JSON payload to the target endpoint.

Additional headers:

```text
X-SignalBox-Event-ID
X-SignalBox-Delivery-ID
X-SignalBox-Source-ID
X-SignalBox-Event-Type
X-SignalBox-Timestamp
X-SignalBox-Signature
```

`X-SignalBox-Signature` is included only when `forward_hmac_key` is configured.

## Signature format

SignalBox signs:

```text
<timestamp>.<raw_json_body>
```

The signature uses HMAC-SHA256 and is sent as:

```text
X-SignalBox-Signature: sha256=<hex_digest>
```

Recommended receiver checks:

- verify the signature;
- reject old timestamps, for example older than 5 minutes;
- make receiver idempotent using `X-SignalBox-Event-ID`;
- return `2xx` only after successful processing;
- return `429` with `Retry-After` when overloaded.

## Admin UI

The embedded Admin UI supports creating a source with:

- Telegram chat id;
- HTTP forward URL;
- HTTP forward HMAC key.

Open:

```text
/admin
```

## Delivery inspection

List HTTP delivery jobs:

```bash
curl "https://signalbox.example.com/v1/deliveries?channel=http" \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

Retry one failed HTTP delivery:

```bash
curl -X POST https://signalbox.example.com/v1/deliveries/<DELIVERY_ID>/retry \
  -H "X-API-Key: <ADMIN_API_KEY>"
```
