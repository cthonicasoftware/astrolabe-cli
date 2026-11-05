package root

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/cthonicasoftware/astrolabe-cli/internal/capture"
	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/cthonicasoftware/astrolabe-cli/internal/tui"
)

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("unplugged") }

func newTestSession(w interface{ Write([]byte) (int, error) }) *captureSession {
	ctx, cancel := context.WithCancel(context.Background())
	s := &captureSession{ctx: ctx, cancel: cancel, tx: make(chan core.Record, 4)}
	if w != nil {
		s.writer = w
	}
	return s
}

func TestCaptureSessionSendWritesAndRecords(t *testing.T) {
	var buf bytes.Buffer
	s := newTestSession(&buf)
	defer s.cancel()

	if !s.CanSend() {
		t.Fatal("session with writer should be able to send")
	}
	if err := s.Send("AT+RST", sources.LineEndingCRLF); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if buf.String() != "AT+RST\r\n" {
		t.Errorf("device got %q, want %q", buf.String(), "AT+RST\r\n")
	}

	rec := <-s.tx
	if rec.Type != capture.RecordTypeTX {
		t.Errorf("type = %q, want %q", rec.Type, capture.RecordTypeTX)
	}
	if rec.Payload["line"] != "AT+RST" || rec.Payload["line_ending"] != "CRLF" {
		t.Errorf("payload = %v", rec.Payload)
	}
	if rec.TS.IsZero() {
		t.Error("TX record has no timestamp")
	}
}

func TestCaptureSessionSendFailureNotRecorded(t *testing.T) {
	s := newTestSession(failWriter{})
	defer s.cancel()

	if err := s.Send("x", sources.LineEndingLF); err == nil {
		t.Fatal("expected write error")
	}
	if len(s.tx) != 0 {
		t.Error("failed write was recorded as sent")
	}
}

func TestCaptureSessionEmptyNoneSendsNothing(t *testing.T) {
	var buf bytes.Buffer
	s := newTestSession(&buf)
	defer s.cancel()

	if err := s.Send("", sources.LineEndingNone); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if buf.Len() != 0 || len(s.tx) != 0 {
		t.Error("empty line with no ending should send and record nothing")
	}
}

func TestCaptureSessionReadOnlyCannotSend(t *testing.T) {
	s := newTestSession(nil)
	defer s.cancel()
	if s.CanSend() {
		t.Fatal("session without writer reports CanSend")
	}
	if err := s.Send("x", sources.LineEndingLF); err == nil {
		t.Fatal("Send on read-only session should fail")
	}
}

func TestCaptureSessionStopClosesSource(t *testing.T) {
	s := newTestSession(nil)
	closed := 0
	s.closeSource = func() { closed++ }
	s.tempRoot = t.TempDir()
	s.done = make(chan tui.CaptureSessionResult, 1)
	s.done <- tui.CaptureSessionResult{RunID: "r1"}

	s.Stop()
	s.Stop()

	if closed != 1 {
		t.Fatalf("source closed %d times, want 1", closed)
	}
}
