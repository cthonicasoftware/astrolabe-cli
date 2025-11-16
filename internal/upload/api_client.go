package upload

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
)

// APIClient handles communication with the QA backend server.
// It's responsible for creating run records and requesting presigned URLs.
type APIClient struct {
	baseURL    string
	authToken  string
	projectID  string
	httpClient *http.Client
}

// NewAPIClient constructs a client for the QA backend API.
func NewAPIClient(baseURL, authToken, projectID string) *APIClient {
	return &APIClient{
		baseURL:   baseURL,
		authToken: authToken,
		projectID: projectID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateRunRequest is sent to the backend to initialize a new run record.
type CreateRunRequest struct {
	ProjectID    string               `json:"project_id"`
	Manifest     core.Manifest        `json:"manifest"`
	Source       core.SourceMeta      `json:"source"`
	Capture      core.CaptureSettings `json:"capture"`
	StartedAt    time.Time            `json:"started_at"`
	Completed    *time.Time           `json:"completed_at,omitempty"`
	RecordsCount uint64               `json:"records_count"`
}

// CreateRunResponse contains the server-assigned run ID.
type CreateRunResponse struct {
	RunID string `json:"run_id"`
}

// CreateRun registers a new run with the QA backend and returns the server-assigned ID.
func (c *APIClient) CreateRun(ctx context.Context, run *core.Run) (string, error) {
	req := CreateRunRequest{
		ProjectID:    c.projectID,
		Manifest:     run.Manifest,
		Source:       run.Source,
		Capture:      run.Capture,
		StartedAt:    run.Started,
		Completed:    run.Completed,
		RecordsCount: run.RecordsCount,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal create run request: %w", err)
	}

	// Build HTTP request to create run endpoint
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/runs/", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build create run request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.authToken)
	// Use the local run.ID (ULID) as the idempotency key for safe retries
	// Backend can use this to deduplicate create requests
	httpReq.Header.Set("Idempotency-Key", run.ID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("create run request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create run failed: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	var createResp CreateRunResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", fmt.Errorf("decode create run response: %w", err)
	}

	return createResp.RunID, nil
}

// ArtifactPresignRequest represents a single artifact in the presign request.
type ArtifactPresignRequest struct {
	Filename       string `json:"filename"`        // Changed from file_name to match backend
	Role           string `json:"role"`            // Required by backend (manifest, data, log, etc.)
	ContentType    string `json:"content_type"`
	SizeBytes      int64  `json:"size_bytes"`
	ChecksumSHA256 string `json:"checksum_sha256"`
	Source         string `json:"source,omitempty"` // Optional, defaults to "cli"
}

// PresignedURLRequest asks the server for presigned URLs (batch format).
type PresignedURLRequest struct {
	Artifacts []ArtifactPresignRequest `json:"artifacts"`
}

// ArtifactPresignResponse represents a single artifact in the presign response.
type ArtifactPresignResponse struct {
	ArtifactID string            `json:"artifact_id"`          // Backend-assigned ULID
	URL        string            `json:"url,omitempty"`        // Presigned URL (empty if existing)
	Method     string            `json:"method,omitempty"`     // Usually "PUT"
	Headers    map[string]string `json:"headers,omitempty"`    // Additional headers required
	ExpiresAt  string            `json:"expires_at,omitempty"` // ISO timestamp
	Status     string            `json:"status,omitempty"`     // "existing" if artifact already uploaded
}

// PresignedURLResponse contains the batch response from the backend.
type PresignedURLResponse struct {
	Artifacts []ArtifactPresignResponse `json:"artifacts"`
}

// GetPresignedURL requests a presigned URL for uploading a single artifact.
// Note: Backend expects batch format, so we wrap the single artifact in an array.
func (c *APIClient) GetPresignedURL(ctx context.Context, remoteRunID string, artifact core.Artifact) (*ArtifactPresignResponse, error) {
	// Wrap single artifact in batch request
	req := PresignedURLRequest{
		Artifacts: []ArtifactPresignRequest{
			{
				Filename:       artifact.Name,
				Role:           string(artifact.Role),
				ContentType:    artifact.MediaType,
				SizeBytes:      artifact.SizeBytes,
				ChecksumSHA256: artifact.Checksum.Value,
				Source:         "cli",
			},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal presigned url request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/runs/%s/artifacts/presign", c.baseURL, remoteRunID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build presigned url request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("presigned url request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("presigned url request failed: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	var presignResp PresignedURLResponse
	if err := json.NewDecoder(resp.Body).Decode(&presignResp); err != nil {
		return nil, fmt.Errorf("decode presigned url response: %w", err)
	}

	// Extract first (and only) artifact from batch response
	if len(presignResp.Artifacts) == 0 {
		return nil, fmt.Errorf("backend returned empty artifacts array")
	}

	return &presignResp.Artifacts[0], nil
}

// ArtifactConfirmRequest represents a single artifact in the confirm request.
type ArtifactConfirmRequest struct {
	ArtifactID     string    `json:"artifact_id"`      // Backend-assigned ULID (required!)
	Filename       string    `json:"filename"`         // Changed from file_name to match backend
	ChecksumSHA256 string    `json:"checksum_sha256"`
	UploadedAt     time.Time `json:"uploaded_at"`
}

// ConfirmUploadRequest notifies the backend that artifact uploads completed (batch format).
type ConfirmUploadRequest struct {
	Artifacts []ArtifactConfirmRequest `json:"artifacts"`
}

// ConfirmUpload tells the backend that an artifact was successfully uploaded.
// Note: Backend expects batch format, so we wrap the single artifact in an array.
func (c *APIClient) ConfirmUpload(ctx context.Context, remoteRunID string, artifact core.Artifact) error {
	// Validate that we have the remote artifact ID (required by backend)
	if artifact.RemoteArtifactID == "" {
		return fmt.Errorf("artifact missing remote_artifact_id (must call GetPresignedURL first)")
	}

	// Wrap single artifact in batch request
	req := ConfirmUploadRequest{
		Artifacts: []ArtifactConfirmRequest{
			{
				ArtifactID:     artifact.RemoteArtifactID,
				Filename:       artifact.Name,
				ChecksumSHA256: artifact.Checksum.Value,
				UploadedAt:     time.Now(),
			},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal confirm upload request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/runs/%s/artifacts/confirm", c.baseURL, remoteRunID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build confirm upload request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("confirm upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("confirm upload failed: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
