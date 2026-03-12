// Package sources provides capture.Source implementations for serial ports,
// TCP connections, and local files.
package sources

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"

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

	mu     sync.Mutex
	port   serial.Port
	ch     chan []byte
	cancel context.CancelFunc
}

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
		Config: cfg,
		ch:     make(chan []byte, 16),
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
				if err == io.EOF {
					return
				}
				// Log error but continue reading
				continue
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

	// Close the port
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.port != nil {
		return s.port.Close()
	}
	return nil
}

//TODO: Create goroutine to refresh available port list on a timer or via os event

