package upload

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/storage"
)

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

