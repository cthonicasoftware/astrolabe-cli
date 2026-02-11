package sources

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// ============================================================================
// TCP High-Throughput Benchmarks
// ============================================================================

// startHighThroughputServer starts a TCP server that sends data as fast as possible
func startHighThroughputServer(tb testing.TB, messageSize int, messageCount int) (int, func()) {
	tb.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("Failed to start test server: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	done := make(chan struct{})

	// Pre-generate message
	msg := make([]byte, messageSize)
	for i := range msg {
		if i == len(msg)-1 {
			msg[i] = '\n'
		} else {
			msg[i] = byte('A' + (i % 26))
		}
	}

	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			go func(c net.Conn) {
				defer c.Close()
				for i := 0; i < messageCount; i++ {
					if _, err := c.Write(msg); err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	cleanup := func() {
		listener.Close()
		<-done
	}

	return port, cleanup
}

// BenchmarkTCP_SmallMessages benchmarks receiving many small messages
func BenchmarkTCP_SmallMessages(b *testing.B) {
	const msgSize = 64
	const msgCount = 1000

	port, cleanup := startHighThroughputServer(b, msgSize, msgCount)
	defer cleanup()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tcp, err := NewTCP("127.0.0.1", port)
		if err != nil {
			b.Fatalf("NewTCP failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		if err := tcp.Open(ctx); err != nil {
			cancel()
			b.Fatalf("Open failed: %v", err)
		}

		count := 0
		for range tcp.Frames() {
			count++
			if count >= msgCount {
				break
			}
		}

		tcp.Close()
		cancel()
	}

	b.SetBytes(int64(msgSize * msgCount))
}

// BenchmarkTCP_LargeMessages benchmarks receiving large messages
func BenchmarkTCP_LargeMessages(b *testing.B) {
	const msgSize = 4096
	const msgCount = 100

	port, cleanup := startHighThroughputServer(b, msgSize, msgCount)
	defer cleanup()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tcp, err := NewTCP("127.0.0.1", port)
		if err != nil {
			b.Fatalf("NewTCP failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		if err := tcp.Open(ctx); err != nil {
			cancel()
			b.Fatalf("Open failed: %v", err)
		}

		count := 0
		for range tcp.Frames() {
			count++
			if count >= msgCount {
				break
			}
		}

		tcp.Close()
		cancel()
	}

	b.SetBytes(int64(msgSize * msgCount))
}

// BenchmarkTCP_HighFrequency benchmarks high-frequency message reception
func BenchmarkTCP_HighFrequency(b *testing.B) {
	const msgSize = 128
	const msgCount = 10000

	port, cleanup := startHighThroughputServer(b, msgSize, msgCount)
	defer cleanup()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tcp, err := NewTCPWithConfig(TCPConfig{
			Host:           "127.0.0.1",
			Port:           port,
			ConnectTimeout: 5 * time.Second,
			BufferSize:     65536, // Larger buffer for high-throughput
		})
		if err != nil {
			b.Fatalf("NewTCPWithConfig failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		if err := tcp.Open(ctx); err != nil {
			cancel()
			b.Fatalf("Open failed: %v", err)
		}

		count := 0
		for range tcp.Frames() {
			count++
			if count >= msgCount {
				break
			}
		}

		tcp.Close()
		cancel()
	}

	b.SetBytes(int64(msgSize * msgCount))
}

// ============================================================================
// Memory Profiling Tests
// ============================================================================

// TestTCP_MemoryUsage_LongRunning tests memory usage during extended capture
func TestTCP_MemoryUsage_LongRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running memory test")
	}

	const msgSize = 256
	const msgCount = 50000

	port, cleanup := startHighThroughputServer(t, msgSize, msgCount)
	defer cleanup()

	// Get baseline memory
	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	tcp, err := NewTCPWithConfig(TCPConfig{
		Host:           "127.0.0.1",
		Port:           port,
		ConnectTimeout: 5 * time.Second,
		BufferSize:     8192,
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

	// Read all messages
	count := 0
	for range tcp.Frames() {
		count++
		if count >= msgCount {
			break
		}
	}

	// Get final memory
	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	heapGrowth := int64(memAfter.HeapAlloc) - int64(memBefore.HeapAlloc)
	t.Logf("Messages received: %d", count)
	t.Logf("Heap growth: %d bytes (%.2f MB)", heapGrowth, float64(heapGrowth)/(1024*1024))
	t.Logf("Total allocs: %d", memAfter.TotalAlloc-memBefore.TotalAlloc)

	// Check for excessive memory growth (allow 50MB for 50k messages)
	var maxHeapGrowth int64 = 50 * 1024 * 1024
	if heapGrowth > maxHeapGrowth {
		t.Errorf("Excessive heap growth: %d bytes (max allowed: %d)", heapGrowth, maxHeapGrowth)
	}
}

// ============================================================================
// Throughput Measurement Tests
// ============================================================================

// TestTCP_Throughput measures actual throughput in messages per second
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

// BenchmarkTCP_BufferSizes compares different buffer sizes
func BenchmarkTCP_BufferSizes(b *testing.B) {
	const msgSize = 1024
	const msgCount = 1000

	bufferSizes := []int{1024, 4096, 16384, 65536}

	for _, bufSize := range bufferSizes {
		b.Run(fmt.Sprintf("buffer_%d", bufSize), func(b *testing.B) {
			port, cleanup := startHighThroughputServer(b, msgSize, msgCount)
			defer cleanup()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				tcp, err := NewTCPWithConfig(TCPConfig{
					Host:           "127.0.0.1",
					Port:           port,
					ConnectTimeout: 5 * time.Second,
					BufferSize:     bufSize,
				})
				if err != nil {
					b.Fatalf("NewTCPWithConfig failed: %v", err)
				}

				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

				if err := tcp.Open(ctx); err != nil {
					cancel()
					b.Fatalf("Open failed: %v", err)
				}

				count := 0
				for range tcp.Frames() {
					count++
					if count >= msgCount {
						break
					}
				}

				tcp.Close()
				cancel()
			}

			b.SetBytes(int64(msgSize * msgCount))
		})
	}
}

// ============================================================================
// File Ingestion Benchmarks
// ============================================================================

// createTestFile creates a temporary file with the specified number of lines
func createTestFile(b testing.TB, lineSize, lineCount int) string {
	b.Helper()

	tmpDir := b.(*testing.B).TempDir()
	filePath := filepath.Join(tmpDir, "test_data.jsonl")

	f, err := os.Create(filePath)
	if err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}
	defer f.Close()

	// Generate line template
	line := make([]byte, lineSize)
	for i := range line {
		if i == len(line)-1 {
			line[i] = '\n'
		} else {
			line[i] = byte('A' + (i % 26))
		}
	}

	for i := 0; i < lineCount; i++ {
		if _, err := f.Write(line); err != nil {
			b.Fatalf("Failed to write to test file: %v", err)
		}
	}

	return filePath
}

// createTestFileWithSize creates a file of approximately the specified size
func createTestFileWithSize(t testing.TB, sizeBytes int64) string {
	t.Helper()

	var tmpDir string
	switch v := t.(type) {
	case *testing.T:
		tmpDir = v.TempDir()
	case *testing.B:
		tmpDir = v.TempDir()
	}

	filePath := filepath.Join(tmpDir, "large_test_data.jsonl")

	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer f.Close()

	// Write in 4KB chunks
	chunk := make([]byte, 4096)
	for i := range chunk {
		if i == len(chunk)-1 {
			chunk[i] = '\n'
		} else if (i+1)%64 == 0 {
			chunk[i] = '\n'
		} else {
			chunk[i] = byte('A' + (i % 26))
		}
	}

	var written int64
	for written < sizeBytes {
		n, err := f.Write(chunk)
		if err != nil {
			t.Fatalf("Failed to write to test file: %v", err)
		}
		written += int64(n)
	}

	return filePath
}

// BenchmarkFile_SmallLines benchmarks reading many small lines
func BenchmarkFile_SmallLines(b *testing.B) {
	const lineSize = 64
	const lineCount = 10000

	filePath := createTestFile(b, lineSize, lineCount)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		file, err := NewFile(filePath)
		if err != nil {
			b.Fatalf("NewFile failed: %v", err)
		}

		ctx := context.Background()
		if err := file.Open(ctx); err != nil {
			b.Fatalf("Open failed: %v", err)
		}

		count := 0
		for range file.Frames() {
			count++
		}

		file.Close()
	}

	b.SetBytes(int64(lineSize * lineCount))
}

// BenchmarkFile_LargeLines benchmarks reading large lines
func BenchmarkFile_LargeLines(b *testing.B) {
	const lineSize = 4096
	const lineCount = 1000

	filePath := createTestFile(b, lineSize, lineCount)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		file, err := NewFile(filePath)
		if err != nil {
			b.Fatalf("NewFile failed: %v", err)
		}

		ctx := context.Background()
		if err := file.Open(ctx); err != nil {
			b.Fatalf("Open failed: %v", err)
		}

		count := 0
		for range file.Frames() {
			count++
		}

		file.Close()
	}

	b.SetBytes(int64(lineSize * lineCount))
}

// TestFile_LargeFileIngestion tests ingestion of multi-MB files
func TestFile_LargeFileIngestion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large file test")
	}

	// Test different file sizes
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

			// Verify file size
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

			// Minimum expected throughput (conservative)
			minMBPerSec := 50.0
			if mbPerSec < minMBPerSec {
				t.Errorf("Throughput too low: %.2f MB/sec (min: %.2f)", mbPerSec, minMBPerSec)
			}
		})
	}
}

// TestFile_MemoryUsage_LargeFile tests memory doesn't grow excessively during large file ingestion
func TestFile_MemoryUsage_LargeFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory test")
	}

	// Create 50MB file
	filePath := createTestFileWithSize(t, 50*1024*1024)

	// Get baseline memory
	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	file, err := NewFile(filePath)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	ctx := context.Background()
	if err := file.Open(ctx); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer file.Close()

	// Read all frames
	count := 0
	for range file.Frames() {
		count++
	}

	// Get final memory
	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	heapGrowth := int64(memAfter.HeapAlloc) - int64(memBefore.HeapAlloc)
	t.Logf("Lines read: %d", count)
	t.Logf("Heap growth: %d bytes (%.2f MB)", heapGrowth, float64(heapGrowth)/(1024*1024))

	// Should not grow more than 20MB for streaming a 50MB file
	var maxHeapGrowth int64 = 20 * 1024 * 1024
	if heapGrowth > maxHeapGrowth {
		t.Errorf("Excessive heap growth: %d bytes (max: %d)", heapGrowth, maxHeapGrowth)
	}
}
