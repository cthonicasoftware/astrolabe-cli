package sources

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileSource_Basic(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create file source
	f, err := NewFile(testFile)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	// Open the file
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := f.Open(ctx); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	// Read frames
	var frames []string
	for frame := range f.Frames() {
		frames = append(frames, string(frame))
	}

	// Verify we got all lines
	expected := 3
	if len(frames) != expected {
		t.Errorf("Expected %d frames, got %d", expected, len(frames))
	}

	// Verify content
	if len(frames) > 0 && frames[0] != "line1\n" {
		t.Errorf("Expected first frame to be 'line1\\n', got %q", frames[0])
	}
}

func TestFileSource_SkipLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.csv")

	content := "header1,header2\nval1,val2\nval3,val4\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create file source with skip lines
	cfg := FileConfig{
		Path:      testFile,
		SkipLines: 1, // skip header
	}

	f, err := NewFileWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewFileWithConfig failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := f.Open(ctx); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	var frames []string
	for frame := range f.Frames() {
		frames = append(frames, string(frame))
	}

	// Should only have 2 lines (header skipped)
	expected := 2
	if len(frames) != expected {
		t.Errorf("Expected %d frames, got %d", expected, len(frames))
	}

	// First frame should be first data row, not header
	if len(frames) > 0 && frames[0] != "val1,val2\n" {
		t.Errorf("Expected first frame to be 'val1,val2\\n', got %q", frames[0])
	}
}

func TestFileSource_NonExistentFile(t *testing.T) {
	_, err := NewFile("/nonexistent/path/to/file.txt")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestFileSource_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := NewFile(tmpDir)
	if err == nil {
		t.Error("Expected error for directory path, got nil")
	}
}

func TestFileSource_EmptyPath(t *testing.T) {
	_, err := NewFileWithConfig(FileConfig{Path: ""})
	if err == nil {
		t.Error("Expected error for empty path, got nil")
	}
}

func TestFileSource_Meta(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	f, err := NewFile(testFile)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	meta := f.Meta()

	if meta.Kind != "file" {
		t.Errorf("Expected Kind=file, got %q", meta.Kind)
	}

	if meta.Path != testFile {
		t.Errorf("Expected Path=%q, got %q", testFile, meta.Path)
	}
}

func TestFileSource_ChunkedMode(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")

	content := []byte("0123456789abcdef")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Read in 4-byte chunks
	cfg := FileConfig{
		Path:      testFile,
		ChunkSize: 4,
	}

	f, err := NewFileWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewFileWithConfig failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := f.Open(ctx); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	var chunks [][]byte
	for chunk := range f.Frames() {
		chunks = append(chunks, chunk)
	}

	// Should have 4 chunks of 4 bytes each
	expected := 4
	if len(chunks) != expected {
		t.Errorf("Expected %d chunks, got %d", expected, len(chunks))
	}

	// Verify first chunk
	if len(chunks) > 0 && string(chunks[0]) != "0123" {
		t.Errorf("Expected first chunk to be '0123', got %q", chunks[0])
	}
}

func TestFileSource_CloseWhileReadLoopActive(t *testing.T) {
	// Write enough data that the read loop won't finish before we cancel.
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	var content []byte
	for i := 0; i < 1000; i++ {
		content = append(content, []byte("line\n")...)
	}
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	f, err := NewFile(testFile)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	ctx := context.Background()
	if err := f.Open(ctx); err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Read a few frames to confirm the loop is live, then close mid-stream.
	ch := f.Frames()
	for i := 0; i < 3; i++ {
		select {
		case _, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed unexpectedly after %d frames", i)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for frame %d", i)
		}
	}

	// Close while readLoop is still running — must not double-close the fd.
	if err := f.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	// Drain the channel to let readLoop finish; channel must close cleanly.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range f.Frames() {
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("channel did not close after Close()")
	}
}

func TestFileSource_OpenTwiceWithoutCloseFails(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("line1\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	f, err := NewFile(testFile)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := f.Open(ctx); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	if err := f.Open(ctx); err == nil {
		t.Fatalf("expected second open to fail, got nil")
	}
}

func TestFileSource_ReopenAfterClose(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("line1\nline2\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	f, err := NewFile(testFile)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	readAll := func() []string {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := f.Open(ctx); err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		frames := []string{}
		for frame := range f.Frames() {
			frames = append(frames, string(frame))
		}
		if err := f.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
		return frames
	}

	first := readAll()
	second := readAll()

	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("expected two frames in each run, got %d and %d", len(first), len(second))
	}
	if first[0] != "line1\n" || second[0] != "line1\n" {
		t.Fatalf("unexpected first frame values: %q and %q", first[0], second[0])
	}
}
