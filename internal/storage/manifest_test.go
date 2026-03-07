package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

func writeTestManifest(t *testing.T, dir string, doc ManifestDocument) string {
	t.Helper()
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func TestLoadManifestRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	completed := now.Add(10 * time.Second)
	doc := ManifestDocument{
		RunID:        "run-abc123",
		Schema:       "v1",
		Source:       core.SourceMeta{Kind: "serial", Port: "/dev/ttyUSB0", Baud: 115200},
		Manifest:     core.Manifest{SchemaVersion: "v1", Device: core.DeviceInfo{ID: "dev-1"}, Test: core.TestInfo{Plan: "burn-in"}},
		Capture:      core.CaptureSettings{SampleRateHz: 9600, Channels: []string{"ch1"}},
		Started:      now,
		Completed:    &completed,
		RecordsCount: 42,
		PrimaryData:  "run-abc123/data.ndjson",
	}

	path := writeTestManifest(t, tmp, doc)
	loaded, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if loaded.RunID != doc.RunID {
		t.Fatalf("RunID = %q, want %q", loaded.RunID, doc.RunID)
	}
	if loaded.Schema != doc.Schema {
		t.Fatalf("Schema = %q, want %q", loaded.Schema, doc.Schema)
	}
	if loaded.RecordsCount != doc.RecordsCount {
		t.Fatalf("RecordsCount = %d, want %d", loaded.RecordsCount, doc.RecordsCount)
	}
	if loaded.PrimaryData != doc.PrimaryData {
		t.Fatalf("PrimaryData = %q, want %q", loaded.PrimaryData, doc.PrimaryData)
	}
	if loaded.Source.Kind != doc.Source.Kind {
		t.Fatalf("Source.Kind = %q, want %q", loaded.Source.Kind, doc.Source.Kind)
	}
	if loaded.Completed == nil {
		t.Fatal("Completed is nil, want non-nil")
	}
}

func TestLoadManifestFileNotFound(t *testing.T) {
	_, err := LoadManifest("/nonexistent/path/manifest.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadManifestInvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "manifest.json")
	if err := os.WriteFile(path, []byte("not json {{"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoadManifestMissingRunID(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "manifest.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":"v1","records_count":1}`), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("expected error for missing run_id, got nil")
	}
}
