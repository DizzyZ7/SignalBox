package delivery

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DizzyZ7/SignalBox/internal/domain"
)

func TestHTTPForwardProviderBuildJobPayload(t *testing.T) {
	provider := NewHTTPForwardProvider()
	forwardURL := " https://example.com/signalbox "
	event := domain.Event{Payload: []byte(`{"type":"lead.created"}`)}
	source := domain.Source{ForwardURL: &forwardURL}

	job, err := provider.BuildJobPayload(context.Background(), event, source)
	if err != nil {
		t.Fatalf("BuildJobPayload returned error: %v", err)
	}
	if job.Channel != HTTPForwardChannel {
		t.Fatalf("channel = %q, want %q", job.Channel, HTTPForwardChannel)
	}
	if job.Destination != "https://example.com/signalbox" {
		t.Fatalf("destination = %q", job.Destination)
	}
	if string(job.Payload) != string(event.Payload) {
		t.Fatalf("payload = %s, want %s", job.Payload, event.Payload)
	}
}

func TestHTTPForwardProviderNotConfigured(t *testing.T) {
	provider := NewHTTPForwardProvider()
	_, err := provider.BuildJobPayload(context.Background(), domain.Event{}, domain.Source{})
	if !errors.Is(err, ErrProviderNotConfigured) {
		t.Fatalf("error = %v, want ErrProviderNotConfigured", err)
	}
}

func TestHTTPForwardProviderRejectsUnsafeDestination(t *testing.T) {
	provider := NewHTTPForwardProvider()
	result := provider.Deliver(context.Background(), domain.DeliveryJob{
		Destination: "http://127.0.0.1:8080/hook",
		Payload:     []byte(`{}`),
	}, domain.Event{})

	if result.Sent {
		t.Fatal("expected unsafe destination to fail")
	}
	if result.Retryable {
		t.Fatal("unsafe destination should not be retryable")
	}
	if !strings.Contains(result.Error, "unsafe http forward destination") {
		t.Fatalf("error = %q", result.Error)
	}
}
