package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/DizzyZ7/SignalBox/internal/delivery"
	"github.com/DizzyZ7/SignalBox/internal/domain"
	"github.com/DizzyZ7/SignalBox/internal/security"
)

func (s *Server) previewTelegramTemplate(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SourceName       string         `json:"source_name"`
		TelegramTemplate string         `json:"telegram_template"`
		EventType        string         `json:"event_type"`
		Origin           string         `json:"origin"`
		ExternalID       string         `json:"external_id"`
		Payload          map[string]any `json:"payload"`
	}

	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body", requestID(r))
		return
	}

	templateText := strings.TrimSpace(input.TelegramTemplate)
	if templateText == "" {
		writeError(w, http.StatusBadRequest, "telegram_template is required", requestID(r))
		return
	}
	if len(templateText) > 4000 {
		writeError(w, http.StatusBadRequest, "telegram_template must be shorter than 4000 characters", requestID(r))
		return
	}

	sourceName := strings.TrimSpace(input.SourceName)
	if sourceName == "" {
		sourceName = "Preview source"
	}
	if len(sourceName) > 120 {
		writeError(w, http.StatusBadRequest, "source_name must be shorter than 120 characters", requestID(r))
		return
	}

	payload := input.Payload
	if payload == nil {
		payload = map[string]any{"type": "preview.event"}
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, "payload must be valid json", requestID(r))
		return
	}

	eventType := stringPtr(strings.TrimSpace(input.EventType))
	if eventType == nil {
		eventType = stringPtr("preview.event")
	}
	origin := stringPtr(strings.TrimSpace(input.Origin))
	externalID := stringPtr(strings.TrimSpace(input.ExternalID))

	event := domain.Event{
		PublicID:    "preview-" + security.RandomHex(8),
		EventType:   eventType,
		Origin:      origin,
		ExternalID:  externalID,
		Payload:     payloadBytes,
		CreatedAt:   time.Now().UTC(),
		IsDuplicate: false,
	}
	source := domain.Source{
		PublicID: "preview-source",
		Name:     sourceName,
	}

	text, err := delivery.RenderTelegramTemplate(templateText, event, source)
	if err != nil {
		writeError(w, http.StatusBadRequest, "template render failed", requestID(r))
		return
	}
	if strings.TrimSpace(text) == "" {
		writeError(w, http.StatusBadRequest, "template rendered empty text", requestID(r))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"text":       text,
		"event_id":   event.PublicID,
		"source_id":  source.PublicID,
		"event_type": *eventType,
	})
}
