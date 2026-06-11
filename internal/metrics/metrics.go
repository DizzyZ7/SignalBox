package metrics

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Metrics struct {
	mu             sync.RWMutex
	httpRequests   map[labelKey]counter
	webhookEvents  map[labelKey]uint64
	startedAt      time.Time
}

type labelKey string

type counter struct {
	Count       uint64
	DurationSum float64
}

func New() *Metrics {
	return &Metrics{
		httpRequests:  make(map[labelKey]counter),
		webhookEvents: make(map[labelKey]uint64),
		startedAt:     time.Now().UTC(),
	}
}

func (m *Metrics) ObserveHTTPRequest(method, path string, status int, duration time.Duration) {
	if m == nil {
		return
	}
	key := makeKey(method, path, strconv.Itoa(status))
	m.mu.Lock()
	item := m.httpRequests[key]
	item.Count++
	item.DurationSum += duration.Seconds()
	m.httpRequests[key] = item
	m.mu.Unlock()
}

func (m *Metrics) IncWebhookEvent(sourceID, eventType string, duplicate bool) {
	if m == nil {
		return
	}
	if eventType == "" {
		eventType = "unknown"
	}
	key := makeKey(sourceID, eventType, strconv.FormatBool(duplicate))
	m.mu.Lock()
	m.webhookEvents[key]++
	m.mu.Unlock()
}

func (m *Metrics) WritePrometheus(w io.Writer, db DatabaseSnapshot) {
	if m == nil {
		return
	}

	m.mu.RLock()
	httpRequests := copyHTTP(m.httpRequests)
	webhookEvents := copyWebhook(m.webhookEvents)
	startedAt := m.startedAt
	m.mu.RUnlock()

	writeHelp(w, "signalbox_build_info", "SignalBox service metadata.")
	writeType(w, "signalbox_build_info", "gauge")
	_, _ = fmt.Fprintf(w, "signalbox_build_info{service=\"signalbox\"} 1\n")

	writeHelp(w, "signalbox_uptime_seconds", "SignalBox process uptime in seconds.")
	writeType(w, "signalbox_uptime_seconds", "gauge")
	_, _ = fmt.Fprintf(w, "signalbox_uptime_seconds %.0f\n", time.Since(startedAt).Seconds())

	writeHelp(w, "signalbox_http_requests_total", "Total HTTP requests by method, path and status.")
	writeType(w, "signalbox_http_requests_total", "counter")
	for _, key := range sortedHTTPKeys(httpRequests) {
		method, path, status := splitKey(key)
		_, _ = fmt.Fprintf(w, "signalbox_http_requests_total{method=\"%s\",path=\"%s\",status=\"%s\"} %d\n", escape(method), escape(path), escape(status), httpRequests[key].Count)
	}

	writeHelp(w, "signalbox_http_request_duration_seconds_sum", "Total HTTP request duration in seconds.")
	writeType(w, "signalbox_http_request_duration_seconds_sum", "counter")
	for _, key := range sortedHTTPKeys(httpRequests) {
		method, path, status := splitKey(key)
		_, _ = fmt.Fprintf(w, "signalbox_http_request_duration_seconds_sum{method=\"%s\",path=\"%s\",status=\"%s\"} %.6f\n", escape(method), escape(path), escape(status), httpRequests[key].DurationSum)
	}

	writeHelp(w, "signalbox_http_request_duration_seconds_count", "Total observed HTTP request durations.")
	writeType(w, "signalbox_http_request_duration_seconds_count", "counter")
	for _, key := range sortedHTTPKeys(httpRequests) {
		method, path, status := splitKey(key)
		_, _ = fmt.Fprintf(w, "signalbox_http_request_duration_seconds_count{method=\"%s\",path=\"%s\",status=\"%s\"} %d\n", escape(method), escape(path), escape(status), httpRequests[key].Count)
	}

	writeHelp(w, "signalbox_webhook_events_total", "Total accepted webhook events by source, type and duplicate flag.")
	writeType(w, "signalbox_webhook_events_total", "counter")
	for _, key := range sortedWebhookKeys(webhookEvents) {
		sourceID, eventType, duplicate := splitKey(key)
		_, _ = fmt.Fprintf(w, "signalbox_webhook_events_total{source=\"%s\",type=\"%s\",duplicate=\"%s\"} %d\n", escape(sourceID), escape(eventType), escape(duplicate), webhookEvents[key])
	}

	writeHelp(w, "signalbox_events_total", "Total events stored in PostgreSQL.")
	writeType(w, "signalbox_events_total", "gauge")
	_, _ = fmt.Fprintf(w, "signalbox_events_total %d\n", db.TotalEvents)

	writeHelp(w, "signalbox_events_unique_total", "Total unique events stored in PostgreSQL.")
	writeType(w, "signalbox_events_unique_total", "gauge")
	_, _ = fmt.Fprintf(w, "signalbox_events_unique_total %d\n", db.UniqueEvents)

	writeHelp(w, "signalbox_events_duplicate_total", "Total duplicate events stored in PostgreSQL.")
	writeType(w, "signalbox_events_duplicate_total", "gauge")
	_, _ = fmt.Fprintf(w, "signalbox_events_duplicate_total %d\n", db.DuplicateEvents)

	writeHelp(w, "signalbox_events_24h_total", "Events stored in the last 24 hours.")
	writeType(w, "signalbox_events_24h_total", "gauge")
	_, _ = fmt.Fprintf(w, "signalbox_events_24h_total %d\n", db.Events24h)

	writeHelp(w, "signalbox_sources_total", "Total webhook sources.")
	writeType(w, "signalbox_sources_total", "gauge")
	_, _ = fmt.Fprintf(w, "signalbox_sources_total %d\n", db.Sources)

	writeHelp(w, "signalbox_sources_active_total", "Total active webhook sources.")
	writeType(w, "signalbox_sources_active_total", "gauge")
	_, _ = fmt.Fprintf(w, "signalbox_sources_active_total %d\n", db.ActiveSources)

	writeHelp(w, "signalbox_delivery_jobs_by_status", "Delivery jobs grouped by status.")
	writeType(w, "signalbox_delivery_jobs_by_status", "gauge")
	for _, item := range db.DeliveryJobsByStatus {
		_, _ = fmt.Fprintf(w, "signalbox_delivery_jobs_by_status{status=\"%s\"} %d\n", escape(item.Status), item.Count)
	}
}

type DatabaseSnapshot struct {
	TotalEvents          int64
	UniqueEvents         int64
	DuplicateEvents      int64
	Events24h            int64
	Sources              int64
	ActiveSources        int64
	DeliveryJobsByStatus []DeliveryStatus
}

type DeliveryStatus struct {
	Status string
	Count  int64
}

func writeHelp(w io.Writer, name, help string) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n", name, help)
}

func writeType(w io.Writer, name, typ string) {
	_, _ = fmt.Fprintf(w, "# TYPE %s %s\n", name, typ)
}

func makeKey(parts ...string) labelKey {
	return labelKey(strings.Join(parts, "\xff"))
}

func splitKey(key labelKey) []string {
	return strings.Split(string(key), "\xff")
}

func escape(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return value
}

func copyHTTP(in map[labelKey]counter) map[labelKey]counter {
	out := make(map[labelKey]counter, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func copyWebhook(in map[labelKey]uint64) map[labelKey]uint64 {
	out := make(map[labelKey]uint64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func sortedHTTPKeys(values map[labelKey]counter) []labelKey {
	keys := make([]labelKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func sortedWebhookKeys(values map[labelKey]uint64) []labelKey {
	keys := make([]labelKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}
