package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	HTTPAddr              string
	DatabaseURL           string
	AdminAPIKey           string
	AutoMigrate           bool
	TelegramBotToken      string
	TelegramDefaultChatID string
	MaxBodyBytes          int64
}

type Source struct {
	ID             int64     `json:"-"`
	PublicID       string    `json:"id"`
	Name           string    `json:"name"`
	TokenHash      string    `json:"-"`
	TokenHint      string    `json:"token_hint"`
	TelegramChatID *string   `json:"telegram_chat_id,omitempty"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Token          string    `json:"token,omitempty"`
}

type Event struct {
	ID          int64           `json:"-"`
	PublicID    string          `json:"id"`
	SourceID    int64           `json:"-"`
	Source      *Source         `json:"source,omitempty"`
	EventType   *string         `json:"event_type,omitempty"`
	Origin      *string         `json:"origin,omitempty"`
	ExternalID  *string         `json:"external_id,omitempty"`
	Payload     json.RawMessage `json:"payload"`
	PayloadHash string          `json:"payload_hash"`
	IP          *string         `json:"ip,omitempty"`
	UserAgent   *string         `json:"user_agent,omitempty"`
	IsDuplicate bool            `json:"is_duplicate"`
	CreatedAt   time.Time       `json:"created_at"`
}

type App struct {
	cfg    Config
	pool   *pgxpool.Pool
	log    *slog.Logger
	client *http.Client
}

var errNotFound = errors.New("not found")

const migrationSQL = `
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

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := loadConfig()
	if err != nil {
		logger.Error("config error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := connectDB(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	app := &App{cfg: cfg, pool: pool, log: logger, client: &http.Client{Timeout: 8 * time.Second}}
	if cfg.AutoMigrate {
		if err := app.migrate(ctx); err != nil {
			logger.Error("migration failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", app.health)
	mux.HandleFunc("GET /readyz", app.ready)
	mux.HandleFunc("POST /v1/hooks/{token}", app.receiveWebhook)
	mux.Handle("POST /v1/sources", app.admin(http.HandlerFunc(app.createSource)))
	mux.Handle("GET /v1/sources", app.admin(http.HandlerFunc(app.listSources)))
	mux.Handle("GET /v1/events", app.admin(http.HandlerFunc(app.listEvents)))
	mux.Handle("GET /v1/events/{id}", app.admin(http.HandlerFunc(app.getEvent)))

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           app.accessLog(app.recover(app.requestID(mux))),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       time.Minute,
	}

	errs := make(chan error, 1)
	go func() {
		logger.Info("http server listening", slog.String("addr", cfg.HTTPAddr))
		errs <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errs:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	case sig := <-stop:
		logger.Info("shutdown requested", slog.String("signal", sig.String()))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", slog.String("error", err.Error()))
		_ = server.Close()
	}
}

func loadConfig() (Config, error) {
	cfg := Config{
		HTTPAddr:              env("HTTP_ADDR", ":8080"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		AdminAPIKey:           os.Getenv("ADMIN_API_KEY"),
		AutoMigrate:           envBool("AUTO_MIGRATE", false),
		TelegramBotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramDefaultChatID: os.Getenv("TELEGRAM_DEFAULT_CHAT_ID"),
		MaxBodyBytes:          envInt64("MAX_BODY_BYTES", 1<<20),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if len(cfg.AdminAPIKey) < 16 {
		return Config{}, errors.New("ADMIN_API_KEY must be at least 16 characters")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func connectDB(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
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

func (a *App) migrate(ctx context.Context) error {
	if _, err := a.pool.Exec(ctx, `SELECT pg_advisory_lock(775913040001)`); err != nil {
		return err
	}
	defer func() { _, _ = a.pool.Exec(context.Background(), `SELECT pg_advisory_unlock(775913040001)`) }()

	if _, err := a.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		return err
	}
	var exists bool
	if err := a.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = '001_init')`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, migrationSQL); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES('001_init')`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.pool.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable", requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *App) createSource(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name           string  `json:"name"`
		TelegramChatID *string `json:"telegram_chat_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body", requestID(r))
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 120 {
		writeError(w, http.StatusBadRequest, "source name is required and must be shorter than 120 characters", requestID(r))
		return
	}
	token, err := randomToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token generation failed", requestID(r))
		return
	}
	publicID, err := randomUUID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "id generation failed", requestID(r))
		return
	}
	source := Source{PublicID: publicID, Name: name, TokenHash: hashString(token), TokenHint: tokenHint(token), TelegramChatID: normalizePtr(input.TelegramChatID)}
	var chat sql.NullString
	if err := a.pool.QueryRow(r.Context(), `
		INSERT INTO webhook_sources(public_id, name, token_hash, token_hint, telegram_chat_id)
		VALUES($1, $2, $3, $4, $5)
		RETURNING id, public_id, name, token_hash, token_hint, telegram_chat_id, is_active, created_at, updated_at
	`, source.PublicID, source.Name, source.TokenHash, source.TokenHint, source.TelegramChatID).Scan(
		&source.ID, &source.PublicID, &source.Name, &source.TokenHash, &source.TokenHint, &chat, &source.IsActive, &source.CreatedAt, &source.UpdatedAt,
	); err != nil {
		a.log.Error("create source failed", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}
	source.TelegramChatID = nullStringPtr(chat)
	source.Token = token
	writeJSON(w, http.StatusCreated, source)
}

func (a *App) listSources(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `SELECT id, public_id, name, token_hash, token_hint, telegram_chat_id, is_active, created_at, updated_at FROM webhook_sources ORDER BY created_at DESC`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}
	defer rows.Close()
	items := make([]Source, 0)
	for rows.Next() {
		var s Source
		var chat sql.NullString
		if err := rows.Scan(&s.ID, &s.PublicID, &s.Name, &s.TokenHash, &s.TokenHint, &chat, &s.IsActive, &s.CreatedAt, &s.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
			return
		}
		s.TelegramChatID = nullStringPtr(chat)
		items = append(items, s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *App) receiveWebhook(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.PathValue("token"))
	if token == "" {
		writeError(w, http.StatusNotFound, "source not found", requestID(r))
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.Contains(strings.ToLower(ct), "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "content type must be application/json", requestID(r))
		return
	}
	var payload map[string]any
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, a.cfg.MaxBodyBytes))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil || len(payload) == 0 {
		writeError(w, http.StatusBadRequest, "payload must be a non-empty json object", requestID(r))
		return
	}
	source, err := a.findSourceByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "source not found", requestID(r))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}
	canonical, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload", requestID(r))
		return
	}
	event, isNew, err := a.insertEvent(r.Context(), source, payload, canonical, clientIP(r), r.UserAgent())
	if err != nil {
		a.log.Error("insert event failed", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}
	if isNew {
		a.notifyTelegram(event, source)
	}
	writeJSON(w, http.StatusAccepted, event)
}

func (a *App) findSourceByToken(ctx context.Context, token string) (Source, error) {
	var s Source
	var chat sql.NullString
	err := a.pool.QueryRow(ctx, `
		SELECT id, public_id, name, token_hash, token_hint, telegram_chat_id, is_active, created_at, updated_at
		FROM webhook_sources WHERE token_hash = $1 AND is_active = TRUE LIMIT 1
	`, hashString(token)).Scan(&s.ID, &s.PublicID, &s.Name, &s.TokenHash, &s.TokenHint, &chat, &s.IsActive, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Source{}, errNotFound
		}
		return Source{}, err
	}
	s.TelegramChatID = nullStringPtr(chat)
	return s, nil
}

func (a *App) insertEvent(ctx context.Context, source Source, payload map[string]any, canonical []byte, ip, ua string) (Event, bool, error) {
	publicID, err := randomUUID()
	if err != nil {
		return Event{}, false, err
	}
	event := Event{PublicID: publicID, SourceID: source.ID, EventType: extractString(payload, "type", "event_type", "event"), Origin: extractString(payload, "source", "origin", "channel"), ExternalID: extractString(payload, "id", "external_id", "request_id"), Payload: canonical, PayloadHash: hashBytes(canonical), IP: stringPtr(ip), UserAgent: stringPtr(ua)}
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return Event{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `INSERT INTO event_dedup_keys(source_id, payload_hash) VALUES($1, $2) ON CONFLICT DO NOTHING`, source.ID, event.PayloadHash)
	if err != nil {
		return Event{}, false, err
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
		return Event{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Event{}, false, err
	}
	event.EventType = nullStringPtr(eventType)
	event.Origin = nullStringPtr(origin)
	event.ExternalID = nullStringPtr(externalID)
	event.IP = nullStringPtr(eventIP)
	event.UserAgent = nullStringPtr(userAgent)
	event.Source = &source
	return event, isNew, nil
}

func (a *App) listEvents(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := queryInt(r, "offset", 0)
	rows, err := a.pool.Query(r.Context(), `
		SELECT e.id, e.public_id, e.source_id, e.event_type, e.origin, e.external_id, e.payload, e.payload_hash, e.ip, e.user_agent, e.is_duplicate, e.created_at,
		       s.id, s.public_id, s.name, s.token_hash, s.token_hint, s.telegram_chat_id, s.is_active, s.created_at, s.updated_at
		FROM events e JOIN webhook_sources s ON s.id = e.source_id
		ORDER BY e.created_at DESC, e.id DESC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}
	defer rows.Close()
	items := make([]Event, 0)
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
			return
		}
		items = append(items, e)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset})
}

func (a *App) getEvent(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "event id is required", requestID(r))
		return
	}
	row := a.pool.QueryRow(r.Context(), `
		SELECT e.id, e.public_id, e.source_id, e.event_type, e.origin, e.external_id, e.payload, e.payload_hash, e.ip, e.user_agent, e.is_duplicate, e.created_at,
		       s.id, s.public_id, s.name, s.token_hash, s.token_hint, s.telegram_chat_id, s.is_active, s.created_at, s.updated_at
		FROM events e JOIN webhook_sources s ON s.id = e.source_id WHERE e.public_id = $1 LIMIT 1
	`, id)
	e, err := scanEvent(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "event not found", requestID(r))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, e)
}

type scanner interface{ Scan(dest ...any) error }

func scanEvent(row scanner) (Event, error) {
	var e Event
	var s Source
	var eventType, origin, externalID, ip, ua, chat sql.NullString
	if err := row.Scan(&e.ID, &e.PublicID, &e.SourceID, &eventType, &origin, &externalID, &e.Payload, &e.PayloadHash, &ip, &ua, &e.IsDuplicate, &e.CreatedAt, &s.ID, &s.PublicID, &s.Name, &s.TokenHash, &s.TokenHint, &chat, &s.IsActive, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return Event{}, err
	}
	e.EventType, e.Origin, e.ExternalID = nullStringPtr(eventType), nullStringPtr(origin), nullStringPtr(externalID)
	e.IP, e.UserAgent = nullStringPtr(ip), nullStringPtr(ua)
	s.TelegramChatID = nullStringPtr(chat)
	e.Source = &s
	return e, nil
}

func (a *App) notifyTelegram(event Event, source Source) {
	if strings.TrimSpace(a.cfg.TelegramBotToken) == "" {
		return
	}
	chatID := strings.TrimSpace(a.cfg.TelegramDefaultChatID)
	if source.TelegramChatID != nil && strings.TrimSpace(*source.TelegramChatID) != "" {
		chatID = strings.TrimSpace(*source.TelegramChatID)
	}
	if chatID == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		text := fmt.Sprintf("<b>SignalBox: new event</b>\n\n<b>Source:</b> %s\n<b>Type:</b> %s\n<b>ID:</b> <code>%s</code>\n<b>Time:</b> %s", html.EscapeString(source.Name), html.EscapeString(ptrValue(event.EventType, "unknown")), html.EscapeString(event.PublicID), event.CreatedAt.UTC().Format(time.RFC3339))
		body, _ := json.Marshal(map[string]any{"chat_id": chatID, "text": text, "parse_mode": "HTML", "disable_web_page_preview": true})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+a.cfg.TelegramBotToken+"/sendMessage", bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := a.client.Do(req)
		status := "sent"
		var errText *string
		if err != nil {
			status = "failed"
			errText = stringPtr(err.Error())
		} else {
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				status = "failed"
				errText = stringPtr(fmt.Sprintf("telegram status %d", resp.StatusCode))
			}
		}
		_, _ = a.pool.Exec(context.Background(), `INSERT INTO delivery_attempts(event_id, channel, status, error_message) VALUES($1, 'telegram', $2, $3)`, event.ID, status, errText)
	}()
}

func (a *App) admin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-API-Key")), []byte(a.cfg.AdminAPIKey)) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized", requestID(r))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = randomHex(16)
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

func (a *App) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				a.log.Error("panic recovered", slog.Any("panic", recovered), slog.String("path", r.URL.Path))
				writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (a *App) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		a.log.Info("http request", slog.String("method", r.Method), slog.String("path", r.URL.Path), slog.Int("status", rw.status), slog.Duration("duration", time.Since(start)))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

type requestIDKey struct{}

func requestID(r *http.Request) string {
	if value, ok := r.Context().Value(requestIDKey{}).(string); ok && value != "" {
		return value
	}
	return "unknown"
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message, requestID string) {
	writeJSON(w, status, map[string]string{"error": message, "request_id": requestID})
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf), err
}

func randomUUID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:]), nil
}

func randomHex(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "unavailable"
	}
	return hex.EncodeToString(buf)
}

func hashString(value string) string { return hashBytes([]byte(value)) }

func hashBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func tokenHint(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:4] + "..." + token[len(token)-4:]
}

func extractString(payload map[string]any, keys ...string) *string {
	for _, key := range keys {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			trimmed := strings.TrimSpace(value)
			return &trimmed
		}
	}
	return nil
}

func normalizePtr(value *string) *string {
	if value == nil {
		return nil
	}
	return stringPtr(*value)
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

func ptrValue(value *string, fallback string) string {
	if value == nil || *value == "" {
		return fallback
	}
	return *value
}

func queryInt(r *http.Request, key string, fallback int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func clientIP(r *http.Request) string {
	if value := r.Header.Get("X-Forwarded-For"); value != "" {
		return strings.TrimSpace(strings.Split(value, ",")[0])
	}
	if value := r.Header.Get("X-Real-IP"); value != "" {
		return strings.TrimSpace(value)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
