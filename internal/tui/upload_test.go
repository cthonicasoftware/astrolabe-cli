package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cthonicasoftware/astrolabe-cli/internal/upload"
)

func TestUploadModel_Init(t *testing.T) {
	mock := upload.NewMockUploadClient()
	runIDs := []string{"run-1", "run-2", "run-3"}

	model := newUploadModel(mock, runIDs)

	// Verify initial state
	if model.index != 0 {
		t.Errorf("expected index=0, got %d", model.index)
	}
	if model.done {
		t.Error("expected done=false initially")
	}
	if model.failed != 0 {
		t.Errorf("expected failed=0, got %d", model.failed)
	}
	if len(model.runIDs) != 3 {
		t.Errorf("expected 3 runIDs, got %d", len(model.runIDs))
	}
}

// TestUploadModel_StandaloneExits guards the freeze bug: when uploadModel runs
// as the root program (astrolabe upload, no router), ctrl+c and the NavigateMsg
// it emits on completion must both quit, or the summary screen is unexitable.
func TestUploadModel_StandaloneExits(t *testing.T) {
	mock := upload.NewMockUploadClient()
	model := newUploadModel(mock, []string{"run-1"})

	isQuit := func(cmd tea.Cmd) bool {
		if cmd == nil {
			return false
		}
		_, ok := cmd().(tea.QuitMsg)
		return ok
	}

	// ctrl+c must quit immediately.
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !isQuit(cmd) {
		t.Error("ctrl+c did not return tea.Quit")
	}

	// A NavigateMsg (the completion signal, unhandled standalone) must quit.
	_, cmd = model.Update(NavigateMsg{To: ScreenWelcome})
	if !isQuit(cmd) {
		t.Error("NavigateMsg did not return tea.Quit when standalone")
	}
}

// TestUploadModel_SurfacesRealError guards against the misleading
// "check your connection" message: a server-side failure must show the actual
// error in the summary, not a connectivity guess.
func TestUploadModel_SurfacesRealError(t *testing.T) {
	const realErr = "confirm upload for data.jsonl: status=422 body=artifact_not_uploaded"
	mock := upload.NewMockUploadClient()
	mock.SetError("run-1", realErr)

	model := newUploadModel(mock, []string{"run-1"})
	updated, _ := model.Update(uploadedRunMsg{runID: "run-1", err: errors.New(realErr)})
	model = updated.(uploadModel)

	status := model.buildStatus()
	if !strings.Contains(status.Body, realErr) {
		t.Errorf("summary omits real error.\nbody:\n%s", status.Body)
	}
	if strings.Contains(status.Body, "connection") {
		t.Errorf("summary still shows misleading connection text.\nbody:\n%s", status.Body)
	}
}

func TestUploadModel_SingleRun_Success(t *testing.T) {
	mock := upload.NewMockUploadClient()
	mock.SetSuccess("run-1")

	runIDs := []string{"run-1"}
	model := newUploadModel(mock, runIDs)

	// Simulate receiving upload success message
	msg := uploadedRunMsg{
		runID: "run-1",
		err:   nil,
	}

	updatedModel, cmd := model.Update(msg)
	model = updatedModel.(uploadModel)

	if !model.done {
		t.Error("expected done=true after single upload")
	}
	if model.failed != 0 {
		t.Errorf("expected failed=0, got %d", model.failed)
	}
	if cmd == nil {
		t.Error("expected command to be returned")
	}
}

func TestUploadModel_SingleRun_Failure(t *testing.T) {
	mock := upload.NewMockUploadClient()
	mock.SetError("run-1", "upload failed")

	runIDs := []string{"run-1"}
	model := newUploadModel(mock, runIDs)

	// Simulate receiving upload failure message
	msg := uploadedRunMsg{
		runID: "run-1",
		err:   fmt.Errorf("upload failed"),
	}

	updatedModel, _ := model.Update(msg)
	model = updatedModel.(uploadModel)

	if !model.done {
		t.Error("expected done=true after single upload")
	}
	if model.failed != 1 {
		t.Errorf("expected failed=1, got %d", model.failed)
	}
}

func TestUploadModel_MultipleRuns_AllSuccess(t *testing.T) {
	mock := upload.NewMockUploadClient()
	mock.SetSuccess("run-1")
	mock.SetSuccess("run-2")
	mock.SetSuccess("run-3")

	runIDs := []string{"run-1", "run-2", "run-3"}
	model := newUploadModel(mock, runIDs)

	// Upload run-1
	msg1 := uploadedRunMsg{runID: "run-1", err: nil}
	updatedModel, _ := model.Update(msg1)
	model = updatedModel.(uploadModel)

	if model.done {
		t.Error("expected done=false after first upload")
	}
	if model.index != 1 {
		t.Errorf("expected index=1, got %d", model.index)
	}

	// Upload run-2
	msg2 := uploadedRunMsg{runID: "run-2", err: nil}
	updatedModel, _ = model.Update(msg2)
	model = updatedModel.(uploadModel)

	if model.done {
		t.Error("expected done=false after second upload")
	}
	if model.index != 2 {
		t.Errorf("expected index=2, got %d", model.index)
	}

	// Upload run-3 (last one)
	msg3 := uploadedRunMsg{runID: "run-3", err: nil}
	updatedModel, _ = model.Update(msg3)
	model = updatedModel.(uploadModel)

	if !model.done {
		t.Error("expected done=true after all uploads")
	}
	if model.failed != 0 {
		t.Errorf("expected failed=0, got %d", model.failed)
	}
}

func TestUploadModel_MultipleRuns_PartialFailure(t *testing.T) {
	mock := upload.NewMockUploadClient()
	mock.SetSuccess("run-1")
	mock.SetError("run-2", "network error")
	mock.SetSuccess("run-3")

	runIDs := []string{"run-1", "run-2", "run-3"}
	model := newUploadModel(mock, runIDs)

	// Upload run-1 (success)
	msg1 := uploadedRunMsg{runID: "run-1", err: nil}
	updatedModel, _ := model.Update(msg1)
	model = updatedModel.(uploadModel)

	if model.failed != 0 {
		t.Errorf("expected failed=0 after first upload, got %d", model.failed)
	}

	// Upload run-2 (failure)
	msg2 := uploadedRunMsg{runID: "run-2", err: fmt.Errorf("network error")}
	updatedModel, _ = model.Update(msg2)
	model = updatedModel.(uploadModel)

	if model.failed != 1 {
		t.Errorf("expected failed=1 after second upload, got %d", model.failed)
	}

	// Upload run-3 (success)
	msg3 := uploadedRunMsg{runID: "run-3", err: nil}
	updatedModel, _ = model.Update(msg3)
	model = updatedModel.(uploadModel)

	if !model.done {
		t.Error("expected done=true after all uploads")
	}
	if model.failed != 1 {
		t.Errorf("expected failed=1, got %d", model.failed)
	}
}

func TestUploadModel_View(t *testing.T) {
	mock := upload.NewMockUploadClient()
	runIDs := []string{"run-1", "run-2", "run-3"}

	model := newUploadModel(mock, runIDs)
	model.width = 80 // Set a reasonable terminal width

	// Test view while uploading
	view := model.View()
	if view == "" {
		t.Error("expected non-empty view")
	}

	// Test view when done
	model.done = true
	model.failed = 1
	view = model.View()
	if view == "" {
		t.Error("expected non-empty view when done")
	}
	// Should contain success/failure counts
	// This is a basic check - actual rendering details may vary
}

func TestUploadModel_WindowSizeMsg(t *testing.T) {
	mock := upload.NewMockUploadClient()
	runIDs := []string{"run-1"}

	model := newUploadModel(mock, runIDs)

	// Send window size message
	msg := tea.WindowSizeMsg{Width: 100, Height: 40}
	updatedModel, _ := model.Update(msg)
	model = updatedModel.(uploadModel)

	if model.width != 100 {
		t.Errorf("expected width=100, got %d", model.width)
	}
	if model.height != 40 {
		t.Errorf("expected height=40, got %d", model.height)
	}
}

func TestUploadModel_QuitKeys(t *testing.T) {
	mock := upload.NewMockUploadClient()
	runIDs := []string{"run-1"}

	model := newUploadModel(mock, runIDs)

	// Test quit key handling
	// Note: Full keyboard testing in bubbletea requires more sophisticated setup
	// This is a simplified test to verify the model handles KeyMsg
	msg := tea.KeyMsg{Type: tea.KeyRunes}
	_, cmd := model.Update(msg)

	// The model should handle key messages without panicking
	_ = cmd
}

func TestUploadModel_ProgressTracking(t *testing.T) {
	mock := upload.NewMockUploadClient()
	runIDs := []string{"run-1", "run-2", "run-3", "run-4", "run-5"}

	model := newUploadModel(mock, runIDs)

	// Upload each run and verify progress
	for i, runID := range runIDs {
		isLast := i == len(runIDs)-1
		msg := uploadedRunMsg{runID: runID, err: nil}
		updatedModel, _ := model.Update(msg)
		model = updatedModel.(uploadModel)

		if isLast {
			if !model.done {
				t.Error("expected done=true after last upload")
			}
		} else {
			if model.done {
				t.Errorf("expected done=false at upload %d/%d", i+1, len(runIDs))
			}
			expectedIndex := i + 1
			if model.index != expectedIndex {
				t.Errorf("expected index=%d after upload %d, got %d", expectedIndex, i+1, model.index)
			}
		}
	}
}

func TestMockUploadClient_Integration(t *testing.T) {
	// Test that the mock client works correctly
	mock := upload.NewMockUploadClient()

	// Set up different results
	mock.SetSuccess("success-run")
	mock.SetError("error-run", "simulated error")

	// Test success case
	err := mock.UploadRun(context.Background(), "success-run")
	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}

	// Test error case
	err = mock.UploadRun(context.Background(), "error-run")
	if err == nil {
		t.Error("expected error, got success")
	}

	// Test default success
	err = mock.UploadRun(context.Background(), "unknown-run")
	if err != nil {
		t.Errorf("expected default success, got error: %v", err)
	}

	// Verify call tracking
	if mock.CallCount() != 3 {
		t.Errorf("expected 3 calls, got %d", mock.CallCount())
	}

	if !mock.CalledWith("success-run") {
		t.Error("expected CalledWith('success-run') to be true")
	}

	if !mock.CalledWith("error-run") {
		t.Error("expected CalledWith('error-run') to be true")
	}
}

func TestMockUploadClient_CustomFunction(t *testing.T) {
	mock := upload.NewMockUploadClient()

	callOrder := []string{}
	mock.UploadFunc = func(ctx context.Context, runID string) error {
		callOrder = append(callOrder, runID)
		if runID == "fail-me" {
			return fmt.Errorf("intentional failure")
		}
		return nil
	}

	// Upload several runs
	mock.UploadRun(context.Background(), "run-1")
	mock.UploadRun(context.Background(), "fail-me")
	mock.UploadRun(context.Background(), "run-2")

	// Verify call order
	if len(callOrder) != 3 {
		t.Errorf("expected 3 calls, got %d", len(callOrder))
	}
	if callOrder[0] != "run-1" || callOrder[1] != "fail-me" || callOrder[2] != "run-2" {
		t.Errorf("unexpected call order: %v", callOrder)
	}

	// Verify failure was recorded
	if mock.Calls[1].Error == nil {
		t.Error("expected error for 'fail-me' call")
	}
}

func TestUploadModel_Cancellation(t *testing.T) {
	mock := upload.NewMockUploadClient()
	mock.SetSuccess("run-1")
	mock.SetError("run-2", "network error")
	mock.SetSuccess("run-3")
	mock.SetSuccess("run-4")
	mock.SetSuccess("run-5")

	runIDs := []string{"run-1", "run-2", "run-3", "run-4", "run-5"}
	model := newUploadModel(mock, runIDs)

	// Upload run-1 (success)
	msg1 := uploadedRunMsg{runID: "run-1", err: nil}
	updatedModel, _ := model.Update(msg1)
	model = updatedModel.(uploadModel)

	if model.succeeded != 1 {
		t.Errorf("expected succeeded=1, got %d", model.succeeded)
	}
	if model.failed != 0 {
		t.Errorf("expected failed=0, got %d", model.failed)
	}

	// Upload run-2 (failure)
	msg2 := uploadedRunMsg{runID: "run-2", err: fmt.Errorf("network error")}
	updatedModel, _ = model.Update(msg2)
	model = updatedModel.(uploadModel)

	if model.succeeded != 1 {
		t.Errorf("expected succeeded=1, got %d", model.succeeded)
	}
	if model.failed != 1 {
		t.Errorf("expected failed=1, got %d", model.failed)
	}

	// User cancels (hits ESC)
	keyMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = model.Update(keyMsg)
	model = updatedModel.(uploadModel)

	// Verify cancellation state
	if !model.cancelled {
		t.Error("expected cancelled=true")
	}
	if !model.done {
		t.Error("expected done=true")
	}

	// Verify counts are correct
	if model.succeeded != 1 {
		t.Errorf("expected succeeded=1 after cancel, got %d", model.succeeded)
	}
	if model.failed != 1 {
		t.Errorf("expected failed=1 after cancel, got %d", model.failed)
	}

	// Remaining should be calculated correctly: 5 total - 1 success - 1 failure = 3 remaining
	totalRuns := len(runIDs)
	remaining := totalRuns - model.succeeded - model.failed
	if remaining != 3 {
		t.Errorf("expected 3 remaining runs, got %d", remaining)
	}
}

func TestUploadModel_EarlyTermination(t *testing.T) {
	mock := upload.NewMockUploadClient()
	// All uploads will fail
	mock.SetError("run-1", "network error")
	mock.SetError("run-2", "network error")
	mock.SetError("run-3", "network error")
	mock.SetError("run-4", "network error")
	mock.SetError("run-5", "network error")

	runIDs := []string{"run-1", "run-2", "run-3", "run-4", "run-5"}
	model := newUploadModel(mock, runIDs)

	// Verify max consecutive fails is set
	if model.maxConsecutiveFails != 3 {
		t.Errorf("expected maxConsecutiveFails=3, got %d", model.maxConsecutiveFails)
	}

	// Upload run-1 (fail)
	msg1 := uploadedRunMsg{runID: "run-1", err: fmt.Errorf("network error")}
	updatedModel, _ := model.Update(msg1)
	model = updatedModel.(uploadModel)

	if model.consecutiveFails != 1 {
		t.Errorf("expected consecutiveFails=1, got %d", model.consecutiveFails)
	}
	if model.aborted {
		t.Error("should not abort after 1 failure")
	}

	// Upload run-2 (fail)
	msg2 := uploadedRunMsg{runID: "run-2", err: fmt.Errorf("network error")}
	updatedModel, _ = model.Update(msg2)
	model = updatedModel.(uploadModel)

	if model.consecutiveFails != 2 {
		t.Errorf("expected consecutiveFails=2, got %d", model.consecutiveFails)
	}
	if model.aborted {
		t.Error("should not abort after 2 failures")
	}

	// Upload run-3 (fail) - should trigger abort
	msg3 := uploadedRunMsg{runID: "run-3", err: fmt.Errorf("network error")}
	updatedModel, _ = model.Update(msg3)
	model = updatedModel.(uploadModel)

	if model.consecutiveFails != 3 {
		t.Errorf("expected consecutiveFails=3, got %d", model.consecutiveFails)
	}
	if !model.aborted {
		t.Error("expected abort after 3 consecutive failures")
	}
	if !model.done {
		t.Error("expected done=true after abort")
	}

	// Verify final counts
	if model.succeeded != 0 {
		t.Errorf("expected succeeded=0, got %d", model.succeeded)
	}
	if model.failed != 3 {
		t.Errorf("expected failed=3, got %d", model.failed)
	}

	// Remaining runs should be 2 (run-4 and run-5 not attempted)
	totalRuns := len(runIDs)
	remaining := totalRuns - model.succeeded - model.failed
	if remaining != 2 {
		t.Errorf("expected 2 remaining runs, got %d", remaining)
	}
}

func TestUploadModel_ConsecutiveFailsResetOnSuccess(t *testing.T) {
	mock := upload.NewMockUploadClient()
	mock.SetError("run-1", "network error")
	mock.SetError("run-2", "network error")
	mock.SetSuccess("run-3") // Success resets consecutive counter
	mock.SetError("run-4", "network error")
	mock.SetSuccess("run-5")

	runIDs := []string{"run-1", "run-2", "run-3", "run-4", "run-5"}
	model := newUploadModel(mock, runIDs)

	// Fail 1
	msg1 := uploadedRunMsg{runID: "run-1", err: fmt.Errorf("network error")}
	updatedModel, _ := model.Update(msg1)
	model = updatedModel.(uploadModel)

	if model.consecutiveFails != 1 {
		t.Errorf("expected consecutiveFails=1, got %d", model.consecutiveFails)
	}

	// Fail 2
	msg2 := uploadedRunMsg{runID: "run-2", err: fmt.Errorf("network error")}
	updatedModel, _ = model.Update(msg2)
	model = updatedModel.(uploadModel)

	if model.consecutiveFails != 2 {
		t.Errorf("expected consecutiveFails=2, got %d", model.consecutiveFails)
	}

	// Success - should reset counter
	msg3 := uploadedRunMsg{runID: "run-3", err: nil}
	updatedModel, _ = model.Update(msg3)
	model = updatedModel.(uploadModel)

	if model.consecutiveFails != 0 {
		t.Errorf("expected consecutiveFails=0 after success, got %d", model.consecutiveFails)
	}
	if model.aborted {
		t.Error("should not abort when consecutive failures are reset")
	}

	// Fail 4
	msg4 := uploadedRunMsg{runID: "run-4", err: fmt.Errorf("network error")}
	updatedModel, _ = model.Update(msg4)
	model = updatedModel.(uploadModel)

	if model.consecutiveFails != 1 {
		t.Errorf("expected consecutiveFails=1 after reset, got %d", model.consecutiveFails)
	}

	// Success 5 - completes normally
	msg5 := uploadedRunMsg{runID: "run-5", err: nil}
	updatedModel, _ = model.Update(msg5)
	model = updatedModel.(uploadModel)

	if model.aborted {
		t.Error("should complete normally without abort")
	}
	if !model.done {
		t.Error("expected done=true")
	}
	if model.succeeded != 2 {
		t.Errorf("expected succeeded=2, got %d", model.succeeded)
	}
	if model.failed != 3 {
		t.Errorf("expected failed=3, got %d", model.failed)
	}
}
