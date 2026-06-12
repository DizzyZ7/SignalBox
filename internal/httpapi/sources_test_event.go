package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/DizzyZ7/SignalBox/internal/storage"
)

func (s *Server) sendSourceTestEvent(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "source id is required", requestID(r))
		return
	}

	var input struct {
		Payload map[string]any `json:"payload"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body", requestID(r))
		return
	}

	source, err := s.repo.GetSourceByPublicID(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "source not found", requestID(r))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}
	if !source.IsActive {
		writeError(w, http.StatusBadRequest, "source is inactive", requestID(r))
		return
	}

	payload := input.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	if len(payload) == 0 {
		payload = map[string]any{
			"type":        "signalbox.test",
			"source":      "admin-ui",
			"external_id": "test-" + time.Now().UTC().Format("20060102T150405Z"),
			"message":     "Test event from SignalBox Admin UI",
		}
	}

	canonical, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, "payload must be valid json", requestID(r))
		return
	}

	event, isNew, err := s.repo.InsertEvent(r.Context(), source, payload, canonical, clientIP(r), r.UserAgent())
	if err != nil {
		s.log.Error("insert test event failed", "error", err.Error(), "source_id", source.PublicID)
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}

	queued := false
	if isNew && s.notifier != nil {
		s.notifier.Notify(event, source)
		queued = true
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status": "accepted",
		"is_new": isNew,
		"queued": queued,
		"event":  event,
	})
}
