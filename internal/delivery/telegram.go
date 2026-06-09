package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/DizzyZ7/SignalBox/internal/domain"
)

type DeliveryRecorder interface {
	RecordDeliveryAttempt(ctx context.Context, eventID int64, channel, status string, errText *string)
}

type TelegramNotifier struct {
	botToken      string
	defaultChatID string
	client        *http.Client
	recorder      DeliveryRecorder
	log           *slog.Logger
}

func NewTelegramNotifier(botToken, defaultChatID string, recorder DeliveryRecorder, log *slog.Logger) *TelegramNotifier {
	return &TelegramNotifier{
		botToken:      strings.TrimSpace(botToken),
		defaultChatID: strings.TrimSpace(defaultChatID),
		client:        &http.Client{Timeout: 8 * time.Second},
		recorder:      recorder,
		log:           log,
	}
}

func (n *TelegramNotifier) Notify(event domain.Event, source domain.Source) {
	if n == nil || n.botToken == "" {
		return
	}
	chatID := n.defaultChatID
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
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+n.botToken+"/sendMessage", bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := n.client.Do(req)
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
		if n.recorder != nil {
			n.recorder.RecordDeliveryAttempt(context.Background(), event.ID, "telegram", status, errText)
		}
	}()
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
