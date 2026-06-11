package httpapi

import (
	"net/http"
	"strings"
	"time"

	appmetrics "github.com/DizzyZ7/SignalBox/internal/metrics"
	"github.com/DizzyZ7/SignalBox/internal/storage"
)

func WithMetrics(next http.Handler, repo *storage.Repository) http.Handler {
	registry := appmetrics.New()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			writeMetrics(w, r, repo, registry)
			return
		}

		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)

		path := safeLogPath(r)
		registry.ObserveHTTPRequest(r.Method, path, rw.status, time.Since(start))
		if strings.HasPrefix(r.URL.Path, "/v1/hooks/") && rw.status == http.StatusAccepted {
			registry.IncWebhookEvent("unknown", "unknown", false)
		}
	})
}

func writeMetrics(w http.ResponseWriter, r *http.Request, repo *storage.Repository, registry *appmetrics.Metrics) {
	stats, err := repo.Stats(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "metrics unavailable", requestID(r))
		return
	}
	deliveries, err := repo.DeliveryStats(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "metrics unavailable", requestID(r))
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	registry.WritePrometheus(w, appmetrics.DatabaseSnapshot{
		TotalEvents:     stats.TotalEvents,
		UniqueEvents:    stats.UniqueEvents,
		DuplicateEvents: stats.DuplicateEvents,
		Events24h:       stats.Events24h,
		Sources:         stats.Sources,
		ActiveSources:   stats.ActiveSources,
		DeliveryJobsByStatus: []appmetrics.DeliveryStatus{
			{Status: "pending", Count: deliveries.Pending},
			{Status: "processing", Count: deliveries.Processing},
			{Status: "sent", Count: deliveries.Sent},
			{Status: "failed", Count: deliveries.Failed},
		},
	})
}
