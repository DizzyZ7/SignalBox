package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/DizzyZ7/SignalBox/internal/domain"
	"github.com/DizzyZ7/SignalBox/internal/security"
)

const HTTPForwardChannel = "http"

type HTTPForwardProvider struct {
	client *http.Client
}

func NewHTTPForwardProvider() *HTTPForwardProvider {
	return &HTTPForwardProvider{client: &http.Client{Timeout: 10 * time.Second}}
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

func (p *HTTPForwardProvider) Deliver(ctx context.Context, job domain.DeliveryJob, event domain.Event) ProviderResult {
	client := p.client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	destination := strings.TrimSpace(job.Destination)
	if err := security.ValidateForwardURL(destination, false); err != nil {
		return ProviderFailed("unsafe http forward destination: "+err.Error(), false, 0)
	}

	parsed, err := url.Parse(destination)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ProviderFailed("invalid http forward destination", false, 0)
	}

	resolveCtx, resolveCancel := context.WithTimeout(ctx, 3*time.Second)
	defer resolveCancel()
	if err := security.ValidateResolvedForwardHost(resolveCtx, parsed.Hostname(), false); err != nil {
		return ProviderFailed("unsafe http forward destination: "+err.Error(), false, 0)
	}

	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	deliveryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(deliveryCtx, http.MethodPost, destination, bytes.NewReader(job.Payload))
	if err != nil {
		return ProviderFailed(err.Error(), true, int64(backoff(job.Attempts).Seconds()))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "SignalBox/1.0")
	req.Header.Set("X-SignalBox-Event-ID", event.PublicID)
	req.Header.Set("X-SignalBox-Delivery-ID", job.PublicID)
	req.Header.Set("X-SignalBox-Timestamp", timestamp)
	if event.Source != nil {
		req.Header.Set("X-SignalBox-Source-ID", event.Source.PublicID)
	}
	if event.EventType != nil {
		req.Header.Set("X-SignalBox-Event-Type", *event.EventType)
	}
	if event.Source != nil && event.Source.ForwardHMACKey != nil && strings.TrimSpace(*event.Source.ForwardHMACKey) != "" {
		signature := signPayload(*event.Source.ForwardHMACKey, timestamp, job.Payload)
		req.Header.Set("X-SignalBox-Signature", "sha256="+signature)
	}

	resp, err := client.Do(req)
	if err != nil {
		return ProviderFailed(err.Error(), true, int64(backoff(job.Attempts).Seconds()))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		errText := strings.TrimSpace(fmt.Sprintf("http forward status %d %s", resp.StatusCode, string(body)))
		return ProviderFailed(errText, true, int64(retryAfter(resp, backoff(job.Attempts)).Seconds()))
	}

	return ProviderSent()
}
