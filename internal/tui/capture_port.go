package tui

import "context"

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
}

// CaptureSessionResult holds the outcome after a capture session ends.
type CaptureSessionResult struct {
	Saved bool
	RunID string
	Err   error
}
