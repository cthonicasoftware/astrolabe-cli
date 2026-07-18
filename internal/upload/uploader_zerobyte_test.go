package upload

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

// TestUploadZeroByteArtifact probes exactly what the uploader transmits for an
// empty artifact (the data.jsonl case behind the failed Orrery uploads).
func TestUploadZeroByteArtifact(t *testing.T) {
	var gotLen int64 = -1
	var gotBodyLen int
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotLen = r.ContentLength
		b, _ := io.ReadAll(r.Body)
		gotBodyLen = len(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dir := t.TempDir()
	empty := filepath.Join(dir, "data.jsonl")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}

	art := core.Artifact{
		Name:      "data.jsonl",
		Path:      empty,
		MediaType: "application/x-ndjson",
		SizeBytes: 0,
	}
	presign := &ArtifactPresignResponse{URL: srv.URL, Method: "PUT"}

	u := NewUploader(0)
	res, err := u.Upload(context.Background(), art, presign)
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if !res.Success {
		t.Fatalf("Upload not marked success")
	}
	t.Logf("server saw: method=%s content_length=%d body_bytes=%d", gotMethod, gotLen, gotBodyLen)
	if gotMethod != "PUT" {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	if gotLen != 0 {
		t.Errorf("content_length = %d, want 0", gotLen)
	}
	if gotBodyLen != 0 {
		t.Errorf("body bytes = %d, want 0", gotBodyLen)
	}
}
