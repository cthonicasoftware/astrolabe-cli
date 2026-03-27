package tui

import "context"

// UploadPort abstracts run uploads for the TUI.
type UploadPort interface {
	UploadRun(ctx context.Context, runID string) error
}
