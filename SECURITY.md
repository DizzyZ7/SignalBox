# Security Policy

## Supported versions

Security fixes are applied to the `main` branch and the latest released Docker image.

| Version | Supported |
| --- | --- |
| latest | yes |
| older tags | best effort |

## Reporting a vulnerability

If you find a vulnerability, do not open a public issue with exploit details.

Preferred contact:

- GitHub profile: https://github.com/DizzyZ7
- Telegram: https://t.me/dizzy_dev

Please include:

- affected endpoint or component;
- impact description;
- reproduction steps;
- whether secrets, tokens or data can be exposed;
- suggested fix if available.

Please do not include real production secrets, source tokens, admin keys, Telegram bot tokens, customer payloads or private infrastructure URLs in reports. Use synthetic examples whenever possible.

## Scope

Security-sensitive areas include:

- webhook source token handling;
- admin authentication and authorization;
- admin audit logging;
- source token rotation;
- HTTP forwarding and SSRF protection;
- HMAC signing for forwarded events;
- event replay;
- delivery retry and failure handling;
- backup and restore scripts;
- Admin UI handling of sensitive fields.

## Security baseline

SignalBox is designed with the following baseline:

- source webhook tokens are stored as SHA-256 hashes;
- source tokens are returned only once on creation or rotation;
- webhook source tokens are redacted from access logs;
- admin endpoints require `X-API-Key`;
- admin endpoints are rate-limited;
- public webhook endpoint is rate-limited;
- API responses use `Cache-Control: no-store`;
- security headers are set by the application;
- HTTP forwarding blocks localhost and private destinations by default;
- forwarding HMAC keys are write-only from the API/UI perspective;
- PostgreSQL data can be backed up and restored with checked scripts;
- CI runs Go checks, shell script checks, CodeQL analysis and Trivy filesystem/image scans.

## Operational recommendations

For production deployments:

- run behind HTTPS;
- keep admin keys, source tokens and Telegram bot tokens private;
- rotate tokens if exposed;
- keep PostgreSQL backups outside the VPS;
- test restore procedures;
- keep Dependabot, CodeQL and Trivy alerts enabled;
- review failed deliveries and replay events when needed.

## Disclosure expectations

Please give the project a reasonable amount of time to investigate and fix confirmed vulnerabilities before public disclosure.

Security fixes may be shipped as patch releases or direct updates to the latest release line depending on severity.
