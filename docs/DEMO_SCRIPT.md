# SignalBox Demo Script

This script can be used to record a short demo video, GIF or live walkthrough.

Goal: show that SignalBox can be installed quickly, receive events, queue deliveries, expose operations data and help debug incidents.

## Demo length

Recommended duration: 2-4 minutes.

## Setup before recording

Prepare:

- local SignalBox running with Docker Compose;
- Admin UI open in browser;
- one source created;
- example payloads from `examples/payloads`;
- optional Telegram chat or mock HTTP receiver.

## Scene 1: Product intro

Show the repository or a clean title slide.

Say:

```text
SignalBox is a lightweight self-hosted webhook and event gateway built with Go and PostgreSQL.
It stores events, deduplicates payloads, queues deliveries, supports replay, and gives teams an embedded Admin UI for operations.
```

## Scene 2: Admin UI overview

Open `/admin`.

Show:

- Sources;
- Events;
- Deliveries;
- Admin audit log;
- Source incident snapshot.

Message:

```text
Everything needed for small-team webhook operations is in one embedded UI.
```

## Scene 3: Send a test event

Open a source and use the admin-only test event feature.

Paste `examples/payloads/lead.created.json`.

Submit.

Show the event appearing in Events.

Message:

```text
The source test event path lets operators test delivery without exposing a public source token in demos.
```

## Scene 4: Delivery visibility

Open Deliveries.

Show queued or sent delivery rows.

Filter by source if needed.

Message:

```text
Each delivery job is visible, retryable and connected back to the original event.
```

## Scene 5: Incident snapshot

Open Source incident snapshot.

Show:

- latest events;
- latest deliveries;
- recent audit activity;
- copy diagnostics action.

Message:

```text
When something breaks, operators can copy a compact source diagnostic snapshot instead of hunting through logs.
```

## Scene 6: Replay

Open an event and replay it.

Show a new delivery job created from the stored event.

Message:

```text
Replay helps recover from temporary downstream failures without asking the original sender to resend the webhook.
```

## Scene 7: Metrics and self-hosting

Briefly show docs:

- `docs/SELF_HOSTING.md`;
- `docs/METRICS.md`;
- `docs/RUNBOOK.md`.

Message:

```text
SignalBox is built for self-hosting: Docker Compose, PostgreSQL, metrics, backups and operational docs are included.
```

## Closing

End with:

```text
SignalBox is for teams that need a production-ready webhook inbox without running a heavy event platform.
```

Show:

- GitHub repository;
- documentation index;
- roadmap.
