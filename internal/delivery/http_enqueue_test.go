package delivery

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DizzyZ7/SignalBox/internal/domain"
)

func TestEnqueueHTTPUsesProviderPayload(t *testing.T) {
	store := &enqueueCaptureStore{}
	n := &TelegramNotifier{store: store, maxAttempts: 3}
	forwardURL := " https://example.com/provider-hook "
	event := domain.Event{ID: 42, Payload: []byte(`{"type":"provider.test"}`)}
	source := domain.Source{PublicID: "src_test", ForwardURL: &forwardURL}

	n.enqueueHTTP(event, source)

	if !store.enqueued {
		t.Fatal("expected delivery job to be enqueued")
	}
	if store.eventID != event.ID {
		t.Fatalf("eventID = %d, want %d", store.eventID, event.ID)
	}
	if store.channel != HTTPForwardChannel {
		t.Fatalf("channel = %q, want %q", store.channel, HTTPForwardChannel)
	}
	if store.destination != "https://example.com/provider-hook" {
		t.Fatalf("destination = %q", store.destination)
	}
	if string(store.payload) != string(event.Payload) {
		t.Fatalf("payload = %s, want %s", store.payload, event.Payload)
	}
	if store.maxAttempts != 3 {
		t.Fatalf("maxAttempts = %d, want 3", store.maxAttempts)
	}
	if len(store.attempts) != 1 || store.attempts[0] != "http:queued" {
		t.Fatalf("attempts = %#v", store.attempts)
	}
}

type enqueueCaptureStore struct {
	enqueued    bool
	eventID     int64
	channel     string
	destination string
	payload     json.RawMessage
	maxAttempts int
	attempts    []string
}

func (s *enqueueCaptureStore) EnqueueDeliveryJob(_ context.Context, eventID int64, channel, destination string, payload json.RawMessage, maxAttempts int) error {
	s.enqueued = true
	s.eventID = eventID
	s.channel = channel
	s.destination = destination
	s.payload = append(json.RawMessage(nil), payload...)
	s.maxAttempts = maxAttempts
	return nil
}

func (s *enqueueCaptureStore) ClaimDeliveryJobs(context.Context, string, int, time.Duration) ([]domain.DeliveryJob, error) {
	return nil, nil
}

func (s *enqueueCaptureStore) MarkDeliveryJobSent(context.Context, int64) error {
	return nil
}

func (s *enqueueCaptureStore) MarkDeliveryJobFailed(context.Context, int64, string, time.Duration) error {
	return nil
}

func (s *enqueueCaptureStore) RecordDeliveryAttempt(_ context.Context, _ int64, channel, status string, _ *string) {
	s.attempts = append(s.attempts, channel+":"+status)
}

func (s *enqueueCaptureStore) GetEventByInternalID(context.Context, int64) (domain.Event, error) {
	return domain.Event{}, nil
}
