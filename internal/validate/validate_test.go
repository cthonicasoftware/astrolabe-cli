package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/storage"
)

func createValidRunDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	runDir := filepath.Join(tmp, "run-abc123")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("create run dir: %v", err)
	}

	started := time.Date(2024, 12, 1, 12, 0, 0, 0, time.UTC)
	completed := started.Add(5 * time.Second)

	manifest := storage.ManifestDocument{
		RunID:  "run-abc123",
		Schema: "v1",
		Source: core.SourceMeta{
			Kind: "serial",
			Port: "/dev/ttyUSB0",
			Baud: 115200,
		},
		Manifest: core.Manifest{
			SchemaVersion: "v1",
			Device: core.DeviceInfo{
				ID: "device-001",
			},
			Test: core.TestInfo{
				Plan: "burn-in",
			},
		},
		Started:      started,
		Completed:    &completed,
		RecordsCount: 2,
	}

	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(runDir, "manifest.json"), manifestData, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	dataLines := []string{
		`{"ts":"2024-12-01T12:00:01Z","seq":1,"type":"sample","payload":{"value":42}}`,
		`{"ts":"2024-12-01T12:00:02Z","seq":2,"type":"sample","payload":{"value":43}}`,
	}
	dataContent := dataLines[0] + "\n" + dataLines[1] + "\n"
	if err := os.WriteFile(filepath.Join(runDir, "data.jsonl"), []byte(dataContent), 0o644); err != nil {
		t.Fatalf("write data: %v", err)
	}

	return runDir
}

func TestValidateValidRunDir(t *testing.T) {
	runDir := createValidRunDir(t)

	v := New(runDir)
	result := v.Run()

	if !result.Valid {
		t.Errorf("expected valid=true, got false with errors: %v", result.Errors)
	}

	if result.RunID != "run-abc123" {
		t.Errorf("expected run_id=run-abc123, got %s", result.RunID)
	}

	expectedChecks := []string{
		"directory_exists",
		"manifest_parse",
		"manifest_fields",
		"data_exists",
		"data_ndjson",
		"data_checksum",
		"records_count",
		"timestamps",
		"upload_state",
	}

	if len(result.Checks) != len(expectedChecks) {
		t.Errorf("expected %d checks, got %d", len(expectedChecks), len(result.Checks))
	}

	for i, name := range expectedChecks {
		if i >= len(result.Checks) {
			break
		}
		if result.Checks[i].Name != name {
			t.Errorf("check %d: expected name %s, got %s", i, name, result.Checks[i].Name)
		}
	}

	// upload_state should be skipped with a warning
	uploadCheck := findCheck(result.Checks, "upload_state")
	if uploadCheck == nil || uploadCheck.Status != StatusSkipped {
		t.Errorf("expected upload_state to be skipped")
	}
	if len(result.Warnings) == 0 {
		t.Error("expected warning about missing upload_state.json")
	}
}

func TestValidateNonexistentDir(t *testing.T) {
	v := New("/nonexistent/path/to/run")
	result := v.Run()

	if result.Valid {
		t.Error("expected valid=false for nonexistent directory")
	}

	dirCheck := findCheck(result.Checks, "directory_exists")
	if dirCheck == nil || dirCheck.Status != StatusFailed {
		t.Error("expected directory_exists check to fail")
	}

	// All subsequent checks should be skipped
	for _, check := range result.Checks {
		if check.Name != "directory_exists" && check.Status != StatusSkipped {
			t.Errorf("expected %s to be skipped after directory_exists failed", check.Name)
		}
	}
}

func TestValidateInvalidManifestJSON(t *testing.T) {
	tmp := t.TempDir()
	runDir := filepath.Join(tmp, "bad-manifest")
	os.MkdirAll(runDir, 0o755)

	os.WriteFile(filepath.Join(runDir, "manifest.json"), []byte("not valid json"), 0o644)
	os.WriteFile(filepath.Join(runDir, "data.jsonl"), []byte(`{"test":1}`+"\n"), 0o644)

	v := New(runDir)
	result := v.Run()

	if result.Valid {
		t.Error("expected valid=false for invalid manifest JSON")
	}

	manifestCheck := findCheck(result.Checks, "manifest_parse")
	if manifestCheck == nil || manifestCheck.Status != StatusFailed {
		t.Error("expected manifest_parse check to fail")
	}
}

func TestValidateMissingManifestFields(t *testing.T) {
	tmp := t.TempDir()
	runDir := filepath.Join(tmp, "missing-fields")
	os.MkdirAll(runDir, 0o755)

	// Minimal manifest missing required fields
	manifest := map[string]any{
		"run_id":         "run-123",
		"schema_version": "v1",
		"started":        "2024-12-01T12:00:00Z",
		"source":         map[string]any{"kind": "serial"}, // missing port
		"manifest": map[string]any{
			"device": map[string]any{}, // missing id
			"test":   map[string]any{}, // missing plan
		},
	}
	manifestData, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(runDir, "manifest.json"), manifestData, 0o644)
	os.WriteFile(filepath.Join(runDir, "data.jsonl"), []byte(`{"test":1}`+"\n"), 0o644)

	v := New(runDir)
	result := v.Run()

	if result.Valid {
		t.Error("expected valid=false for missing manifest fields")
	}

	fieldsCheck := findCheck(result.Checks, "manifest_fields")
	if fieldsCheck == nil || fieldsCheck.Status != StatusFailed {
		t.Error("expected manifest_fields check to fail")
	}
}

func TestValidateMissingDataFile(t *testing.T) {
	tmp := t.TempDir()
	runDir := filepath.Join(tmp, "no-data")
	os.MkdirAll(runDir, 0o755)

	started := time.Now()
	manifest := storage.ManifestDocument{
		RunID:   "run-123",
		Schema:  "v1",
		Source:  core.SourceMeta{Kind: "tcp", Addr: "localhost:8080"},
		Started: started,
		Manifest: core.Manifest{
			Device: core.DeviceInfo{ID: "dev-1"},
			Test:   core.TestInfo{Plan: "test-plan"},
		},
	}
	manifestData, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(runDir, "manifest.json"), manifestData, 0o644)
	// No data.jsonl file

	v := New(runDir)
	result := v.Run()

	if result.Valid {
		t.Error("expected valid=false for missing data file")
	}

	dataCheck := findCheck(result.Checks, "data_exists")
	if dataCheck == nil || dataCheck.Status != StatusFailed {
		t.Error("expected data_exists check to fail")
	}
}

func TestValidateInvalidNDJSON(t *testing.T) {
	tmp := t.TempDir()
	runDir := filepath.Join(tmp, "bad-ndjson")
	os.MkdirAll(runDir, 0o755)

	started := time.Now()
	manifest := storage.ManifestDocument{
		RunID:        "run-123",
		Schema:       "v1",
		Source:       core.SourceMeta{Kind: "file", Path: "/data/test.csv"},
		Started:      started,
		RecordsCount: 3,
		Manifest: core.Manifest{
			Device: core.DeviceInfo{ID: "dev-1"},
			Test:   core.TestInfo{Plan: "test-plan"},
		},
	}
	manifestData, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(runDir, "manifest.json"), manifestData, 0o644)

	// Invalid JSON on line 2
	dataContent := `{"valid":1}` + "\n" + `not json` + "\n" + `{"valid":3}` + "\n"
	os.WriteFile(filepath.Join(runDir, "data.jsonl"), []byte(dataContent), 0o644)

	v := New(runDir)
	result := v.Run()

	if result.Valid {
		t.Error("expected valid=false for invalid NDJSON")
	}

	ndjsonCheck := findCheck(result.Checks, "data_ndjson")
	if ndjsonCheck == nil || ndjsonCheck.Status != StatusFailed {
		t.Error("expected data_ndjson check to fail")
	}
}

func TestValidateRecordsCountMismatch(t *testing.T) {
	tmp := t.TempDir()
	runDir := filepath.Join(tmp, "count-mismatch")
	os.MkdirAll(runDir, 0o755)

	started := time.Now()
	manifest := storage.ManifestDocument{
		RunID:        "run-123",
		Schema:       "v1",
		Source:       core.SourceMeta{Kind: "serial", Port: "/dev/ttyUSB0"},
		Started:      started,
		RecordsCount: 10, // Says 10 records
		Manifest: core.Manifest{
			Device: core.DeviceInfo{ID: "dev-1"},
			Test:   core.TestInfo{Plan: "test-plan"},
		},
	}
	manifestData, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(runDir, "manifest.json"), manifestData, 0o644)

	// Only 2 actual records
	dataContent := `{"r":1}` + "\n" + `{"r":2}` + "\n"
	os.WriteFile(filepath.Join(runDir, "data.jsonl"), []byte(dataContent), 0o644)

	v := New(runDir)
	result := v.Run()

	if result.Valid {
		t.Error("expected valid=false for records count mismatch")
	}

	countCheck := findCheck(result.Checks, "records_count")
	if countCheck == nil || countCheck.Status != StatusFailed {
		t.Error("expected records_count check to fail")
	}
}

func TestValidateInvalidTimestamps(t *testing.T) {
	tmp := t.TempDir()
	runDir := filepath.Join(tmp, "bad-timestamps")
	os.MkdirAll(runDir, 0o755)

	started := time.Date(2024, 12, 1, 12, 0, 0, 0, time.UTC)
	completed := started.Add(-5 * time.Second) // Completed BEFORE started

	manifest := storage.ManifestDocument{
		RunID:        "run-123",
		Schema:       "v1",
		Source:       core.SourceMeta{Kind: "serial", Port: "/dev/ttyUSB0"},
		Started:      started,
		Completed:    &completed,
		RecordsCount: 1,
		Manifest: core.Manifest{
			Device: core.DeviceInfo{ID: "dev-1"},
			Test:   core.TestInfo{Plan: "test-plan"},
		},
	}
	manifestData, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(runDir, "manifest.json"), manifestData, 0o644)
	os.WriteFile(filepath.Join(runDir, "data.jsonl"), []byte(`{"r":1}`+"\n"), 0o644)

	v := New(runDir)
	result := v.Run()

	if result.Valid {
		t.Error("expected valid=false for invalid timestamps")
	}

	tsCheck := findCheck(result.Checks, "timestamps")
	if tsCheck == nil || tsCheck.Status != StatusFailed {
		t.Error("expected timestamps check to fail")
	}
}

func TestValidateWithUploadState(t *testing.T) {
	runDir := createValidRunDir(t)

	// Add valid upload_state.json
	uploadState := core.UploadState{
		Status:   core.UploadStatusSucceeded,
		Attempts: 1,
	}
	uploadData, _ := json.Marshal(uploadState)
	os.WriteFile(filepath.Join(runDir, "upload_state.json"), uploadData, 0o644)

	v := New(runDir)
	result := v.Run()

	if !result.Valid {
		t.Errorf("expected valid=true, got errors: %v", result.Errors)
	}

	uploadCheck := findCheck(result.Checks, "upload_state")
	if uploadCheck == nil || uploadCheck.Status != StatusPassed {
		t.Error("expected upload_state check to pass")
	}

	if len(result.Warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", result.Warnings)
	}
}

func TestValidateInvalidUploadState(t *testing.T) {
	runDir := createValidRunDir(t)

	// Add invalid upload_state.json
	os.WriteFile(filepath.Join(runDir, "upload_state.json"), []byte("not json"), 0o644)

	v := New(runDir)
	result := v.Run()

	if result.Valid {
		t.Error("expected valid=false for invalid upload_state.json")
	}

	uploadCheck := findCheck(result.Checks, "upload_state")
	if uploadCheck == nil || uploadCheck.Status != StatusFailed {
		t.Error("expected upload_state check to fail")
	}
}

func TestValidateResultJSON(t *testing.T) {
	runDir := createValidRunDir(t)

	v := New(runDir)
	result := v.Run()

	// Verify result can be marshaled to JSON
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var decoded Result
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if decoded.Valid != result.Valid {
		t.Error("JSON round-trip changed Valid field")
	}
	if decoded.RunID != result.RunID {
		t.Error("JSON round-trip changed RunID field")
	}
	if len(decoded.Checks) != len(result.Checks) {
		t.Error("JSON round-trip changed Checks count")
	}
}

func TestValidateRunsParentDirectory(t *testing.T) {
	// Create a structure like ~/.astrolabe/runs/ with subdirectories containing runs
	tmp := t.TempDir()
	runsDir := filepath.Join(tmp, "runs")
	os.MkdirAll(runsDir, 0o755)

	// Create a valid run subdirectory
	runDir := filepath.Join(runsDir, "run-123")
	os.MkdirAll(runDir, 0o755)

	manifest := storage.ManifestDocument{
		RunID:   "run-123",
		Schema:  "v1",
		Source:  core.SourceMeta{Kind: "serial", Port: "/dev/ttyUSB0"},
		Started: time.Now(),
		Manifest: core.Manifest{
			Device: core.DeviceInfo{ID: "dev-1"},
			Test:   core.TestInfo{Plan: "test-plan"},
		},
	}
	manifestData, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(runDir, "manifest.json"), manifestData, 0o644)
	os.WriteFile(filepath.Join(runDir, "data.jsonl"), []byte(`{"r":1}`+"\n"), 0o644)

	// Now try to validate the parent "runs" directory (not the run itself)
	v := New(runsDir)
	result := v.Run()

	if result.Valid {
		t.Error("expected valid=false for runs parent directory")
	}

	dirCheck := findCheck(result.Checks, "directory_exists")
	if dirCheck == nil || dirCheck.Status != StatusFailed {
		t.Error("expected directory_exists check to fail")
	}

	if dirCheck != nil && dirCheck.Error == "" {
		t.Error("expected error message about runs parent directory")
	}

	// Error should mention it's a runs parent directory
	if dirCheck != nil && !strings.Contains(dirCheck.Error, "parent") {
		t.Errorf("expected error to mention 'parent', got: %s", dirCheck.Error)
	}
}

func findCheck(checks []Check, name string) *Check {
	for i := range checks {
		if checks[i].Name == name {
			return &checks[i]
		}
	}
	return nil
}
