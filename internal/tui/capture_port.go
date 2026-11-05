package tui

import (
	"context"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
)

// CaptureSessionPort abstracts the live capture subsystem so the router can
// start sessions without importing concrete source implementations.
type CaptureSessionPort interface {
	Start(ctx context.Context, cfg CaptureConfig) (CaptureSession, error)
}

// CaptureSession represents an active capture session from the TUI's perspective.
type CaptureSession interface {
	Feed() <-chan FeedChunk
	// Stop cancels the session, waits for finalization, and returns the outcome.
	Stop() CaptureSessionResult
	// RequestSave marks the session so that Stop promotes artifacts instead of discarding them.
	RequestSave()
	// CanSend reports whether the source accepts input (serial, TCP).
	CanSend() bool
	// Send writes line plus the line ending to the device and records it in
	// the run. It returns the time recorded for the send. It may block (e.g.
	// flow control), so call it off the UI loop.
	Send(line string, le sources.LineEnding) (time.Time, error)
	// SetReconnect turns reconnecting after a dropped link on or off. It
	// does not block.
	SetReconnect(on bool)
}

// FeedChunk is a chunk of received text and the time it arrived. The same
// time is recorded in the run's data, so the live view and data.jsonl agree.
// A chunk with Link set reports a link event instead and carries no data.
type FeedChunk struct {
	Data string
	At   time.Time
	Link *core.LinkEvent // nil for data
}

// CaptureSessionResult holds the outcome after a capture session ends.
type CaptureSessionResult struct {
	Saved bool
	RunID string
	Err   error
}

// LiveViewPrefs are the live-view settings remembered between sessions.
// LineEnding and Echo only apply to sources that accept input.
type LiveViewPrefs struct {
	LineEnding sources.LineEnding
	Echo       bool
	Timestamps bool
	// Reconnect is whether a dropped serial/TCP link is reopened.
	Reconnect bool
}

// DefaultLiveViewPrefs returns LF with echo on, timestamps off and
// reconnect on.
func DefaultLiveViewPrefs() LiveViewPrefs {
	return LiveViewPrefs{LineEnding: sources.DefaultLineEnding, Echo: true, Reconnect: true}
}

// LiveViewPrefsPort loads and saves LiveViewPrefs.
type LiveViewPrefsPort interface {
	Load() (LiveViewPrefs, error)
	Save(LiveViewPrefs) error
}
