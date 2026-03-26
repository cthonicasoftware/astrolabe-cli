package upload

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/storage"
)

func writeTestRun(t *testing.T, root, runID string) string {
	t.Helper()

	runDir := filepath.Join(root, runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("create run dir: %v", err)
	}

	manifest := storage.ManifestDocument{
		RunID:  runID,
		Schema: "v1alpha1",
		Source: core.SourceMeta{
			Kind: "serial",
			Port: "/dev/ttyUSB0",
			Baud: 115200,
		},
		Manifest: core.Manifest{
			SchemaVersion: "v1alpha1",
			Device: core.DeviceInfo{
				ID: "device-001",
			},
			Test: core.TestInfo{
				Plan: "smoke-test",
			},
		},
		Capture: core.CaptureSettings{
			Channels: []string{"serial"},
		},
		Started:      time.Now(),
		RecordsCount: 1,
		PrimaryData:  filepath.Join(runDir, "data.jsonl"),
	}

	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(runDir, "manifest.json"), manifestData, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	dataPath := filepath.Join(runDir, "data.jsonl")
	if err := os.WriteFile(dataPath, []byte(`{"ts":"2024-01-01T00:00:00Z","seq":1,"type":"test","payload":{}}`+"\n"), 0o644); err != nil {
		t.Fatalf("write data: %v", err)
	}

	manifestChecksum, err := computeFileSHA256(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatalf("compute manifest checksum: %v", err)
	}
	dataChecksum, err := computeFileSHA256(dataPath)
	if err != nil {
		t.Fatalf("compute data checksum: %v", err)
	}
	manifestStat, err := os.Stat(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatalf("stat manifest: %v", err)
	}
	dataStat, err := os.Stat(dataPath)
	if err != nil {
		t.Fatalf("stat data: %v", err)
	}

	artifacts := storage.ArtifactsDocument{
		SchemaVersion: "1",
		Artifacts: []storage.ArtifactRecord{
			{
				Name:      "manifest.json",
				RelPath:   "manifest.json",
				MediaType: "application/json",
				Role:      core.ArtifactRoleManifest,
				SizeBytes: manifestStat.Size(),
				Checksum: core.Checksum{
					Algorithm: "sha256",
					Value:     manifestChecksum,
				},
				CreatedAt: manifest.Started,
			},
			{
				Name:      "data.jsonl",
				RelPath:   "data.jsonl",
				MediaType: "application/x-ndjson",
				Role:      core.ArtifactRoleData,
				SizeBytes: dataStat.Size(),
				Checksum: core.Checksum{
					Algorithm: "sha256",
					Value:     dataChecksum,
				},
				CreatedAt: manifest.Started,
			},
		},
	}
	artifactsData, err := json.MarshalIndent(artifacts, "", "  ")
	if err != nil {
		t.Fatalf("marshal artifacts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, storage.ArtifactsFileName), artifactsData, 0o644); err != nil {
		t.Fatalf("write artifacts: %v", err)
	}

	return runDir
}

// TestUploadRun_Integration demonstrates the full upload flow.
// This test shows how the upload system works end-to-end.
func TestUploadRun_Integration(t *testing.T) {
	// Skip in short mode since this is an integration test
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Create a temporary directory for test data
	tmpDir := t.TempDir()

	// Create a mock run directory
	runID := "test-run-12345"
	runDir := filepath.Join(tmpDir, runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("create run dir: %v", err)
	}

	// Create a mock manifest
	manifest := storage.ManifestDocument{
		RunID:  runID,
		Schema: "v1alpha1",
		Source: core.SourceMeta{
			Kind: "serial",
			Port: "/dev/ttyUSB0",
			Baud: 115200,
		},
		Manifest: core.Manifest{
			SchemaVersion: "v1alpha1",
			Device: core.DeviceInfo{
				ID: "device-001",
			},
			Test: core.TestInfo{
				Plan: "smoke-test",
			},
		},
		Capture: core.CaptureSettings{
			Channels: []string{"serial"},
		},
		Started:      time.Now(),
		RecordsCount: 100,
		PrimaryData:  filepath.Join(runDir, "data.jsonl"),
	}

	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(runDir, "manifest.json"), manifestData, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Create mock data file
	dataFile := filepath.Join(runDir, "data.jsonl")
	if err := os.WriteFile(dataFile, []byte(`{"ts":"2024-01-01T00:00:00Z","seq":1,"type":"test","payload":{}}`+"\n"), 0o644); err != nil {
		t.Fatalf("write data: %v", err)
	}

	// Create a mock server that simulates the QA backend
	uploadedArtifacts := make(map[string]bool)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/runs/":
			// Create run endpoint
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(CreateRunResponse{
				RunID: "remote-run-67890",
			})

		case "/api/v1/runs/remote-run-67890/artifacts/presign":
			// Presigned URL endpoint (batch format)
			// In a real system, this would return a URL to S3/cloud storage
			// For testing, we return a URL to our mock server
			var req PresignedURLRequest
			json.NewDecoder(r.Body).Decode(&req)

			// Generate artifact IDs and presigned URLs for each artifact
			var artifacts []ArtifactPresignResponse
			for i, art := range req.Artifacts {
				artifacts = append(artifacts, ArtifactPresignResponse{
					ArtifactID: fmt.Sprintf("artifact-%d-%s", i, art.Filename),
					URL:        server.URL + "/upload",
					Method:     "PUT",
				})
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(PresignedURLResponse{
				Artifacts: artifacts,
			})

		case "/upload":
			// Mock S3 upload endpoint
			w.WriteHeader(http.StatusOK)

		case "/api/v1/runs/remote-run-67890/artifacts/confirm":
			// Confirm upload endpoint (batch format)
			var req ConfirmUploadRequest
			json.NewDecoder(r.Body).Decode(&req)
			for _, art := range req.Artifacts {
				uploadedArtifacts[art.Filename] = true
			}
			w.WriteHeader(http.StatusOK)

		default:
			t.Logf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create upload client
	client := NewClient(Config{
		APIURL:     server.URL,
		AuthToken:  "test-token",
		ProjectID:  "test-project",
		CacheRoot:  tmpDir,
		MaxRetries: 2,
	})

	// Upload the run
	ctx := context.Background()
	if err := client.UploadRun(ctx, runID); err != nil {
		t.Fatalf("upload run: %v", err)
	}

	// Verify that artifacts were uploaded
	if !uploadedArtifacts["manifest.json"] {
		t.Error("manifest.json was not uploaded")
	}
	if !uploadedArtifacts["data.jsonl"] {
		t.Error("data.jsonl was not uploaded")
	}

	// Verify that upload state was saved
	statePath := filepath.Join(runDir, "upload_state.json")
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read upload state: %v", err)
	}

	var state core.UploadState
	if err := json.Unmarshal(stateData, &state); err != nil {
		t.Fatalf("parse upload state: %v", err)
	}

	if state.Status != core.UploadStatusSucceeded {
		t.Errorf("expected status=succeeded, got %s", state.Status)
	}
	if state.RemoteRunID != "remote-run-67890" {
		t.Errorf("expected remote_run_id=remote-run-67890, got %s", state.RemoteRunID)
	}
}

func TestUploadRun_ReusesExistingRemoteRunID(t *testing.T) {
	tmpDir := t.TempDir()
	runID := "test-run-resume"
	runDir := writeTestRun(t, tmpDir, runID)

	stateData, _ := json.Marshal(core.UploadState{
		Status:      core.UploadStatusInFlight,
		RemoteRunID: "remote-run-existing",
	})
	if err := os.WriteFile(filepath.Join(runDir, "upload_state.json"), stateData, 0o644); err != nil {
		t.Fatalf("write upload state: %v", err)
	}

	createRunCalls := 0
	presignCalls := 0

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/runs/":
			createRunCalls++
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(CreateRunResponse{RunID: "should-not-be-used"})
		case "/api/v1/runs/remote-run-existing/artifacts/presign":
			presignCalls++
			var req PresignedURLRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode presign request: %v", err)
			}
			var artifacts []ArtifactPresignResponse
			for i, art := range req.Artifacts {
				artifacts = append(artifacts, ArtifactPresignResponse{
					ArtifactID: fmt.Sprintf("artifact-%d-%s", i, art.Filename),
					URL:        server.URL + "/upload",
					Method:     http.MethodPut,
				})
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(PresignedURLResponse{Artifacts: artifacts})
		case "/upload":
			w.WriteHeader(http.StatusOK)
		case "/api/v1/runs/remote-run-existing/artifacts/confirm":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(Config{
		APIURL:     server.URL,
		AuthToken:  "test-token",
		ProjectID:  "test-project",
		CacheRoot:  tmpDir,
		MaxRetries: 1,
	})

	if err := client.UploadRun(context.Background(), runID); err != nil {
		t.Fatalf("upload run: %v", err)
	}

	if createRunCalls != 0 {
		t.Fatalf("expected CreateRun to be skipped, got %d calls", createRunCalls)
	}
	if presignCalls == 0 {
		t.Fatal("expected presign calls when resuming upload")
	}
}

func TestUploadRun_InvalidUploadStateFails(t *testing.T) {
	tmpDir := t.TempDir()
	runID := "test-run-invalid-state"
	runDir := writeTestRun(t, tmpDir, runID)

	if err := os.WriteFile(filepath.Join(runDir, "upload_state.json"), []byte("not-json"), 0o644); err != nil {
		t.Fatalf("write upload state: %v", err)
	}

	client := NewClient(Config{
		APIURL:    "https://example.invalid",
		CacheRoot: tmpDir,
	})

	err := client.UploadRun(context.Background(), runID)
	if err == nil {
		t.Fatal("expected invalid upload_state.json to fail")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "parse upload state") {
		t.Fatalf("expected parse upload state error, got %v", err)
	}
}

func TestLoadRun_LegacyArtifactsFallback(t *testing.T) {
	tmpDir := t.TempDir()
	runID := "test-run-legacy"
	runDir := writeTestRun(t, tmpDir, runID)

	if err := os.Remove(filepath.Join(runDir, storage.ArtifactsFileName)); err != nil {
		t.Fatalf("remove artifacts sidecar: %v", err)
	}

	client := NewClient(Config{
		APIURL:    "https://example.invalid",
		CacheRoot: tmpDir,
	})

	run, err := client.loadRun(runID)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(run.Artifacts) != 2 {
		t.Fatalf("artifacts count = %d, want 2", len(run.Artifacts))
	}
	if run.Artifacts[0].Checksum.Value == "" || run.Artifacts[1].Checksum.Value == "" {
		t.Fatal("legacy fallback should populate checksums")
	}
}

func TestLoadRun_RejectsUnsafeArtifactsRelPath(t *testing.T) {
	tmpDir := t.TempDir()
	runID := "test-run-unsafe-artifacts"
	runDir := writeTestRun(t, tmpDir, runID)

	doc := storage.ArtifactsDocument{
		SchemaVersion: "1",
		Artifacts: []storage.ArtifactRecord{
			{
				Name:      "manifest.json",
				RelPath:   "..\\manifest.json",
				MediaType: "application/json",
				Role:      core.ArtifactRoleManifest,
				Checksum:  core.Checksum{Algorithm: "sha256", Value: "abc"},
				CreatedAt: time.Now(),
			},
		},
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal artifacts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, storage.ArtifactsFileName), data, 0o644); err != nil {
		t.Fatalf("write artifacts: %v", err)
	}

	client := NewClient(Config{
		APIURL:    "https://example.invalid",
		CacheRoot: tmpDir,
	})

	_, err = client.loadRun(runID)
	if err == nil {
		t.Fatal("expected invalid artifacts rel_path to fail")
	}
	if !strings.Contains(err.Error(), "invalid rel_path") {
		t.Fatalf("expected invalid rel_path error, got %v", err)
	}
}
