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
- Telegram: https://t.me/DizZy_Z7

Please include:

- affected endpoint or component;
- impact description;
- reproduction steps;
- whether secrets, tokens or data can be exposed;
- suggested fix if available.

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
- PostgreSQL data can be backed up and restored with checked scripts;
- CI runs Go checks, shell script checks and CodeQL analysis.

## Operational recommendations

For production deployments:

- run behind HTTPS;
- keep `ADMIN_API_KEY`, source tokens and Telegram bot token private;
- rotate tokens if exposed;
- keep PostgreSQL backups outside the VPS;
- test restore procedures;
- keep Dependabot and CodeQL alerts enabled;
- review failed deliveries and replay events when needed.
