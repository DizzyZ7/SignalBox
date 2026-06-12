package delivery

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/DizzyZ7/SignalBox/internal/domain"
)

const HTTPForwardChannel = "http"

type HTTPForwardProvider struct{}

func NewHTTPForwardProvider() *HTTPForwardProvider {
	return &HTTPForwardProvider{}
}

func (p *HTTPForwardProvider) Channel() string {
	return HTTPForwardChannel
}

func (p *HTTPForwardProvider) CanDeliver(source domain.Source) bool {
	return source.ForwardURL != nil && strings.TrimSpace(*source.ForwardURL) != ""
}

func (p *HTTPForwardProvider) BuildJobPayload(_ context.Context, event domain.Event, source domain.Source) (ProviderJob, error) {
	if !p.CanDeliver(source) {
		return ProviderJob{}, ErrProviderNotConfigured
	}

	return ProviderJob{
		Channel:     p.Channel(),
		Destination: strings.TrimSpace(*source.ForwardURL),
		Payload:     json.RawMessage(event.Payload),
	}, nil
}

func (p *HTTPForwardProvider) Deliver(_ context.Context, _ domain.DeliveryJob, _ domain.Event) ProviderResult {
	return ProviderFailed("http provider dispatch is not wired yet", false, 0)
}
