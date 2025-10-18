package upload

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/storage"
)

// Client orchestrates the full upload process:
// 1. Load run metadata from disk
// 2. Create run record on server
// 3. Request presigned URLs for each artifact
// 4. Upload files with retry
// 5. Confirm successful uploads
// 6. Update local state
type Client struct {
	apiClient  *APIClient
	uploader   *Uploader
	cacheRoot  string
}

// Config holds the settings needed to create an upload client.
type Config struct {
	APIURL     string
	AuthToken  string
	ProjectID  string
	CacheRoot  string
	MaxRetries int
}

// NewClient constructs an upload client from configuration.
func NewClient(cfg Config) *Client {
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3 // Sensible default
	}

	return &Client{
		apiClient:  NewAPIClient(cfg.APIURL, cfg.AuthToken, cfg.ProjectID),
		uploader:   NewUploader(maxRetries),
		cacheRoot:  cfg.CacheRoot,
	}
}

// UploadRun uploads a single run and all its artifacts to the server.
func (c *Client) UploadRun(ctx context.Context, runID string) error {
	// Load run metadata from disk
	run, err := c.loadRun(runID)
	if err != nil {
		return fmt.Errorf("load run: %w", err)
	}

	// Check if already uploaded
	if run.Upload.Status == core.UploadStatusSucceeded {
		return fmt.Errorf("run %s already uploaded", runID)
	}

	// Update upload state to queued
	run.Upload.Status = core.UploadStatusQueued
	if err := c.saveRunState(run); err != nil {
		return fmt.Errorf("save queued state: %w", err)
	}

	// Create run record on server
	remoteRunID, err := c.apiClient.CreateRun(ctx, run)
	if err != nil {
		run.Upload.Status = core.UploadStatusFailed
		run.Upload.LastError = err.Error()
		now := time.Now()
		run.Upload.LastAttempt = &now
		run.Upload.Attempts++
		_ = c.saveRunState(run)
		return fmt.Errorf("create run on server: %w", err)
	}

	run.Upload.RemoteRunID = remoteRunID
	run.Upload.Status = core.UploadStatusInFlight
	if err := c.saveRunState(run); err != nil {
		return fmt.Errorf("save in-flight state: %w", err)
	}

	// Upload each artifact
	for i := range run.Artifacts {
		artifact := &run.Artifacts[i]

		// Request presigned URL
		presignResp, err := c.apiClient.GetPresignedURL(ctx, remoteRunID, *artifact)
		if err != nil {
			run.Upload.Status = core.UploadStatusFailed
			run.Upload.LastError = fmt.Sprintf("presigned url for %s: %v", artifact.Name, err)
			now := time.Now()
			run.Upload.LastAttempt = &now
			run.Upload.Attempts++
			_ = c.saveRunState(run)
			return fmt.Errorf("get presigned url for %s: %w", artifact.Name, err)
		}

		// Upload the file
		result, err := c.uploader.Upload(ctx, *artifact, presignResp)
		if err != nil {
			run.Upload.Status = core.UploadStatusFailed
			run.Upload.LastError = fmt.Sprintf("upload %s: %v", artifact.Name, err)
			now := time.Now()
			run.Upload.LastAttempt = &now
			run.Upload.Attempts += result.Attempts
			_ = c.saveRunState(run)
			return fmt.Errorf("upload artifact %s: %w", artifact.Name, err)
		}

		// Update artifact metadata
		artifact.UploadedAt = result.UploadedAt

		// Confirm upload with server
		if err := c.apiClient.ConfirmUpload(ctx, remoteRunID, *artifact); err != nil {
			run.Upload.Status = core.UploadStatusFailed
			run.Upload.LastError = fmt.Sprintf("confirm upload for %s: %v", artifact.Name, err)
			now := time.Now()
			run.Upload.LastAttempt = &now
			run.Upload.Attempts++
			_ = c.saveRunState(run)
			return fmt.Errorf("confirm upload for %s: %w", artifact.Name, err)
		}
	}

	// Mark upload as succeeded
	run.Upload.Status = core.UploadStatusSucceeded
	now := time.Now()
	run.Upload.CompletedAt = &now
	run.Upload.LastError = ""
	if err := c.saveRunState(run); err != nil {
		return fmt.Errorf("save success state: %w", err)
	}

	return nil
}

// loadRun reads the run manifest from disk and reconstructs the Run object.
func (c *Client) loadRun(runID string) (*core.Run, error) {
	runDir := filepath.Join(c.cacheRoot, runID)
	manifestPath := filepath.Join(runDir, "manifest.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var doc storage.ManifestDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	// Load upload state if it exists
	uploadStatePath := filepath.Join(runDir, "upload_state.json")
	var uploadState core.UploadState
	if stateData, err := os.ReadFile(uploadStatePath); err == nil {
		_ = json.Unmarshal(stateData, &uploadState)
	} else {
		// Initialize default state
		uploadState = core.UploadState{
			Status: core.UploadStatusPending,
		}
	}

	// Reconstruct artifacts list
	var artifacts []core.Artifact

	// Manifest artifact
	manifestStat, err := os.Stat(manifestPath)
	if err == nil {
		artifacts = append(artifacts, core.Artifact{
			Name:      "manifest.json",
			Path:      manifestPath,
			MediaType: "application/json",
			Role:      core.ArtifactRoleManifest,
			SizeBytes: manifestStat.Size(),
			CreatedAt: doc.Started,
		})
	}

	// Data artifact
	dataPath := filepath.Join(runDir, "data.jsonl")
	dataStat, err := os.Stat(dataPath)
	if err == nil {
		artifacts = append(artifacts, core.Artifact{
			Name:      "data.jsonl",
			Path:      dataPath,
			MediaType: "application/x-ndjson",
			Role:      core.ArtifactRoleData,
			SizeBytes: dataStat.Size(),
			CreatedAt: doc.Started,
		})
	}

	run := &core.Run{
		ID:             doc.RunID,
		Source:         doc.Source,
		Manifest:       doc.Manifest,
		Capture:        doc.Capture,
		Started:        doc.Started,
		Completed:      doc.Completed,
		RecordsCount:   doc.RecordsCount,
		PrimaryDataURI: doc.PrimaryData,
		Artifacts:      artifacts,
		Upload:         uploadState,
	}

	return run, nil
}

// saveRunState persists the upload state to disk so we can resume later.
func (c *Client) saveRunState(run *core.Run) error {
	runDir := filepath.Join(c.cacheRoot, run.ID)
	statePath := filepath.Join(runDir, "upload_state.json")

	data, err := json.MarshalIndent(run.Upload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal upload state: %w", err)
	}

	if err := os.WriteFile(statePath, data, 0o644); err != nil {
		return fmt.Errorf("write upload state: %w", err)
	}

	return nil
}
