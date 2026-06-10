package storage

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/DizzyZ7/SignalBox/internal/domain"
)

func (r *Repository) ListEventsCursor(ctx context.Context, filter domain.EventFilter) ([]domain.Event, error) {
	query := `
		SELECT e.id, e.public_id, e.source_id, e.event_type, e.origin, e.external_id, e.payload, e.payload_hash, e.ip, e.user_agent, e.is_duplicate, e.created_at,
		       s.id, s.public_id, s.name, s.token_hash, s.token_hint, s.telegram_chat_id, s.is_active, s.created_at, s.updated_at
		FROM events e JOIN webhook_sources s ON s.id = e.source_id
		WHERE 1 = 1
	`
	args := make([]any, 0, 10)
	addFilter := func(sqlPart string, value any) {
		args = append(args, value)
		placeholder := "$" + strconv.Itoa(len(args))
		query += " AND " + strings.Replace(sqlPart, "?", placeholder, 1)
	}
	if filter.Source != "" {
		addFilter("s.public_id = ?", filter.Source)
	}
	if filter.EventType != "" {
		addFilter("e.event_type = ?", filter.EventType)
	}
	if filter.Origin != "" {
		addFilter("e.origin = ?", filter.Origin)
	}
	if filter.Duplicate != nil {
		addFilter("e.is_duplicate = ?", *filter.Duplicate)
	}
	if filter.From != nil {
		addFilter("e.created_at >= ?", *filter.From)
	}
	if filter.To != nil {
		addFilter("e.created_at <= ?", *filter.To)
	}
	if filter.CursorCreatedAt != nil && filter.CursorID > 0 {
		args = append(args, *filter.CursorCreatedAt, filter.CursorID)
		query += fmt.Sprintf(" AND (e.created_at, e.id) < ($%d, $%d)", len(args)-1, len(args))
		filter.Offset = 0
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	args = append(args, filter.Limit, filter.Offset)
	query += fmt.Sprintf(" ORDER BY e.created_at DESC, e.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.Event, 0)
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, rows.Err()
}
