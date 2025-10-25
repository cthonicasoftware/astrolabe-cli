package testutil_test

import (
	"context"
	"testing"
	"time"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/sources"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/testutil"
)

// This file demonstrates how to use MockSerial in integration tests
// that simulate real-world capture scenarios.

// simulateCaptureSession represents a typical capture workflow
func simulateCaptureSession(t *testing.T, device interface {
	Open(ctx context.Context) error
	Close() error
	Frames() <-chan []byte
}) int {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := device.Open(ctx); err != nil {
		t.Fatalf("Failed to open device: %v", err)
	}
	defer device.Close()

	recordCount := 0
	timeout := time.After(500 * time.Millisecond)

	for {
		select {
		case frame, ok := <-device.Frames():
			if !ok {
				return recordCount
			}
			if frame != nil {
				recordCount++
			}
		case <-timeout:
			return recordCount
		}
	}
}

// TestIntegration_CaptureWorkflow demonstrates using MockSerial in a real workflow test
func TestIntegration_CaptureWorkflow(t *testing.T) {
	// Simulate a sensor that emits 10 temperature readings
	sensorData := make([][]byte, 10)
	for i := 0; i < 10; i++ {
		sensorData[i] = []byte("TEMP:23.5C\n")
	}

	mock := testutil.NewMockSerial(
		testutil.WithFrames(sensorData),
		testutil.WithFrameDelay(20*time.Millisecond),
	)

	recordCount := simulateCaptureSession(t, mock)

	if recordCount != 10 {
		t.Errorf("Expected 10 records, got %d", recordCount)
	}
}

// TestIntegration_DeviceReconnection tests handling device disconnection
func TestIntegration_DeviceReconnection(t *testing.T) {
	// First connection - a few frames then error
	mock := testutil.NewMockSerial(
		testutil.WithFrames([][]byte{
			[]byte("Frame 1\n"),
			[]byte("Frame 2\n"),
		}),
		testutil.WithFrameDelay(10*time.Millisecond),
	)

	recordCount := simulateCaptureSession(t, mock)

	if recordCount != 2 {
		t.Errorf("First session: expected 2 records, got %d", recordCount)
	}

	// Simulate reconnection by creating a new mock
	// (Reset doesn't clear frames, it just resets position)
	mock2 := testutil.NewMockSerial(
		testutil.WithFrames([][]byte{
			[]byte("Frame 3\n"),
			[]byte("Frame 4\n"),
		}),
		testutil.WithFrameDelay(10*time.Millisecond),
	)

	recordCount = simulateCaptureSession(t, mock2)

	if recordCount != 2 {
		t.Errorf("Second session: expected 2 records, got %d", recordCount)
	}
}

// TestIntegration_HighThroughput tests performance with many frames
func TestIntegration_HighThroughput(t *testing.T) {
	// Simulate high-speed data acquisition
	frameCount := 1000
	frames := make([][]byte, frameCount)
	for i := 0; i < frameCount; i++ {
		frames[i] = []byte("DATA\n")
	}

	mock := testutil.NewMockSerial(
		testutil.WithFrames(frames),
		testutil.WithFrameDelay(1*time.Millisecond), // Fast!
	)

	ctx := context.Background()
	if err := mock.Open(ctx); err != nil {
		t.Fatalf("Failed to open mock: %v", err)
	}
	defer mock.Close()

	start := time.Now()
	recordCount := 0
	timeout := time.After(5 * time.Second)

	for recordCount < frameCount {
		select {
		case frame, ok := <-mock.Frames():
			if !ok {
				// Channel closed early
				t.Logf("Channel closed after %d frames in %v", recordCount, time.Since(start))
				if recordCount != frameCount {
					t.Errorf("Expected %d frames, got %d", frameCount, recordCount)
				}
				return
			}
			if frame != nil {
				recordCount++
			}
		case <-timeout:
			t.Fatalf("Timeout: only received %d/%d frames", recordCount, frameCount)
		}
	}

	t.Logf("Successfully received %d frames in %v", recordCount, time.Since(start))
}

// TestIntegration_ConfigurationVariations tests different serial configs
func TestIntegration_ConfigurationVariations(t *testing.T) {
	configs := []sources.Config{
		{Port: "/dev/ttyUSB0", Baud: 9600, Parity: "N", DataBits: 8, StopBits: "1"},
		{Port: "/dev/ttyUSB1", Baud: 115200, Parity: "E", DataBits: 7, StopBits: "2"},
		{Port: "/dev/ttyUSB2", Baud: 230400, Parity: "O", DataBits: 8, StopBits: "1"},
	}

	testData := [][]byte{[]byte("TEST\n")}

	for _, cfg := range configs {
		t.Run(cfg.Port, func(t *testing.T) {
			mock := testutil.NewMockSerial(
				testutil.WithConfig(cfg),
				testutil.WithFrames(testData),
			)

			meta := mock.Meta()
			if meta.Port != cfg.Port {
				t.Errorf("Port mismatch: expected %s, got %s", cfg.Port, meta.Port)
			}
			if meta.Baud != cfg.Baud {
				t.Errorf("Baud mismatch: expected %d, got %d", cfg.Baud, meta.Baud)
			}

			recordCount := simulateCaptureSession(t, mock)
			if recordCount != len(testData) {
				t.Errorf("Expected %d records, got %d", len(testData), recordCount)
			}
		})
	}
}
