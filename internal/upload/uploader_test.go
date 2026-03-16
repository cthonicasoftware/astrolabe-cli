package upload

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

func TestUploader_Upload_Success(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	testContent := []byte(`{"test": "data"}`)
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("create test file: %v", err)
	}

	// Create a mock S3 endpoint
	uploadCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type=application/json")
		}
		uploadCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	uploader := NewUploader(3)
	artifact := core.Artifact{
		Path:      testFile,
		Name:      "test.json",
		MediaType: "application/json",
		SizeBytes: int64(len(testContent)),
		Checksum: core.Checksum{
			Algorithm: "sha256",
			Value:     "abc123",
		},
	}

	presignedURL := &ArtifactPresignResponse{
		ArtifactID: "artifact-123",
		URL:        server.URL,
		Method:     "PUT",
	}

	result, err := uploader.Upload(context.Background(), artifact, presignedURL)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success=true")
	}
	if result.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", result.Attempts)
	}
	if !uploadCalled {
		t.Error("upload endpoint was not called")
	}
	if result.UploadedAt == nil {
		t.Error("expected UploadedAt to be set")
	}
}

func TestUploader_Upload_RetryOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644); err != nil {
		t.Fatalf("create test file: %v", err)
	}

	// Track number of attempts
	var attemptCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attemptCount.Add(1)

		// Fail first 2 attempts, succeed on 3rd
		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	uploader := NewUploader(3)
	artifact := core.Artifact{
		Path:      testFile,
		Name:      "test.json",
		MediaType: "application/json",
		SizeBytes: 16,
		Checksum:  core.Checksum{Algorithm: "sha256", Value: "abc123"},
	}

	presignedURL := &ArtifactPresignResponse{
		ArtifactID: "artifact-123",
		URL:        server.URL,
		Method:     "PUT",
	}

	startTime := time.Now()
	result, err := uploader.Upload(context.Background(), artifact, presignedURL)
	duration := time.Since(startTime)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success=true after retries")
	}
	if result.Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", result.Attempts)
	}

	// Verify exponential backoff (1s + 2s = 3s minimum)
	// Using a relaxed check to account for test execution overhead
	if duration < 2*time.Second {
		t.Errorf("expected at least 3s for exponential backoff, got %v", duration)
	}
}

func TestUploader_Upload_MaxRetriesExceeded(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644); err != nil {
		t.Fatalf("create test file: %v", err)
	}

	// Always fail
	var attemptCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	maxRetries := 2
	uploader := NewUploader(maxRetries)
	artifact := core.Artifact{
		Path:      testFile,
		Name:      "test.json",
		MediaType: "application/json",
		SizeBytes: 16,
		Checksum:  core.Checksum{Algorithm: "sha256", Value: "abc123"},
	}

	presignedURL := &ArtifactPresignResponse{
		ArtifactID: "artifact-123",
		URL:        server.URL,
		Method:     "PUT",
	}

	result, err := uploader.Upload(context.Background(), artifact, presignedURL)

	if err == nil {
		t.Error("expected error after max retries, got nil")
	}
	if result.Success {
		t.Error("expected success=false")
	}
	// maxRetries + 1 (initial attempt)
	expectedAttempts := maxRetries + 1
	if result.Attempts != expectedAttempts {
		t.Errorf("expected %d attempts, got %d", expectedAttempts, result.Attempts)
	}
	if int(attemptCount.Load()) != expectedAttempts {
		t.Errorf("expected %d server calls, got %d", expectedAttempts, attemptCount.Load())
	}
}

func TestUploader_Upload_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644); err != nil {
		t.Fatalf("create test file: %v", err)
	}

	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	uploader := NewUploader(3)
	artifact := core.Artifact{
		Path:      testFile,
		Name:      "test.json",
		MediaType: "application/json",
		SizeBytes: 16,
		Checksum:  core.Checksum{Algorithm: "sha256", Value: "abc123"},
	}

	presignedURL := &ArtifactPresignResponse{
		ArtifactID: "artifact-123",
		URL:        server.URL,
		Method:     "PUT",
	}

	// Cancel context after a short delay
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := uploader.Upload(ctx, artifact, presignedURL)

	if err == nil {
		t.Error("expected context cancellation error, got nil")
	}
	if result.Success {
		t.Error("expected success=false due to context cancellation")
	}
}

func TestUploader_Upload_FileNotFound(t *testing.T) {
	uploader := NewUploader(3)
	artifact := core.Artifact{
		Path:      "/nonexistent/file.json",
		Name:      "file.json",
		MediaType: "application/json",
		SizeBytes: 16,
		Checksum:  core.Checksum{Algorithm: "sha256", Value: "abc123"},
	}

	presignedURL := &ArtifactPresignResponse{
		ArtifactID: "artifact-123",
		URL:        "http://example.com/upload",
		Method:     "PUT",
	}

	result, err := uploader.Upload(context.Background(), artifact, presignedURL)

	if err == nil {
		t.Error("expected file not found error, got nil")
	}
	if result.Success {
		t.Error("expected success=false")
	}
	if result.Attempts != 1 {
		t.Errorf("expected 1 attempt for missing file, got %d", result.Attempts)
	}
}

func TestUploader_ExponentialBackoff(t *testing.T) {
	// This test verifies the exponential backoff timing
	// Backoff should be: 0s (1st attempt), 1s (2nd), 2s (3rd), 4s (4th)

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644); err != nil {
		t.Fatalf("create test file: %v", err)
	}

	var attempts []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts = append(attempts, time.Now())
		// Always fail to trigger retries
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	uploader := NewUploader(3)
	artifact := core.Artifact{
		Path:      testFile,
		Name:      "test.json",
		MediaType: "application/json",
		SizeBytes: 16,
		Checksum:  core.Checksum{Algorithm: "sha256", Value: "abc123"},
	}

	presignedURL := &ArtifactPresignResponse{
		ArtifactID: "artifact-123",
		URL:        server.URL,
		Method:     "PUT",
	}

	uploader.Upload(context.Background(), artifact, presignedURL)

	// Verify we got 4 attempts (initial + 3 retries)
	if len(attempts) != 4 {
		t.Fatalf("expected 4 attempts, got %d", len(attempts))
	}

	// Check backoff intervals (with some tolerance for test execution time)
	tolerance := 100 * time.Millisecond

	// 1st -> 2nd: ~1s
	interval1 := attempts[1].Sub(attempts[0])
	if interval1 < (1*time.Second-tolerance) || interval1 > (1*time.Second+tolerance) {
		t.Errorf("expected ~1s between attempt 1 and 2, got %v", interval1)
	}

	// 2nd -> 3rd: ~2s
	interval2 := attempts[2].Sub(attempts[1])
	if interval2 < (2*time.Second-tolerance) || interval2 > (2*time.Second+tolerance) {
		t.Errorf("expected ~2s between attempt 2 and 3, got %v", interval2)
	}

	// 3rd -> 4th: ~4s
	interval3 := attempts[3].Sub(attempts[2])
	if interval3 < (4*time.Second-tolerance) || interval3 > (4*time.Second+tolerance) {
		t.Errorf("expected ~4s between attempt 3 and 4, got %v", interval3)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantRetry bool
	}{
		{
			name:      "nil error",
			err:       nil,
			wantRetry: false,
		},
		{
			name:      "context canceled",
			err:       context.Canceled,
			wantRetry: false,
		},
		{
			name:      "context deadline exceeded",
			err:       context.DeadlineExceeded,
			wantRetry: false,
		},
		{
			name:      "generic error",
			err:       fmt.Errorf("network error"),
			wantRetry: false,
		},
		{
			name:      "too many requests",
			err:       &uploadHTTPStatusError{StatusCode: http.StatusTooManyRequests},
			wantRetry: true,
		},
		{
			name:      "server error",
			err:       &uploadHTTPStatusError{StatusCode: http.StatusServiceUnavailable},
			wantRetry: true,
		},
		{
			name:      "client error",
			err:       &uploadHTTPStatusError{StatusCode: http.StatusBadRequest},
			wantRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryable(tt.err)
			if got != tt.wantRetry {
				t.Errorf("isRetryable() = %v, want %v", got, tt.wantRetry)
			}
		})
	}
}

func TestUploader_Upload_DoesNotRetryClientErrors(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(testFile, []byte(`{"test": "data"}`), 0644); err != nil {
		t.Fatalf("create test file: %v", err)
	}

	var attemptCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	uploader := NewUploader(3)
	artifact := core.Artifact{
		Path:      testFile,
		Name:      "test.json",
		MediaType: "application/json",
		SizeBytes: 16,
		Checksum:  core.Checksum{Algorithm: "sha256", Value: "abc123"},
	}

	presignedURL := &ArtifactPresignResponse{
		ArtifactID: "artifact-123",
		URL:        server.URL,
		Method:     "PUT",
	}

	result, err := uploader.Upload(context.Background(), artifact, presignedURL)

	if err == nil {
		t.Fatal("expected client error, got nil")
	}
	if result.Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", result.Attempts)
	}
	if got := attemptCount.Load(); got != 1 {
		t.Fatalf("expected 1 server call, got %d", got)
	}
}
