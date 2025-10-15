package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
)

func TestSaveAndLoadMetadata(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmp)
	if origHome == "" {
		t.Setenv("USERPROFILE", tmp) // for Windows compatibility
	}

	meta := Metadata{
		Operator: "alice",
		Location: "lab",
		Device: core.DeviceInfo{
			ID:           "dev-1",
			Firmware:     "1.0.0",
			FirmwareHash: "deadbeef",
		},
		Test: core.TestInfo{
			Plan:    "burn-in",
			Variant: "optical",
			Run:     "001",
		},
		Tags: []string{"smoke", "release", "smoke"},
		Attributes: map[string]string{
			"fixture": "bench-3",
		},
	}

	path, err := SaveMetadata(meta)
	if err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("metadata file not written: %v", err)
	}

	loaded, err := LoadMetadata()
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}

	if loaded.Operator != meta.Operator {
		t.Fatalf("operator mismatch: %s", loaded.Operator)
	}
	if len(loaded.Tags) != 2 {
		t.Fatalf("expected duplicate tags to be deduped, got %v", loaded.Tags)
	}
	if loaded.Attributes["fixture"] != "bench-3" {
		t.Fatalf("attribute mismatch: %v", loaded.Attributes)
	}
}

func TestMetadataValidate(t *testing.T) {
	meta := Metadata{
		Tags:       []string{"ok", " "},
		Attributes: map[string]string{"": "value"},
	}

	if err := meta.Validate(); err == nil {
		t.Fatalf("expected validation failure for empty tag and key")
	}

	meta = Metadata{
		Tags: []string{"ok"},
		Attributes: map[string]string{
			"name": "",
		},
	}
	if err := meta.Validate(); err == nil {
		t.Fatalf("expected validation failure for empty attribute value")
	}

	meta = Metadata{
		Tags:       []string{"ok"},
		Attributes: map[string]string{"name": "value"},
	}
	if err := meta.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestMetadataFilePathUsesHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	path, err := metadataFilePath()
	if err != nil {
		t.Fatalf("metadataFilePath: %v", err)
	}
	want := filepath.Join(tmp, ".astrolabe", metadataFileName)
	if path != want {
		t.Fatalf("expected %s, got %s", want, path)
	}
}
