package telemetry

import (
	"expvar"
	"sync"
)

// ExpvarMetrics implements Metrics using Go's built-in expvar package.
// Expvar publishes metrics at /debug/vars (HTTP handler can be registered separately).
type ExpvarMetrics struct {
	mu sync.Mutex

	// Capture metrics
	captureCount  *expvar.Int
	captureByKind *expvar.Map

	// Byte metrics by stage
	bytesIngested   *expvar.Int
	bytesNormalized *expvar.Int
	bytesUploaded   *expvar.Int

	// Upload metrics
	uploadAttempts *expvar.Int
	uploadSuccess  *expvar.Int
	uploadFailures *expvar.Int
	uploadDuration *expvar.Int // total ms

	// Error metrics
	errors *expvar.Map
}

// NewExpvarMetrics creates and registers an expvar-based metrics collector.
func NewExpvarMetrics() *ExpvarMetrics {
	e := &ExpvarMetrics{
		captureCount:    getOrCreateInt("astrolabe.captures.total"),
		captureByKind:   getOrCreateMap("astrolabe.captures.by_kind"),
		bytesIngested:   getOrCreateInt("astrolabe.bytes.ingested"),
		bytesNormalized: getOrCreateInt("astrolabe.bytes.normalized"),
		bytesUploaded:   getOrCreateInt("astrolabe.bytes.uploaded"),
		uploadAttempts:  getOrCreateInt("astrolabe.uploads.attempts"),
		uploadSuccess:   getOrCreateInt("astrolabe.uploads.success"),
		uploadFailures:  getOrCreateInt("astrolabe.uploads.failures"),
		uploadDuration:  getOrCreateInt("astrolabe.uploads.duration_ms"),
		errors:          getOrCreateMap("astrolabe.errors"),
	}
	return e
}

// getOrCreateInt gets or creates an expvar.Int, avoiding panic on duplicate names
func getOrCreateInt(name string) *expvar.Int {
	v := expvar.Get(name)
	if v != nil {
		if intVar, ok := v.(*expvar.Int); ok {
			return intVar
		}
	}
	return expvar.NewInt(name)
}

// getOrCreateMap gets or creates an expvar.Map, avoiding panic on duplicate names
func getOrCreateMap(name string) *expvar.Map {
	v := expvar.Get(name)
	if v != nil {
		if mapVar, ok := v.(*expvar.Map); ok {
			return mapVar
		}
	}
	return expvar.NewMap(name)
}

func (e *ExpvarMetrics) RecordCapture(sourceKind string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.captureCount.Add(1)
	e.captureByKind.Add(sourceKind, 1)
}

func (e *ExpvarMetrics) RecordBytes(stage string, n int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch stage {
	case "ingested":
		e.bytesIngested.Add(n)
	case "normalized":
		e.bytesNormalized.Add(n)
	case "uploaded":
		e.bytesUploaded.Add(n)
	}
}

func (e *ExpvarMetrics) RecordUpload(success bool, durationMs int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.uploadAttempts.Add(1)
	if success {
		e.uploadSuccess.Add(1)
	} else {
		e.uploadFailures.Add(1)
	}
	e.uploadDuration.Add(durationMs)
}

func (e *ExpvarMetrics) RecordError(category string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.errors.Add(category, 1)
}

func (e *ExpvarMetrics) Snapshot() map[string]any {
	e.mu.Lock()
	defer e.mu.Unlock()

	captureByKind := make(map[string]int64)
	e.captureByKind.Do(func(kv expvar.KeyValue) {
		if v, ok := kv.Value.(*expvar.Int); ok {
			captureByKind[kv.Key] = v.Value()
		}
	})

	errors := make(map[string]int64)
	e.errors.Do(func(kv expvar.KeyValue) {
		if v, ok := kv.Value.(*expvar.Int); ok {
			errors[kv.Key] = v.Value()
		}
	})

	return map[string]any{
		"captures": map[string]any{
			"total":   e.captureCount.Value(),
			"by_kind": captureByKind,
		},
		"bytes": map[string]any{
			"ingested":   e.bytesIngested.Value(),
			"normalized": e.bytesNormalized.Value(),
			"uploaded":   e.bytesUploaded.Value(),
		},
		"uploads": map[string]any{
			"attempts":    e.uploadAttempts.Value(),
			"success":     e.uploadSuccess.Value(),
			"failures":    e.uploadFailures.Value(),
			"duration_ms": e.uploadDuration.Value(),
		},
		"errors": errors,
	}
}
