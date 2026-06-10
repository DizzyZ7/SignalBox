package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/DizzyZ7/SignalBox/internal/domain"
	"github.com/DizzyZ7/SignalBox/internal/security"
	"github.com/jackc/pgx/v5"
)

const migration003DeliveryJobsSQL = `
CREATE TABLE IF NOT EXISTS delivery_jobs (
    id BIGSERIAL PRIMARY KEY,
    public_id TEXT NOT NULL UNIQUE,
    event_id BIGINT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    channel TEXT NOT NULL,
    destination TEXT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 8,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_until TIMESTAMPTZ,
    locked_by TEXT,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_delivery_jobs_ready ON delivery_jobs(status, next_attempt_at, id);
CREATE INDEX IF NOT EXISTS idx_delivery_jobs_event_created_at ON delivery_jobs(event_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_delivery_jobs_locked_until ON delivery_jobs(locked_until);
`

func init() {
	migrations = append(migrations, migration{version: "003_delivery_jobs", sql: migration003DeliveryJobsSQL})
}

func (r *Repository) EnqueueDeliveryJob(ctx context.Context, eventID int64, channel, destination string, payload json.RawMessage, maxAttempts int) error {
	publicID, err := security.RandomUUID()
	if err != nil {
		return err
	}
	if maxAttempts <= 0 {
		maxAttempts = 8
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO delivery_jobs(public_id, event_id, channel, destination, payload, max_attempts)
		VALUES($1, $2, $3, $4, $5::jsonb, $6)
	`, publicID, eventID, channel, destination, string(payload), maxAttempts)
	return err
}

func (r *Repository) ClaimDeliveryJobs(ctx context.Context, workerID string, limit int, lockFor time.Duration) ([]domain.DeliveryJob, error) {
	if limit <= 0 {
		limit = 10
	}
	if lockFor <= 0 {
		lockFor = time.Minute
	}
	rows, err := r.pool.Query(ctx, `
		UPDATE delivery_jobs
		SET status = 'processing', locked_by = $1, locked_until = NOW() + make_interval(secs => $2::int), updated_at = NOW()
		WHERE id IN (
			SELECT id
			FROM delivery_jobs
			WHERE status = 'pending'
			  AND attempts < max_attempts
			  AND next_attempt_at <= NOW()
			  AND (locked_until IS NULL OR locked_until < NOW())
			ORDER BY next_attempt_at ASC, id ASC
			LIMIT $3
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, public_id, event_id, channel, destination, payload, status, attempts, max_attempts, next_attempt_at, locked_until, locked_by, last_error, created_at, updated_at, sent_at
	`, workerID, durationSeconds(lockFor), limit)
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

func (r *Repository) MarkDeliveryJobSent(ctx context.Context, jobID int64) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE delivery_jobs
		SET status = 'sent', attempts = attempts + 1, locked_until = NULL, locked_by = NULL, last_error = NULL, sent_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'processing'
	`, jobID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) MarkDeliveryJobFailed(ctx context.Context, jobID int64, errText string, retryAfter time.Duration) error {
	if retryAfter <= 0 {
		retryAfter = time.Minute
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE delivery_jobs
		SET attempts = attempts + 1,
		    status = CASE WHEN attempts + 1 >= max_attempts THEN 'failed' ELSE 'pending' END,
		    next_attempt_at = CASE WHEN attempts + 1 >= max_attempts THEN next_attempt_at ELSE NOW() + make_interval(secs => $2::int) END,
		    locked_until = NULL,
		    locked_by = NULL,
		    last_error = $3,
		    updated_at = NOW()
		WHERE id = $1 AND status = 'processing'
	`, jobID, durationSeconds(retryAfter), errText)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanDeliveryJob(row scanner) (domain.DeliveryJob, error) {
	var job domain.DeliveryJob
	var lockedUntil, sentAt sql.NullTime
	var lockedBy, lastError sql.NullString
	if err := row.Scan(
		&job.ID,
		&job.PublicID,
		&job.EventID,
		&job.Channel,
		&job.Destination,
		&job.Payload,
		&job.Status,
		&job.Attempts,
		&job.MaxAttempts,
		&job.NextAttemptAt,
		&lockedUntil,
		&lockedBy,
		&lastError,
		&job.CreatedAt,
		&job.UpdatedAt,
		&sentAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DeliveryJob{}, ErrNotFound
		}
		return domain.DeliveryJob{}, err
	}
	job.LockedUntil = nullTimePtr(lockedUntil)
	job.LockedBy = nullStringPtr(lockedBy)
	job.LastError = nullStringPtr(lastError)
	job.SentAt = nullTimePtr(sentAt)
	return job, nil
}

func durationSeconds(value time.Duration) int {
	seconds := int(value.Seconds())
	if seconds < 1 {
		return 1
	}
	return seconds
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
