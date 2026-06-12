package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/DizzyZ7/SignalBox/internal/domain"
	"github.com/DizzyZ7/SignalBox/internal/security"
)

const adminAuditSchemaSQL = `
CREATE TABLE IF NOT EXISTS admin_audit_events (
    id BIGSERIAL PRIMARY KEY,
    public_id TEXT NOT NULL UNIQUE,
    request_id TEXT NOT NULL,
    action TEXT NOT NULL,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    target_type TEXT,
    target_id TEXT,
    status_code INTEGER NOT NULL,
    ip TEXT,
    user_agent TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_admin_audit_events_created_at ON admin_audit_events(created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_admin_audit_events_action_created_at ON admin_audit_events(action, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_admin_audit_events_target_created_at ON admin_audit_events(target_type, target_id, created_at DESC);
`

func (r *Repository) RecordAdminAuditEvent(ctx context.Context, event domain.AdminAuditEvent) error {
	if err := r.ensureAdminAuditSchema(ctx); err != nil {
		return err
	}
	publicID, err := security.RandomUUID()
	if err != nil {
		return err
	}
	metadata := event.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO admin_audit_events(public_id, request_id, action, method, path, target_type, target_id, status_code, ip, user_agent, metadata)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb)
	`, publicID, event.RequestID, event.Action, event.Method, event.Path, event.TargetType, event.TargetID, event.StatusCode, event.IP, event.UserAgent, string(metadata))
	return err
}

func (r *Repository) ListAdminAuditEvents(ctx context.Context, filter domain.AdminAuditEventFilter) ([]domain.AdminAuditEvent, error) {
	if err := r.ensureAdminAuditSchema(ctx); err != nil {
		return nil, err
	}
	query := `
		SELECT id, public_id, request_id, action, method, path, target_type, target_id, status_code, ip, user_agent, metadata, created_at
		FROM admin_audit_events
		WHERE 1 = 1
	`
	args := make([]any, 0, 5)
	addFilter := func(sqlPart string, value any) {
		args = append(args, value)
		placeholder := "$" + strconv.Itoa(len(args))
		query += " AND " + strings.Replace(sqlPart, "?", placeholder, 1)
	}
	if filter.Action != "" {
		addFilter("action = ?", filter.Action)
	}
	if filter.TargetType != "" {
		addFilter("target_type = ?", filter.TargetType)
	}
	if filter.TargetID != "" {
		addFilter("target_id = ?", filter.TargetID)
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	args = append(args, filter.Limit, filter.Offset)
	query += " ORDER BY created_at DESC, id DESC LIMIT $" + strconv.Itoa(len(args)-1) + " OFFSET $" + strconv.Itoa(len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.AdminAuditEvent, 0)
	for rows.Next() {
		item, err := scanAdminAuditEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) ensureAdminAuditSchema(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, adminAuditSchemaSQL)
	return err
}

func scanAdminAuditEvent(row scanner) (domain.AdminAuditEvent, error) {
	var item domain.AdminAuditEvent
	var targetType, targetID, ip, userAgent sql.NullString
	if err := row.Scan(&item.ID, &item.PublicID, &item.RequestID, &item.Action, &item.Method, &item.Path, &targetType, &targetID, &item.StatusCode, &ip, &userAgent, &item.Metadata, &item.CreatedAt); err != nil {
		return domain.AdminAuditEvent{}, err
	}
	item.TargetType = nullStringPtr(targetType)
	item.TargetID = nullStringPtr(targetID)
	item.IP = nullStringPtr(ip)
	item.UserAgent = nullStringPtr(userAgent)
	return item, nil
}
