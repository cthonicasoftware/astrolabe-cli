package root

import (
	"context"
	"testing"
	"time"
)

func TestUploadRunWithTimeout_AssignsFreshDeadlinePerRun(t *testing.T) {
	parent := context.Background()
	var deadlines []time.Time

	recordDeadline := func(ctx context.Context, runID string) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatalf("expected deadline for run %s", runID)
		}
		deadlines = append(deadlines, deadline)
		return nil
	}

	if err := uploadRunWithTimeout(parent, 50*time.Millisecond, "run-1", recordDeadline); err != nil {
		t.Fatalf("first uploadRunWithTimeout: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := uploadRunWithTimeout(parent, 50*time.Millisecond, "run-2", recordDeadline); err != nil {
		t.Fatalf("second uploadRunWithTimeout: %v", err)
	}

	if len(deadlines) != 2 {
		t.Fatalf("expected 2 deadlines, got %d", len(deadlines))
	}
	if !deadlines[1].After(deadlines[0]) {
		t.Fatalf("expected second run deadline %v to be after first %v", deadlines[1], deadlines[0])
	}
}
