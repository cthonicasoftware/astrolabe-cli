package upload

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeFileSHA256(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		content        string
		expectedSHA256 string
	}{
		{
			name:           "empty file",
			content:        "",
			expectedSHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:           "hello world",
			content:        "hello world",
			expectedSHA256: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
		{
			name:           "json content",
			content:        `{"test": "data", "value": 123}`,
			expectedSHA256: "15257cb2ecbd02cdf22924e793a2739cdcd478701681801cadec05c89f88e0af",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFile := filepath.Join(tmpDir, "test.txt")
			if err := os.WriteFile(testFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("create test file: %v", err)
			}
			defer os.Remove(testFile)

			// Compute checksum
			checksum, err := computeFileSHA256(testFile)
			if err != nil {
				t.Fatalf("computeFileSHA256: %v", err)
			}

			// Verify checksum
			if checksum != tt.expectedSHA256 {
				t.Errorf("expected checksum %s, got %s", tt.expectedSHA256, checksum)
			}
		})
	}
}

func TestComputeFileSHA256_FileNotFound(t *testing.T) {
	_, err := computeFileSHA256("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestComputeFileSHA256_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	largeFile := filepath.Join(tmpDir, "large.txt")

	// Create a 1MB file
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	if err := os.WriteFile(largeFile, data, 0644); err != nil {
		t.Fatalf("create large file: %v", err)
	}

	// Should compute without error
	checksum, err := computeFileSHA256(largeFile)
	if err != nil {
		t.Fatalf("computeFileSHA256: %v", err)
	}

	// Verify it returns a valid hex string
	if len(checksum) != 64 {
		t.Errorf("expected 64-character SHA256 hex string, got %d characters", len(checksum))
	}

	// Verify it's lowercase hex
	for _, ch := range checksum {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Errorf("checksum contains non-hex character: %c", ch)
		}
	}
}
