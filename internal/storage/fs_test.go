package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
)

func TestFSStorePersistsArtifacts(t *testing.T) {
	tmp := t.TempDir()
	store := NewFS(tmp)

	run := &core.Run{
		ID: "run-abc123",
		Source: core.SourceMeta{
			Kind: "serial",
			Port: "/dev/ttyUSB0",
			Baud: 115200,
		},
		Manifest: core.Manifest{
			SchemaVersion: "v1",
			Device: core.DeviceInfo{
				ID: "fixture-1",
			},
			Test: core.TestInfo{
				Plan: "burn-in",
			},
			Operator: "alice",
		},
		Capture: core.CaptureSettings{
			SampleRateHz: 9600,
			Channels:     []string{"ch1"},
		},
		Started: time.Date(2024, time.December, 1, 12, 0, 0, 0, time.UTC),
	}

	if err := store.Start(run); err != nil {
		t.Fatalf("start store: %v", err)
	}

	recordTS := run.Started.Add(3 * time.Second)
	records := []core.Record{
		{
			TS:      recordTS,
			Seq:     1,
			Type:    "sample",
			Payload: map[string]any{"value": 42},
		},
	}

	if err := store.Write(records); err != nil {
		t.Fatalf("write records: %v", err)
	}

	completed := run.Started.Add(5 * time.Second)
	run.Completed = &completed
	run.RecordsCount = uint64(len(records))

	if err := store.Finalize(run); err != nil {
		t.Fatalf("finalize store: %v", err)
	}

	if run.PrimaryDataURI == "" {
		t.Fatalf("primary data uri not set")
	}

	artifacts := store.Artifacts()
	if len(artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(artifacts))
	}

	if artifacts[0].Role != core.ArtifactRoleManifest {
		t.Fatalf("expected first artifact to be manifest, got %s", artifacts[0].Role)
	}

	if artifacts[1].Role != core.ArtifactRoleData {
		t.Fatalf("expected second artifact to be data, got %s", artifacts[1].Role)
	}

	dataBytes, err := os.ReadFile(artifacts[1].Path)
	if err != nil {
		t.Fatalf("read data: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(dataBytes)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 record line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], `"value":42`) {
		t.Fatalf("record line missing payload: %s", lines[0])
	}

	manifestBytes, err := os.ReadFile(artifacts[0].Path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var manifestDoc ManifestDocument
	if err := json.Unmarshal(manifestBytes, &manifestDoc); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	if manifestDoc.RunID != run.ID {
		t.Fatalf("manifest run_id mismatch: %s", manifestDoc.RunID)
	}
	if manifestDoc.RecordsCount != run.RecordsCount {
		t.Fatalf("manifest records mismatch: %d", manifestDoc.RecordsCount)
	}
	if manifestDoc.PrimaryData != run.PrimaryDataURI {
		t.Fatalf("manifest primary data mismatch: %s", manifestDoc.PrimaryData)
	}

	// Ensure files live under run directory.
	runDir := filepath.Join(tmp, run.ID)
	for _, artifact := range artifacts {
		if !strings.HasPrefix(artifact.Path, runDir) {
			t.Fatalf("artifact path outside run dir: %s", artifact.Path)
		}
		if artifact.Checksum.Value == "" {
			t.Fatalf("missing checksum for %s", artifact.Name)
		}
	}
}
