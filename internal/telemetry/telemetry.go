// Package telemetry provides a global metrics registry with pluggable backends
// (expvar, Prometheus, or no-op) for recording anonymous usage statistics.
package telemetry

import (
	"context"
	"sync"
	"time"
)

// Metrics represents the core telemetry interface for collecting anonymized metrics.
// Implementations include expvar (default) and prometheus (optional).
type Metrics interface {
	// RecordCapture increments capture event count and tracks source type
	RecordCapture(sourceKind string)

	// RecordBytes tracks bytes processed (ingested, normalized, uploaded)
	RecordBytes(stage string, n int64)

	// RecordUpload increments upload attempts and tracks success/failure
	RecordUpload(success bool, durationMs int64)

	// RecordError increments error count by category
	RecordError(category string)

	// Snapshot returns current metrics as a map for debugging/export
	Snapshot() map[string]any
}

// NoopMetrics is a silent implementation for when telemetry is disabled.
type NoopMetrics struct{}

func (n *NoopMetrics) RecordCapture(sourceKind string)             {}
func (n *NoopMetrics) RecordBytes(stage string, bytes int64)       {}
func (n *NoopMetrics) RecordUpload(success bool, durationMs int64) {}
func (n *NoopMetrics) RecordError(category string)                 {}
func (n *NoopMetrics) Snapshot() map[string]any                    { return nil }

// Global registry - safe for concurrent access
var (
	mu      sync.RWMutex
	current Metrics = &NoopMetrics{}
)

// Init sets the global metrics implementation.
// Call this once at startup with your chosen backend.
func Init(m Metrics) {
	mu.Lock()
	defer mu.Unlock()
	if m == nil {
		current = &NoopMetrics{}
	} else {
		current = m
	}
}

// Current returns the active metrics implementation
func Current() Metrics {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Helper functions for ergonomic access

// RecordCapture increments the capture event counter for the given source kind.
func RecordCapture(sourceKind string) {
	Current().RecordCapture(sourceKind)
}

// RecordBytes records n bytes processed at the named pipeline stage.
func RecordBytes(stage string, n int64) {
	Current().RecordBytes(stage, n)
}

// RecordUpload records the outcome and duration of an upload attempt.
func RecordUpload(success bool, durationMs int64) {
	Current().RecordUpload(success, durationMs)
}

// RecordError increments the error counter for the given category.
func RecordError(category string) {
	Current().RecordError(category)
}

// Snapshot returns the current metric values as a plain map, useful for debugging.
func Snapshot() map[string]any {
	return Current().Snapshot()
}

// TimedUpload is a helper to measure and record upload duration
func TimedUpload(ctx context.Context, fn func(context.Context) error) error {
	start := time.Now()
	err := fn(ctx)
	durationMs := time.Since(start).Milliseconds()
	RecordUpload(err == nil, durationMs)
	return err
}
