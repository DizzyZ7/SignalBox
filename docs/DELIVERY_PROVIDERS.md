# Delivery Providers

SignalBox currently delivers outbound notifications through the durable `delivery_jobs` table.

Implemented channels:

```text
telegram
http
```

The first production implementation keeps the queue worker in `internal/delivery/telegram.go`, but the product direction is a provider-based architecture.

## Target contract

The provider contract lives in:

```text
internal/delivery/provider.go
```

It defines:

```text
Provider
ProviderJob
ProviderResult
```

The intended model is:

```text
accepted event -> provider.CanDeliver(source) -> provider.BuildJobPayload(event, source) -> delivery_jobs -> provider.Deliver(job, event)
```

This keeps the durable queue as the central reliability primitive while allowing each integration to live in its own file/package.

## Provider responsibilities

A provider should own:

- channel name;
- destination selection;
- payload rendering;
- HTTP/API request construction;
- provider-specific retry decisions;
- provider-specific headers/signatures;
- provider-specific error messages.

A provider should not own:

- source token validation;
- event deduplication;
- queue locking;
- queue retry counters;
- admin authorization;
- audit logging.

Those stay in the existing core layers.

## Migration plan

Move gradually, one provider at a time:

1. Extract HTTP forwarding into `internal/delivery/http_provider.go`.
2. Extract Telegram into `internal/delivery/telegram_provider.go`.
3. Add a provider registry for enabled channels.
4. Keep `delivery_jobs.channel` as the stable dispatch key.
5. Add Slack only after Telegram and HTTP are extracted.
6. Add Discord after Slack if the registry remains clean.
7. Add email last, because it needs more configuration and deliverability rules.

## Why this matters

This prevents the delivery worker from becoming a giant switch full of provider-specific code.

The product can grow into:

```text
Telegram
HTTP forwarding
Slack
Discord
Email
Custom webhook transformations
```

without turning the core queue into a hard-to-maintain integration blob.

## Enterprise rule

Every new provider must satisfy these rules:

- no secrets in logs;
- no secrets in audit metadata;
- deterministic retry behavior;
- clear failure messages in `delivery_jobs.last_error`;
- idempotency guidance in docs;
- provider-specific documentation;
- tests for payload rendering and retry decisions.
