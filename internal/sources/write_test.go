package sources

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"go.bug.st/serial"
)

// fakePort is an in-memory serial.Port. Reads block until data is queued or
// the port fails; writes are captured.
type fakePort struct {
	mu      sync.Mutex
	written []byte
	reads   chan []byte
	readErr chan error
	reads0  int // number of Read calls, to detect spinning

	stall      chan struct{} // when non-nil, Write blocks until it is closed
	writeCalls []time.Time   // time of each successful Write call
	purges     int
	closes     int
	closeCh    chan struct{}
}

func newFakePort() *fakePort {
	return &fakePort{reads: make(chan []byte, 4), readErr: make(chan error, 1), closeCh: make(chan struct{})}
}

func (f *fakePort) Read(p []byte) (int, error) {
	f.mu.Lock()
	f.reads0++
	f.mu.Unlock()
	select {
	case b := <-f.reads:
		return copy(p, b), nil
	case err := <-f.readErr:
		return 0, err
	case <-f.closeCh:
		return 0, errors.New("port closed")
	}
}

func (f *fakePort) Write(p []byte) (int, error) {
	f.mu.Lock()
	stall := f.stall
	f.mu.Unlock()
	if stall != nil {
		<-stall
		return 0, errors.New("write aborted")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.written = append(f.written, p...)
	f.writeCalls = append(f.writeCalls, time.Now())
	return len(p), nil
}

// ResetOutputBuffer aborts a stalled write, like PurgeComm(PURGE_TXABORT).
func (f *fakePort) ResetOutputBuffer() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.purges++
	if f.stall != nil {
		close(f.stall)
		f.stall = nil
	}
	return nil
}

func (f *fakePort) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closes++
	if f.closes == 1 {
		close(f.closeCh)
	}
	return nil
}

func (f *fakePort) readCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads0
}

func (f *fakePort) SetMode(*serial.Mode) error                           { return nil }
func (f *fakePort) Drain() error                                         { return nil }
func (f *fakePort) ResetInputBuffer() error                              { return nil }
func (f *fakePort) SetDTR(bool) error                                    { return nil }
func (f *fakePort) SetRTS(bool) error                                    { return nil }
func (f *fakePort) GetModemStatusBits() (*serial.ModemStatusBits, error) { return nil, nil }
func (f *fakePort) SetReadTimeout(time.Duration) error                   { return nil }
func (f *fakePort) Break(time.Duration) error                            { return nil }

func TestSerial_WriteNotOpen(t *testing.T) {
	s := NewSerial("COM_TEST", 115200)
	if _, err := s.Write([]byte("hi")); err == nil {
		t.Fatal("Write on unopened port should fail")
	}
}

func TestSerial_WriteSendsBytes(t *testing.T) {
	s := NewSerial("COM_TEST", 115200)
	fp := newFakePort()
	s.port = fp

	n, err := s.Write([]byte("AT\r\n"))
	if err != nil || n != 4 {
		t.Fatalf("Write = (%d, %v), want (4, nil)", n, err)
	}
	if got := string(fp.written); got != "AT\r\n" {
		t.Errorf("written = %q, want %q", got, "AT\r\n")
	}
}

// Regression: a Pico SDK shell kept only the first byte of "ls\n" when the
// line arrived in one USB packet. Serial sends must go out byte by byte.
func TestSerial_WritePacesBytes(t *testing.T) {
	s := NewSerial("COM_TEST", 115200)
	fp := newFakePort()
	s.port = fp

	if _, err := s.Write([]byte("ls\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if len(fp.writeCalls) != 3 {
		t.Fatalf("got %d port writes, want 3 (one per byte)", len(fp.writeCalls))
	}
	for i := 1; i < len(fp.writeCalls); i++ {
		if gap := fp.writeCalls[i].Sub(fp.writeCalls[i-1]); gap < DefaultSerialCharDelay {
			t.Errorf("gap before byte %d = %s, want >= %s", i, gap, DefaultSerialCharDelay)
		}
	}
	if string(fp.written) != "ls\n" {
		t.Errorf("written = %q", fp.written)
	}
}

func TestSerial_WriteTimesOutAndAbortsStalledWrite(t *testing.T) {
	s := NewSerial("COM_TEST", 115200)
	s.writeTimeout = 50 * time.Millisecond
	fp := newFakePort()
	fp.stall = make(chan struct{})
	s.port = fp

	start := time.Now()
	_, err := s.Write([]byte("ls\r\n"))
	if err == nil {
		t.Fatal("stalled write should return an error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Write took %s; should give up near the timeout", elapsed)
	}
	if fp.purges == 0 {
		t.Error("timed-out write was not aborted with ResetOutputBuffer")
	}

	// The port is usable again once the stalled write is aborted.
	if _, err := s.Write([]byte("ok")); err != nil {
		t.Fatalf("write after abort: %v", err)
	}
}

func TestSerial_CloseIsIdempotentAndUnblocksReadLoop(t *testing.T) {
	s := NewSerial("COM_TEST", 115200)
	fp := newFakePort()
	s.port = fp
	go s.readLoop(context.Background())

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if fp.closes != 1 {
		t.Errorf("port closed %d times, want 1", fp.closes)
	}

	select {
	case _, ok := <-s.ch:
		if ok {
			t.Fatal("expected frames channel to close")
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not unblock the read loop")
	}

	if _, err := s.Write([]byte("x")); err == nil {
		t.Error("Write after Close should fail")
	}
}

func TestSerial_ReadLoopStopsOnPortError(t *testing.T) {
	s := NewSerial("COM_TEST", 115200)
	fp := newFakePort()
	s.port = fp

	go s.readLoop(context.Background())

	fp.reads <- []byte("hello\n")
	if got := string(<-s.ch); got != "hello\n" {
		t.Fatalf("frame = %q, want %q", got, "hello\n")
	}

	// Simulate the device being unplugged.
	fp.readErr <- errors.New("device disconnected")

	select {
	case _, ok := <-s.ch:
		if ok {
			t.Fatal("expected channel to close after port error")
		}
	case <-time.After(time.Second):
		t.Fatal("readLoop did not stop after port error")
	}
	if calls := fp.readCalls(); calls > 2 {
		t.Errorf("readLoop kept reading after error: %d Read calls", calls)
	}
	if err := s.Err(); err == nil || err.Error() != "device disconnected" {
		t.Errorf("Err() = %v, want %q", err, "device disconnected")
	}
}

func TestSerial_ErrNilAfterClose(t *testing.T) {
	s := NewSerial("COM_TEST", 115200)
	fp := newFakePort()
	s.port = fp
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go s.readLoop(ctx)

	_ = s.Close() // closing the port makes the pending Read fail
	select {
	case _, ok := <-s.ch:
		if ok {
			t.Fatal("expected channel to close")
		}
	case <-time.After(time.Second):
		t.Fatal("readLoop did not stop after Close")
	}
	if err := s.Err(); err != nil {
		t.Errorf("Err() after Close = %v, want nil", err)
	}
}

func TestTCP_ErrIsEOFWhenServerCloses(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
	}()
	src, err := NewTCP("127.0.0.1", listener.Addr().(*net.TCPAddr).Port)
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	select {
	case <-drain(src.Frames()):
	case <-time.After(time.Second):
		t.Fatal("frames channel did not close")
	}
	if err := src.Err(); !errors.Is(err, io.EOF) {
		t.Errorf("Err() = %v, want io.EOF", err)
	}
}

func TestTCP_ErrNilAfterClose(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	hold := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			hold <- conn
		}
	}()
	src, err := NewTCP("127.0.0.1", listener.Addr().(*net.TCPAddr).Port)
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = src.Close()
	select {
	case <-drain(src.Frames()):
	case <-time.After(time.Second):
		t.Fatal("frames channel did not close")
	}
	if err := src.Err(); err != nil {
		t.Errorf("Err() after Close = %v, want nil", err)
	}
	select {
	case c := <-hold:
		c.Close()
	default:
	}
}

// drain returns a channel closed once ch has been drained and closed.
func drain(ch <-chan []byte) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()
	return done
}

func TestTCP_WriteReachesServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	got := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		line, _ := bufio.NewReader(conn).ReadString('\n')
		got <- line
		_, _ = io.Copy(io.Discard, conn)
	}()

	src, err := NewTCP("127.0.0.1", listener.Addr().(*net.TCPAddr).Port)
	if err != nil {
		t.Fatalf("NewTCP: %v", err)
	}
	if _, err := src.Write([]byte("x")); err == nil {
		t.Fatal("Write before Open should fail")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := src.Open(ctx); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer src.Close()

	if _, err := src.Write([]byte("PING\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	select {
	case line := <-got:
		if line != "PING\n" {
			t.Errorf("server got %q, want %q", line, "PING\n")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server never received data")
	}
}

func TestIsWritable(t *testing.T) {
	if !IsWritable(NewSerial("COM_TEST", 9600)) {
		t.Error("Serial should be writable")
	}
	if IsWritable(struct{}{}) {
		t.Error("plain struct should not be writable")
	}
}
