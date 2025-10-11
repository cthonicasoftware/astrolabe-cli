package sources

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
	"go.bug.st/serial"
)

type Serial struct {
	Port        string
	Baud        int
	Parity      string // "N", "O", "E", "M", "S"
	DataBits    int    // 5, 6, 7, 8
	StopBits    string // "1", "1.5", "2"
	FlowControl string // "none", "hardware", "software"

	mu     sync.Mutex
	port   serial.Port
	ch     chan []byte
	cancel context.CancelFunc
}

func NewSerial(port string, baud int) *Serial {
	return &Serial{
		Port:        port,
		Baud:        baud,
		Parity:      "N",
		DataBits:    8,
		StopBits:    "1",
		FlowControl: "none",
		ch:          make(chan []byte, 16),
	}
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

func (s *Serial) Frames() <-chan []byte {
	return s.ch
}

func (s *Serial) Meta() core.SourceMeta {
	return core.SourceMeta{Kind: "serial", Port: s.Port, Baud: s.Baud}
}

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
