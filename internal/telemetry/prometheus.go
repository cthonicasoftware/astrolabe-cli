package telemetry

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	captureCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "astrolabe_captures_total",
			Help: "Total number of captures by source kind",
		},
		[]string{"source_kind"},
	)

	bytesCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "astrolabe_bytes_total",
			Help: "Total bytes processed by stage",
		},
		[]string{"stage"},
	)

	uploadCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "astrolabe_uploads_total",
			Help: "Total upload attempts",
		},
		[]string{"status"},
	)

	uploadDurationHistogram = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "astrolabe_upload_duration_ms",
			Help:    "Upload duration in milliseconds",
			Buckets: prometheus.ExponentialBuckets(10, 2, 10),
		},
	)

	errorCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "astrolabe_errors_total",
			Help: "Total errors by category",
		},
		[]string{"category"},
	)
)

// PrometheusMetrics provides Prometheus-compatible metrics collection.
type PrometheusMetrics struct {
	mu sync.Mutex

	// In-memory counters for snapshot functionality
	captureCount    int64
	captureByKind   map[string]int64
	bytesIngested   int64
	bytesNormalized int64
	bytesUploaded   int64
	uploadAttempts  int64
	uploadSuccess   int64
	uploadFailures  int64
	uploadDuration  int64
	errors          map[string]int64
}

// NewPrometheusMetrics creates a new Prometheus metrics collector.
func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		captureByKind: make(map[string]int64),
		errors:        make(map[string]int64),
	}
}

// RecordCapture increments the capture counter for a given source kind.
func (p *PrometheusMetrics) RecordCapture(sourceKind string) {
	captureCounter.WithLabelValues(sourceKind).Inc()

	// Also update in-memory counters for snapshot
	p.mu.Lock()
	defer p.mu.Unlock()
	p.captureCount++
	p.captureByKind[sourceKind]++
}

// RecordBytes records the number of bytes processed at a given stage.
func (p *PrometheusMetrics) RecordBytes(stage string, n int64) {
	bytesCounter.WithLabelValues(stage).Add(float64(n))

	// Also update in-memory counters for snapshot
	p.mu.Lock()
	defer p.mu.Unlock()
	switch stage {
	case "ingested":
		p.bytesIngested += n
	case "normalized":
		p.bytesNormalized += n
	case "uploaded":
		p.bytesUploaded += n
	}
}

// RecordUpload records an upload attempt with its success status and duration.
func (p *PrometheusMetrics) RecordUpload(success bool, durationMs int64) {
	status := "failure"
	if success {
		status = "success"
	}
	uploadCounter.WithLabelValues(status).Inc()
	uploadDurationHistogram.Observe(float64(durationMs))

	// Also update in-memory counters for snapshot
	p.mu.Lock()
	defer p.mu.Unlock()
	p.uploadAttempts++
	if success {
		p.uploadSuccess++
	} else {
		p.uploadFailures++
	}
	p.uploadDuration += durationMs
}

// RecordError increments the error counter for a given category.
func (p *PrometheusMetrics) RecordError(category string) {
	errorCounter.WithLabelValues(category).Inc()

	// Also update in-memory counters for snapshot
	p.mu.Lock()
	defer p.mu.Unlock()
	p.errors[category]++
}

// Snapshot returns a point-in-time view of all metrics.
// This provides a convenient way to view metrics without scraping the /metrics endpoint.
func (p *PrometheusMetrics) Snapshot() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	captureByKind := make(map[string]int64)
	for k, v := range p.captureByKind {
		captureByKind[k] = v
	}

	errors := make(map[string]int64)
	for k, v := range p.errors {
		errors[k] = v
	}

	return map[string]interface{}{
		"captures": map[string]interface{}{
			"total":   p.captureCount,
			"by_kind": captureByKind,
		},
		"bytes": map[string]interface{}{
			"ingested":   p.bytesIngested,
			"normalized": p.bytesNormalized,
			"uploaded":   p.bytesUploaded,
		},
		"uploads": map[string]interface{}{
			"attempts":    p.uploadAttempts,
			"success":     p.uploadSuccess,
			"failures":    p.uploadFailures,
			"duration_ms": p.uploadDuration,
		},
		"errors": errors,
	}
}

// Handler returns an HTTP handler for the /metrics endpoint.
// Use this to expose Prometheus metrics via HTTP.
//
// Example usage:
//
//	http.Handle("/metrics", metrics.Handler())
//	http.ListenAndServe(":2112", nil)
func (p *PrometheusMetrics) Handler() http.Handler {
	return promhttp.Handler()
}
