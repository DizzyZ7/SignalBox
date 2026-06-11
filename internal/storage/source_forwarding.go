package storage

const migration004SourceForwardingSQL = `
ALTER TABLE webhook_sources ADD COLUMN IF NOT EXISTS forward_url TEXT;
ALTER TABLE webhook_sources ADD COLUMN IF NOT EXISTS forward_hmac_key TEXT;
`

func init() {
	migrations = append(migrations, migration{version: "004_source_forwarding", sql: migration004SourceForwardingSQL})
}
