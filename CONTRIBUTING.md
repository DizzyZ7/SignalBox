# Contributing to SignalBox

Thanks for your interest in SignalBox.

SignalBox is an open-source, self-hosted webhook and event gateway built with Go and PostgreSQL. The project values reliability, simple self-hosting, clear operations and production-ready code.

## Project direction

SignalBox should stay:

- easy to run with Docker Compose;
- Go-native;
- PostgreSQL-first;
- useful for small teams and internal automation;
- safe with secrets;
- observable through logs, metrics, audit and Admin UI;
- simple enough to understand without operating a heavy event platform.

## Development setup

```bash
git clone https://github.com/DizzyZ7/SignalBox.git
cd SignalBox
cp .env.example .env
docker compose --env-file .env up --build
```

Run checks locally:

```bash
make fmt
make vet
make test
make build
```

## Pull request expectations

Before opening a pull request:

- keep changes focused;
- add tests for behavior changes;
- update docs for user-facing changes;
- avoid logging secrets;
- avoid storing secrets in audit metadata;
- keep backward compatibility where possible;
- do not break Docker Compose quickstart;
- keep CI green.

## Code style

- Use idiomatic Go.
- Prefer small functions with clear ownership.
- Keep domain, storage, delivery and HTTP layers separated.
- Do not duplicate query logic across handlers.
- Keep provider-specific delivery logic out of generic queue code.
- Avoid adding infrastructure dependencies unless there is a strong reason.

## Security rules

Never log or expose:

- source tokens;
- admin API keys;
- HMAC keys;
- webhook payload secrets;
- customer PII unless explicitly required by the feature.

Source tokens should remain hashed. HMAC keys should remain write-only from API/UI perspective.

## Good first issues

Good first contribution areas:

- docs improvements;
- webhook recipes;
- Admin UI polish;
- tests around existing behavior;
- provider documentation;
- small delivery provider improvements;
- metrics and alert examples.

## Provider contributions

New providers should follow the provider contract in:

```text
internal/delivery/provider.go
```

A provider should include:

- payload rendering tests;
- retry behavior tests;
- security notes;
- documentation;
- no secret leakage in logs or audit.

## Reporting security issues

Do not open public issues for vulnerabilities. Use the security policy in `SECURITY.md`.
