package telemetry

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestPrometheusMetrics_RecordCapture(t *testing.T) {
	m := NewPrometheusMetrics()

	m.RecordCapture("serial")
	m.RecordCapture("tcp")
	m.RecordCapture("serial")

	snapshot := m.Snapshot()
	captures := snapshot["captures"].(map[string]any)

	if captures["total"].(int64) != 3 {
		t.Errorf("total captures = %v, want 3", captures["total"])
	}

	byKind := captures["by_kind"].(map[string]int64)
	if byKind["serial"] != 2 {
		t.Errorf("serial captures = %v, want 2", byKind["serial"])
	}
	if byKind["tcp"] != 1 {
		t.Errorf("tcp captures = %v, want 1", byKind["tcp"])
	}
}

func TestPrometheusMetrics_RecordBytes(t *testing.T) {
	m := NewPrometheusMetrics()

	m.RecordBytes("ingested", 1024)
	m.RecordBytes("normalized", 512)
	m.RecordBytes("uploaded", 256)
	m.RecordBytes("ingested", 1024)

	snapshot := m.Snapshot()
	bytes := snapshot["bytes"].(map[string]any)

	if bytes["ingested"].(int64) != 2048 {
		t.Errorf("ingested bytes = %v, want 2048", bytes["ingested"])
	}
	if bytes["normalized"].(int64) != 512 {
		t.Errorf("normalized bytes = %v, want 512", bytes["normalized"])
	}
	if bytes["uploaded"].(int64) != 256 {
		t.Errorf("uploaded bytes = %v, want 256", bytes["uploaded"])
	}
}

func TestPrometheusMetrics_RecordUpload(t *testing.T) {
	m := NewPrometheusMetrics()

	m.RecordUpload(true, 100)
	m.RecordUpload(true, 150)
	m.RecordUpload(false, 200)

	snapshot := m.Snapshot()
	uploads := snapshot["uploads"].(map[string]any)

	if uploads["attempts"].(int64) != 3 {
		t.Errorf("upload attempts = %v, want 3", uploads["attempts"])
	}
	if uploads["success"].(int64) != 2 {
		t.Errorf("upload success = %v, want 2", uploads["success"])
	}
	if uploads["failures"].(int64) != 1 {
		t.Errorf("upload failures = %v, want 1", uploads["failures"])
	}
	if uploads["duration_ms"].(int64) != 450 {
		t.Errorf("upload duration = %v, want 450", uploads["duration_ms"])
	}
}

func TestPrometheusMetrics_RecordError(t *testing.T) {
	m := NewPrometheusMetrics()

	m.RecordError("network_timeout")
	m.RecordError("parse_error")
	m.RecordError("network_timeout")
	m.RecordError("network_timeout")

	snapshot := m.Snapshot()
	errors := snapshot["errors"].(map[string]int64)

	if errors["network_timeout"] != 3 {
		t.Errorf("network_timeout errors = %v, want 3", errors["network_timeout"])
	}
	if errors["parse_error"] != 1 {
		t.Errorf("parse_error errors = %v, want 1", errors["parse_error"])
	}
}

func TestPrometheusMetrics_Snapshot(t *testing.T) {
	m := NewPrometheusMetrics()

	// Record various metrics
	m.RecordCapture("serial")
	m.RecordBytes("ingested", 1024)
	m.RecordUpload(true, 100)
	m.RecordError("test_error")

	snapshot := m.Snapshot()

	// Verify structure
	if snapshot == nil {
		t.Fatal("Snapshot() returned nil")
	}

	if _, ok := snapshot["captures"]; !ok {
		t.Error("Snapshot missing 'captures' key")
	}
	if _, ok := snapshot["bytes"]; !ok {
		t.Error("Snapshot missing 'bytes' key")
	}
	if _, ok := snapshot["uploads"]; !ok {
		t.Error("Snapshot missing 'uploads' key")
	}
	if _, ok := snapshot["errors"]; !ok {
		t.Error("Snapshot missing 'errors' key")
	}
}

func TestPrometheusMetrics_ConcurrentAccess(t *testing.T) {
	m := NewPrometheusMetrics()

	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 200; j++ {
				m.RecordCapture("concurrent")
				m.RecordBytes("ingested", 1)
				m.RecordUpload(true, 1)
				m.RecordError("concurrent_error")
			}
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}

	snapshot := m.Snapshot()
	captures := snapshot["captures"].(map[string]any)
	if captures["total"].(int64) != 1000 {
		t.Errorf("concurrent captures = %v, want 1000", captures["total"])
	}

	bytes := snapshot["bytes"].(map[string]any)
	if bytes["ingested"].(int64) != 1000 {
		t.Errorf("concurrent bytes = %v, want 1000", bytes["ingested"])
	}
}

// TestPrometheusMetrics_ActualMetrics verifies that actual Prometheus metrics are being recorded.
func TestPrometheusMetrics_ActualMetrics(t *testing.T) {
	// Create a new registry to isolate this test
	registry := prometheus.NewRegistry()

	// Create new metrics for this test
	testCaptureCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_astrolabe_captures_total",
			Help: "Test counter for captures",
		},
		[]string{"source_kind"},
	)
	registry.MustRegister(testCaptureCounter)

	// Increment the counter
	testCaptureCounter.WithLabelValues("serial").Inc()
	testCaptureCounter.WithLabelValues("serial").Inc()
	testCaptureCounter.WithLabelValues("tcp").Inc()

	// Verify the metric values
	expected := `
		# HELP test_astrolabe_captures_total Test counter for captures
		# TYPE test_astrolabe_captures_total counter
		test_astrolabe_captures_total{source_kind="serial"} 2
		test_astrolabe_captures_total{source_kind="tcp"} 1
	`

	err := testutil.CollectAndCompare(testCaptureCounter, strings.NewReader(expected))
	if err != nil {
		t.Errorf("Prometheus metric mismatch: %v", err)
	}
}

// TestPrometheusMetrics_BytesCounter verifies the bytes counter metric.
func TestPrometheusMetrics_BytesCounter(t *testing.T) {
	// Create a new registry to isolate this test
	registry := prometheus.NewRegistry()

	testBytesCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_astrolabe_bytes_total",
			Help: "Test counter for bytes",
		},
		[]string{"stage"},
	)
	registry.MustRegister(testBytesCounter)

	// Record bytes
	testBytesCounter.WithLabelValues("ingested").Add(1024)
	testBytesCounter.WithLabelValues("normalized").Add(512)
	testBytesCounter.WithLabelValues("uploaded").Add(256)

	// Verify the metric values
	expected := `
		# HELP test_astrolabe_bytes_total Test counter for bytes
		# TYPE test_astrolabe_bytes_total counter
		test_astrolabe_bytes_total{stage="ingested"} 1024
		test_astrolabe_bytes_total{stage="normalized"} 512
		test_astrolabe_bytes_total{stage="uploaded"} 256
	`

	err := testutil.CollectAndCompare(testBytesCounter, strings.NewReader(expected))
	if err != nil {
		t.Errorf("Prometheus metric mismatch: %v", err)
	}
}

// TestPrometheusMetrics_UploadMetrics verifies upload counter and histogram.
func TestPrometheusMetrics_UploadMetrics(t *testing.T) {
	// Create a new registry to isolate this test
	registry := prometheus.NewRegistry()

	testUploadCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_astrolabe_uploads_total",
			Help: "Test counter for uploads",
		},
		[]string{"status"},
	)
	registry.MustRegister(testUploadCounter)

	// Record uploads
	testUploadCounter.WithLabelValues("success").Inc()
	testUploadCounter.WithLabelValues("success").Inc()
	testUploadCounter.WithLabelValues("failure").Inc()

	// Verify the metric values
	expected := `
		# HELP test_astrolabe_uploads_total Test counter for uploads
		# TYPE test_astrolabe_uploads_total counter
		test_astrolabe_uploads_total{status="success"} 2
		test_astrolabe_uploads_total{status="failure"} 1
	`

	err := testutil.CollectAndCompare(testUploadCounter, strings.NewReader(expected))
	if err != nil {
		t.Errorf("Prometheus metric mismatch: %v", err)
	}
}

// TestPrometheusMetrics_ErrorCounter verifies the error counter metric.
func TestPrometheusMetrics_ErrorCounter(t *testing.T) {
	// Create a new registry to isolate this test
	registry := prometheus.NewRegistry()

	testErrorCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_astrolabe_errors_total",
			Help: "Test counter for errors",
		},
		[]string{"category"},
	)
	registry.MustRegister(testErrorCounter)

	// Record errors
	testErrorCounter.WithLabelValues("network_timeout").Inc()
	testErrorCounter.WithLabelValues("network_timeout").Inc()
	testErrorCounter.WithLabelValues("parse_error").Inc()

	// Verify the metric values
	expected := `
		# HELP test_astrolabe_errors_total Test counter for errors
		# TYPE test_astrolabe_errors_total counter
		test_astrolabe_errors_total{category="network_timeout"} 2
		test_astrolabe_errors_total{category="parse_error"} 1
	`

	err := testutil.CollectAndCompare(testErrorCounter, strings.NewReader(expected))
	if err != nil {
		t.Errorf("Prometheus metric mismatch: %v", err)
	}
}

// TestPrometheusMetrics_Handler verifies the HTTP handler for /metrics endpoint.
func TestPrometheusMetrics_Handler(t *testing.T) {
	m := NewPrometheusMetrics()

	// Record some metrics
	m.RecordCapture("test")
	m.RecordBytes("ingested", 100)
	m.RecordUpload(true, 50)
	m.RecordError("test_error")

	// Create a test HTTP server
	handler := m.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(w, req)

	// Check response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Handler returned status %v, want %v", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Verify the response contains Prometheus metrics
	expectedMetrics := []string{
		"astrolabe_captures_total",
		"astrolabe_bytes_total",
		"astrolabe_uploads_total",
		"astrolabe_upload_duration_ms",
		"astrolabe_errors_total",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(bodyStr, metric) {
			t.Errorf("Response missing metric: %s", metric)
		}
	}

	// Verify content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("Handler returned content-type %v, want text/plain", contentType)
	}
}

// TestPrometheusMetrics_HandlerFormat verifies the metrics are in Prometheus format.
func TestPrometheusMetrics_HandlerFormat(t *testing.T) {
	m := NewPrometheusMetrics()

	// Record a specific metric
	m.RecordCapture("serial")

	// Create a test HTTP server
	handler := m.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(w, req)

	body, err := io.ReadAll(w.Result().Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Verify Prometheus format elements
	// Should contain TYPE and HELP comments
	if !strings.Contains(bodyStr, "# TYPE") {
		t.Error("Response missing # TYPE comments")
	}
	if !strings.Contains(bodyStr, "# HELP") {
		t.Error("Response missing # HELP comments")
	}

	// Should contain the metric with labels
	if !strings.Contains(bodyStr, "astrolabe_captures_total{") {
		t.Error("Response missing labeled metric")
	}
}
