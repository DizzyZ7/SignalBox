package delivery

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/DizzyZ7/SignalBox/internal/domain"
)

var ErrProviderNotConfigured = errors.New("delivery provider is not configured for this source")

// Provider is the contract for a concrete outbound delivery integration.
//
// The current runtime still uses TelegramNotifier as the queue worker, but this
// contract defines the target shape for extracting Telegram, HTTP forwarding,
// Slack, Discord and email into separate providers without changing the durable
// delivery_jobs storage model.
type Provider interface {
	Channel() string
	CanDeliver(source domain.Source) bool
	BuildJobPayload(ctx context.Context, event domain.Event, source domain.Source) (ProviderJob, error)
	Deliver(ctx context.Context, job domain.DeliveryJob, event domain.Event) ProviderResult
}

type ProviderJob struct {
	Channel     string
	Destination string
	Payload     json.RawMessage
}

type ProviderResult struct {
	Sent       bool
	Retryable  bool
	RetryAfter int64
	Error      string
}

func ProviderSent() ProviderResult {
	return ProviderResult{Sent: true}
}

func ProviderFailed(errText string, retryable bool, retryAfterSeconds int64) ProviderResult {
	return ProviderResult{Sent: false, Retryable: retryable, RetryAfter: retryAfterSeconds, Error: errText}
}
