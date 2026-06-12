# SignalBox Examples

This directory contains safe synthetic payloads for demos, local testing, screenshots and documentation.

The examples do not contain real customer data, real secrets or real service credentials.

## Payloads

- [`payloads/lead.created.json`](payloads/lead.created.json) — landing form lead.
- [`payloads/repository.push.json`](payloads/repository.push.json) — repository push notification.
- [`payloads/checkout.completed.json`](payloads/checkout.completed.json) — checkout/payment-style event.
- [`payloads/monitoring.alert.json`](payloads/monitoring.alert.json) — monitoring alert.

## Recommended use

Use these payloads with the Admin UI source test event feature:

1. Open the Admin UI.
2. Create or select a source.
3. Open the source test event area.
4. Paste one example payload.
5. Submit the test event.
6. Check Events, Deliveries and Source incident snapshot.

This path is admin-only and does not require exposing a public source token during demos.

## Demo ideas

Use these examples to show:

- event ingestion;
- deduplication;
- delivery queue behavior;
- Telegram notifications;
- HTTP forwarding;
- failed delivery retry;
- replay;
- admin audit log;
- incident diagnostics.

## Adding new examples

When adding a new example payload:

- keep it synthetic;
- use realistic field names;
- avoid private URLs;
- avoid real personal data;
- avoid real service credentials;
- keep the payload small enough for documentation and screenshots.
