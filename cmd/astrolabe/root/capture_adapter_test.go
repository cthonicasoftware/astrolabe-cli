package root

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cthonicasoftware/astrolabe-cli/internal/tui"
)

func TestCaptureSessionStop_SavesArtifacts(t *testing.T) {
	cacheRoot := t.TempDir()
	tempRoot := filepath.Join(cacheRoot, ".tmp-run-save")
	runID := "run-save"
	runDir := filepath.Join(tempRoot, runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir temp run dir: %v", err)
	}
	artifactPath := filepath.Join(runDir, "data.jsonl")
	if err := os.WriteFile(artifactPath, []byte("test\n"), 0o644); err != nil {
		t.Fatalf("write temp artifact: %v", err)
	}

	sess := &captureSession{
		cancel:        func() {},
		done:          make(chan tui.CaptureSessionResult, 1),
		tempRoot:      tempRoot,
		cacheRoot:     cacheRoot,
		saveRequested: true,
	}
	sess.done <- tui.CaptureSessionResult{RunID: runID}

	result := sess.Stop()
	if result.Err != nil {
		t.Fatalf("Stop() error = %v", result.Err)
	}
	if !result.Saved {
		t.Fatalf("Stop() Saved = false, want true")
	}
	if result.RunID != runID {
		t.Fatalf("Stop() RunID = %q, want %q", result.RunID, runID)
	}

	finalArtifactPath := filepath.Join(cacheRoot, runID, "data.jsonl")
	if _, err := os.Stat(finalArtifactPath); err != nil {
		t.Fatalf("saved artifact missing at %s: %v", finalArtifactPath, err)
	}
	if _, err := os.Stat(tempRoot); !os.IsNotExist(err) {
		t.Fatalf("temp root should be removed, stat err = %v", err)
	}
}

func TestCaptureSessionStop_DiscardsArtifactsWithoutSaveRequest(t *testing.T) {
	cacheRoot := t.TempDir()
	tempRoot := filepath.Join(cacheRoot, ".tmp-run-discard")
	runID := "run-discard"
	runDir := filepath.Join(tempRoot, runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir temp run dir: %v", err)
	}

	sess := &captureSession{
		cancel:    func() {},
		done:      make(chan tui.CaptureSessionResult, 1),
		tempRoot:  tempRoot,
		cacheRoot: cacheRoot,
	}
	sess.done <- tui.CaptureSessionResult{RunID: runID}

	result := sess.Stop()
	if result.Err != nil {
		t.Fatalf("Stop() error = %v", result.Err)
	}
	if result.Saved {
		t.Fatalf("Stop() Saved = true, want false")
	}

	finalRunDir := filepath.Join(cacheRoot, runID)
	if _, err := os.Stat(finalRunDir); !os.IsNotExist(err) {
		t.Fatalf("final run dir should not exist, stat err = %v", err)
	}
	if _, err := os.Stat(tempRoot); !os.IsNotExist(err) {
		t.Fatalf("temp root should be removed, stat err = %v", err)
	}
}

func TestCaptureSessionStop_IsIdempotent(t *testing.T) {
	cacheRoot := t.TempDir()
	tempRoot := filepath.Join(cacheRoot, ".tmp-run-idempotent")
	runID := "run-idempotent"
	runDir := filepath.Join(tempRoot, runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir temp run dir: %v", err)
	}

	sess := &captureSession{
		cancel:        func() {},
		done:          make(chan tui.CaptureSessionResult, 1),
		tempRoot:      tempRoot,
		cacheRoot:     cacheRoot,
		saveRequested: true,
	}
	sess.done <- tui.CaptureSessionResult{RunID: runID}

	first := sess.Stop()
	second := sess.Stop()

	if first != second {
		t.Fatalf("Stop() results differ: first=%+v second=%+v", first, second)
	}
}

func TestCaptureSessionStop_PropagatesPipelineError(t *testing.T) {
	cacheRoot := t.TempDir()
	tempRoot := filepath.Join(cacheRoot, ".tmp-run-error")
	if err := os.MkdirAll(tempRoot, 0o755); err != nil {
		t.Fatalf("mkdir temp root: %v", err)
	}

	wantErr := context.Canceled
	sess := &captureSession{
		cancel:    func() {},
		done:      make(chan tui.CaptureSessionResult, 1),
		tempRoot:  tempRoot,
		cacheRoot: cacheRoot,
	}
	sess.done <- tui.CaptureSessionResult{Err: wantErr}

	result := sess.Stop()
	if result.Err != wantErr {
		t.Fatalf("Stop() error = %v, want %v", result.Err, wantErr)
	}
	if _, err := os.Stat(tempRoot); !os.IsNotExist(err) {
		t.Fatalf("temp root should be removed, stat err = %v", err)
	}
}
