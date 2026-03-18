package tui

import "context"

// CaptureSessionPort abstracts the live capture subsystem so the router can
// start and collect sessions without importing concrete source implementations.
type CaptureSessionPort interface {
	Start(ctx context.Context, cfg CaptureConfig) (CaptureSession, error)
	Collect(session CaptureSession) CaptureSessionResult
}

// CaptureSession represents an active capture session from the TUI's perspective.
type CaptureSession interface {
	Feed() <-chan string
	Stop()
}

// CaptureSessionResult holds the outcome after a capture session ends.
type CaptureSessionResult struct {
	Saved bool
	RunID string
	Err   error
}
