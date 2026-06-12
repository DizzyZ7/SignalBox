package domain

import (
	"encoding/json"
	"time"
)

type DeliveryJob struct {
	ID            int64           `json:"-"`
	PublicID      string          `json:"id"`
	EventID       int64           `json:"event_id"`
	Channel       string          `json:"channel"`
	Destination   string          `json:"destination"`
	Payload       json.RawMessage `json:"payload"`
	Status        string          `json:"status"`
	Attempts      int             `json:"attempts"`
	MaxAttempts   int             `json:"max_attempts"`
	NextAttemptAt time.Time       `json:"next_attempt_at"`
	LockedUntil   *time.Time      `json:"locked_until,omitempty"`
	LockedBy      *string         `json:"locked_by,omitempty"`
	LastError     *string         `json:"last_error,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	SentAt        *time.Time      `json:"sent_at,omitempty"`
}

type DeliveryJobFilter struct {
	Limit   int
	Offset  int
	Status  string
	Channel string
	Source  string
	EventID string
}
