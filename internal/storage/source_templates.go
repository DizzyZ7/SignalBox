package storage

const migration005SourceTemplatesSQL = `
ALTER TABLE webhook_sources ADD COLUMN IF NOT EXISTS telegram_template TEXT;
`

func init() {
	migrations = append(migrations, migration{version: "005_source_templates", sql: migration005SourceTemplatesSQL})
}
