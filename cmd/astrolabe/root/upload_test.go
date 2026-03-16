package root

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/storage"
)

func TestFindPendingRuns(t *testing.T) {
	tmpDir := t.TempDir()

	// Helper to create a run directory with manifest
	createRunDir := func(runID string, uploadState *core.UploadState) {
		runDir := filepath.Join(tmpDir, runID)
		if err := os.MkdirAll(runDir, 0755); err != nil {
			t.Fatalf("create run dir: %v", err)
		}

		// Create manifest
		manifest := storage.ManifestDocument{
			RunID:  runID,
			Schema: "v1alpha1",
		}
		manifestData, _ := json.Marshal(manifest)
		manifestPath := filepath.Join(runDir, "manifest.json")
		if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}

		// Create upload state if provided
		if uploadState != nil {
			stateData, _ := json.Marshal(uploadState)
			statePath := filepath.Join(runDir, "upload_state.json")
			if err := os.WriteFile(statePath, stateData, 0644); err != nil {
				t.Fatalf("write upload state: %v", err)
			}
		}
	}

	// Create test runs with different upload states
	createRunDir("run-never-uploaded", nil) // No upload_state.json
	createRunDir("run-succeeded", &core.UploadState{
		Status:      core.UploadStatusSucceeded,
		CompletedAt: timePtr(time.Now()),
		RemoteRunID: "remote-123",
	})
	createRunDir("run-failed", &core.UploadState{
		Status:    core.UploadStatusFailed,
		Attempts:  3,
		LastError: "network error",
	})
	createRunDir("run-in-flight", &core.UploadState{
		Status:      core.UploadStatusInFlight,
		RemoteRunID: "remote-456",
	})
	createRunDir("run-pending", &core.UploadState{
		Status: core.UploadStatusPending,
	})
	createRunDir("run-queued", &core.UploadState{
		Status: core.UploadStatusQueued,
	})

	// Find pending runs
	pending, err := findPendingRuns(tmpDir)
	if err != nil {
		t.Fatalf("findPendingRuns: %v", err)
	}

	// Verify results
	expectedPending := map[string]bool{
		"run-never-uploaded": true,
		"run-failed":         true,
		"run-in-flight":      true,
		"run-pending":        true,
		"run-queued":         true,
	}

	// Succeeded run should NOT be in pending
	for _, runID := range pending {
		if runID == "run-succeeded" {
			t.Errorf("run-succeeded should not be in pending list")
		}
		if !expectedPending[runID] {
			t.Errorf("unexpected run in pending list: %s", runID)
		}
	}

	// Check all expected runs are present
	pendingMap := make(map[string]bool)
	for _, runID := range pending {
		pendingMap[runID] = true
	}

	for expectedRunID := range expectedPending {
		if !pendingMap[expectedRunID] {
			t.Errorf("expected run %s to be in pending list, but it wasn't", expectedRunID)
		}
	}

	// Verify count
	if len(pending) != len(expectedPending) {
		t.Errorf("expected %d pending runs, got %d", len(expectedPending), len(pending))
	}
}

func TestFindPendingRuns_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	pending, err := findPendingRuns(tmpDir)
	if err != nil {
		t.Fatalf("findPendingRuns: %v", err)
	}

	if len(pending) != 0 {
		t.Errorf("expected 0 pending runs in empty directory, got %d", len(pending))
	}
}

func TestFindPendingRuns_NonexistentDirectory(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "nonexistent")

	pending, err := findPendingRuns(tmpDir)
	if err != nil {
		t.Fatalf("findPendingRuns should not error on nonexistent dir: %v", err)
	}

	if len(pending) != 0 {
		t.Errorf("expected 0 pending runs, got %d", len(pending))
	}
}

func TestFindPendingRuns_InvalidStateFile(t *testing.T) {
	tmpDir := t.TempDir()
	runID := "run-with-invalid-state"
	runDir := filepath.Join(tmpDir, runID)

	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatalf("create run dir: %v", err)
	}

	// Create manifest
	manifest := storage.ManifestDocument{
		RunID:  runID,
		Schema: "v1alpha1",
	}
	manifestData, _ := json.Marshal(manifest)
	manifestPath := filepath.Join(runDir, "manifest.json")
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Create invalid upload state (not valid JSON)
	statePath := filepath.Join(runDir, "upload_state.json")
	if err := os.WriteFile(statePath, []byte("invalid json{{{"), 0644); err != nil {
		t.Fatalf("write invalid state: %v", err)
	}

	// Should still find the run as pending (treat invalid state as pending)
	pending, err := findPendingRuns(tmpDir)
	if err != nil {
		t.Fatalf("findPendingRuns: %v", err)
	}

	if len(pending) != 1 {
		t.Errorf("expected 1 pending run with invalid state, got %d", len(pending))
	}

	if len(pending) > 0 && pending[0] != runID {
		t.Errorf("expected pending run to be %s, got %s", runID, pending[0])
	}
}

func TestUploadRunWithTimeout_AssignsFreshDeadlinePerRun(t *testing.T) {
	parent := context.Background()
	var deadlines []time.Time

	recordDeadline := func(ctx context.Context, runID string) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatalf("expected deadline for run %s", runID)
		}
		deadlines = append(deadlines, deadline)
		return nil
	}

	if err := uploadRunWithTimeout(parent, 50*time.Millisecond, "run-1", recordDeadline); err != nil {
		t.Fatalf("first uploadRunWithTimeout: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := uploadRunWithTimeout(parent, 50*time.Millisecond, "run-2", recordDeadline); err != nil {
		t.Fatalf("second uploadRunWithTimeout: %v", err)
	}

	if len(deadlines) != 2 {
		t.Fatalf("expected 2 deadlines, got %d", len(deadlines))
	}
	if !deadlines[1].After(deadlines[0]) {
		t.Fatalf("expected second run deadline %v to be after first %v", deadlines[1], deadlines[0])
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
