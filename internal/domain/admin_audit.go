package domain

import (
	"encoding/json"
	"time"
)

type AdminAuditEvent struct {
	ID         int64           `json:"-"`
	PublicID   string          `json:"id"`
	RequestID  string          `json:"request_id"`
	Action     string          `json:"action"`
	Method     string          `json:"method"`
	Path       string          `json:"path"`
	TargetType *string         `json:"target_type,omitempty"`
	TargetID   *string         `json:"target_id,omitempty"`
	StatusCode int             `json:"status_code"`
	IP         *string         `json:"ip,omitempty"`
	UserAgent  *string         `json:"user_agent,omitempty"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

type AdminAuditEventFilter struct {
	Limit      int
	Offset     int
	Action     string
	TargetType string
	TargetID   string
}
