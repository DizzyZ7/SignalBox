# Getting Started

This guide gets SignalBox running locally in about five minutes.

SignalBox is a self-hosted webhook gateway for teams that want durable event storage, deduplication, retryable delivery, Telegram alerts, HTTP forwarding, metrics and an embedded Admin UI without operating a large event platform.

## Requirements

- Docker
- Docker Compose
- Git

## 1. Clone the repository

```bash
git clone https://github.com/DizzyZ7/SignalBox.git
cd SignalBox
```

## 2. Create local environment

```bash
cp .env.example .env
```

Edit `.env` and set a strong admin key:

```env
ADMIN_API_KEY=change-me-to-a-long-random-secret
```

Telegram is optional. HTTP forwarding works without Telegram.

## 3. Start SignalBox

```bash
docker compose --env-file .env up --build
```

Check health:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

Open the Admin UI:

```text
http://localhost:8080/admin
```

In the UI set:

```text
API base URL: http://localhost:8080
X-API-Key: value from ADMIN_API_KEY
```

## 4. Create a source

Using the Admin UI, create a source named `Local test`.

Or use curl:

```bash
curl -X POST http://localhost:8080/v1/sources \
  -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"name":"Local test"}'
```

Save the returned `token`. It is shown only once.

## 5. Send an event

```bash
curl -X POST http://localhost:8080/v1/hooks/<SOURCE_TOKEN> \
  -H "Content-Type: application/json" \
  -d '{"type":"lead.created","source":"local","email":"demo@example.com"}'
```

Then open `/admin` and check:

- Events
- Deliveries
- Admin audit log
- Source incident snapshot

## 6. Test HTTP forwarding

Create or edit a source with a forward URL:

```text
https://example.com/webhooks/signalbox
```

SignalBox queues delivery jobs and sends events with headers:

```text
X-SignalBox-Event-ID
X-SignalBox-Delivery-ID
X-SignalBox-Timestamp
X-SignalBox-Source-ID
X-SignalBox-Event-Type
X-SignalBox-Signature
```

Private and local network destinations are blocked by the SSRF guard.

## 7. Test replay and retries

Replay an event:

```bash
curl -X POST http://localhost:8080/v1/events/<EVENT_ID>/replay \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

Retry a failed delivery:

```bash
curl -X POST http://localhost:8080/v1/deliveries/<DELIVERY_ID>/retry \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

## 8. Check metrics

```bash
curl http://localhost:8080/metrics
```

Use this endpoint with Prometheus-compatible scrapers.

## Next steps

- Read [`SELF_HOSTING.md`](SELF_HOSTING.md) for production deployment.
- Read [`HTTP_FORWARDING.md`](HTTP_FORWARDING.md) for outgoing webhook signatures.
- Read [`ADMIN_UI.md`](ADMIN_UI.md) for the embedded operations console.
- Read [`RUNBOOK.md`](RUNBOOK.md) for operations and incident response.
