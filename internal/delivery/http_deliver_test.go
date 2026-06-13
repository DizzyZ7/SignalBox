package delivery

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/DizzyZ7/SignalBox/internal/domain"
)

func TestDeliverHTTPDispatchesProviderFailure(t *testing.T) {
	store := &httpDeliverCaptureStore{
		event: domain.Event{
			ID:       11,
			PublicID: "evt_test",
			Source:   &domain.Source{PublicID: "src_test"},
		},
	}
	n := &TelegramNotifier{store: store}
	job := domain.DeliveryJob{
		ID:          7,
		PublicID:    "del_test",
		EventID:     11,
		Channel:     HTTPForwardChannel,
		Destination: "http://127.0.0.1:8080/hook",
		Payload:     []byte(`{}`),
	}

	n.deliverHTTP(context.Background(), job)

	if store.sent {
		t.Fatal("unsafe destination must not be marked as sent")
	}
	if !store.failed {
		t.Fatal("expected failed delivery job")
	}
	if store.retryDelay != 0 {
		t.Fatalf("retryDelay = %s, want 0", store.retryDelay)
	}
	if !strings.Contains(store.failText, "unsafe http forward destination") {
		t.Fatalf("failText = %q", store.failText)
	}
	if len(store.attempts) != 1 || store.attempts[0] != "http:failed" {
		t.Fatalf("attempts = %#v", store.attempts)
	}
}

type httpDeliverCaptureStore struct {
	event      domain.Event
	sent       bool
	failed     bool
	failText   string
	retryDelay time.Duration
	attempts   []string
}

func (s *httpDeliverCaptureStore) EnqueueDeliveryJob(context.Context, int64, string, string, json.RawMessage, int) error {
	return nil
}

func (s *httpDeliverCaptureStore) ClaimDeliveryJobs(context.Context, string, int, time.Duration) ([]domain.DeliveryJob, error) {
	return nil, nil
}

func (s *httpDeliverCaptureStore) MarkDeliveryJobSent(context.Context, int64) error {
	s.sent = true
	return nil
}

func (s *httpDeliverCaptureStore) MarkDeliveryJobFailed(_ context.Context, _ int64, errText string, retryAfter time.Duration) error {
	s.failed = true
	s.failText = errText
	s.retryDelay = retryAfter
	return nil
}

func (s *httpDeliverCaptureStore) RecordDeliveryAttempt(_ context.Context, _ int64, channel, status string, _ *string) {
	s.attempts = append(s.attempts, channel+":"+status)
}

func (s *httpDeliverCaptureStore) GetEventByInternalID(context.Context, int64) (domain.Event, error) {
	return s.event, nil
}
