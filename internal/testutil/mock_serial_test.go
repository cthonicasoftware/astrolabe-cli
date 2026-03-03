package testutil

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
)

func TestMockSerial_BasicOperation(t *testing.T) {
	// Create mock with test data
	frames := [][]byte{
		[]byte("Frame 1\n"),
		[]byte("Frame 2\n"),
		[]byte("Frame 3\n"),
	}

	mock := NewMockSerial(
		WithFrames(frames),
		WithFrameDelay(5*time.Millisecond),
	)

	ctx := context.Background()
	if err := mock.Open(ctx); err != nil {
		t.Fatalf("Failed to open mock serial: %v", err)
	}
	defer mock.Close()

	// Read all frames
	received := [][]byte{}
	timeout := time.After(1 * time.Second)

	for i := 0; i < len(frames); i++ {
		select {
		case frame := <-mock.Frames():
			received = append(received, frame)
		case <-timeout:
			t.Fatal("Timeout waiting for frames")
		}
	}

	// Verify we got all frames
	if len(received) != len(frames) {
		t.Fatalf("Expected %d frames, got %d", len(frames), len(received))
	}

	for i, frame := range received {
		if string(frame) != string(frames[i]) {
			t.Errorf("Frame %d mismatch: expected %q, got %q", i, frames[i], frame)
		}
	}
}

func TestMockSerial_ConfigurationOptions(t *testing.T) {
	cfg := sources.Config{
		Port:        "/dev/ttyUSB_TEST",
		Baud:        9600,
		Parity:      "E",
		DataBits:    7,
		StopBits:    "2",
		FlowControl: "hardware",
	}

	mock := NewMockSerial(WithConfig(cfg))

	if mock.Config.Port != cfg.Port {
		t.Errorf("Port mismatch: expected %s, got %s", cfg.Port, mock.Config.Port)
	}
	if mock.Config.Baud != cfg.Baud {
		t.Errorf("Baud mismatch: expected %d, got %d", cfg.Baud, mock.Config.Baud)
	}
	if mock.Config.Parity != cfg.Parity {
		t.Errorf("Parity mismatch: expected %s, got %s", cfg.Parity, mock.Config.Parity)
	}

	meta := mock.Meta()
	if meta.Port != cfg.Port {
		t.Errorf("Meta port mismatch: expected %s, got %s", cfg.Port, meta.Port)
	}
	if meta.Baud != cfg.Baud {
		t.Errorf("Meta baud mismatch: expected %d, got %d", cfg.Baud, meta.Baud)
	}
}

func TestMockSerial_ContextCancellation(t *testing.T) {
	// Create mock with many frames and slow delay
	frames := make([][]byte, 100)
	for i := range frames {
		frames[i] = []byte("Frame\n")
	}

	mock := NewMockSerial(
		WithFrames(frames),
		WithFrameDelay(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	if err := mock.Open(ctx); err != nil {
		t.Fatalf("Failed to open mock serial: %v", err)
	}
	defer mock.Close()

	// Read a few frames
	for i := 0; i < 5; i++ {
		select {
		case <-mock.Frames():
			// Got a frame
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for frame")
		}
	}

	// Cancel context
	cancel()

	// Channel should close
	timeout := time.After(500 * time.Millisecond)
	select {
	case _, ok := <-mock.Frames():
		if ok {
			t.Error("Expected channel to be closed after context cancellation")
		}
	case <-timeout:
		t.Error("Channel did not close after context cancellation")
	}
}

func TestMockSerial_ReadError(t *testing.T) {
	expectedError := errors.New("simulated read error")
	frames := [][]byte{
		[]byte("Frame 1\n"),
		[]byte("Frame 2\n"),
	}

	mock := NewMockSerial(
		WithFrames(frames),
		WithFrameDelay(5*time.Millisecond),
		WithReadError(expectedError),
	)

	ctx := context.Background()
	if err := mock.Open(ctx); err != nil {
		t.Fatalf("Failed to open mock serial: %v", err)
	}
	defer mock.Close()

	// Read available frames
	received := 0
	timeout := time.After(1 * time.Second)

readLoop:
	for {
		select {
		case frame, ok := <-mock.Frames():
			if !ok {
				// Channel closed (expected after error)
				break readLoop
			}
			if frame != nil {
				received++
			}
		case <-timeout:
			t.Fatal("Timeout waiting for frames or error")
		}
	}

	if received != len(frames) {
		t.Errorf("Expected to receive %d frames before error, got %d", len(frames), received)
	}
}

func TestMockSerial_Reset(t *testing.T) {
	frames := [][]byte{
		[]byte("Frame 1\n"),
		[]byte("Frame 2\n"),
	}

	mock := NewMockSerial(
		WithFrames(frames),
		WithFrameDelay(5*time.Millisecond),
	)

	// First run
	ctx := context.Background()
	if err := mock.Open(ctx); err != nil {
		t.Fatalf("Failed to open mock serial: %v", err)
	}

	// Read one frame
	select {
	case <-mock.Frames():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for first frame")
	}

	mock.Close()

	// Reset and run again
	mock.Reset()

	if err := mock.Open(ctx); err != nil {
		t.Fatalf("Failed to reopen mock serial: %v", err)
	}
	defer mock.Close()

	// Should be able to read all frames again
	received := 0
	timeout := time.After(500 * time.Millisecond)

	for i := 0; i < len(frames); i++ {
		select {
		case <-mock.Frames():
			received++
		case <-timeout:
			t.Fatal("Timeout waiting for frames after reset")
		}
	}

	if received != len(frames) {
		t.Errorf("Expected %d frames after reset, got %d", len(frames), received)
	}
}

func TestMockSerial_AddFrame(t *testing.T) {
	mock := NewMockSerial(
		WithFrames([][]byte{[]byte("Initial frame\n")}),
		WithFrameDelay(5*time.Millisecond),
	)

	// Add frames dynamically
	mock.AddFrame([]byte("Added frame 1\n"))
	mock.AddFrame([]byte("Added frame 2\n"))

	ctx := context.Background()
	if err := mock.Open(ctx); err != nil {
		t.Fatalf("Failed to open mock serial: %v", err)
	}
	defer mock.Close()

	// Read all frames (1 initial + 2 added)
	received := 0
	timeout := time.After(500 * time.Millisecond)

	for i := 0; i < 3; i++ {
		select {
		case <-mock.Frames():
			received++
		case <-timeout:
			t.Fatal("Timeout waiting for frames")
		}
	}

	if received != 3 {
		t.Errorf("Expected 3 frames (1 initial + 2 added), got %d", received)
	}
}

func TestMockSerial_FramesRemaining(t *testing.T) {
	frames := [][]byte{
		[]byte("Frame 1\n"),
		[]byte("Frame 2\n"),
		[]byte("Frame 3\n"),
	}

	mock := NewMockSerial(
		WithFrames(frames),
		WithFrameDelay(20*time.Millisecond), // Slower delay for predictable timing
	)

	if remaining := mock.FramesRemaining(); remaining != 3 {
		t.Errorf("Expected 3 frames remaining initially, got %d", remaining)
	}

	ctx := context.Background()
	if err := mock.Open(ctx); err != nil {
		t.Fatalf("Failed to open mock serial: %v", err)
	}
	defer mock.Close()

	// Read one frame
	select {
	case <-mock.Frames():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for first frame")
	}

	// Check immediately after receiving - cursor should be updated
	remaining := mock.FramesRemaining()
	if remaining < 1 || remaining > 2 {
		t.Errorf("Expected 1-2 frames remaining after reading 1, got %d", remaining)
	}

	// Read second frame
	select {
	case <-mock.Frames():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for second frame")
	}

	// Should have 1 or 0 frames remaining (depending on timing)
	remaining = mock.FramesRemaining()
	if remaining < 0 || remaining > 1 {
		t.Errorf("Expected 0-1 frames remaining after reading 2, got %d", remaining)
	}
}

func TestMockSerial_DoubleOpenError(t *testing.T) {
	mock := NewMockSerial()

	ctx := context.Background()
	if err := mock.Open(ctx); err != nil {
		t.Fatalf("First open failed: %v", err)
	}
	defer mock.Close()

	// Attempt to open again
	if err := mock.Open(ctx); err == nil {
		t.Error("Expected error when opening already-opened mock serial")
	}
}

func TestMockSerial_DoubleCloseError(t *testing.T) {
	mock := NewMockSerial()

	ctx := context.Background()
	if err := mock.Open(ctx); err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if err := mock.Close(); err != nil {
		t.Fatalf("First close failed: %v", err)
	}

	// Attempt to close again
	if err := mock.Close(); err == nil {
		t.Error("Expected error when closing already-closed mock serial")
	}
}

