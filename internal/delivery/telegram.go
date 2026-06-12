package delivery

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/DizzyZ7/SignalBox/internal/domain"
	"github.com/DizzyZ7/SignalBox/internal/security"
)

type DeliveryStore interface {
	EnqueueDeliveryJob(ctx context.Context, eventID int64, channel, destination string, payload json.RawMessage, maxAttempts int) error
	ClaimDeliveryJobs(ctx context.Context, workerID string, limit int, lockFor time.Duration) ([]domain.DeliveryJob, error)
	MarkDeliveryJobSent(ctx context.Context, jobID int64) error
	MarkDeliveryJobFailed(ctx context.Context, jobID int64, errText string, retryAfter time.Duration) error
	RecordDeliveryAttempt(ctx context.Context, eventID int64, channel, status string, errText *string)
	GetEventByInternalID(ctx context.Context, eventID int64) (domain.Event, error)
}

type TelegramNotifier struct {
	botToken      string
	defaultChatID string
	client        *http.Client
	store         DeliveryStore
	log           *slog.Logger
	maxAttempts   int
}

func NewTelegramNotifier(botToken, defaultChatID string, store DeliveryStore, log *slog.Logger, maxAttempts int) *TelegramNotifier {
	if maxAttempts <= 0 {
		maxAttempts = 8
	}
	return &TelegramNotifier{
		botToken:      strings.TrimSpace(botToken),
		defaultChatID: strings.TrimSpace(defaultChatID),
		client:        &http.Client{Timeout: 10 * time.Second},
		store:         store,
		log:           log,
		maxAttempts:   maxAttempts,
	}
}

func (n *TelegramNotifier) Enabled() bool {
	return n != nil && n.store != nil
}

func (n *TelegramNotifier) CanNotify(source domain.Source) bool {
	if n == nil {
		return false
	}
	return n.canTelegramNotify(source) || n.canHTTPForward(source)
}

func (n *TelegramNotifier) canTelegramNotify(source domain.Source) bool {
	return n != nil && n.botToken != "" && n.store != nil && n.chatIDFor(source) != ""
}

func (n *TelegramNotifier) canHTTPForward(source domain.Source) bool {
	return n != nil && n.store != nil && source.ForwardURL != nil && strings.TrimSpace(*source.ForwardURL) != ""
}

func (n *TelegramNotifier) Notify(event domain.Event, source domain.Source) {
	if n == nil || n.store == nil {
		return
	}
	if n.canTelegramNotify(source) {
		n.enqueueTelegram(event, source)
	}
	if n.canHTTPForward(source) {
		n.enqueueHTTP(event, source)
	}
}

func (n *TelegramNotifier) enqueueTelegram(event domain.Event, source domain.Source) {
	chatID := n.chatIDFor(source)
	text := formatTelegramMessage(event, source)
	if source.TelegramTemplate != nil && strings.TrimSpace(*source.TelegramTemplate) != "" {
		customText, err := RenderTelegramTemplate(*source.TelegramTemplate, event, source)
		if err != nil {
			n.log.Error("telegram template render failed", slog.Int64("event_id", event.ID), slog.String("source_id", source.PublicID), slog.String("error", err.Error()))
			n.store.RecordDeliveryAttempt(context.Background(), event.ID, "telegram", "template_failed", stringPtr(err.Error()))
		} else if strings.TrimSpace(customText) != "" {
			text = customText
		}
	}
	body, err := json.Marshal(map[string]any{"chat_id": chatID, "text": text, "parse_mode": "HTML", "disable_web_page_preview": true})
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := n.store.EnqueueDeliveryJob(ctx, event.ID, "telegram", chatID, body, n.maxAttempts); err != nil {
		n.log.Error("enqueue telegram delivery failed", slog.Int64("event_id", event.ID), slog.String("error", err.Error()))
		n.store.RecordDeliveryAttempt(context.Background(), event.ID, "telegram", "enqueue_failed", stringPtr(err.Error()))
		return
	}
	n.store.RecordDeliveryAttempt(context.Background(), event.ID, "telegram", "queued", nil)
}

func (n *TelegramNotifier) enqueueHTTP(event domain.Event, source domain.Source) {
	destination := strings.TrimSpace(*source.ForwardURL)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := n.store.EnqueueDeliveryJob(ctx, event.ID, "http", destination, event.Payload, n.maxAttempts); err != nil {
		n.log.Error("enqueue http delivery failed", slog.Int64("event_id", event.ID), slog.String("error", err.Error()))
		n.store.RecordDeliveryAttempt(context.Background(), event.ID, "http", "enqueue_failed", stringPtr(err.Error()))
		return
	}
	n.store.RecordDeliveryAttempt(context.Background(), event.ID, "http", "queued", nil)
}

func (n *TelegramNotifier) chatIDFor(source domain.Source) string {
	if n == nil {
		return ""
	}
	chatID := n.defaultChatID
	if source.TelegramChatID != nil && strings.TrimSpace(*source.TelegramChatID) != "" {
		chatID = strings.TrimSpace(*source.TelegramChatID)
	}
	return chatID
}

func (n *TelegramNotifier) Start(ctx context.Context, interval time.Duration, batchSize int, lockFor time.Duration) {
	if !n.Enabled() {
		return
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 10
	}
	if lockFor <= 0 {
		lockFor = time.Minute
	}
	workerID := fmt.Sprintf("delivery-%d", time.Now().UnixNano())

	go func() {
		n.log.Info("delivery worker started", slog.String("worker_id", workerID), slog.Duration("interval", interval), slog.Int("batch_size", batchSize))
		n.processBatch(ctx, workerID, batchSize, lockFor)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				n.log.Info("delivery worker stopped", slog.String("worker_id", workerID))
				return
			case <-ticker.C:
				n.processBatch(ctx, workerID, batchSize, lockFor)
			}
		}
	}()
}

func (n *TelegramNotifier) processBatch(ctx context.Context, workerID string, batchSize int, lockFor time.Duration) {
	claimCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	jobs, err := n.store.ClaimDeliveryJobs(claimCtx, workerID, batchSize, lockFor)
	cancel()
	if err != nil {
		n.log.Error("claim delivery jobs failed", slog.String("error", err.Error()))
		return
	}
	for _, job := range jobs {
		n.deliver(ctx, job)
	}
}

func (n *TelegramNotifier) deliver(ctx context.Context, job domain.DeliveryJob) {
	switch job.Channel {
	case "telegram":
		n.deliverTelegram(ctx, job)
	case "http":
		n.deliverHTTP(ctx, job)
	default:
		n.failJob(job, "unsupported delivery channel: "+job.Channel, 0)
	}
}

func (n *TelegramNotifier) deliverTelegram(ctx context.Context, job domain.DeliveryJob) {
	if n.botToken == "" {
		n.failJob(job, "telegram bot token is not configured", backoff(job.Attempts))
		return
	}
	deliveryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(deliveryCtx, http.MethodPost, "https://api.telegram.org/bot"+n.botToken+"/sendMessage", bytes.NewReader(job.Payload))
	if err != nil {
		n.failJob(job, err.Error(), backoff(job.Attempts))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		n.failJob(job, err.Error(), backoff(job.Attempts))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		errText := strings.TrimSpace(fmt.Sprintf("telegram status %d %s", resp.StatusCode, string(body)))
		n.failJob(job, errText, retryAfter(resp, backoff(job.Attempts)))
		return
	}

	if n.markSent(job) {
		n.store.RecordDeliveryAttempt(context.Background(), job.EventID, "telegram", "sent", nil)
	}
}

func (n *TelegramNotifier) deliverHTTP(ctx context.Context, job domain.DeliveryJob) {
	destination := strings.TrimSpace(job.Destination)
	if err := security.ValidateForwardURL(destination, false); err != nil {
		n.failJob(job, "unsafe http forward destination: "+err.Error(), 0)
		return
	}
	parsed, err := url.Parse(destination)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		n.failJob(job, "invalid http forward destination", 0)
		return
	}
	resolveCtx, resolveCancel := context.WithTimeout(ctx, 3*time.Second)
	if err := security.ValidateResolvedForwardHost(resolveCtx, parsed.Hostname(), false); err != nil {
		resolveCancel()
		n.failJob(job, "unsafe http forward destination: "+err.Error(), 0)
		return
	}
	resolveCancel()

	eventCtx, eventCancel := context.WithTimeout(ctx, 3*time.Second)
	event, err := n.store.GetEventByInternalID(eventCtx, job.EventID)
	eventCancel()
	if err != nil {
		n.failJob(job, "load event for forwarding failed: "+err.Error(), backoff(job.Attempts))
		return
	}

	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	deliveryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(deliveryCtx, http.MethodPost, destination, bytes.NewReader(job.Payload))
	if err != nil {
		n.failJob(job, err.Error(), backoff(job.Attempts))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "SignalBox/1.0")
	req.Header.Set("X-SignalBox-Event-ID", event.PublicID)
	req.Header.Set("X-SignalBox-Delivery-ID", job.PublicID)
	req.Header.Set("X-SignalBox-Timestamp", timestamp)
	if event.Source != nil {
		req.Header.Set("X-SignalBox-Source-ID", event.Source.PublicID)
	}
	if event.EventType != nil {
		req.Header.Set("X-SignalBox-Event-Type", *event.EventType)
	}
	if event.Source != nil && event.Source.ForwardHMACKey != nil && strings.TrimSpace(*event.Source.ForwardHMACKey) != "" {
		signature := signPayload(*event.Source.ForwardHMACKey, timestamp, job.Payload)
		req.Header.Set("X-SignalBox-Signature", "sha256="+signature)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		n.failJob(job, err.Error(), backoff(job.Attempts))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		errText := strings.TrimSpace(fmt.Sprintf("http forward status %d %s", resp.StatusCode, string(body)))
		n.failJob(job, errText, retryAfter(resp, backoff(job.Attempts)))
		return
	}

	if n.markSent(job) {
		n.store.RecordDeliveryAttempt(context.Background(), job.EventID, "http", "sent", nil)
	}
}

func (n *TelegramNotifier) markSent(job domain.DeliveryJob) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := n.store.MarkDeliveryJobSent(ctx, job.ID); err != nil {
		n.log.Error("mark delivery job sent failed", slog.Int64("job_id", job.ID), slog.String("error", err.Error()))
		return false
	}
	return true
}

func (n *TelegramNotifier) failJob(job domain.DeliveryJob, errText string, retryDelay time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := n.store.MarkDeliveryJobFailed(ctx, job.ID, errText, retryDelay); err != nil {
		n.log.Error("mark delivery job failed", slog.Int64("job_id", job.ID), slog.String("error", err.Error()))
		return
	}
	n.store.RecordDeliveryAttempt(context.Background(), job.EventID, job.Channel, "failed", stringPtr(errText))
}

func formatTelegramMessage(event domain.Event, source domain.Source) string {
	return fmt.Sprintf(
		"<b>SignalBox: new event</b>\n\n<b>Source:</b> %s\n<b>Type:</b> %s\n<b>ID:</b> <code>%s</code>\n<b>Time:</b> %s",
		html.EscapeString(source.Name),
		html.EscapeString(ptrValue(event.EventType, "unknown")),
		html.EscapeString(event.PublicID),
		event.CreatedAt.UTC().Format(time.RFC3339),
	)
}

func RenderTelegramTemplate(raw string, event domain.Event, source domain.Source) (string, error) {
	payload := make(map[string]any)
	if len(event.Payload) > 0 {
		_ = json.Unmarshal(event.Payload, &payload)
	}
	data := map[string]any{
		"Source": map[string]any{
			"ID":   source.PublicID,
			"Name": source.Name,
		},
		"Event": map[string]any{
			"ID":          event.PublicID,
			"Type":        ptrValue(event.EventType, "unknown"),
			"Origin":      ptrValue(event.Origin, ""),
			"ExternalID":  ptrValue(event.ExternalID, ""),
			"CreatedAt":   event.CreatedAt.UTC().Format(time.RFC3339),
			"IsDuplicate": event.IsDuplicate,
		},
		"Payload": payload,
	}
	funcs := template.FuncMap{
		"json": func(value any) string {
			encoded, err := json.Marshal(value)
			if err != nil {
				return ""
			}
			return string(encoded)
		},
		"html": html.EscapeString,
	}
	tmpl, err := template.New("telegram_message").Funcs(funcs).Option("missingkey=zero").Parse(raw)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	text := strings.TrimSpace(buf.String())
	if len(text) > 4096 {
		text = text[:4096]
	}
	return text, nil
}

func signPayload(key, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(key)))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func retryAfter(resp *http.Response, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if value == "" {
		return fallback
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func backoff(attempts int) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	if attempts > 6 {
		attempts = 6
	}
	delay := 30 * time.Second
	for i := 0; i < attempts; i++ {
		delay *= 2
	}
	if delay > 15*time.Minute {
		return 15 * time.Minute
	}
	return delay
}

func ptrValue(value *string, fallback string) string {
	if value == nil || *value == "" {
		return fallback
	}
	return *value
}

func stringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
