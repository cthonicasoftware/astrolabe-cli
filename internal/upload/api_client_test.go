package upload

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
)

func TestAPIClient_CreateRun(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		wantRunID      string
	}{
		{
			name: "successful creation",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Errorf("expected auth header with token")
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("expected json content type")
				}

				// Decode request body to verify structure
				var req CreateRunRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("failed to decode request: %v", err)
				}
				if req.ProjectID != "test-project" {
					t.Errorf("expected project_id=test-project, got %s", req.ProjectID)
				}

				// Send success response
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(CreateRunResponse{
					RunID: "remote-run-123",
				})
			},
			wantErr:   false,
			wantRunID: "remote-run-123",
		},
		{
			name: "server error",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("internal server error"))
			},
			wantErr: true,
		},
		{
			name: "unauthorized",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("invalid token"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			client := NewAPIClient(server.URL, "test-token", "test-project")

			run := &core.Run{
				Manifest: core.Manifest{
					SchemaVersion: "v1alpha1",
				},
				Source: core.SourceMeta{
					Kind: "serial",
					Port: "/dev/ttyUSB0",
				},
				Capture: core.CaptureSettings{
					Channels: []string{"serial"},
				},
				Started:      time.Now(),
				RecordsCount: 100,
			}

			runID, err := client.CreateRun(context.Background(), run)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if runID != tt.wantRunID {
					t.Errorf("expected runID=%s, got %s", tt.wantRunID, runID)
				}
			}
		})
	}
}

func TestAPIClient_GetPresignedURL(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		wantURL        string
		wantMethod     string
	}{
		{
			name: "successful presign request",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Errorf("expected auth header with token")
				}

				// Decode and verify request body
				var req PresignedURLRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("failed to decode request: %v", err)
				}
				if req.FileName != "manifest.json" {
					t.Errorf("expected file_name=manifest.json, got %s", req.FileName)
				}

				// Send presigned URL response
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(PresignedURLResponse{
					URL:    "https://s3.example.com/bucket/path?signature=xyz",
					Method: "PUT",
					Headers: map[string]string{
						"Content-MD5": "abc123",
					},
				})
			},
			wantErr:    false,
			wantURL:    "https://s3.example.com/bucket/path?signature=xyz",
			wantMethod: "PUT",
		},
		{
			name: "server error",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("service unavailable"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			client := NewAPIClient(server.URL, "test-token", "test-project")

			artifact := core.Artifact{
				Name:      "manifest.json",
				MediaType: "application/json",
				SizeBytes: 1024,
				Checksum: core.Checksum{
					Algorithm: "sha256",
					Value:     "abc123",
				},
			}

			resp, err := client.GetPresignedURL(context.Background(), "remote-run-123", artifact)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if resp.URL != tt.wantURL {
					t.Errorf("expected URL=%s, got %s", tt.wantURL, resp.URL)
				}
				if resp.Method != tt.wantMethod {
					t.Errorf("expected Method=%s, got %s", tt.wantMethod, resp.Method)
				}
			}
		})
	}
}

func TestAPIClient_ConfirmUpload(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
	}{
		{
			name: "successful confirmation",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Errorf("expected auth header with token")
				}

				// Decode and verify request body
				var req ConfirmUploadRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("failed to decode request: %v", err)
				}
				if req.FileName != "manifest.json" {
					t.Errorf("expected file_name=manifest.json, got %s", req.FileName)
				}

				w.WriteHeader(http.StatusOK)
			},
			wantErr: false,
		},
		{
			name: "no content response",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
			wantErr: false,
		},
		{
			name: "server error",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("internal error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			client := NewAPIClient(server.URL, "test-token", "test-project")

			artifact := core.Artifact{
				Name:      "manifest.json",
				MediaType: "application/json",
				SizeBytes: 1024,
				Checksum: core.Checksum{
					Algorithm: "sha256",
					Value:     "abc123",
				},
			}

			err := client.ConfirmUpload(context.Background(), "remote-run-123", artifact)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestAPIClient_ContextCancellation(t *testing.T) {
	// Test that context cancellation is properly handled
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow server
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token", "test-project")

	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	run := &core.Run{
		Manifest:     core.Manifest{SchemaVersion: "v1alpha1"},
		Source:       core.SourceMeta{Kind: "serial"},
		Capture:      core.CaptureSettings{},
		Started:      time.Now(),
		RecordsCount: 100,
	}

	_, err := client.CreateRun(ctx, run)
	if err == nil {
		t.Error("expected context cancellation error, got nil")
	}
}
