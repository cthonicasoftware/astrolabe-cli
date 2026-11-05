// Package sources provides capture.Source implementations for serial ports,
// TCP connections, and local files.
package sources

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"go.bug.st/serial"
)

// Config captures the serial connection parameters that can be shared across UIs and sources.
type Config struct {
	Port        string
	Baud        int
	Parity      string // "N", "O", "E", "M", "S"
	DataBits    int    // 5, 6, 7, 8
	StopBits    string // "1", "1.5", "2"
	FlowControl string // "none", "hardware", "software"
}

// SerialOption exposes a code/label pair for presenting serial settings.
type SerialOption struct {
	Code  string
	Label string
}

// DefaultConfig returns the standard serial defaults used throughout the application.
func DefaultConfig() Config {
	return Config{
		Baud:        115200,
		Parity:      "N",
		DataBits:    8,
		StopBits:    "1",
		FlowControl: "none",
	}
}

// CommonBaudRates lists baud rates we routinely offer to users.
var CommonBaudRates = []int{9600, 19200, 38400, 57600, 115200, 230400, 460800, 921600}

// ParityOptions enumerates supported parity settings and their display labels.
var ParityOptions = []SerialOption{
	{Code: "N", Label: "None"},
	{Code: "O", Label: "Odd"},
	{Code: "E", Label: "Even"},
	{Code: "M", Label: "Mark"},
	{Code: "S", Label: "Space"},
}

// DataBitsOptions enumerates supported data bit widths.
var DataBitsOptions = []int{5, 6, 7, 8}

// StopBitsOptions enumerates supported stop bit configurations.
var StopBitsOptions = []SerialOption{
	{Code: "1", Label: "1"},
	{Code: "1.5", Label: "1.5"},
	{Code: "2", Label: "2"},
}

// FlowControlOptions enumerates supported flow control strategies.
var FlowControlOptions = []SerialOption{
	{Code: "none", Label: "None"},
	{Code: "hardware", Label: "Hardware (RTS/CTS)"},
	{Code: "software", Label: "Software (XON/XOFF)"},
}

// Serial is a capture source that reads newline-delimited frames from a
// hardware serial port. It opens the port in a background goroutine and
// streams frames over the channel returned by Frames.
type Serial struct {
	Config

	mu      sync.Mutex
	writeMu sync.Mutex
	port    serial.Port
	ch      chan []byte
	cancel  context.CancelFunc

	writeTimeout time.Duration // 0 uses DefaultSerialWriteTimeout
	charDelay    time.Duration // gap between sent bytes; 0 sends all at once
}

// DefaultSerialCharDelay is the gap between bytes sent to a serial device.
// 5ms keeps simple firmware shells from dropping characters while a typical
// command still goes out in well under a second.
const DefaultSerialCharDelay = 5 * time.Millisecond

// DefaultSerialWriteTimeout bounds how long a send may wait for the device
// to accept bytes before it is reported as failed.
const DefaultSerialWriteTimeout = 3 * time.Second

// writeAbortGrace is how long Write waits for an aborted write to unwind.
const writeAbortGrace = 500 * time.Millisecond

// NewSerialWithConfig constructs a Serial source from an explicit configuration.
func NewSerialWithConfig(cfg Config) *Serial {
	defaults := DefaultConfig()

	if cfg.Baud == 0 {
		cfg.Baud = defaults.Baud
	}
	if cfg.Parity == "" {
		cfg.Parity = defaults.Parity
	}
	if cfg.DataBits == 0 {
		cfg.DataBits = defaults.DataBits
	}
	if cfg.StopBits == "" {
		cfg.StopBits = defaults.StopBits
	}
	if cfg.FlowControl == "" {
		cfg.FlowControl = defaults.FlowControl
	}

	return &Serial{
		Config:    cfg,
		ch:        make(chan []byte, 16),
		charDelay: DefaultSerialCharDelay,
	}
}

// NewSerial keeps backward compatibility for callers that only supply port and baud.
func NewSerial(port string, baud int) *Serial {
	cfg := DefaultConfig()
	cfg.Port = port
	cfg.Baud = baud
	return NewSerialWithConfig(cfg)
}

func (s *Serial) parseParity() serial.Parity {
	switch s.Parity {
	case "O":
		return serial.OddParity
	case "E":
		return serial.EvenParity
	case "M":
		return serial.MarkParity
	case "S":
		return serial.SpaceParity
	default:
		return serial.NoParity
	}
}

func (s *Serial) parseStopBits() serial.StopBits {
	switch s.StopBits {
	case "1.5":
		return serial.OnePointFiveStopBits
	case "2":
		return serial.TwoStopBits
	default:
		return serial.OneStopBit
	}
}

// Open configures and opens the serial port, then starts the background read loop.
// The context controls the lifetime of the read loop; cancelling it stops reading.
func (s *Serial) Open(ctx context.Context) error {
	mode := &serial.Mode{
		BaudRate: s.Baud,
		DataBits: s.DataBits,
		Parity:   s.parseParity(),
		StopBits: s.parseStopBits(),
	}

	port, err := serial.Open(s.Port, mode)
	if err != nil {
		return fmt.Errorf("failed to open serial port %s: %w", s.Port, err)
	}

	// Set flow control if hardware flow control is requested
	if s.FlowControl == "hardware" {
		err = port.SetRTS(true)
		if err != nil {
			port.Close()
			return fmt.Errorf("failed to set RTS: %w", err)
		}
		err = port.SetDTR(true)
		if err != nil {
			port.Close()
			return fmt.Errorf("failed to set DTR: %w", err)
		}
	}

	s.mu.Lock()
	s.port = port
	s.mu.Unlock()

	// Create cancellable context for read loop
	readCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	// Start reading loop
	go s.readLoop(readCtx)

	return nil
}

func (s *Serial) readLoop(ctx context.Context) {
	defer close(s.ch)

	s.mu.Lock()
	port := s.port
	s.mu.Unlock()

	if port == nil {
		return
	}

	reader := bufio.NewReader(port)
	buf := make([]byte, 4096)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := reader.Read(buf)
			if err != nil {
				// go.bug.st/serial reports timeouts as (0, nil), so any error
				// means the port is gone (unplugged) or closed. Stop instead of
				// spinning; closing the channel tells consumers the device left.
				return
			}

			if n > 0 {
				// Copy data to avoid buffer reuse issues
				frame := make([]byte, n)
				copy(frame, buf[:n])

				select {
				case s.ch <- frame:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// Write sends p to the open serial port. It is safe to call concurrently
// with the read loop.
func (s *Serial) Write(p []byte) (int, error) {
	// One write at a time: a new send must not interleave with, or queue
	// silently behind, a write the device has not accepted yet.
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	s.mu.Lock()
	port := s.port
	s.mu.Unlock()

	if port == nil {
		return 0, fmt.Errorf("serial port %s is not open", s.Port)
	}

	delay := s.charDelay
	if delay <= 0 {
		return s.writeChunk(port, p)
	}

	// Pace bytes like a typing operator. Simple device shells often read one
	// byte per loop and drop the rest of a USB packet, so a whole line sent
	// at once arrives as its first character only.
	for i := range p {
		if i > 0 {
			time.Sleep(delay)
		}
		if _, err := s.writeChunk(port, p[i:i+1]); err != nil {
			return i, err
		}
	}
	return len(p), nil
}

// writeChunk writes p with a timeout. go.bug.st/serial sets no write timeout,
// so a device that stops reading would otherwise block forever.
func (s *Serial) writeChunk(port serial.Port, p []byte) (int, error) {
	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	go func() {
		n, err := port.Write(p)
		done <- result{n, err}
	}()

	timeout := s.writeTimeout
	if timeout <= 0 {
		timeout = DefaultSerialWriteTimeout
	}

	select {
	case r := <-done:
		if r.err != nil {
			return r.n, fmt.Errorf("write to serial port %s: %w", s.Port, r.err)
		}
		return r.n, nil
	case <-time.After(timeout):
		// Purge aborts the pending write on Windows; give it a moment, then
		// report failure either way.
		_ = port.ResetOutputBuffer()
		select {
		case r := <-done:
			return r.n, fmt.Errorf("serial port %s: device did not accept data within %s", s.Port, timeout)
		case <-time.After(writeAbortGrace):
			return 0, fmt.Errorf("serial port %s: device did not accept data within %s", s.Port, timeout)
		}
	}
}

// Frames returns the channel on which raw byte frames are delivered.
// The channel is closed when the read loop exits.
func (s *Serial) Frames() <-chan []byte {
	return s.ch
}

// Meta returns the source metadata (kind, port, baud) for this serial source.
func (s *Serial) Meta() core.SourceMeta {
	return core.SourceMeta{Kind: "serial", Port: s.Port, Baud: s.Baud}
}

// Close cancels the read loop and closes the underlying serial port.
func (s *Serial) Close() error {
	// Cancel the read loop
	if s.cancel != nil {
		s.cancel()
	}

	s.mu.Lock()
	port := s.port
	s.port = nil // makes Close idempotent and fails later Writes fast
	s.mu.Unlock()

	if port == nil {
		return nil
	}
	// Abort any write the device never accepted; a pending write can keep
	// the handle (and the COM port) held even after Close.
	_ = port.ResetOutputBuffer()
	// Closing the port also unblocks a Read pending in readLoop.
	return port.Close()
}

//TODO: Create goroutine to refresh available port list on a timer or via os event

// Kind implements Spec; serial runs record source kind "serial".
func (c Config) Kind() string { return "serial" }

// NewSource implements Spec, constructing the serial source. Serial
// configuration is normalized rather than rejected, so this never errors.
func (c Config) NewSource() (ManagedSource, error) { return NewSerialWithConfig(c), nil }

// ManifestAttrs implements Spec, describing the serial port and line rate.
func (c Config) ManifestAttrs() map[string]string {
	return map[string]string{
		"source_kind": "serial",
		"port":        c.Port,
		"baud":        strconv.Itoa(c.Baud),
	}
}

// CaptureSettings implements Spec.
func (c Config) CaptureSettings() core.CaptureSettings {
	return core.CaptureSettings{
		Channels: []string{"serial"},
		Notes:    fmt.Sprintf("serial capture from %s @ %d baud", c.Port, c.Baud),
	}
}
