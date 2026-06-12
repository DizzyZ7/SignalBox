# SignalBox Enterprise Readiness Audit

This document tracks SignalBox as a product, not just as a portfolio project. It is intended to prevent duplicate feature work and keep future changes aligned with production value.

## Current product position

SignalBox is a compact self-hosted webhook gateway:

```text
external webhook -> public source endpoint -> PostgreSQL event log -> deduplication -> delivery queue -> Telegram / HTTP forwarding -> Admin UI / metrics
```

The current implementation is already beyond a demo:

- public webhook endpoint per source;
- admin API protected by `X-API-Key`;
- source tokens stored as SHA-256 hashes only;
- source create/list/update/disable/rotate-token;
- event storage in PostgreSQL;
- duplicate detection with duplicate audit records preserved;
- cursor-based event pagination;
- manual event replay;
- durable `delivery_jobs` queue;
- Telegram delivery through the queue;
- HTTP forwarding through the queue;
- HMAC-SHA256 signature for HTTP forwarding;
- retry/backoff delivery processing;
- delivery list/get/retry endpoints;
- delivery filters by status, channel, source public ID and event public ID;
- aggregate stats endpoint;
- Prometheus-compatible `/metrics`;
- embedded `/admin` UI without Node/npm;
- source create/edit in UI;
- Telegram templates and template preview;
- editable source test event payloads;
- backup/restore scripts;
- production Docker Compose setup;
- README/API/Architecture/Runbook/Admin UI/HTTP forwarding docs;
- Dependabot, CodeQL, Trivy and ShellCheck automation;
- security headers;
- admin/webhook rate limiting;
- webhook token redaction in access logs.

## Duplication rules

Before adding a new feature, check whether it belongs to an existing layer:

| Feature area | Existing home |
| --- | --- |
| HTTP routes and shared middleware | `internal/httpapi/server.go` |
| Event cursor pagination | `internal/httpapi/events_cursor.go`, `internal/storage/events_cursor.go` |
| Event replay | `internal/httpapi/events_replay.go` |
| Source test events | `internal/httpapi/sources_test_event.go` |
| Delivery admin endpoints | `internal/httpapi/deliveries.go`, `internal/storage/delivery_admin.go` |
| Queue operations | `internal/storage/delivery_jobs.go` |
| Telegram and HTTP delivery worker | `internal/delivery/telegram.go` |
| Telegram template preview | `internal/httpapi/template_preview.go` |
| Admin UI | `internal/httpapi/admin/*` |
| Metrics wrapper | `internal/httpapi/metrics.go` and `internal/metrics/*` |
| Security helpers | `internal/security/*` |
| Product docs | `README.md`, `docs/*.md`, `docs/openapi.yaml` |

Do not create parallel implementations for these areas. Extend the existing layer instead.

## Security baseline

Current strong points:

- source tokens are never stored in plaintext;
- source token is returned only on create/rotate;
- public webhook tokens are redacted from access logs;
- admin endpoints require `X-API-Key`;
- admin and webhook endpoints are rate-limited;
- HTTP forwarding supports HMAC signatures;
- HTTP forwarding now has a delivery-time SSRF guard for private/local targets;
- security headers are enabled;
- backups and restore require explicit operator action.

Open hardening items:

1. Reject unsafe `forward_url` during source create/update, not only during delivery.
2. Add an explicit forward allowlist model for private self-hosted topologies.
3. Add admin audit log for create/update/disable/rotate/replay/retry/test-event actions.
4. Add API key rotation or multiple admin keys.
5. Add optional reverse-proxy auth documentation for `/admin` and `/metrics`.
6. Add OpenAPI contract tests or a simple docs drift check.

## Product gaps before v1.0.0

Priority order:

1. Synchronize `docs/openapi.yaml` with all current endpoints and schemas.
2. Add source detail view in Admin UI: source info, last events, last deliveries and health.
3. Add audit log for admin actions.
4. Add smoke tests for admin endpoints.
5. Add release notes for `v1.0.0`.
6. Add SBOM/provenance/signing for GHCR image.
7. Add Slack/Discord delivery providers after the queue/provider interface is split cleanly.
8. Add hosted-mode security controls only after self-hosted security defaults are stable.

## Enterprise positioning

SignalBox should be positioned as:

```text
A lightweight self-hosted webhook gateway with durable storage, deduplication, replay, delivery queue, Telegram/HTTP forwarding, metrics and an embedded operations console.
```

Avoid positioning it as a generic queue, generic ETL platform or Zapier clone. The product is strongest when it solves webhook reliability and operational visibility with minimal infrastructure.

## Next engineering recommendation

The next high-value layer is an admin audit log:

```text
admin action -> actor/request metadata -> target type/id -> action payload summary -> immutable audit_events table -> UI/API view
```

This improves enterprise trust, incident response and compliance without changing the public webhook flow.
