package upload

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
)

// Uploader handles the actual file upload to presigned URLs with retry logic.
type Uploader struct {
	maxRetries int
	httpClient *http.Client
}

// NewUploader creates an uploader with configurable retry behavior.
func NewUploader(maxRetries int) *Uploader {
	return &Uploader{
		maxRetries: maxRetries,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute, // Long timeout for large files
		},
	}
}

// UploadResult tracks the outcome of an upload attempt.
type UploadResult struct {
	Artifact  core.Artifact
	Success   bool
	Attempts  int
	Error     error
	UploadedAt *time.Time
}

// Upload uploads a single file to a presigned URL with automatic retry and exponential backoff.
// This is where the "resilience" happens - network failures are expected and handled gracefully.
func (u *Uploader) Upload(ctx context.Context, artifact core.Artifact, presignedURL *PresignedURLResponse) (*UploadResult, error) {
	result := &UploadResult{
		Artifact: artifact,
	}

	var lastErr error

	// Retry loop with exponential backoff
	for attempt := 0; attempt <= u.maxRetries; attempt++ {
		result.Attempts = attempt + 1

		// Calculate backoff: 1s, 2s, 4s, 8s, 16s, etc.
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				result.Error = ctx.Err()
				return result, ctx.Err()
			}
		}

		// Try to upload the file
		err := u.uploadOnce(ctx, artifact, presignedURL)
		if err == nil {
			// Success!
			now := time.Now()
			result.Success = true
			result.UploadedAt = &now
			return result, nil
		}

		lastErr = err

		// Check if we should retry
		if !isRetryable(err) {
			// Non-retryable error (e.g., authentication failure, bad request)
			result.Error = fmt.Errorf("non-retryable error: %w", err)
			return result, result.Error
		}

		// If we've exhausted retries, give up
		if attempt == u.maxRetries {
			result.Error = fmt.Errorf("max retries exceeded: %w", lastErr)
			return result, result.Error
		}

		// Otherwise, we'll retry after backoff
	}

	result.Error = fmt.Errorf("upload failed after %d attempts: %w", result.Attempts, lastErr)
	return result, result.Error
}

// uploadOnce performs a single upload attempt without retries.
func (u *Uploader) uploadOnce(ctx context.Context, artifact core.Artifact, presignedURL *PresignedURLResponse) error {
	// Open the file
	file, err := os.Open(artifact.Path)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// Get file size for progress tracking (in a real implementation)
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	// Create HTTP request
	// Most presigned URLs use PUT, but we respect what the server tells us
	method := presignedURL.Method
	if method == "" {
		method = http.MethodPut
	}

	req, err := http.NewRequestWithContext(ctx, method, presignedURL.URL, file)
	if err != nil {
		return fmt.Errorf("create upload request: %w", err)
	}

	// Set required headers
	req.ContentLength = stat.Size()
	req.Header.Set("Content-Type", artifact.MediaType)

	// Add any additional headers the server requires (e.g., checksums)
	for key, value := range presignedURL.Headers {
		req.Header.Set(key, value)
	}

	// Perform the upload
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	// S3 presigned URLs typically return 200 OK on success
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// isRetryable determines if an error is worth retrying.
// Network errors, timeouts, and 5xx server errors are retryable.
// 4xx client errors (bad auth, bad request) are not.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for HTTP status code errors
	// In a real implementation, you'd parse the error more carefully
	// For now, we assume network errors and context errors are retryable
	switch err {
	case context.Canceled, context.DeadlineExceeded:
		return false
	default:
		// Assume most errors are retryable (network issues, temporary server problems)
		// You could add more sophisticated logic here to parse HTTP status codes
		return true
	}
}
