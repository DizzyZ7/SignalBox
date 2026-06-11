package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/DizzyZ7/SignalBox/internal/domain"
	"github.com/DizzyZ7/SignalBox/internal/storage"
)

type sourceNotifier interface {
	CanNotify(source domain.Source) bool
}

func (s *Server) replayEvent(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "event id is required", requestID(r))
		return
	}
	if s.notifier == nil {
		writeError(w, http.StatusServiceUnavailable, "notifier is not configured", requestID(r))
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
	if event.Source == nil {
		writeError(w, http.StatusInternalServerError, "event source is missing", requestID(r))
		return
	}
	if !event.Source.IsActive {
		writeError(w, http.StatusConflict, "event source is inactive", requestID(r))
		return
	}
	if checker, ok := s.notifier.(sourceNotifier); ok && !checker.CanNotify(*event.Source) {
		writeError(w, http.StatusServiceUnavailable, "notifier is not ready for this source", requestID(r))
		return
	}
	s.notifier.Notify(event, *event.Source)
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued", "event": event})
}
