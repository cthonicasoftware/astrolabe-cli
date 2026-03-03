package sources

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

// TCPConfig captures the TCP connection parameters.
type TCPConfig struct {
	Host           string        // hostname or IP address
	Port           int           // TCP port
	ConnectTimeout time.Duration // connection timeout
	ReadTimeout    time.Duration // read timeout (0 = no timeout)
	BufferSize     int           // read buffer size in bytes
}

// DefaultTCPConfig returns the standard TCP defaults.
func DefaultTCPConfig() TCPConfig {
	return TCPConfig{
		ConnectTimeout: 10 * time.Second,
		ReadTimeout:    0, // no read timeout by default
		BufferSize:     4096,
	}
}

// TCP represents a TCP socket data source for ingesting streaming data.
type TCP struct {
	TCPConfig

	mu     sync.Mutex
	conn   net.Conn
	ch     chan []byte
	cancel context.CancelFunc
}

// NewTCPWithConfig constructs a TCP source from an explicit configuration.
func NewTCPWithConfig(cfg TCPConfig) (*TCP, error) {
	defaults := DefaultTCPConfig()

	if cfg.Host == "" {
		return nil, fmt.Errorf("TCP host is required")
	}

	if cfg.Port <= 0 || cfg.Port > 65535 {
		return nil, fmt.Errorf("TCP port must be between 1 and 65535, got %d", cfg.Port)
	}

	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = defaults.ConnectTimeout
	}

	if cfg.ReadTimeout < 0 {
		cfg.ReadTimeout = 0
	}

	if cfg.BufferSize == 0 {
		cfg.BufferSize = defaults.BufferSize
	}

	return &TCP{
		TCPConfig: cfg,
		ch:        make(chan []byte, 16),
	}, nil
}

// NewTCP creates a TCP source with just host and port (backward compatibility).
func NewTCP(host string, port int) (*TCP, error) {
	return NewTCPWithConfig(TCPConfig{
		Host: host,
		Port: port,
	})
}

// Open establishes the TCP connection and starts streaming data.
func (t *TCP) Open(ctx context.Context) error {
	// Create connection with timeout
	addr := fmt.Sprintf("%s:%d", t.Host, t.Port)

	dialer := &net.Dialer{
		Timeout: t.ConnectTimeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	t.mu.Lock()
	t.conn = conn
	t.mu.Unlock()

	// Create cancellable context for read loop
	readCtx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	// Start reading loop
	go t.readLoop(readCtx)

	return nil
}

// readLoop streams TCP data through the channel.
func (t *TCP) readLoop(ctx context.Context) {
	defer close(t.ch)

	t.mu.Lock()
	conn := t.conn
	t.mu.Unlock()

	if conn == nil {
		return
	}

	reader := bufio.NewReader(conn)
	buf := make([]byte, t.BufferSize)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Set read deadline if configured
			if t.ReadTimeout > 0 {
				deadline := time.Now().Add(t.ReadTimeout)
				if err := conn.SetReadDeadline(deadline); err != nil {
					return
				}
			}

			n, err := reader.Read(buf)
			if err != nil {
				if err == io.EOF {
					return
				}
				// Check if it's a timeout error
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Timeout is expected if ReadTimeout is set, continue reading
					continue
				}
				// Other error - stop reading
				return
			}

			if n > 0 {
				// Copy data to avoid buffer reuse issues
				frame := make([]byte, n)
				copy(frame, buf[:n])

				select {
				case t.ch <- frame:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// Frames returns the channel for receiving TCP data.
func (t *TCP) Frames() <-chan []byte {
	return t.ch
}

// Meta returns source metadata for the TCP connection.
func (t *TCP) Meta() core.SourceMeta {
	return core.SourceMeta{
		Kind: "tcp",
		Addr: fmt.Sprintf("%s:%d", t.Host, t.Port),
	}
}

// Close closes the TCP connection and stops the read loop.
func (t *TCP) Close() error {
	// Cancel the read loop
	if t.cancel != nil {
		t.cancel()
	}

	// Close the connection
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}

// Address returns the full TCP address (host:port).
func (t *TCP) Address() string {
	return fmt.Sprintf("%s:%d", t.Host, t.Port)
}

