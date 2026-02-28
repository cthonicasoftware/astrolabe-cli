package telemetry

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusMetrics provides Prometheus-compatible metrics collection.
// Each instance owns its own prometheus.Registry for full isolation.
// The Prometheus counters are the single source of truth — Snapshot()
// reads directly from them, eliminating dual bookkeeping.
type PrometheusMetrics struct {
	registry *prometheus.Registry

	captureCounter          *prometheus.CounterVec
	bytesCounter            *prometheus.CounterVec
	uploadCounter           *prometheus.CounterVec
	uploadDurationHistogram prometheus.Histogram
	errorCounter            *prometheus.CounterVec
}

// NewPrometheusMetrics creates a new Prometheus metrics collector with its own registry.
func NewPrometheusMetrics() *PrometheusMetrics {
	reg := prometheus.NewRegistry()

	captureCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "astrolabe_captures_total",
			Help: "Total number of captures by source kind",
		},
		[]string{"source_kind"},
	)

	bytesCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "astrolabe_bytes_total",
			Help: "Total bytes processed by stage",
		},
		[]string{"stage"},
	)

	uploadCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "astrolabe_uploads_total",
			Help: "Total upload attempts",
		},
		[]string{"status"},
	)

	uploadDurationHistogram := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "astrolabe_upload_duration_ms",
			Help:    "Upload duration in milliseconds",
			Buckets: prometheus.ExponentialBuckets(10, 2, 10),
		},
	)

	errorCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "astrolabe_errors_total",
			Help: "Total errors by category",
		},
		[]string{"category"},
	)

	reg.MustRegister(captureCounter, bytesCounter, uploadCounter, uploadDurationHistogram, errorCounter)

	return &PrometheusMetrics{
		registry:                reg,
		captureCounter:          captureCounter,
		bytesCounter:            bytesCounter,
		uploadCounter:           uploadCounter,
		uploadDurationHistogram: uploadDurationHistogram,
		errorCounter:            errorCounter,
	}
}

// RecordCapture increments the capture counter for a given source kind.
func (p *PrometheusMetrics) RecordCapture(sourceKind string) {
	p.captureCounter.WithLabelValues(sourceKind).Inc()
}

// RecordBytes records the number of bytes processed at a given stage.
func (p *PrometheusMetrics) RecordBytes(stage string, n int64) {
	p.bytesCounter.WithLabelValues(stage).Add(float64(n))
}

// RecordUpload records an upload attempt with its success status and duration.
func (p *PrometheusMetrics) RecordUpload(success bool, durationMs int64) {
	status := "failure"
	if success {
		status = "success"
	}
	p.uploadCounter.WithLabelValues(status).Inc()
	p.uploadDurationHistogram.Observe(float64(durationMs))
}

// RecordError increments the error counter for a given category.
func (p *PrometheusMetrics) RecordError(category string) {
	p.errorCounter.WithLabelValues(category).Inc()
}

// Snapshot returns a point-in-time view of all metrics by reading directly
// from the Prometheus registry. This is the single source of truth.
func (p *PrometheusMetrics) Snapshot() map[string]any {
	mfs, err := p.registry.Gather()
	if err != nil {
		return nil
	}

	var captureTotal int64
	captureByKind := make(map[string]int64)
	bytesMap := make(map[string]int64)
	var uploadSuccess, uploadFailure int64
	var uploadDuration int64
	errors := make(map[string]int64)

	for _, mf := range mfs {
		switch mf.GetName() {
		case "astrolabe_captures_total":
			for _, m := range mf.GetMetric() {
				val := int64(m.GetCounter().GetValue())
				captureTotal += val
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "source_kind" {
						captureByKind[lp.GetValue()] = val
					}
				}
			}
		case "astrolabe_bytes_total":
			for _, m := range mf.GetMetric() {
				val := int64(m.GetCounter().GetValue())
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "stage" {
						bytesMap[lp.GetValue()] = val
					}
				}
			}
		case "astrolabe_uploads_total":
			for _, m := range mf.GetMetric() {
				val := int64(m.GetCounter().GetValue())
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "status" {
						switch lp.GetValue() {
						case "success":
							uploadSuccess = val
						case "failure":
							uploadFailure = val
						}
					}
				}
			}
		case "astrolabe_upload_duration_ms":
			for _, m := range mf.GetMetric() {
				uploadDuration = int64(m.GetHistogram().GetSampleSum())
			}
		case "astrolabe_errors_total":
			for _, m := range mf.GetMetric() {
				val := int64(m.GetCounter().GetValue())
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "category" {
						errors[lp.GetValue()] = val
					}
				}
			}
		}
	}

	return map[string]any{
		"captures": map[string]any{
			"total":   captureTotal,
			"by_kind": captureByKind,
		},
		"bytes": map[string]any{
			"ingested":   bytesMap["ingested"],
			"normalized": bytesMap["normalized"],
			"uploaded":   bytesMap["uploaded"],
		},
		"uploads": map[string]any{
			"attempts":    uploadSuccess + uploadFailure,
			"success":     uploadSuccess,
			"failures":    uploadFailure,
			"duration_ms": uploadDuration,
		},
		"errors": errors,
	}
}

// Handler returns an HTTP handler for the /metrics endpoint.
// The handler only exposes metrics from this instance's registry.
//
// Example usage:
//
//	http.Handle("/metrics", metrics.Handler())
//	http.ListenAndServe(":2112", nil)
func (p *PrometheusMetrics) Handler() http.Handler {
	return promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{})
}
