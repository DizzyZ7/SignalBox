package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/DizzyZ7/SignalBox/internal/domain"
	"github.com/DizzyZ7/SignalBox/internal/storage"
)

func (s *Server) listDeliveries(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseDeliveryJobFilter(w, r)
	if !ok {
		return
	}
	items, err := s.repo.ListDeliveryJobs(r.Context(), filter)
	if err != nil {
		s.log.Error("list delivery jobs failed", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": filter.Limit, "offset": filter.Offset})
}

func (s *Server) getDelivery(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "delivery id is required", requestID(r))
		return
	}
	job, err := s.repo.GetDeliveryJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "delivery job not found", requestID(r))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) retryDelivery(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "delivery id is required", requestID(r))
		return
	}
	job, err := s.repo.RetryDeliveryJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "delivery job not found or not retryable", requestID(r))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func parseDeliveryJobFilter(w http.ResponseWriter, r *http.Request) (domain.DeliveryJobFilter, bool) {
	limit := queryInt(r, "limit", 50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := queryInt(r, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "" && !allowedDeliveryStatus(status) {
		writeError(w, http.StatusBadRequest, "status must be pending, processing, sent or failed", requestID(r))
		return domain.DeliveryJobFilter{}, false
	}
	channel := strings.TrimSpace(r.URL.Query().Get("channel"))
	if len(channel) > 50 {
		writeError(w, http.StatusBadRequest, "channel must be shorter than 50 characters", requestID(r))
		return domain.DeliveryJobFilter{}, false
	}
	source := strings.TrimSpace(r.URL.Query().Get("source"))
	if len(source) > 120 {
		writeError(w, http.StatusBadRequest, "source must be shorter than 120 characters", requestID(r))
		return domain.DeliveryJobFilter{}, false
	}
	eventID := strings.TrimSpace(r.URL.Query().Get("event_id"))
	if len(eventID) > 120 {
		writeError(w, http.StatusBadRequest, "event_id must be shorter than 120 characters", requestID(r))
		return domain.DeliveryJobFilter{}, false
	}
	return domain.DeliveryJobFilter{Limit: limit, Offset: offset, Status: status, Channel: channel, Source: source, EventID: eventID}, true
}

func allowedDeliveryStatus(status string) bool {
	switch status {
	case "pending", "processing", "sent", "failed":
		return true
	default:
		return false
	}
}
