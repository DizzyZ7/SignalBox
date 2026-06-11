package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DizzyZ7/SignalBox/internal/domain"
	"github.com/DizzyZ7/SignalBox/internal/security"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Repository struct {
	pool *pgxpool.Pool
}

type migration struct {
	version string
	sql     string
}

const migration001SQL = `
CREATE TABLE IF NOT EXISTS webhook_sources (
    id BIGSERIAL PRIMARY KEY,
    public_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    token_hint TEXT NOT NULL,
    telegram_chat_id TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS event_dedup_keys (
    source_id BIGINT NOT NULL REFERENCES webhook_sources(id) ON DELETE CASCADE,
    payload_hash TEXT NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (source_id, payload_hash)
);

CREATE TABLE IF NOT EXISTS events (
    id BIGSERIAL PRIMARY KEY,
    public_id TEXT NOT NULL UNIQUE,
    source_id BIGINT NOT NULL REFERENCES webhook_sources(id) ON DELETE CASCADE,
    event_type TEXT,
    origin TEXT,
    external_id TEXT,
    payload JSONB NOT NULL,
    payload_hash TEXT NOT NULL,
    ip TEXT,
    user_agent TEXT,
    is_duplicate BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_source_id_created_at ON events(source_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_event_type ON events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_origin ON events(origin);
CREATE INDEX IF NOT EXISTS idx_events_duplicate ON events(is_duplicate);

CREATE TABLE IF NOT EXISTS delivery_attempts (
    id BIGSERIAL PRIMARY KEY,
    event_id BIGINT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    channel TEXT NOT NULL,
    status TEXT NOT NULL,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

const migration002SQL = `
CREATE INDEX IF NOT EXISTS idx_sources_active_created_at ON webhook_sources(is_active, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_external_id ON events(external_id);
CREATE INDEX IF NOT EXISTS idx_delivery_attempts_event_created_at ON delivery_attempts(event_id, created_at DESC);
`

var migrations = []migration{
	{version: "001_init", sql: migration001SQL},
	{version: "002_indexes", sql: migration002SQL},
}

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *Repository) Migrate(ctx context.Context) error {
	if _, err := r.pool.Exec(ctx, `SELECT pg_advisory_lock(775913040001)`); err != nil {
		return err
	}
	defer func() { _, _ = r.pool.Exec(context.Background(), `SELECT pg_advisory_unlock(775913040001)`) }()

	if _, err := r.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		return err
	}

	for _, item := range migrations {
		var exists bool
		if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, item.version).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, item.sql); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, item.version); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) CreateSource(ctx context.Context, name string, telegramChatID, forwardURL, forwardHMACKey *string, token string) (domain.Source, error) {
	publicID, err := security.RandomUUID()
	if err != nil {
		return domain.Source{}, err
	}
	source := domain.Source{
		PublicID:       publicID,
		Name:           name,
		TokenHash:      security.HashString(token),
		TokenHint:      security.TokenHint(token),
		TelegramChatID: telegramChatID,
		ForwardURL:     forwardURL,
		ForwardHMACKey: forwardHMACKey,
	}
	var chat, outForwardURL, outForwardHMACKey sql.NullString
	if err := r.pool.QueryRow(ctx, `
		INSERT INTO webhook_sources(public_id, name, token_hash, token_hint, telegram_chat_id, forward_url, forward_hmac_key)
		VALUES($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, public_id, name, token_hash, token_hint, telegram_chat_id, forward_url, forward_hmac_key, is_active, created_at, updated_at
	`, source.PublicID, source.Name, source.TokenHash, source.TokenHint, source.TelegramChatID, source.ForwardURL, source.ForwardHMACKey).Scan(
		&source.ID, &source.PublicID, &source.Name, &source.TokenHash, &source.TokenHint, &chat, &outForwardURL, &outForwardHMACKey, &source.IsActive, &source.CreatedAt, &source.UpdatedAt,
	); err != nil {
		return domain.Source{}, err
	}
	source.TelegramChatID = nullStringPtr(chat)
	source.ForwardURL = nullStringPtr(outForwardURL)
	source.ForwardHMACKey = nullStringPtr(outForwardHMACKey)
	source.ForwardHMACKeySet = source.ForwardHMACKey != nil
	source.Token = token
	return source, nil
}

func (r *Repository) ListSources(ctx context.Context, active *bool) ([]domain.Source, error) {
	query := `SELECT id, public_id, name, token_hash, token_hint, telegram_chat_id, forward_url, forward_hmac_key, is_active, created_at, updated_at FROM webhook_sources`
	args := make([]any, 0, 1)
	if active != nil {
		args = append(args, *active)
		query += ` WHERE is_active = $1`
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.Source, 0)
	for rows.Next() {
		s, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func (r *Repository) GetSourceByPublicID(ctx context.Context, publicID string) (domain.Source, error) {
	var s domain.Source
	var chat, forwardURL, forwardHMACKey sql.NullString
	err := r.pool.QueryRow(ctx, `
		SELECT id, public_id, name, token_hash, token_hint, telegram_chat_id, forward_url, forward_hmac_key, is_active, created_at, updated_at
		FROM webhook_sources WHERE public_id = $1 LIMIT 1
	`, publicID).Scan(&s.ID, &s.PublicID, &s.Name, &s.TokenHash, &s.TokenHint, &chat, &forwardURL, &forwardHMACKey, &s.IsActive, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Source{}, ErrNotFound
		}
		return domain.Source{}, err
	}
	s.TelegramChatID = nullStringPtr(chat)
	s.ForwardURL = nullStringPtr(forwardURL)
	s.ForwardHMACKey = nullStringPtr(forwardHMACKey)
	s.ForwardHMACKeySet = s.ForwardHMACKey != nil
	return s, nil
}

func (r *Repository) FindSourceByToken(ctx context.Context, token string) (domain.Source, error) {
	var s domain.Source
	var chat, forwardURL, forwardHMACKey sql.NullString
	err := r.pool.QueryRow(ctx, `
		SELECT id, public_id, name, token_hash, token_hint, telegram_chat_id, forward_url, forward_hmac_key, is_active, created_at, updated_at
		FROM webhook_sources WHERE token_hash = $1 AND is_active = TRUE LIMIT 1
	`, security.HashString(token)).Scan(&s.ID, &s.PublicID, &s.Name, &s.TokenHash, &s.TokenHint, &chat, &forwardURL, &forwardHMACKey, &s.IsActive, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Source{}, ErrNotFound
		}
		return domain.Source{}, err
	}
	s.TelegramChatID = nullStringPtr(chat)
	s.ForwardURL = nullStringPtr(forwardURL)
	s.ForwardHMACKey = nullStringPtr(forwardHMACKey)
	s.ForwardHMACKeySet = s.ForwardHMACKey != nil
	return s, nil
}

func (r *Repository) UpdateSource(ctx context.Context, id string, name string, chat, forwardURL, forwardHMACKey *string, active bool) (domain.Source, error) {
	var out domain.Source
	var outChat, outForwardURL, outForwardHMACKey sql.NullString
	err := r.pool.QueryRow(ctx, `
		UPDATE webhook_sources
		SET name = $2, telegram_chat_id = $3, forward_url = $4, forward_hmac_key = $5, is_active = $6, updated_at = NOW()
		WHERE public_id = $1
		RETURNING id, public_id, name, token_hash, token_hint, telegram_chat_id, forward_url, forward_hmac_key, is_active, created_at, updated_at
	`, id, name, chat, forwardURL, forwardHMACKey, active).Scan(&out.ID, &out.PublicID, &out.Name, &out.TokenHash, &out.TokenHint, &outChat, &outForwardURL, &outForwardHMACKey, &out.IsActive, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Source{}, ErrNotFound
		}
		return domain.Source{}, err
	}
	out.TelegramChatID = nullStringPtr(outChat)
	out.ForwardURL = nullStringPtr(outForwardURL)
	out.ForwardHMACKey = nullStringPtr(outForwardHMACKey)
	out.ForwardHMACKeySet = out.ForwardHMACKey != nil
	return out, nil
}

func (r *Repository) DisableSource(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE webhook_sources SET is_active = FALSE, updated_at = NOW() WHERE public_id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) RotateSourceToken(ctx context.Context, id string, token string) (domain.Source, error) {
	var s domain.Source
	var chat, forwardURL, forwardHMACKey sql.NullString
	err := r.pool.QueryRow(ctx, `
		UPDATE webhook_sources
		SET token_hash = $2, token_hint = $3, updated_at = NOW()
		WHERE public_id = $1
		RETURNING id, public_id, name, token_hash, token_hint, telegram_chat_id, forward_url, forward_hmac_key, is_active, created_at, updated_at
	`, id, security.HashString(token), security.TokenHint(token)).Scan(&s.ID, &s.PublicID, &s.Name, &s.TokenHash, &s.TokenHint, &chat, &forwardURL, &forwardHMACKey, &s.IsActive, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Source{}, ErrNotFound
		}
		return domain.Source{}, err
	}
	s.TelegramChatID = nullStringPtr(chat)
	s.ForwardURL = nullStringPtr(forwardURL)
	s.ForwardHMACKey = nullStringPtr(forwardHMACKey)
	s.ForwardHMACKeySet = s.ForwardHMACKey != nil
	s.Token = token
	return s, nil
}

func (r *Repository) InsertEvent(ctx context.Context, source domain.Source, payload map[string]any, canonical []byte, ip, ua string) (domain.Event, bool, error) {
	publicID, err := security.RandomUUID()
	if err != nil {
		return domain.Event{}, false, err
	}
	event := domain.Event{
		PublicID:    publicID,
		SourceID:    source.ID,
		EventType:   security.ExtractString(payload, "type", "event_type", "event"),
		Origin:      security.ExtractString(payload, "source", "origin", "channel"),
		ExternalID:  security.ExtractString(payload, "id", "external_id", "request_id"),
		Payload:     canonical,
		PayloadHash: security.HashBytes(canonical),
		IP:          stringPtr(ip),
		UserAgent:   stringPtr(ua),
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Event{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result, err := tx.Exec(ctx, `INSERT INTO event_dedup_keys(source_id, payload_hash) VALUES($1, $2) ON CONFLICT DO NOTHING`, source.ID, event.PayloadHash)
	if err != nil {
		return domain.Event{}, false, err
	}
	isNew := result.RowsAffected() == 1
	event.IsDuplicate = !isNew
	var eventType, origin, externalID, eventIP, userAgent sql.NullString
	err = tx.QueryRow(ctx, `
		INSERT INTO events(public_id, source_id, event_type, origin, external_id, payload, payload_hash, ip, user_agent, is_duplicate)
		VALUES($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9, $10)
		RETURNING id, public_id, source_id, event_type, origin, external_id, payload, payload_hash, ip, user_agent, is_duplicate, created_at
	`, event.PublicID, event.SourceID, event.EventType, event.Origin, event.ExternalID, string(event.Payload), event.PayloadHash, event.IP, event.UserAgent, event.IsDuplicate).Scan(
		&event.ID, &event.PublicID, &event.SourceID, &eventType, &origin, &externalID, &event.Payload, &event.PayloadHash, &eventIP, &userAgent, &event.IsDuplicate, &event.CreatedAt,
	)
	if err != nil {
		return domain.Event{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Event{}, false, err
	}
	event.EventType = nullStringPtr(eventType)
	event.Origin = nullStringPtr(origin)
	event.ExternalID = nullStringPtr(externalID)
	event.IP = nullStringPtr(eventIP)
	event.UserAgent = nullStringPtr(userAgent)
	event.Source = &source
	return event, isNew, nil
}

func (r *Repository) ListEvents(ctx context.Context, filter domain.EventFilter) ([]domain.Event, error) {
	query := `
		SELECT e.id, e.public_id, e.source_id, e.event_type, e.origin, e.external_id, e.payload, e.payload_hash, e.ip, e.user_agent, e.is_duplicate, e.created_at,
		       s.id, s.public_id, s.name, s.token_hash, s.token_hint, s.telegram_chat_id, s.forward_url, s.forward_hmac_key, s.is_active, s.created_at, s.updated_at
		FROM events e JOIN webhook_sources s ON s.id = e.source_id
		WHERE 1 = 1
	`
	args := make([]any, 0, 8)
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

func (r *Repository) GetEvent(ctx context.Context, id string) (domain.Event, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT e.id, e.public_id, e.source_id, e.event_type, e.origin, e.external_id, e.payload, e.payload_hash, e.ip, e.user_agent, e.is_duplicate, e.created_at,
		       s.id, s.public_id, s.name, s.token_hash, s.token_hint, s.telegram_chat_id, s.forward_url, s.forward_hmac_key, s.is_active, s.created_at, s.updated_at
		FROM events e JOIN webhook_sources s ON s.id = e.source_id WHERE e.public_id = $1 LIMIT 1
	`, id)
	e, err := scanEvent(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Event{}, ErrNotFound
		}
		return domain.Event{}, err
	}
	return e, nil
}

func (r *Repository) GetEventByInternalID(ctx context.Context, eventID int64) (domain.Event, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT e.id, e.public_id, e.source_id, e.event_type, e.origin, e.external_id, e.payload, e.payload_hash, e.ip, e.user_agent, e.is_duplicate, e.created_at,
		       s.id, s.public_id, s.name, s.token_hash, s.token_hint, s.telegram_chat_id, s.forward_url, s.forward_hmac_key, s.is_active, s.created_at, s.updated_at
		FROM events e JOIN webhook_sources s ON s.id = e.source_id WHERE e.id = $1 LIMIT 1
	`, eventID)
	e, err := scanEvent(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Event{}, ErrNotFound
		}
		return domain.Event{}, err
	}
	return e, nil
}

func (r *Repository) Stats(ctx context.Context) (domain.StatsResponse, error) {
	var out domain.StatsResponse
	err := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*)::BIGINT,
			COUNT(*) FILTER (WHERE is_duplicate = FALSE)::BIGINT,
			COUNT(*) FILTER (WHERE is_duplicate = TRUE)::BIGINT,
			COUNT(*) FILTER (WHERE created_at >= NOW() - INTERVAL '24 hours')::BIGINT
		FROM events
	`).Scan(&out.TotalEvents, &out.UniqueEvents, &out.DuplicateEvents, &out.Events24h)
	if err != nil {
		return domain.StatsResponse{}, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*)::BIGINT, COUNT(*) FILTER (WHERE is_active = TRUE)::BIGINT FROM webhook_sources`).Scan(&out.Sources, &out.ActiveSources); err != nil {
		return domain.StatsResponse{}, err
	}
	byType, err := r.statRows(ctx, `SELECT COALESCE(event_type, 'unknown') AS key, COUNT(*)::BIGINT AS count FROM events GROUP BY COALESCE(event_type, 'unknown') ORDER BY count DESC, key ASC LIMIT 10`)
	if err != nil {
		return domain.StatsResponse{}, err
	}
	byOrigin, err := r.statRows(ctx, `SELECT COALESCE(origin, 'unknown') AS key, COUNT(*)::BIGINT AS count FROM events GROUP BY COALESCE(origin, 'unknown') ORDER BY count DESC, key ASC LIMIT 10`)
	if err != nil {
		return domain.StatsResponse{}, err
	}
	out.ByType = byType
	out.ByOrigin = byOrigin
	return out, nil
}

func (r *Repository) RecordDeliveryAttempt(ctx context.Context, eventID int64, channel, status string, errText *string) {
	_, _ = r.pool.Exec(ctx, `INSERT INTO delivery_attempts(event_id, channel, status, error_message) VALUES($1, $2, $3, $4)`, eventID, channel, status, errText)
}

func (r *Repository) statRows(ctx context.Context, query string) ([]domain.StatRow, error) {
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.StatRow, 0)
	for rows.Next() {
		var item domain.StatRow
		if err := rows.Scan(&item.Key, &item.Count); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scanSource(row scanner) (domain.Source, error) {
	var s domain.Source
	var chat, forwardURL, forwardHMACKey sql.NullString
	if err := row.Scan(&s.ID, &s.PublicID, &s.Name, &s.TokenHash, &s.TokenHint, &chat, &forwardURL, &forwardHMACKey, &s.IsActive, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return domain.Source{}, err
	}
	s.TelegramChatID = nullStringPtr(chat)
	s.ForwardURL = nullStringPtr(forwardURL)
	s.ForwardHMACKey = nullStringPtr(forwardHMACKey)
	s.ForwardHMACKeySet = s.ForwardHMACKey != nil
	return s, nil
}

func scanEvent(row scanner) (domain.Event, error) {
	var e domain.Event
	var s domain.Source
	var eventType, origin, externalID, ip, ua, chat, forwardURL, forwardHMACKey sql.NullString
	if err := row.Scan(&e.ID, &e.PublicID, &e.SourceID, &eventType, &origin, &externalID, &e.Payload, &e.PayloadHash, &ip, &ua, &e.IsDuplicate, &e.CreatedAt, &s.ID, &s.PublicID, &s.Name, &s.TokenHash, &s.TokenHint, &chat, &forwardURL, &forwardHMACKey, &s.IsActive, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return domain.Event{}, err
	}
	e.EventType, e.Origin, e.ExternalID = nullStringPtr(eventType), nullStringPtr(origin), nullStringPtr(externalID)
	e.IP, e.UserAgent = nullStringPtr(ip), nullStringPtr(ua)
	s.TelegramChatID = nullStringPtr(chat)
	s.ForwardURL = nullStringPtr(forwardURL)
	s.ForwardHMACKey = nullStringPtr(forwardHMACKey)
	s.ForwardHMACKeySet = s.ForwardHMACKey != nil
	e.Source = &s
	return e, nil
}

func stringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
