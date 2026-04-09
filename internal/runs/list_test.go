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

func TestListEnumeratesRuns(t *testing.T) {
	tmp := t.TempDir()

	runDir := filepath.Join(tmp, "run-123")
	if err := os.Mkdir(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	started := time.Date(2024, time.December, 5, 10, 30, 0, 0, time.UTC)
	completed := started.Add(2 * time.Minute)

	doc := storage.ManifestDocument{
		RunID:        "run-123",
		Schema:       "v1alpha1",
		Source:       core.SourceMeta{Kind: "serial", Port: "/dev/ttyUSB0", Baud: 115200},
		Manifest:     core.Manifest{Operator: "alice", Test: core.TestInfo{Plan: "smoke"}},
		Capture:      core.CaptureSettings{Notes: "test capture"},
		Started:      started,
		Completed:    &completed,
		RecordsCount: 42,
		PrimaryData:  filepath.Join(runDir, "data.jsonl"),
	}

	payload, err := jsonMarshalIndent(doc)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "manifest.json"), payload, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "data.jsonl"), []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatalf("write data: %v", err)
	}

	// Corrupt run without manifest should produce warning but not fail.
	badDir := filepath.Join(tmp, "run-bad")
	if err := os.Mkdir(badDir, 0o755); err != nil {
		t.Fatalf("mkdir bad dir: %v", err)
	}

	result, err := List(tmp)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}

	if len(result.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(result.Runs))
	}

	run := result.Runs[0]
	if run.ID != doc.RunID {
		t.Fatalf("run id mismatch: %s", run.ID)
	}
	if run.Records != doc.RecordsCount {
		t.Fatalf("records mismatch: %d", run.Records)
	}
	if run.DataSizeBytes == 0 {
		t.Fatalf("data size not detected")
	}
	if run.DurationSeconds != completed.Sub(started).Seconds() {
		t.Fatalf("duration mismatch: %f", run.DurationSeconds)
	}

	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(result.Warnings))
	}
	if result.Warnings[0].RunDir != badDir {
		t.Fatalf("unexpected warning run dir: %s", result.Warnings[0].RunDir)
	}
}

func jsonMarshalIndent(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
