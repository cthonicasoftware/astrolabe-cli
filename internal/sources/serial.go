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
	Port string
	Baud int

	mu     sync.Mutex
	port   serial.Port
	ch     chan []byte
	cancel context.CancelFunc
}

func NewSerial(port string, baud int) *Serial {
	return &Serial{
		Port: port,
		Baud: baud,
		ch:   make(chan []byte, 16),
	}
}

func (s *Serial) Open(ctx context.Context) error {
	mode := &serial.Mode{
		BaudRate: s.Baud,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(s.Port, mode)
	if err != nil {
		return fmt.Errorf("failed to open serial port %s: %w", s.Port, err)
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
