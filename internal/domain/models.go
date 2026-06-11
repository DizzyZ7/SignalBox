package domain

import (
	"encoding/json"
	"time"
)

type Source struct {
	ID                      int64     `json:"-"`
	PublicID                string    `json:"id"`
	Name                    string    `json:"name"`
	TokenHash               string    `json:"-"`
	TokenHint               string    `json:"token_hint"`
	TelegramChatID          *string   `json:"telegram_chat_id,omitempty"`
	ForwardURL              *string   `json:"forward_url,omitempty"`
	ForwardSigningSecret    *string   `json:"-"`
	ForwardSigningSecretSet bool      `json:"forward_signing_secret_set"`
	IsActive                bool      `json:"is_active"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
	Token                   string    `json:"token,omitempty"`
}

type Event struct {
	ID          int64           `json:"-"`
	PublicID    string          `json:"id"`
	SourceID    int64           `json:"-"`
	Source      *Source         `json:"source,omitempty"`
	EventType   *string         `json:"event_type,omitempty"`
	Origin      *string         `json:"origin,omitempty"`
	ExternalID  *string         `json:"external_id,omitempty"`
	Payload     json.RawMessage `json:"payload"`
	PayloadHash string          `json:"payload_hash"`
	IP          *string         `json:"ip,omitempty"`
	UserAgent   *string         `json:"user_agent,omitempty"`
	IsDuplicate bool            `json:"is_duplicate"`
	CreatedAt   time.Time       `json:"created_at"`
}

type StatRow struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

type DeliveryStats struct {
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Sent       int64 `json:"sent"`
	Failed     int64 `json:"failed"`
}

type StatsResponse struct {
	TotalEvents     int64         `json:"total_events"`
	UniqueEvents    int64         `json:"unique_events"`
	DuplicateEvents int64         `json:"duplicate_events"`
	Events24h       int64         `json:"events_24h"`
	Sources         int64         `json:"sources"`
	ActiveSources   int64         `json:"active_sources"`
	Deliveries      DeliveryStats `json:"deliveries"`
	ByType          []StatRow     `json:"by_type"`
	ByOrigin        []StatRow     `json:"by_origin"`
}

type EventFilter struct {
	Limit           int
	Offset          int
	Source          string
	EventType       string
	Origin          string
	Duplicate       *bool
	From            *time.Time
	To              *time.Time
	CursorCreatedAt *time.Time
	CursorID        int64
}
