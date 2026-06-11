# Telegram Templates

SignalBox supports custom Telegram messages per webhook source.

If a source has `telegram_template`, SignalBox renders it with Go `text/template` before enqueueing the Telegram delivery job. If rendering fails, SignalBox falls back to the default message and records a `template_failed` delivery attempt.

## Source setting

```json
{
  "name": "GitHub stars",
  "telegram_chat_id": "123456789",
  "telegram_template": "<b>{{ .Source.Name }}</b>\nType: {{ .Event.Type }}\nRepo: {{ index .Payload \"repository\" \"name\" }}"
}
```

`telegram_template` is optional and limited to 4000 characters.

## Available variables

### Source

```text
.Source.ID
.Source.Name
```

### Event

```text
.Event.ID
.Event.Type
.Event.Origin
.Event.ExternalID
.Event.CreatedAt
.Event.IsDuplicate
```

### Payload

The original webhook body is available as:

```text
.Payload
```

Use the built-in `index` function to access nested JSON fields:

```text
{{ index .Payload "repository" "name" }}
{{ index .Payload "sender" "login" }}
```

## Helper functions

```text
html VALUE
json VALUE
```

Examples:

```text
{{ html .Source.Name }}
{{ json .Payload }}
```

## Example: GitHub star event

```text
⭐ <b>New GitHub activity</b>

Repository: {{ index .Payload "repository" "full_name" }}
Sender: {{ index .Payload "sender" "login" }}
Event: {{ .Event.Type }}
Time: {{ .Event.CreatedAt }}
```

## Example: Grafana alert

```text
🚨 <b>Grafana alert</b>

Status: {{ index .Payload "status" }}
Title: {{ index .Payload "title" }}
Source: {{ .Source.Name }}
```

## Telegram formatting

SignalBox sends Telegram messages with `parse_mode=HTML`.

Useful tags:

```text
<b>bold</b>
<i>italic</i>
<code>inline code</code>
<pre>code block</pre>
```

If you insert untrusted payload fields, prefer escaping:

```text
{{ html (index .Payload "message") }}
```

## Admin UI

Open:

```text
/admin
```

Use the `Telegram template` field when creating a source.
