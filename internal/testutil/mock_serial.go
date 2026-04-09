package testutil

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
)

// MockSerial simulates a serial device for testing.
// It provides configurable behavior including data streams, delays, and error injection.
type MockSerial struct {
	Config sources.Config

	// Test configuration
	frames      [][]byte      // Pre-configured frames to emit
	frameCursor int           // Current position in frames
	frameDelay  time.Duration // Delay between frames
	readError   error         // Error to return after configured frames

	// State
	mu     sync.Mutex
	ch     chan []byte
	cancel context.CancelFunc
	opened bool
	closed bool
}

// MockSerialOption configures a MockSerial device.
type MockSerialOption func(*MockSerial)

// WithFrames sets the sequence of byte frames to emit.
func WithFrames(frames [][]byte) MockSerialOption {
	return func(m *MockSerial) {
		m.frames = frames
	}
}

// WithFrameDelay sets the delay between frame emissions.
func WithFrameDelay(delay time.Duration) MockSerialOption {
	return func(m *MockSerial) {
		m.frameDelay = delay
	}
}

// WithReadError configures an error to return after all frames are exhausted.
func WithReadError(err error) MockSerialOption {
	return func(m *MockSerial) {
		m.readError = err
	}
}

// WithConfig sets the serial configuration.
func WithConfig(cfg sources.Config) MockSerialOption {
	return func(m *MockSerial) {
		m.Config = cfg
	}
}

// NewMockSerial creates a new mock serial device with the given options.
func NewMockSerial(opts ...MockSerialOption) *MockSerial {
	m := &MockSerial{
		Config:     sources.DefaultConfig(),
		frames:     [][]byte{},
		frameDelay: 10 * time.Millisecond, // Reasonable default
		ch:         make(chan []byte, 16),
	}

	// Set default port name for testing
	m.Config.Port = "/dev/ttyUSB_MOCK"

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Open simulates opening the serial port and starts the read loop.
func (m *MockSerial) Open(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.opened {
		return fmt.Errorf("mock serial already opened")
	}
	if m.closed {
		return fmt.Errorf("mock serial already closed")
	}

	m.opened = true

	// Create cancellable context for read loop
	readCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	// Start reading loop
	go m.readLoop(readCtx)

	return nil
}

// readLoop simulates reading data from the serial port.
func (m *MockSerial) readLoop(ctx context.Context) {
	defer close(m.ch)

	ticker := time.NewTicker(m.frameDelay)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			m.mu.Lock()
			if m.frameCursor >= len(m.frames) {
				// All frames exhausted
				if m.readError != nil {
					// Could emit error on channel if we extend the interface
					m.mu.Unlock()
					return
				}
				// No more frames and no error - keep running but emit nothing
				m.mu.Unlock()
				continue
			}

			frame := m.frames[m.frameCursor]
			m.frameCursor++
			m.mu.Unlock()

			// Emit the frame
			select {
			case m.ch <- frame:
			case <-ctx.Done():
				return
			}
		}
	}
}

// Frames returns the channel for reading frames from the mock serial port.
func (m *MockSerial) Frames() <-chan []byte {
	return m.ch
}

// Meta returns the source metadata for the mock device.
func (m *MockSerial) Meta() core.SourceMeta {
	return core.SourceMeta{
		Kind: "serial",
		Port: m.Config.Port,
		Baud: m.Config.Baud,
	}
}

// Close simulates closing the serial port.
func (m *MockSerial) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("mock serial already closed")
	}

	m.closed = true

	// Cancel the read loop
	if m.cancel != nil {
		m.cancel()
	}

	return nil
}

// Reset allows the mock to be reused for another test.
func (m *MockSerial) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.frameCursor = 0
	m.opened = false
	m.closed = false
	m.ch = make(chan []byte, 16)
}

// AddFrame appends a frame to the mock's frame list (useful for dynamic testing).
func (m *MockSerial) AddFrame(frame []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.frames = append(m.frames, frame)
}

// FramesRemaining returns the number of frames yet to be emitted.
func (m *MockSerial) FramesRemaining() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.frames) - m.frameCursor
}
