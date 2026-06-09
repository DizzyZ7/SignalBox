# Security Policy

## Supported versions

The `main` branch is the supported development line.

## Secrets

Never commit `.env`, Telegram bot tokens, database passwords, source tokens, or admin API keys.

## Reporting

Open a private report or contact the repository owner if you find a security issue.

## Operational recommendations

- Run SignalBox behind HTTPS.
- Use a long random `ADMIN_API_KEY`.
- Rotate source tokens if they are exposed.
- Restrict database access to the application network.
- Keep PostgreSQL backups enabled.
- Keep dependencies updated.
