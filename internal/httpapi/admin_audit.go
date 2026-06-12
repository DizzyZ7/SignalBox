package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/DizzyZ7/SignalBox/internal/domain"
)

func (s *Server) listAdminAuditEvents(w http.ResponseWriter, r *http.Request) {
	filter := domain.AdminAuditEventFilter{
		Limit:      queryInt(r, "limit", 50),
		Offset:     queryInt(r, "offset", 0),
		Action:     strings.TrimSpace(r.URL.Query().Get("action")),
		TargetType: strings.TrimSpace(r.URL.Query().Get("target_type")),
		TargetID:   strings.TrimSpace(r.URL.Query().Get("target_id")),
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	items, err := s.repo.ListAdminAuditEvents(r.Context(), filter)
	if err != nil {
		s.log.Error("list admin audit events failed", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "internal error", requestID(r))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": filter.Limit, "offset": filter.Offset})
}

func (s *Server) recordAdminAudit(r *http.Request, statusCode int) {
	if s == nil || s.repo == nil || r == nil {
		return
	}
	if r.Method == http.MethodGet {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	targetType, targetID := auditTarget(r)
	metadata, _ := json.Marshal(map[string]any{
		"query": r.URL.RawQuery,
	})

	if err := s.repo.RecordAdminAuditEvent(ctx, domain.AdminAuditEvent{
		RequestID:  requestID(r),
		Action:     auditAction(r),
		Method:     r.Method,
		Path:       r.URL.Path,
		TargetType: stringPtr(targetType),
		TargetID:   stringPtr(targetID),
		StatusCode: statusCode,
		IP:         stringPtr(clientIP(r)),
		UserAgent:  stringPtr(r.UserAgent()),
		Metadata:   metadata,
	}); err != nil {
		s.log.Error("record admin audit event failed", "error", err.Error(), "path", safeLogPath(r), "status", statusCode)
	}
}

func auditAction(r *http.Request) string {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] != "v1" {
		return strings.ToLower(r.Method) + ":unknown"
	}

	resource := parts[1]
	if len(parts) >= 4 {
		switch parts[3] {
		case "rotate-token":
			return resource + ":rotate-token"
		case "test-event":
			return resource + ":test-event"
		case "replay":
			return resource + ":replay"
		case "retry":
			return resource + ":retry"
		}
	}

	switch r.Method {
	case http.MethodPost:
		return resource + ":create"
	case http.MethodPatch:
		return resource + ":update"
	case http.MethodDelete:
		return resource + ":delete"
	default:
		return strings.ToLower(r.Method) + ":" + resource
	}
}

func auditTarget(r *http.Request) (string, string) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[0] != "v1" {
		return "", ""
	}
	resource := parts[1]
	targetID := strings.TrimSpace(parts[2])
	if targetID == "" {
		return resource, ""
	}
	return strings.TrimSuffix(resource, "s"), targetID
}
