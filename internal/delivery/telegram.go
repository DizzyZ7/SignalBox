package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DizzyZ7/SignalBox/internal/domain"
)

type DeliveryStore interface {
	EnqueueDeliveryJob(ctx context.Context, eventID int64, channel, destination string, payload json.RawMessage, maxAttempts int) error
	ClaimDeliveryJobs(ctx context.Context, workerID string, limit int, lockFor time.Duration) ([]domain.DeliveryJob, error)
	MarkDeliveryJobSent(ctx context.Context, jobID int64) error
	MarkDeliveryJobFailed(ctx context.Context, jobID int64, errText string, retryAfter time.Duration) error
	RecordDeliveryAttempt(ctx context.Context, eventID int64, channel, status string, errText *string)
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
		client:        &http.Client{Timeout: 8 * time.Second},
		store:         store,
		log:           log,
		maxAttempts:   maxAttempts,
	}
}

func (n *TelegramNotifier) Enabled() bool {
	return n != nil && n.botToken != "" && n.store != nil
}

func (n *TelegramNotifier) CanNotify(source domain.Source) bool {
	if !n.Enabled() {
		return false
	}
	return n.chatIDFor(source) != ""
}

func (n *TelegramNotifier) Notify(event domain.Event, source domain.Source) {
	if !n.CanNotify(source) {
		return
	}
	chatID := n.chatIDFor(source)

	text := formatTelegramMessage(event, source)
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
	workerID := fmt.Sprintf("telegram-%d", time.Now().UnixNano())

	go func() {
		n.log.Info("telegram delivery worker started", slog.String("worker_id", workerID), slog.Duration("interval", interval), slog.Int("batch_size", batchSize))
		n.processBatch(ctx, workerID, batchSize, lockFor)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				n.log.Info("telegram delivery worker stopped", slog.String("worker_id", workerID))
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
	if job.Channel != "telegram" {
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

	markCtx, markCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer markCancel()
	if err := n.store.MarkDeliveryJobSent(markCtx, job.ID); err != nil {
		n.log.Error("mark delivery job sent failed", slog.Int64("job_id", job.ID), slog.String("error", err.Error()))
		return
	}
	n.store.RecordDeliveryAttempt(context.Background(), job.EventID, "telegram", "sent", nil)
}

func (n *TelegramNotifier) failJob(job domain.DeliveryJob, errText string, retryDelay time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := n.store.MarkDeliveryJobFailed(ctx, job.ID, errText, retryDelay); err != nil {
		n.log.Error("mark delivery job failed", slog.Int64("job_id", job.ID), slog.String("error", err.Error()))
		return
	}
	n.store.RecordDeliveryAttempt(context.Background(), job.EventID, "telegram", "failed", stringPtr(errText))
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
