package telemetry

import (
	"testing"
)

func TestExpvarMetrics_RecordCapture(t *testing.T) {
	m := NewExpvarMetrics()

	m.RecordCapture("serial")
	m.RecordCapture("tcp")
	m.RecordCapture("serial")

	snapshot := m.Snapshot()
	captures := snapshot["captures"].(map[string]interface{})

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

func TestExpvarMetrics_RecordBytes(t *testing.T) {
	m := NewExpvarMetrics()

	m.RecordBytes("ingested", 1024)
	m.RecordBytes("normalized", 512)
	m.RecordBytes("uploaded", 256)
	m.RecordBytes("ingested", 1024) // add more

	snapshot := m.Snapshot()
	bytes := snapshot["bytes"].(map[string]interface{})

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

func TestExpvarMetrics_RecordUpload(t *testing.T) {
	m := NewExpvarMetrics()

	m.RecordUpload(true, 100)
	m.RecordUpload(true, 150)
	m.RecordUpload(false, 200)

	snapshot := m.Snapshot()
	uploads := snapshot["uploads"].(map[string]interface{})

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

func TestExpvarMetrics_RecordError(t *testing.T) {
	m := NewExpvarMetrics()

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

func TestExpvarMetrics_Snapshot(t *testing.T) {
	m := NewExpvarMetrics()

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

func TestExpvarMetrics_ConcurrentAccess(t *testing.T) {
	m := NewExpvarMetrics()

	// Take snapshot before
	before := m.Snapshot()
	beforeCaptures := before["captures"].(map[string]interface{})["total"].(int64)
	beforeBytes := before["bytes"].(map[string]interface{})["ingested"].(int64)

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

	// Check delta instead of absolute values
	snapshot := m.Snapshot()
	captures := snapshot["captures"].(map[string]interface{})
	afterCaptures := captures["total"].(int64)
	if afterCaptures-beforeCaptures != 1000 {
		t.Errorf("concurrent captures delta = %v, want 1000", afterCaptures-beforeCaptures)
	}

	bytes := snapshot["bytes"].(map[string]interface{})
	afterBytes := bytes["ingested"].(int64)
	if afterBytes-beforeBytes != 1000 {
		t.Errorf("concurrent bytes delta = %v, want 1000", afterBytes-beforeBytes)
	}
}
