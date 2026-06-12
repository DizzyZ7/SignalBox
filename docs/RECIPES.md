# Recipes

These recipes show practical ways to use SignalBox.

They are intentionally simple and production-oriented. Each recipe can be tested locally with Docker Compose and then moved to a VPS or internal infrastructure.

## Recipe 1: Landing form -> Telegram

Use this when a landing page or form backend needs to notify a team chat.

### 1. Create a source

```bash
curl -X POST http://localhost:8080/v1/sources \
  -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"name":"Landing leads","telegram_chat_id":"<CHAT_ID>"}'
```

Save the returned source token.

### 2. Send a lead event

```bash
curl -X POST http://localhost:8080/v1/hooks/<SOURCE_TOKEN> \
  -H "Content-Type: application/json" \
  -d '{
    "type":"lead.created",
    "source":"landing",
    "name":"Alex",
    "email":"alex@example.com",
    "message":"Need a webhook gateway"
  }'
```

### 3. Operate

Use Admin UI:

```text
/admin -> Events
/admin -> Deliveries
/admin -> Source incident snapshot
```

## Recipe 2: GitHub webhook -> SignalBox -> Telegram

Use this for repository events, deployment notifications and team alerts.

### 1. Create a source

```bash
curl -X POST http://localhost:8080/v1/sources \
  -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"name":"GitHub repository","telegram_chat_id":"<CHAT_ID>"}'
```

### 2. Configure GitHub webhook

GitHub repository settings:

```text
Payload URL: https://signalbox.example.com/v1/hooks/<SOURCE_TOKEN>
Content type: application/json
Events: push, pull_request, workflow_run
```

### 3. Debug

If the message does not arrive:

- check Events;
- check Deliveries;
- filter Deliveries by source ID;
- use Source incident snapshot;
- copy diagnostics.

## Recipe 3: SignalBox -> external HTTP endpoint

Use this when SignalBox should store, dedupe and retry before forwarding events to another service.

### 1. Create a forwarding source

```bash
curl -X POST http://localhost:8080/v1/sources \
  -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "name":"CRM forwarding",
    "forward_url":"https://crm.example.com/webhooks/signalbox",
    "forward_hmac_key":"shared-secret"
  }'
```

### 2. Send event

```bash
curl -X POST http://localhost:8080/v1/hooks/<SOURCE_TOKEN> \
  -H "Content-Type: application/json" \
  -d '{"type":"customer.created","external_id":"crm-1001","email":"demo@example.com"}'
```

### 3. Verify signature on receiver

SignalBox sends:

```text
X-SignalBox-Event-ID
X-SignalBox-Delivery-ID
X-SignalBox-Timestamp
X-SignalBox-Source-ID
X-SignalBox-Event-Type
X-SignalBox-Signature
```

Signature format:

```text
sha256=<hex hmac sha256>
```

The signature is calculated over:

```text
timestamp + "." + raw request body
```

## Recipe 4: Replay a failed event

Use this after fixing a downstream receiver.

### 1. Find event

```bash
curl "http://localhost:8080/v1/events?source=<SOURCE_ID>&limit=20" \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

### 2. Replay

```bash
curl -X POST http://localhost:8080/v1/events/<EVENT_ID>/replay \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

### 3. Watch deliveries

```bash
curl "http://localhost:8080/v1/deliveries?event_id=<EVENT_ID>" \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

## Recipe 5: Retry failed deliveries

Use this when downstream was temporarily unavailable.

```bash
curl "http://localhost:8080/v1/deliveries?status=failed&source=<SOURCE_ID>" \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

Then retry one job:

```bash
curl -X POST http://localhost:8080/v1/deliveries/<DELIVERY_ID>/retry \
  -H "X-API-Key: <ADMIN_API_KEY>"
```

## Recipe 6: Admin-only test event

Use this to test a source without exposing the public source token.

```bash
curl -X POST http://localhost:8080/v1/sources/<SOURCE_ID>/test-event \
  -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"payload":{"type":"signalbox.test","source":"admin","external_id":"test-1"}}'
```

This creates a real event and sends it through the same delivery queue.

## Recipe 7: Incident response with Admin UI

Open:

```text
/admin
```

Then:

1. Select the source.
2. Click `Source events`.
3. Click `Source deliveries`.
4. Click `Source audit`.
5. Open `Source incident snapshot`.
6. Click `Copy diagnostics`.

Paste the copied diagnostics into an issue, support chat or incident channel.

## Recipe 8: Local development without Telegram

Telegram is optional.

Create a source without chat configuration and use only event storage:

```bash
curl -X POST http://localhost:8080/v1/sources \
  -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"name":"Local inbox"}'
```

Then send events and inspect them in Admin UI.

## Safety notes

- Source tokens are shown only once.
- Source tokens are stored as hashes.
- Admin API requires `X-API-Key`.
- HTTP forwarding blocks localhost and private network targets by default.
- HMAC keys are write-only from API/UI perspective.
- Do not paste secrets into issue reports.
