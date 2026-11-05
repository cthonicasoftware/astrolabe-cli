package tui

import (
	"context"

	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
)

// CaptureSessionPort abstracts the live capture subsystem so the router can
// start sessions without importing concrete source implementations.
type CaptureSessionPort interface {
	Start(ctx context.Context, cfg CaptureConfig) (CaptureSession, error)
}

// CaptureSession represents an active capture session from the TUI's perspective.
type CaptureSession interface {
	Feed() <-chan string
	// Stop cancels the session, waits for finalization, and returns the outcome.
	Stop() CaptureSessionResult
	// RequestSave marks the session so that Stop promotes artifacts instead of discarding them.
	RequestSave()
	// CanSend reports whether the source accepts input (serial, TCP).
	CanSend() bool
	// Send writes line plus the line ending to the device and records it in
	// the run. It may block (e.g. flow control), so call it off the UI loop.
	Send(line string, le sources.LineEnding) error
}

// CaptureSessionResult holds the outcome after a capture session ends.
type CaptureSessionResult struct {
	Saved bool
	RunID string
	Err   error
}

// SendPrefs are the live-view send settings remembered between sessions.
type SendPrefs struct {
	LineEnding sources.LineEnding
	Echo       bool
}

// DefaultSendPrefs returns LF with echo on.
func DefaultSendPrefs() SendPrefs {
	return SendPrefs{LineEnding: sources.DefaultLineEnding, Echo: true}
}

// SendPrefsPort loads and saves SendPrefs.
type SendPrefsPort interface {
	Load() (SendPrefs, error)
	Save(SendPrefs) error
}
