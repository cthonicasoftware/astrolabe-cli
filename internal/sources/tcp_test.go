package sources

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// startTestTCPServer starts a TCP server on a random port and returns the port number and server close function
func startTestTCPServer(t *testing.T, dataToSend []string) (int, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			go func(c net.Conn) {
				defer c.Close()
				for _, data := range dataToSend {
					if _, err := c.Write([]byte(data)); err != nil {
						return
					}
					time.Sleep(10 * time.Millisecond) // Small delay between sends
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

func TestTCP_BasicConnection(t *testing.T) {
	// Start test server
	testData := []string{"line1\n", "line2\n", "line3\n"}
	port, cleanup := startTestTCPServer(t, testData)
	defer cleanup()

	// Create TCP source
	tcp, err := NewTCP("127.0.0.1", port)
	if err != nil {
		t.Fatalf("NewTCP failed: %v", err)
	}

	// Open connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := tcp.Open(ctx); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer tcp.Close()

	// Read frames
	var frames []string
	timeout := time.After(1 * time.Second)

	for {
		select {
		case frame, ok := <-tcp.Frames():
			if !ok {
				goto done
			}
			frames = append(frames, string(frame))
			if len(frames) >= len(testData) {
				goto done
			}
		case <-timeout:
			goto done
		}
	}

done:
	// Verify we got all data
	if len(frames) < len(testData) {
		t.Errorf("Expected at least %d frames, got %d", len(testData), len(frames))
	}
}

func TestTCP_InvalidPort(t *testing.T) {
	testCases := []struct {
		name string
		port int
	}{
		{"zero port", 0},
		{"negative port", -1},
		{"too large", 70000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewTCP("127.0.0.1", tc.port)
			if err == nil {
				t.Error("Expected error for invalid port, got nil")
			}
		})
	}
}

func TestTCP_EmptyHost(t *testing.T) {
	_, err := NewTCPWithConfig(TCPConfig{
		Host: "",
		Port: 8080,
	})
	if err == nil {
		t.Error("Expected error for empty host, got nil")
	}
}

func TestTCP_ConnectionRefused(t *testing.T) {
	// Use a port that's unlikely to be in use
	tcp, err := NewTCP("127.0.0.1", 59999)
	if err != nil {
		t.Fatalf("NewTCP failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = tcp.Open(ctx)
	if err == nil {
		defer tcp.Close()
		t.Error("Expected connection error, got nil")
	}
}

func TestTCP_Meta(t *testing.T) {
	tcp, err := NewTCP("example.com", 9000)
	if err != nil {
		t.Fatalf("NewTCP failed: %v", err)
	}

	meta := tcp.Meta()

	if meta.Kind != "tcp" {
		t.Errorf("Expected Kind=tcp, got %q", meta.Kind)
	}

	expectedAddr := "example.com:9000"
	if meta.Addr != expectedAddr {
		t.Errorf("Expected Addr=%q, got %q", expectedAddr, meta.Addr)
	}
}

func TestTCP_CloseWithoutOpen(t *testing.T) {
	tcp, err := NewTCP("127.0.0.1", 8080)
	if err != nil {
		t.Fatalf("NewTCP failed: %v", err)
	}

	// Should not panic
	if err := tcp.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestTCP_ContextCancellation(t *testing.T) {
	// Start server that sends data slowly
	port, cleanup := startTestTCPServer(t, []string{"data1\n", "data2\n", "data3\n"})
	defer cleanup()

	tcp, err := NewTCP("127.0.0.1", port)
	if err != nil {
		t.Fatalf("NewTCP failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	if err := tcp.Open(ctx); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer tcp.Close()

	// Read one frame
	select {
	case <-tcp.Frames():
		// Got data, good
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for data")
	}

	// Cancel context
	cancel()

	// Channel should close
	select {
	case _, ok := <-tcp.Frames():
		if ok {
			// Keep draining until closed
			for range tcp.Frames() {
			}
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Channel didn't close after context cancellation")
	}
}

func TestTCP_CustomConfig(t *testing.T) {
	port, cleanup := startTestTCPServer(t, []string{"test\n"})
	defer cleanup()

	cfg := TCPConfig{
		Host:           "127.0.0.1",
		Port:           port,
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    0,
		BufferSize:     8192,
	}

	tcp, err := NewTCPWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTCPWithConfig failed: %v", err)
	}

	// Verify config was applied
	if tcp.ConnectTimeout != 5*time.Second {
		t.Errorf("Expected ConnectTimeout=5s, got %v", tcp.ConnectTimeout)
	}

	if tcp.BufferSize != 8192 {
		t.Errorf("Expected BufferSize=8192, got %d", tcp.BufferSize)
	}

	// Should connect successfully
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := tcp.Open(ctx); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer tcp.Close()
}

func TestTCP_Address(t *testing.T) {
	tcp, err := NewTCP("192.168.1.1", 5000)
	if err != nil {
		t.Fatalf("NewTCP failed: %v", err)
	}

	expected := "192.168.1.1:5000"
	if tcp.Address() != expected {
		t.Errorf("Expected Address()=%q, got %q", expected, tcp.Address())
	}
}

func TestTCP_LargeData(t *testing.T) {
	// Send large chunk of data
	largeData := make([]byte, 10000)
	for i := range largeData {
		largeData[i] = byte('A' + (i % 26))
	}
	largeData[len(largeData)-1] = '\n'

	port, cleanup := startTestTCPServer(t, []string{string(largeData)})
	defer cleanup()

	tcp, err := NewTCP("127.0.0.1", port)
	if err != nil {
		t.Fatalf("NewTCP failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := tcp.Open(ctx); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer tcp.Close()

	// Collect all frames
	var received []byte
	timeout := time.After(1 * time.Second)

loop:
	for {
		select {
		case frame, ok := <-tcp.Frames():
			if !ok {
				break loop
			}
			received = append(received, frame...)
			if len(received) >= len(largeData) {
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	// Should have received all data
	if len(received) < len(largeData) {
		t.Errorf("Expected at least %d bytes, got %d", len(largeData), len(received))
	}
}

func TestTCP_MultipleConnections(t *testing.T) {
	// Test that server can handle multiple connections
	port, cleanup := startTestTCPServer(t, []string{"data\n"})
	defer cleanup()

	// Create two TCP sources to same server
	tcp1, err := NewTCP("127.0.0.1", port)
	if err != nil {
		t.Fatalf("NewTCP 1 failed: %v", err)
	}

	tcp2, err := NewTCP("127.0.0.1", port)
	if err != nil {
		t.Fatalf("NewTCP 2 failed: %v", err)
	}

	ctx := context.Background()

	// Open both connections
	if err := tcp1.Open(ctx); err != nil {
		t.Fatalf("Open 1 failed: %v", err)
	}
	defer tcp1.Close()

	if err := tcp2.Open(ctx); err != nil {
		t.Fatalf("Open 2 failed: %v", err)
	}
	defer tcp2.Close()

	// Both should receive data
	timeout := time.After(1 * time.Second)

	select {
	case <-tcp1.Frames():
		// Good
	case <-timeout:
		t.Error("tcp1 didn't receive data")
	}

	select {
	case <-tcp2.Frames():
		// Good
	case <-timeout:
		t.Error("tcp2 didn't receive data")
	}
}

func TestTCP_DefaultConfig(t *testing.T) {
	defaults := DefaultTCPConfig()

	if defaults.ConnectTimeout != 10*time.Second {
		t.Errorf("Expected default ConnectTimeout=10s, got %v", defaults.ConnectTimeout)
	}

	if defaults.ReadTimeout != 0 {
		t.Errorf("Expected default ReadTimeout=0, got %v", defaults.ReadTimeout)
	}

	if defaults.BufferSize != 4096 {
		t.Errorf("Expected default BufferSize=4096, got %d", defaults.BufferSize)
	}
}

func TestTCP_ReconnectAfterClose(t *testing.T) {
	port, cleanup := startTestTCPServer(t, []string{"data\n"})
	defer cleanup()

	tcp, err := NewTCP("127.0.0.1", port)
	if err != nil {
		t.Fatalf("NewTCP failed: %v", err)
	}

	ctx := context.Background()

	// First connection
	if err := tcp.Open(ctx); err != nil {
		t.Fatalf("First Open failed: %v", err)
	}

	// Read some data
	select {
	case <-tcp.Frames():
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for data")
	}

	// Close
	if err := tcp.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Try to open again - this should work with a new connection
	// (Note: This requires recreating the TCP source in practice,
	// but tests that Close properly cleans up)
	tcp2, err := NewTCP("127.0.0.1", port)
	if err != nil {
		t.Fatalf("NewTCP 2 failed: %v", err)
	}

	if err := tcp2.Open(ctx); err != nil {
		t.Fatalf("Second Open failed: %v", err)
	}
	defer tcp2.Close()
}

// ============================================================================
// High-Frequency Data Stream Tests
// ============================================================================

// startSequencedTCPServer sends numbered messages at high speed with no inter-message delay.
// Each message is: 4-byte big-endian sequence number + payload padded to msgSize, terminated by '\n'.
// Returns port, total bytes sent per connection, and cleanup func.
func startSequencedTCPServer(t *testing.T, msgSize int, msgCount int) (port int, bytesPerConn int, cleanup func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start sequenced server: %v", err)
	}

	port = listener.Addr().(*net.TCPAddr).Port
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, msgSize)
				for i := 0; i < msgCount; i++ {
					// Embed sequence number in first 4 bytes
					binary.BigEndian.PutUint32(buf[0:4], uint32(i))
					// Fill rest with deterministic pattern
					for j := 4; j < msgSize-1; j++ {
						buf[j] = byte('A' + (j % 26))
					}
					buf[msgSize-1] = '\n'
					if _, err := c.Write(buf); err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	bytesPerConn = msgSize * msgCount
	cleanup = func() {
		listener.Close()
		<-done
	}
	return
}

// TestTCP_HighFrequency_NoDataLoss verifies that all bytes arrive intact when a server
// blasts data with zero delay between writes (simulating a high-frequency test bench).
func TestTCP_HighFrequency_NoDataLoss(t *testing.T) {
	const msgSize = 128  // bytes per message
	const msgCount = 5000 // total messages

	port, expectedBytes, cleanup := startSequencedTCPServer(t, msgSize, msgCount)
	defer cleanup()

	tcp, err := NewTCPWithConfig(TCPConfig{
		Host:           "127.0.0.1",
		Port:           port,
		ConnectTimeout: 5 * time.Second,
		BufferSize:     65536,
	})
	if err != nil {
		t.Fatalf("NewTCPWithConfig: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := tcp.Open(ctx); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer tcp.Close()

	var received []byte
	timeout := time.After(5 * time.Second)

loop:
	for {
		select {
		case frame, ok := <-tcp.Frames():
			if !ok {
				break loop
			}
			received = append(received, frame...)
			if len(received) >= expectedBytes {
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	if len(received) != expectedBytes {
		t.Fatalf("byte count mismatch: got %d, want %d", len(received), expectedBytes)
	}

	// Verify every sequence number is present and in order
	for i := 0; i < msgCount; i++ {
		offset := i * msgSize
		seq := binary.BigEndian.Uint32(received[offset : offset+4])
		if int(seq) != i {
			t.Fatalf("sequence mismatch at message %d: got seq %d", i, seq)
		}
	}
}

// TestTCP_HighFrequency_Integrity hashes the sent and received streams
// to confirm bit-perfect transfer under sustained load.
func TestTCP_HighFrequency_Integrity(t *testing.T) {
	const msgSize = 256
	const msgCount = 3000

	// Pre-compute expected hash of the full stream
	expectedHash := sha256.New()
	buf := make([]byte, msgSize)
	for i := 0; i < msgCount; i++ {
		binary.BigEndian.PutUint32(buf[0:4], uint32(i))
		for j := 4; j < msgSize-1; j++ {
			buf[j] = byte('A' + (j % 26))
		}
		buf[msgSize-1] = '\n'
		expectedHash.Write(buf)
	}
	want := expectedHash.Sum(nil)

	port, expectedBytes, cleanup := startSequencedTCPServer(t, msgSize, msgCount)
	defer cleanup()

	tcp, err := NewTCPWithConfig(TCPConfig{
		Host:           "127.0.0.1",
		Port:           port,
		ConnectTimeout: 5 * time.Second,
		BufferSize:     65536,
	})
	if err != nil {
		t.Fatalf("NewTCPWithConfig: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := tcp.Open(ctx); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer tcp.Close()

	gotHash := sha256.New()
	totalBytes := 0
	timeout := time.After(5 * time.Second)

loop:
	for {
		select {
		case frame, ok := <-tcp.Frames():
			if !ok {
				break loop
			}
			gotHash.Write(frame)
			totalBytes += len(frame)
			if totalBytes >= expectedBytes {
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	if totalBytes != expectedBytes {
		t.Fatalf("byte count mismatch: got %d, want %d", totalBytes, expectedBytes)
	}

	got := gotHash.Sum(nil)
	if fmt.Sprintf("%x", got) != fmt.Sprintf("%x", want) {
		t.Fatalf("SHA-256 mismatch:\n  got  %x\n  want %x", got, want)
	}
}

// TestTCP_HighFrequency_ChannelBackpressure verifies that a slow consumer
// doesn't cause the TCP source to lose data (channel has buffer of 16).
func TestTCP_HighFrequency_ChannelBackpressure(t *testing.T) {
	const msgSize = 64
	const msgCount = 500

	port, expectedBytes, cleanup := startSequencedTCPServer(t, msgSize, msgCount)
	defer cleanup()

	tcp, err := NewTCPWithConfig(TCPConfig{
		Host:           "127.0.0.1",
		Port:           port,
		ConnectTimeout: 5 * time.Second,
		BufferSize:     4096, // Small buffer to stress backpressure path
	})
	if err != nil {
		t.Fatalf("NewTCPWithConfig: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := tcp.Open(ctx); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer tcp.Close()

	var received []byte
	timeout := time.After(10 * time.Second)

loop:
	for {
		select {
		case frame, ok := <-tcp.Frames():
			if !ok {
				break loop
			}
			received = append(received, frame...)
			// Simulate slow consumer — sleep every 50 frames
			if len(received)/(msgSize) > 0 && (len(received)/msgSize)%50 == 0 {
				time.Sleep(5 * time.Millisecond)
			}
			if len(received) >= expectedBytes {
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	if len(received) != expectedBytes {
		t.Fatalf("slow consumer lost data: got %d bytes, want %d", len(received), expectedBytes)
	}
}

// TestTCP_HighFrequency_ServerCloseMidStream verifies graceful handling when
// the server disconnects partway through a high-frequency stream.
func TestTCP_HighFrequency_ServerCloseMidStream(t *testing.T) {
	const msgSize = 64
	const totalMsgs = 2000

	// Server that closes connection after sending half the messages
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	var serverSent atomic.Int64
	done := make(chan struct{})

	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, msgSize)
		for j := 4; j < msgSize-1; j++ {
			buf[j] = byte('A' + (j % 26))
		}
		buf[msgSize-1] = '\n'

		for i := 0; i < totalMsgs/2; i++ {
			binary.BigEndian.PutUint32(buf[0:4], uint32(i))
			n, err := conn.Write(buf)
			if err != nil {
				return
			}
			serverSent.Add(int64(n))
		}
		// Abrupt close after half the messages
	}()

	defer func() {
		listener.Close()
		<-done
	}()

	tcp, err := NewTCP("127.0.0.1", port)
	if err != nil {
		t.Fatalf("NewTCP: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := tcp.Open(ctx); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer tcp.Close()

	var totalReceived int
	for frame := range tcp.Frames() {
		totalReceived += len(frame)
	}

	sent := int(serverSent.Load())
	if totalReceived != sent {
		t.Fatalf("after server close: got %d bytes, server sent %d", totalReceived, sent)
	}
	t.Logf("server sent %d/%d messages then closed; client received all %d bytes",
		totalMsgs/2, totalMsgs, totalReceived)
}

// TestTCP_HighFrequency_BurstPattern simulates bursty traffic: rapid bursts
// separated by pauses, typical of real test-bench data acquisition.
func TestTCP_HighFrequency_BurstPattern(t *testing.T) {
	const msgSize = 128
	const burstSize = 100  // messages per burst
	const burstCount = 10
	const burstPause = 50 * time.Millisecond

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	totalMsgs := burstSize * burstCount
	expectedBytes := msgSize * totalMsgs

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, msgSize)
		for j := 4; j < msgSize-1; j++ {
			buf[j] = byte('A' + (j % 26))
		}
		buf[msgSize-1] = '\n'

		seq := 0
		for b := 0; b < burstCount; b++ {
			// Send burst with no delay
			for i := 0; i < burstSize; i++ {
				binary.BigEndian.PutUint32(buf[0:4], uint32(seq))
				if _, err := conn.Write(buf); err != nil {
					return
				}
				seq++
			}
			// Pause between bursts
			if b < burstCount-1 {
				time.Sleep(burstPause)
			}
		}
	}()

	defer func() {
		listener.Close()
		<-done
	}()

	tcp, err := NewTCPWithConfig(TCPConfig{
		Host:           "127.0.0.1",
		Port:           port,
		ConnectTimeout: 5 * time.Second,
		BufferSize:     32768,
	})
	if err != nil {
		t.Fatalf("NewTCPWithConfig: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := tcp.Open(ctx); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer tcp.Close()

	var received []byte
	timeout := time.After(8 * time.Second)

loop:
	for {
		select {
		case frame, ok := <-tcp.Frames():
			if !ok {
				break loop
			}
			received = append(received, frame...)
			if len(received) >= expectedBytes {
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	if len(received) != expectedBytes {
		t.Fatalf("burst pattern: got %d bytes, want %d", len(received), expectedBytes)
	}

	// Verify sequence continuity across bursts
	for i := 0; i < totalMsgs; i++ {
		offset := i * msgSize
		seq := binary.BigEndian.Uint32(received[offset : offset+4])
		if int(seq) != i {
			t.Fatalf("burst sequence break at msg %d: got seq %d", i, seq)
		}
	}

	t.Logf("received %d messages across %d bursts with no drops", totalMsgs, burstCount)
}

// Benchmark TCP reading performance
func BenchmarkTCP_Reading(b *testing.B) {
	// Create test data
	testData := make([]string, 100)
	for i := range testData {
		testData[i] = fmt.Sprintf("line%d\n", i)
	}

	port, cleanup := startTestTCPServer(nil, testData)
	defer cleanup()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tcp, _ := NewTCP("127.0.0.1", port)
		ctx := context.Background()
		tcp.Open(ctx)

		count := 0
		for range tcp.Frames() {
			count++
			if count >= len(testData) {
				break
			}
		}

		tcp.Close()
	}
}
