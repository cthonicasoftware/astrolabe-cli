package telemetry

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

	m.RecordCapture("serial")
	m.RecordBytes("ingested", 1024)
	m.RecordUpload(true, 100)
	m.RecordError("test_error")

	snapshot := m.Snapshot()

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

// TestPrometheusMetrics_CaptureCounterPrometheus verifies the actual Prometheus counter
// is updated through the production code path.
func TestPrometheusMetrics_CaptureCounterPrometheus(t *testing.T) {
	m := NewPrometheusMetrics()

	m.RecordCapture("serial")
	m.RecordCapture("serial")
	m.RecordCapture("tcp")

	if got := testutil.ToFloat64(m.captureCounter.WithLabelValues("serial")); got != 2 {
		t.Errorf("prometheus serial captures = %v, want 2", got)
	}
	if got := testutil.ToFloat64(m.captureCounter.WithLabelValues("tcp")); got != 1 {
		t.Errorf("prometheus tcp captures = %v, want 1", got)
	}
}

// TestPrometheusMetrics_BytesCounterPrometheus verifies the bytes counter metric.
func TestPrometheusMetrics_BytesCounterPrometheus(t *testing.T) {
	m := NewPrometheusMetrics()

	m.RecordBytes("ingested", 1024)
	m.RecordBytes("normalized", 512)
	m.RecordBytes("uploaded", 256)

	if got := testutil.ToFloat64(m.bytesCounter.WithLabelValues("ingested")); got != 1024 {
		t.Errorf("prometheus ingested bytes = %v, want 1024", got)
	}
	if got := testutil.ToFloat64(m.bytesCounter.WithLabelValues("normalized")); got != 512 {
		t.Errorf("prometheus normalized bytes = %v, want 512", got)
	}
	if got := testutil.ToFloat64(m.bytesCounter.WithLabelValues("uploaded")); got != 256 {
		t.Errorf("prometheus uploaded bytes = %v, want 256", got)
	}
}

// TestPrometheusMetrics_UploadCounterPrometheus verifies upload counter and histogram.
func TestPrometheusMetrics_UploadCounterPrometheus(t *testing.T) {
	m := NewPrometheusMetrics()

	m.RecordUpload(true, 100)
	m.RecordUpload(true, 150)
	m.RecordUpload(false, 200)

	if got := testutil.ToFloat64(m.uploadCounter.WithLabelValues("success")); got != 2 {
		t.Errorf("prometheus upload success = %v, want 2", got)
	}
	if got := testutil.ToFloat64(m.uploadCounter.WithLabelValues("failure")); got != 1 {
		t.Errorf("prometheus upload failure = %v, want 1", got)
	}
}

// TestPrometheusMetrics_ErrorCounterPrometheus verifies the error counter metric.
func TestPrometheusMetrics_ErrorCounterPrometheus(t *testing.T) {
	m := NewPrometheusMetrics()

	m.RecordError("network_timeout")
	m.RecordError("network_timeout")
	m.RecordError("parse_error")

	if got := testutil.ToFloat64(m.errorCounter.WithLabelValues("network_timeout")); got != 2 {
		t.Errorf("prometheus network_timeout errors = %v, want 2", got)
	}
	if got := testutil.ToFloat64(m.errorCounter.WithLabelValues("parse_error")); got != 1 {
		t.Errorf("prometheus parse_error errors = %v, want 1", got)
	}
}

// TestPrometheusMetrics_InstanceIsolation verifies two instances don't share counters.
func TestPrometheusMetrics_InstanceIsolation(t *testing.T) {
	m1 := NewPrometheusMetrics()
	m2 := NewPrometheusMetrics()

	m1.RecordCapture("serial")
	m1.RecordCapture("serial")
	m2.RecordCapture("serial")

	if got := testutil.ToFloat64(m1.captureCounter.WithLabelValues("serial")); got != 2 {
		t.Errorf("m1 prometheus serial captures = %v, want 2", got)
	}
	if got := testutil.ToFloat64(m2.captureCounter.WithLabelValues("serial")); got != 1 {
		t.Errorf("m2 prometheus serial captures = %v, want 1", got)
	}

	snap1 := m1.Snapshot()
	snap2 := m2.Snapshot()
	c1 := snap1["captures"].(map[string]any)["total"].(int64)
	c2 := snap2["captures"].(map[string]any)["total"].(int64)
	if c1 != 2 {
		t.Errorf("m1 snapshot total = %v, want 2", c1)
	}
	if c2 != 1 {
		t.Errorf("m2 snapshot total = %v, want 1", c2)
	}
}

// TestPrometheusMetrics_Handler verifies the HTTP handler for /metrics endpoint.
func TestPrometheusMetrics_Handler(t *testing.T) {
	m := NewPrometheusMetrics()

	m.RecordCapture("test")
	m.RecordBytes("ingested", 100)
	m.RecordUpload(true, 50)
	m.RecordError("test_error")

	handler := m.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Handler returned status %v, want %v", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

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

	// Per-instance handler should NOT contain go runtime metrics from the default registry
	if strings.Contains(bodyStr, "go_goroutines") {
		t.Error("Handler should only expose instance metrics, not default registry metrics")
	}
}

// TestPrometheusMetrics_HandlerFormat verifies the metrics are in Prometheus format.
func TestPrometheusMetrics_HandlerFormat(t *testing.T) {
	m := NewPrometheusMetrics()

	m.RecordCapture("serial")

	handler := m.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body, err := io.ReadAll(w.Result().Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	if !strings.Contains(bodyStr, "# TYPE") {
		t.Error("Response missing # TYPE comments")
	}
	if !strings.Contains(bodyStr, "# HELP") {
		t.Error("Response missing # HELP comments")
	}
	if !strings.Contains(bodyStr, "astrolabe_captures_total{") {
		t.Error("Response missing labeled metric")
	}
}
