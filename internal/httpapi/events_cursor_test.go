package httpapi

import (
	"testing"
	"time"
)

func TestEventCursorRoundTrip(t *testing.T) {
	createdAt := time.Date(2026, 6, 10, 12, 30, 15, 123456789, time.UTC)
	cursor := encodeEventCursor(createdAt, 42)
	gotCreatedAt, gotID, err := decodeEventCursor(cursor)
	if err != nil {
		t.Fatalf("decodeEventCursor() error = %v", err)
	}
	if !gotCreatedAt.Equal(createdAt) {
		t.Fatalf("createdAt = %s, want %s", gotCreatedAt, createdAt)
	}
	if gotID != 42 {
		t.Fatalf("id = %d, want 42", gotID)
	}
}

func TestEventCursorRejectsInvalidValue(t *testing.T) {
	if _, _, err := decodeEventCursor("bad-cursor"); err == nil {
		t.Fatal("decodeEventCursor() expected error")
	}
}
