package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DizzyZ7/SignalBox/internal/config"
	"github.com/DizzyZ7/SignalBox/internal/domain"
	"github.com/DizzyZ7/SignalBox/internal/ratelimit"
	"github.com/DizzyZ7/SignalBox/internal/security"
	"github.com/DizzyZ7/SignalBox/internal/storage"
)

type Notifier interface {
	Notify(event domain.Event, source domain.Source)
}

type Server struct {
	cfg            config.Config
	repo           *storage.Repository
	notifier       Notifier
	log            *slog.Logger
	webhookLimiter *ratelimit.Limiter
	adminLimiter   *ratelimit.Limiter
}

func NewServer(cfg config.Config, repo *storage.Repository, notifier Notifier, log *slog.Logger) *Server {
	return &Server{
		cfg:            cfg,
		repo:           repo,
		notifier:       notifier,
		log:            log,
		webhookLimiter: ratelimit.New(cfg.WebhookRateLimitRequests, cfg.WebhookRateLimitWindow),
		adminLimiter:   ratelimit.New(30, time.Minute),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.Handle("POST /v1/hooks/{token}", s.webhookRateLimit(http.HandlerFunc(s.receiveWebhook)))
	mux.Handle("POST /v1/sources", s.admin(http.HandlerFunc(s.createSource)))
	mux.Handle("GET /v1/sources", s.admin(http.HandlerFunc(s.listSources)))
	mux.Handle("PATCH /v1/sources/{id}", s.admin(http.HandlerFunc(s.updateSource)))
	mux.Handle("DELETE /v1/sources/{id}", s.admin(http.HandlerFunc(s.deleteSource)))
	mux.Handle("POST /v1/sources/{id}/rotate-token", s.admin(http.HandlerFunc(s.rotateSourceToken)))
	mux.Handle("POST /v1/templates/telegram/preview", s.admin(http.HandlerFunc(s.previewTelegramTemplate)))
	mux.Handle("GET /v1/events", s.admin(http.HandlerFunc(s.listEventsCursor)))
	mux.Handle("GET /v1/events/{id}", s.admin(http.HandlerFunc(s.getEvent)))
	mux.Handle("POST /v1/events/{id}/replay", s.admin(http.HandlerFunc(s.replayEvent)))
	mux.Handle("GET /v1/deliveries", s.admin(http.HandlerFunc(s.listDeliveries)))
	mux.Handle("GET /v1/deliveries/{id}", s.admin(http.HandlerFunc(s.getDelivery)))
	mux.Handle("POST /v1/deliveries/{id}/retry", s.admin(http.HandlerFunc(s.retryDelivery)))
	mux.Handle("GET /v1/stats", s.admin(http.HandlerFunc(s.stats)))
	return s.accessLog(s.recover(s.securityHeaders(s.requestID(mux))))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.repo.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable", requestID(r))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) createSource(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name             string  `json:"name"`
		TelegramChatID   *string `json:"telegram_chat_id"`
		TelegramTemplate *string `json:"telegram_template"`
		ForwardURL       *string `json:"forward_url"`
		ForwardHMACKey   *string `json:"forward_hmac_key"`
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

	telegramTemplate, ok := normalizeTelegramTemplate(w, r, input.TelegramTemplate)
	if !ok {
		return
	}
	forwardURL, ok := normalizeForwardURL(w, r, input.ForwardURL)
	if !ok {
		return
	}

	token, err := security.RandomToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token generation failed", requestID(r))
		return
	}

	source, err := s.repo.CreateSource(r.Context(), name, normalizePtr(input.TelegramChatID), telegramTemplate, forwardURL, normalizePtr(input.ForwardHMACKey), token)
	if err != nil {
		s.log.Error("create source failed", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}

	writeJSON(w, http.StatusCreated, source)
}

func (s *Server) listSources(w http.ResponseWriter, r *http.Request) {
	var active *bool

	if raw := strings.TrimSpace(r.URL.Query().Get("active")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "active must be boolean", requestID(r))
			return
		}
		active = &parsed
	}

	items, err := s.repo.ListSources(r.Context(), active)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) updateSource(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "source id is required", requestID(r))
		return
	}

	var input struct {
		Name             *string `json:"name"`
		TelegramChatID   *string `json:"telegram_chat_id"`
		TelegramTemplate *string `json:"telegram_template"`
		ForwardURL       *string `json:"forward_url"`
		ForwardHMACKey   *string `json:"forward_hmac_key"`
		IsActive         *bool   `json:"is_active"`
	}

	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body", requestID(r))
		return
	}

	if input.Name == nil && input.TelegramChatID == nil && input.TelegramTemplate == nil && input.ForwardURL == nil && input.ForwardHMACKey == nil && input.IsActive == nil {
		writeError(w, http.StatusBadRequest, "at least one field is required", requestID(r))
		return
	}

	current, err := s.repo.GetSourceByPublicID(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "source not found", requestID(r))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}

	name := current.Name
	if input.Name != nil {
		name = strings.TrimSpace(*input.Name)
		if name == "" || len(name) > 120 {
			writeError(w, http.StatusBadRequest, "source name must be shorter than 120 characters", requestID(r))
			return
		}
	}

	chat := current.TelegramChatID
	if input.TelegramChatID != nil {
		chat = normalizePtr(input.TelegramChatID)
	}

	telegramTemplate := current.TelegramTemplate
	if input.TelegramTemplate != nil {
		var ok bool
		telegramTemplate, ok = normalizeTelegramTemplate(w, r, input.TelegramTemplate)
		if !ok {
			return
		}
	}

	forwardURL := current.ForwardURL
	if input.ForwardURL != nil {
		var ok bool
		forwardURL, ok = normalizeForwardURL(w, r, input.ForwardURL)
		if !ok {
			return
		}
	}

	forwardHMACKey := current.ForwardHMACKey
	if input.ForwardHMACKey != nil {
		forwardHMACKey = normalizePtr(input.ForwardHMACKey)
	}

	active := current.IsActive
	if input.IsActive != nil {
		active = *input.IsActive
	}

	out, err := s.repo.UpdateSource(r.Context(), id, name, chat, telegramTemplate, forwardURL, forwardHMACKey, active)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}

	writeJSON(w, http.StatusOK, out)
}

func (s *Server) deleteSource(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "source id is required", requestID(r))
		return
	}

	if err := s.repo.DisableSource(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "source not found", requestID(r))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) rotateSourceToken(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "source id is required", requestID(r))
		return
	}

	token, err := security.RandomToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token generation failed", requestID(r))
		return
	}

	out, err := s.repo.RotateSourceToken(r.Context(), id, token)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "source not found", requestID(r))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}

	writeJSON(w, http.StatusOK, out)
}

func (s *Server) receiveWebhook(w http.ResponseWriter, r *http.Request) {
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
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes))
	decoder.UseNumber()

	if err := decoder.Decode(&payload); err != nil || len(payload) == 0 {
		writeError(w, http.StatusBadRequest, "payload must be a non-empty json object", requestID(r))
		return
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "payload must contain a single json object", requestID(r))
		return
	}

	source, err := s.repo.FindSourceByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
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

	event, isNew, err := s.repo.InsertEvent(r.Context(), source, payload, canonical, clientIP(r), r.UserAgent())
	if err != nil {
		s.log.Error("insert event failed", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}

	if isNew && s.notifier != nil {
		s.notifier.Notify(event, source)
	}

	writeJSON(w, http.StatusAccepted, event)
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseEventFilter(w, r)
	if !ok {
		return
	}

	items, err := s.repo.ListEvents(r.Context(), filter)
	if err != nil {
		s.log.Error("list events failed", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

func (s *Server) getEvent(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "event id is required", requestID(r))
		return
	}

	event, err := s.repo.GetEvent(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "event not found", requestID(r))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}

	writeJSON(w, http.StatusOK, event)
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.repo.Stats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}

	deliveryStats, err := s.repo.DeliveryStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}

	stats.Deliveries = deliveryStats
	writeJSON(w, http.StatusOK, stats)
}

func parseEventFilter(w http.ResponseWriter, r *http.Request) (domain.EventFilter, bool) {
	limit := queryInt(r, "limit", 50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	offset := queryInt(r, "offset", 0)
	if offset < 0 {
		offset = 0
	}

	filter := domain.EventFilter{
		Limit:  limit,
		Offset: offset,
		Source: strings.TrimSpace(r.URL.Query().Get("source")),
		Origin: strings.TrimSpace(r.URL.Query().Get("origin")),
	}

	filter.EventType = strings.TrimSpace(r.URL.Query().Get("type"))
	if filter.EventType == "" {
		filter.EventType = strings.TrimSpace(r.URL.Query().Get("event_type"))
	}

	if value := strings.TrimSpace(r.URL.Query().Get("duplicate")); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "duplicate must be boolean", requestID(r))
			return domain.EventFilter{}, false
		}
		filter.Duplicate = &parsed
	}

	if value := strings.TrimSpace(r.URL.Query().Get("from")); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "from must be RFC3339 timestamp", requestID(r))
			return domain.EventFilter{}, false
		}
		filter.From = &parsed
	}

	if value := strings.TrimSpace(r.URL.Query().Get("to")); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "to must be RFC3339 timestamp", requestID(r))
			return domain.EventFilter{}, false
		}
		filter.To = &parsed
	}

	return filter, true
}

func (s *Server) webhookRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientIP(r) + ":" + strings.TrimSpace(r.PathValue("token"))
		allowed, retryAfter := s.webhookLimiter.Allow(key)
		if !allowed {
			retrySeconds := int((retryAfter + time.Second - time.Nanosecond) / time.Second)
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded", requestID(r))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) admin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed, retryAfter := s.adminLimiter.Allow(clientIP(r))
		if !allowed {
			retrySeconds := int((retryAfter + time.Second - time.Nanosecond) / time.Second)
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
			writeError(w, http.StatusTooManyRequests, "admin rate limit exceeded", requestID(r))
			return
		}

		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-API-Key")), []byte(s.cfg.AdminAPIKey)) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized", requestID(r))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		h.Set("Cache-Control", "no-store")

		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = security.RandomHex(16)
		}

		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.log.Error("panic recovered", slog.Any("panic", recovered), slog.String("path", safeLogPath(r)))
				writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		s.log.Info(
			"http request",
			slog.String("method", r.Method),
			slog.String("path", safeLogPath(r)),
			slog.Int("status", rw.status),
			slog.Duration("duration", time.Since(start)),
		)
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
	writeJSON(w, status, map[string]string{
		"error":      message,
		"request_id": requestID,
	})
}

func normalizePtr(value *string) *string {
	if value == nil {
		return nil
	}
	return stringPtr(*value)
}

func normalizeTelegramTemplate(w http.ResponseWriter, r *http.Request, value *string) (*string, bool) {
	if value == nil {
		return nil, true
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, true
	}
	if len(trimmed) > 4000 {
		writeError(w, http.StatusBadRequest, "telegram_template must be shorter than 4000 characters", requestID(r))
		return nil, false
	}
	return &trimmed, true
}

func normalizeForwardURL(w http.ResponseWriter, r *http.Request, value *string) (*string, bool) {
	if value == nil {
		return nil, true
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, true
	}
	if len(trimmed) > 2000 {
		writeError(w, http.StatusBadRequest, "forward_url must be shorter than 2000 characters", requestID(r))
		return nil, false
	}
	if !strings.HasPrefix(trimmed, "https://") && !strings.HasPrefix(trimmed, "http://") {
		writeError(w, http.StatusBadRequest, "forward_url must start with http:// or https://", requestID(r))
		return nil, false
	}
	return &trimmed, true
}

func stringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
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
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func safeLogPath(r *http.Request) string {
	path := r.URL.Path
	if strings.HasPrefix(path, "/v1/hooks/") {
		return "/v1/hooks/<redacted>"
	}
	return path
}
