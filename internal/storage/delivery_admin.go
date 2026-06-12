package storage

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/DizzyZ7/SignalBox/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) ListDeliveryJobs(ctx context.Context, filter domain.DeliveryJobFilter) ([]domain.DeliveryJob, error) {
	query := `
		SELECT dj.id, dj.public_id, dj.event_id, dj.channel, dj.destination, dj.payload, dj.status, dj.attempts, dj.max_attempts, dj.next_attempt_at, dj.locked_until, dj.locked_by, dj.last_error, dj.created_at, dj.updated_at, dj.sent_at
		FROM delivery_jobs dj
	`
	if filter.EventID != "" || filter.Source != "" {
		query += ` JOIN events e ON e.id = dj.event_id`
	}
	if filter.Source != "" {
		query += ` JOIN webhook_sources s ON s.id = e.source_id`
	}
	query += ` WHERE 1 = 1`

	args := make([]any, 0, 6)
	addFilter := func(sqlPart string, value any) {
		args = append(args, value)
		placeholder := "$" + strconv.Itoa(len(args))
		query += " AND " + strings.Replace(sqlPart, "?", placeholder, 1)
	}
	if filter.Status != "" {
		addFilter("dj.status = ?", filter.Status)
	}
	if filter.Channel != "" {
		addFilter("dj.channel = ?", filter.Channel)
	}
	if filter.Source != "" {
		addFilter("s.public_id = ?", filter.Source)
	}
	if filter.EventID != "" {
		addFilter("e.public_id = ?", filter.EventID)
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	args = append(args, filter.Limit, filter.Offset)
	query += fmt.Sprintf(" ORDER BY dj.created_at DESC, dj.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.DeliveryJob, 0)
	for rows.Next() {
		job, err := scanDeliveryJob(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, job)
	}
	return items, rows.Err()
}

func (r *Repository) GetDeliveryJob(ctx context.Context, id string) (domain.DeliveryJob, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, public_id, event_id, channel, destination, payload, status, attempts, max_attempts, next_attempt_at, locked_until, locked_by, last_error, created_at, updated_at, sent_at
		FROM delivery_jobs
		WHERE public_id = $1
		LIMIT 1
	`, id)
	job, err := scanDeliveryJob(row)
	if err != nil {
		return domain.DeliveryJob{}, err
	}
	return job, nil
}

func (r *Repository) RetryDeliveryJob(ctx context.Context, id string) (domain.DeliveryJob, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE delivery_jobs
		SET status = 'pending',
		    next_attempt_at = NOW(),
		    locked_until = NULL,
		    locked_by = NULL,
		    last_error = NULL,
		    updated_at = NOW()
		WHERE public_id = $1 AND status IN ('failed', 'pending')
		RETURNING id, public_id, event_id, channel, destination, payload, status, attempts, max_attempts, next_attempt_at, locked_until, locked_by, last_error, created_at, updated_at, sent_at
	`, id)
	job, err := scanDeliveryJob(row)
	if err != nil {
		if err == ErrNotFound || err == pgx.ErrNoRows {
			return domain.DeliveryJob{}, ErrNotFound
		}
		return domain.DeliveryJob{}, err
	}
	return job, nil
}
