package delivery

import (
	"context"
	"errors"
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
