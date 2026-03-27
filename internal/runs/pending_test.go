package runs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/storage"
)

func TestFindPending(t *testing.T) {
	tmpDir := t.TempDir()

	createRunDir := func(runID string, uploadState *core.UploadState) {
		runDir := filepath.Join(tmpDir, runID)
		if err := os.MkdirAll(runDir, 0o755); err != nil {
			t.Fatalf("create run dir: %v", err)
		}

		manifest := storage.ManifestDocument{
			RunID:  runID,
			Schema: "v1alpha1",
		}
		manifestData, _ := json.Marshal(manifest)
		manifestPath := filepath.Join(runDir, "manifest.json")
		if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}

		if uploadState != nil {
			stateData, _ := json.Marshal(uploadState)
			statePath := filepath.Join(runDir, "upload_state.json")
			if err := os.WriteFile(statePath, stateData, 0o644); err != nil {
				t.Fatalf("write upload state: %v", err)
			}
		}
	}

	createRunDir("run-never-uploaded", nil)
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
	createRunDir("run-pending", &core.UploadState{Status: core.UploadStatusPending})
	createRunDir("run-queued", &core.UploadState{Status: core.UploadStatusQueued})

	pending, err := FindPending(tmpDir)
	if err != nil {
		t.Fatalf("FindPending: %v", err)
	}

	expectedPending := map[string]bool{
		"run-never-uploaded": true,
		"run-failed":         true,
		"run-in-flight":      true,
		"run-pending":        true,
		"run-queued":         true,
	}

	for _, runID := range pending {
		if runID == "run-succeeded" {
			t.Errorf("run-succeeded should not be in pending list")
		}
		if !expectedPending[runID] {
			t.Errorf("unexpected run in pending list: %s", runID)
		}
	}

	pendingMap := make(map[string]bool)
	for _, runID := range pending {
		pendingMap[runID] = true
	}

	for expectedRunID := range expectedPending {
		if !pendingMap[expectedRunID] {
			t.Errorf("expected run %s to be in pending list, but it wasn't", expectedRunID)
		}
	}

	if len(pending) != len(expectedPending) {
		t.Errorf("expected %d pending runs, got %d", len(expectedPending), len(pending))
	}
}

func TestFindPending_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	pending, err := FindPending(tmpDir)
	if err != nil {
		t.Fatalf("FindPending: %v", err)
	}

	if len(pending) != 0 {
		t.Errorf("expected 0 pending runs in empty directory, got %d", len(pending))
	}
}

func TestFindPending_NonexistentDirectory(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "nonexistent")

	pending, err := FindPending(tmpDir)
	if err != nil {
		t.Fatalf("FindPending should not error on nonexistent dir: %v", err)
	}

	if len(pending) != 0 {
		t.Errorf("expected 0 pending runs, got %d", len(pending))
	}
}

func TestFindPending_InvalidStateFile(t *testing.T) {
	tmpDir := t.TempDir()
	runID := "run-with-invalid-state"
	runDir := filepath.Join(tmpDir, runID)

	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("create run dir: %v", err)
	}

	manifest := storage.ManifestDocument{
		RunID:  runID,
		Schema: "v1alpha1",
	}
	manifestData, _ := json.Marshal(manifest)
	manifestPath := filepath.Join(runDir, "manifest.json")
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	statePath := filepath.Join(runDir, "upload_state.json")
	if err := os.WriteFile(statePath, []byte("invalid json{{{"), 0o644); err != nil {
		t.Fatalf("write invalid state: %v", err)
	}

	pending, err := FindPending(tmpDir)
	if err != nil {
		t.Fatalf("FindPending: %v", err)
	}

	if len(pending) != 1 {
		t.Errorf("expected 1 pending run with invalid state, got %d", len(pending))
	}

	if len(pending) > 0 && pending[0] != runID {
		t.Errorf("expected pending run to be %s, got %s", runID, pending[0])
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
