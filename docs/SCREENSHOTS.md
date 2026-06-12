# Screenshots Plan

Screenshots are important for the README, launch posts, demo pages and future website.

Recommended directory:

```text
assets/screenshots/
```

## Required screenshots

### 1. Admin UI overview

Suggested file:

```text
assets/screenshots/admin-overview.png
```

Show the full Admin UI with sources, events, deliveries and stats visible if possible.

### 2. Source editor

Suggested file:

```text
assets/screenshots/source-editor.png
```

Show source settings, Telegram template area, forwarding settings and source test event payload.

### 3. Events list

Suggested file:

```text
assets/screenshots/events-list.png
```

Show stored events, filters and event details.

### 4. Delivery queue

Suggested file:

```text
assets/screenshots/delivery-queue.png
```

Show queued, sent and failed delivery jobs with filters.

### 5. Source incident snapshot

Suggested file:

```text
assets/screenshots/source-incident-snapshot.png
```

Show recent events, deliveries, audit and copy diagnostics action.

### 6. Admin audit log

Suggested file:

```text
assets/screenshots/admin-audit-log.png
```

Show audit events and filters.

## Screenshot rules

- Use synthetic payloads from `examples/payloads`.
- Do not show real tokens.
- Do not show real customer data.
- Do not show real chat IDs.
- Do not show private URLs.
- Use a clean browser window.
- Prefer light theme unless dark theme looks better.
- Keep screenshots wide enough for README readability.

## README placement

Recommended README section order:

1. Hero and badges.
2. Problem statement.
3. Quickstart.
4. Screenshot: Admin UI overview.
5. Feature list.
6. Screenshot: Source incident snapshot.
7. Self-hosting docs links.
8. Roadmap and contributing.

## Demo GIF

Optional GIF:

```text
assets/screenshots/signalbox-demo.gif
```

Recommended flow:

1. Open Admin UI.
2. Select source.
3. Paste test payload.
4. Submit event.
5. Show event.
6. Show delivery.
7. Open incident snapshot.
8. Copy diagnostics.

Keep the GIF under 20-30 seconds.
