# Roadmap

SignalBox is built as an open-source, self-hosted webhook and event gateway with a path toward managed cloud and enterprise support.

The roadmap is intentionally practical: reliability first, then integrations, then hosted product.

## Current focus

SignalBox currently focuses on:

- receiving public webhooks per source;
- source token hashing and rotation;
- PostgreSQL event storage;
- deduplication with duplicate audit records;
- delivery queue with retry/backoff;
- Telegram alerts;
- HTTP forwarding with HMAC signatures;
- SSRF protection for outgoing HTTP forwarding;
- Admin API;
- embedded Admin UI;
- delivery inspection and retry;
- admin audit log;
- source incident diagnostics;
- Prometheus-compatible metrics;
- Docker and Compose deployment.

## Near-term: v1.x

### Open-source launch readiness

- tighten README positioning;
- add screenshots and GIF demos;
- improve quickstart and self-hosting docs;
- document common webhook recipes;
- add comparison docs;
- add contribution guidelines;
- keep CI/security workflows green;
- add more tests around delivery and audit flows.

### Delivery provider architecture

- extract HTTP forwarding into a provider implementation;
- extract Telegram into a provider implementation;
- add provider registry;
- keep `delivery_jobs.channel` as the stable dispatch key;
- add provider-specific tests;
- document provider authoring rules.

### Admin UI and operations

- source health summary;
- source incident snapshot;
- copyable diagnostics;
- better delivery error views;
- audit filters and source drill-down;
- UI screenshots for docs;
- optional read-only/demo mode for public playground.

## Mid-term: integrations

Potential providers:

- Slack;
- Discord;
- email;
- generic webhook transformations;
- custom headers per source;
- provider-specific templates.

Potential source presets:

- GitHub;
- GitLab;
- Stripe;
- payment providers;
- landing forms;
- monitoring alerts;
- CRM events.

## Product direction

Open-source core:

```text
self-hosted SignalBox server
PostgreSQL storage
Admin UI
Telegram/HTTP delivery
metrics/audit/diagnostics
```

Commercial options later:

```text
managed SignalBox Cloud
paid support
enterprise on-prem package
private integration work
SLA and SSO features
```

## Non-goals

SignalBox should not become a heavy event streaming platform.

It should stay:

- easy to self-host;
- Go-native;
- PostgreSQL-first;
- understandable by small teams;
- operationally useful without Redis, Kafka or a separate frontend stack.

## Launch signal checklist

Before a serious public launch, the repository should have:

- clear README hero and positioning;
- five-minute quickstart;
- screenshots;
- Docker image and Compose path;
- public roadmap;
- comparison page;
- contributing guide;
- security policy;
- first release notes;
- good GitHub topics;
- several practical recipes.
