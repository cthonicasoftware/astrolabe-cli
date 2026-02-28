package telemetry

import (
	"context"
	"errors"
	"testing"
)

func TestNoopMetrics(t *testing.T) {
	noop := &NoopMetrics{}

	// Should not panic on any operation
	noop.RecordCapture("serial")
	noop.RecordBytes("ingested", 1024)
	noop.RecordUpload(true, 100)
	noop.RecordError("test_error")

	snapshot := noop.Snapshot()
	if snapshot != nil {
		t.Errorf("NoopMetrics.Snapshot() = %v, want nil", snapshot)
	}
}

func TestGlobalRegistry(t *testing.T) {
	// Save original state
	original := Current()
	defer Init(original)

	// Test initialization with nil defaults to NoopMetrics
	Init(nil)
	if _, ok := Current().(*NoopMetrics); !ok {
		t.Error("Init(nil) should default to NoopMetrics")
	}

	// Test setting custom metrics
	custom := NewExpvarMetrics()
	Init(custom)
	if Current() != custom {
		t.Error("Init() should set the global metrics instance")
	}

	// Test helper functions work with global instance
	RecordCapture("test")
	RecordBytes("ingested", 100)
	RecordUpload(true, 50)
	RecordError("test_error")

	snapshot := Snapshot()
	if snapshot == nil {
		t.Error("Snapshot() should return data from active metrics")
	}
}

func TestTimedUpload(t *testing.T) {
	custom := NewExpvarMetrics()
	Init(custom)
	defer Init(&NoopMetrics{})

	before := Snapshot()
	beforeUploads := before["uploads"].(map[string]any)
	beforeSuccess := beforeUploads["success"].(int64)
	beforeFailures := beforeUploads["failures"].(int64)
	beforeAttempts := beforeUploads["attempts"].(int64)

	// Test successful upload
	err := TimedUpload(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("TimedUpload() error = %v, want nil", err)
	}

	snapshot := Snapshot()
	uploads := snapshot["uploads"].(map[string]any)
	if uploads["success"].(int64)-beforeSuccess != 1 {
		t.Errorf("success delta = %v, want 1", uploads["success"].(int64)-beforeSuccess)
	}

	// Test failed upload
	testErr := errors.New("upload failed")
	err = TimedUpload(context.Background(), func(ctx context.Context) error {
		return testErr
	})
	if err != testErr {
		t.Errorf("TimedUpload() error = %v, want %v", err, testErr)
	}

	snapshot = Snapshot()
	uploads = snapshot["uploads"].(map[string]any)
	if uploads["failures"].(int64)-beforeFailures != 1 {
		t.Errorf("failures delta = %v, want 1", uploads["failures"].(int64)-beforeFailures)
	}
	if uploads["attempts"].(int64)-beforeAttempts != 2 {
		t.Errorf("attempts delta = %v, want 2", uploads["attempts"].(int64)-beforeAttempts)
	}
}

func TestConcurrentAccess(t *testing.T) {
	custom := NewExpvarMetrics()
	Init(custom)
	defer Init(&NoopMetrics{})

	before := Snapshot()
	beforeCaptures := before["captures"].(map[string]any)["total"].(int64)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				RecordCapture("serial")
				RecordBytes("ingested", 10)
				RecordUpload(true, 10)
				RecordError("concurrent_test")
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	snapshot := Snapshot()
	captures := snapshot["captures"].(map[string]any)
	afterCaptures := captures["total"].(int64)
	if afterCaptures-beforeCaptures != 1000 {
		t.Errorf("concurrent capture delta = %v, want 1000", afterCaptures-beforeCaptures)
	}
}
