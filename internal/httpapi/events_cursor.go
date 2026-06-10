package httpapi

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DizzyZ7/SignalBox/internal/domain"
)

func (s *Server) listEventsCursor(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseEventFilter(w, r)
	if !ok {
		return
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("cursor")); raw != "" {
		createdAt, id, err := decodeEventCursor(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid cursor", requestID(r))
			return
		}
		filter.CursorCreatedAt = &createdAt
		filter.CursorID = id
	}
	items, err := s.repo.ListEventsCursor(r.Context(), filter)
	if err != nil {
		s.log.Error("list events failed", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": filter.Limit, "offset": filter.Offset, "next_cursor": nextEventCursor(items)})
}

func nextEventCursor(items []domain.Event) *string {
	if len(items) == 0 {
		return nil
	}
	last := items[len(items)-1]
	value := encodeEventCursor(last.CreatedAt, last.ID)
	return &value
}

func encodeEventCursor(createdAt time.Time, id int64) string {
	payload := fmt.Sprintf("%d:%d", createdAt.UTC().UnixNano(), id)
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func decodeEventCursor(value string) (time.Time, int64, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return time.Time{}, 0, err
	}
	parts := strings.Split(string(decoded), ":")
	if len(parts) != 2 {
		return time.Time{}, 0, errors.New("invalid cursor parts")
	}
	ns, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, 0, err
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return time.Time{}, 0, err
	}
	if ns <= 0 || id <= 0 {
		return time.Time{}, 0, errors.New("invalid cursor values")
	}
	return time.Unix(0, ns).UTC(), id, nil
}
