package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cthonicasoftware/astrolabe-cli/internal/upload"
)

type uploadModel struct {
	runIDs              []string
	index               int
	width               int
	height              int
	spinner             spinner.Model
	progress            progress.Model
	done                bool
	succeeded           int
	failed              int
	consecutiveFails    int
	cancelled           bool
	aborted             bool // Early termination due to repeated failures
	client              upload.UploadClient
	ctx                 context.Context
	maxConsecutiveFails int // Stop after this many consecutive failures
}

type uploadedRunMsg struct {
	runID string
	err   error
}

var (
	currentRunStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	uploadDoneStyle = lipgloss.NewStyle().Margin(1, 2)
)

func newUploadModel(client upload.UploadClient, runIDs []string) uploadModel {
	return uploadModel{
		runIDs:              runIDs,
		spinner:             NewDefaultSpinner(),
		progress:            NewDefaultProgress(40),
		client:              client,
		ctx:                 context.Background(),
		maxConsecutiveFails: 3, // Stop after 3 consecutive failures
	}
}

// NewUploadModel constructs an upload screen model for use with the router.
func NewUploadModel(client upload.UploadClient, runIDs []string) tea.Model {
	return newUploadModel(client, runIDs)
}

func (m uploadModel) Init() tea.Cmd {
	return tea.Batch(
		uploadRun(m.client, m.ctx, m.runIDs[m.index]),
		m.spinner.Tick,
	)
}

func (m uploadModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.cancelled = true
			m.done = true
			return m, navigateToWelcomeCmd(m.buildStatus())
		}

	case uploadedRunMsg:
		runID := m.runIDs[m.index]

		// Track results and consecutive failures
		if msg.err != nil {
			m.failed++
			m.consecutiveFails++
		} else {
			m.succeeded++
			m.consecutiveFails = 0 // Reset on success
		}

		var symbol string
		if msg.err != nil {
			symbol = StyledAlchemyError.String()
		} else {
			symbol = StyledAlchemySuccess.String()
		}

		// Check for early termination due to consecutive failures
		if m.consecutiveFails >= m.maxConsecutiveFails {
			m.done = true
			m.aborted = true
			return m, tea.Sequence(
				tea.Printf("%s %s", symbol, runID),
				navigateToWelcomeCmd(m.buildStatus()),
			)
		}

		// Check if we're done with all uploads
		if m.index >= len(m.runIDs)-1 {
			m.done = true
			return m, tea.Sequence(
				tea.Printf("%s %s", symbol, runID),
				navigateToWelcomeCmd(m.buildStatus()),
			)
		}

		// Update progress bar
		m.index++
		progressCmd := m.progress.SetPercent(float64(m.index) / float64(len(m.runIDs)))

		return m, tea.Batch(
			progressCmd,
			tea.Printf("%s %s", symbol, runID),
			uploadRun(m.client, m.ctx, m.runIDs[m.index]),
		)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		newModel, cmd := m.progress.Update(msg)
		if newModel, ok := newModel.(progress.Model); ok {
			m.progress = newModel
		}
		return m, cmd
	}
	return m, nil
}

func (m uploadModel) View() string {
	n := len(m.runIDs)
	w := lipgloss.Width(fmt.Sprintf("%d", n))

	if m.done {
		remaining := n - m.succeeded - m.failed

		var lines []string
		if m.aborted {
			// Early termination due to consecutive failures
			lines = append(lines, fmt.Sprintf("Upload stopped after %d consecutive failures.", m.maxConsecutiveFails))

			statusLine := fmt.Sprintf("%d succeeded, %d failed", m.succeeded, m.failed)
			if remaining > 0 {
				statusLine += fmt.Sprintf(", %d not attempted", remaining)
			}
			statusLine += "."
			lines = append(lines, statusLine)
			lines = append(lines, "Check your connection settings and try again.")
		} else if m.cancelled {
			lines = append(lines, "Upload cancelled.")

			statusLine := fmt.Sprintf("%d succeeded", m.succeeded)
			if m.failed > 0 {
				statusLine += fmt.Sprintf(", %d failed", m.failed)
			}
			if remaining > 0 {
				statusLine += fmt.Sprintf(", %d not attempted", remaining)
			}
			statusLine += "."
			lines = append(lines, statusLine)
		} else {
			lines = append(lines, "Upload complete!")

			statusLine := fmt.Sprintf("%d succeeded", m.succeeded)
			if m.failed > 0 {
				statusLine += fmt.Sprintf(", %d failed", m.failed)
			}
			statusLine += "."
			lines = append(lines, statusLine)
		}

		summary := strings.Join(lines, "\n")
		return uploadDoneStyle.Render(summary)
	}

	runCount := fmt.Sprintf(" %*d/%*d", w, m.index, w, n)

	spin := m.spinner.View() + " "
	prog := m.progress.View()
	cellsAvail := max(0, m.width-lipgloss.Width(spin+prog+runCount))

	runID := currentRunStyle.Render(m.runIDs[m.index])
	info := lipgloss.NewStyle().MaxWidth(cellsAvail).Render("Uploading " + runID)

	cellsRemaining := max(0, m.width-lipgloss.Width(spin+info+prog+runCount))
	gap := strings.Repeat(" ", cellsRemaining)

	return spin + info + gap + prog + runCount
}

func uploadRun(client upload.UploadClient, ctx context.Context, runID string) tea.Cmd {
	return func() tea.Msg {
		// Add a small delay to make the UI feel responsive
		// Remove this in production if uploads are already slow
		time.Sleep(100 * time.Millisecond)

		err := client.UploadRun(ctx, runID)
		return uploadedRunMsg{
			runID: runID,
			err:   err,
		}
	}
}

// buildStatus generates a StatusMessage reflecting the final upload state.
func (m uploadModel) buildStatus() *StatusMessage {
	totalRuns := len(m.runIDs)
	succeeded := m.succeeded
	failed := m.failed
	remaining := totalRuns - succeeded - failed

	// Handle early termination due to consecutive failures
	if m.aborted {
		lines := []string{
			fmt.Sprintf("Stopped after %d consecutive failures.", m.maxConsecutiveFails),
		}
		if succeeded > 0 {
			lines = append(lines, fmt.Sprintf("%d succeeded, %d failed, %d not attempted.", succeeded, failed, remaining))
		} else {
			lines = append(lines, fmt.Sprintf("%d failed, %d not attempted.", failed, remaining))
		}
		lines = append(lines, "Check your connection settings.")
		return NewStatusMessage(StatusError, "Upload Stopped", strings.Join(lines, "\n"))
	}

	// Handle cancellation
	if m.cancelled {
		if succeeded == 0 {
			lines := []string{
				"Upload cancelled.",
				fmt.Sprintf("No runs were uploaded. %d pending.", remaining),
			}
			return NewStatusMessage(StatusInfo, "Upload Cancelled", strings.Join(lines, "\n"))
		}
		lines := []string{"Upload cancelled."}
		if failed > 0 {
			lines = append(lines, fmt.Sprintf("%d succeeded, %d failed, %d not attempted.", succeeded, failed, remaining))
		} else {
			lines = append(lines, fmt.Sprintf("%d succeeded, %d not attempted.", succeeded, remaining))
		}
		return NewStatusMessage(StatusWarning, "Upload Cancelled", strings.Join(lines, "\n"))
	}

	// Normal completion
	if failed == 0 {
		var msg string
		if totalRuns == 1 {
			msg = "1 run uploaded successfully."
		} else {
			msg = fmt.Sprintf("%d runs uploaded successfully.", succeeded)
		}
		return NewStatusMessage(StatusSuccess, "Upload Complete", msg)
	} else if succeeded == 0 {
		lines := []string{
			fmt.Sprintf("All %d uploads failed.", totalRuns),
			"Check your connection settings and try again.",
		}
		return NewStatusMessage(StatusError, "Upload Failed", strings.Join(lines, "\n"))
	}
	lines := []string{
		fmt.Sprintf("%d succeeded, %d failed.", succeeded, failed),
		"Check your connection for failed runs.",
	}
	return NewStatusMessage(StatusWarning, "Upload Partially Complete", strings.Join(lines, "\n"))
}

// navigateToWelcomeCmd returns a Cmd that emits a NavigateMsg back to the welcome screen.
func navigateToWelcomeCmd(status *StatusMessage) tea.Cmd {
	return func() tea.Msg {
		return NavigateMsg{To: ScreenWelcome, Status: status}
	}
}

