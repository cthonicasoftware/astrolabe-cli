//go:build perf
// +build perf

package sources

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestTCP_Throughput measures actual throughput in messages per second.
func TestTCP_Throughput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping throughput test")
	}

	testCases := []struct {
		name     string
		msgSize  int
		msgCount int
	}{
		{"small_msgs_64B", 64, 10000},
		{"medium_msgs_512B", 512, 5000},
		{"large_msgs_4KB", 4096, 1000},
		{"xlarge_msgs_16KB", 16384, 500},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			port, cleanup := startHighThroughputServer(t, tc.msgSize, tc.msgCount)
			defer cleanup()

			tcp, err := NewTCPWithConfig(TCPConfig{
				Host:           "127.0.0.1",
				Port:           port,
				ConnectTimeout: 5 * time.Second,
				BufferSize:     65536,
			})
			if err != nil {
				t.Fatalf("NewTCPWithConfig failed: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			if err := tcp.Open(ctx); err != nil {
				t.Fatalf("Open failed: %v", err)
			}
			defer tcp.Close()

			start := time.Now()
			count := 0
			totalBytes := 0

			for frame := range tcp.Frames() {
				count++
				totalBytes += len(frame)
				if count >= tc.msgCount {
					break
				}
			}

			duration := time.Since(start)
			msgsPerSec := float64(count) / duration.Seconds()
			bytesPerSec := float64(totalBytes) / duration.Seconds()
			mbPerSec := bytesPerSec / (1024 * 1024)

			t.Logf("Duration: %v", duration)
			t.Logf("Messages: %d (%.0f/sec)", count, msgsPerSec)
			t.Logf("Throughput: %.2f MB/sec", mbPerSec)
		})
	}
}

// TestFile_LargeFileIngestion tests ingestion of multi-MB files.
func TestFile_LargeFileIngestion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large file test")
	}

	testCases := []struct {
		name     string
		sizeMB   int
		expected int64
	}{
		{"10MB", 10, 10 * 1024 * 1024},
		{"50MB", 50, 50 * 1024 * 1024},
		{"100MB", 100, 100 * 1024 * 1024},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filePath := createTestFileWithSize(t, tc.expected)

			info, err := os.Stat(filePath)
			if err != nil {
				t.Fatalf("Failed to stat file: %v", err)
			}
			t.Logf("Actual file size: %d bytes (%.2f MB)", info.Size(), float64(info.Size())/(1024*1024))

			file, err := NewFile(filePath)
			if err != nil {
				t.Fatalf("NewFile failed: %v", err)
			}

			ctx := context.Background()
			if err := file.Open(ctx); err != nil {
				t.Fatalf("Open failed: %v", err)
			}
			defer file.Close()

			start := time.Now()
			count := 0
			totalBytes := 0

			for frame := range file.Frames() {
				count++
				totalBytes += len(frame)
			}

			duration := time.Since(start)
			linesPerSec := float64(count) / duration.Seconds()
			bytesPerSec := float64(totalBytes) / duration.Seconds()
			mbPerSec := bytesPerSec / (1024 * 1024)

			t.Logf("Duration: %v", duration)
			t.Logf("Lines: %d (%.0f/sec)", count, linesPerSec)
			t.Logf("Throughput: %.2f MB/sec", mbPerSec)

			minMBPerSec := 50.0
			if mbPerSec < minMBPerSec {
				t.Errorf("Throughput too low: %.2f MB/sec (min: %.2f)", mbPerSec, minMBPerSec)
			}
		})
	}
}
