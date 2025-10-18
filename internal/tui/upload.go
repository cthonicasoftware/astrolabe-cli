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

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/upload"
)

type uploadModel struct {
	runIDs   []string
	index    int
	width    int
	height   int
	spinner  spinner.Model
	progress progress.Model
	done     bool
	failed   int
	client   upload.UploadClient
	ctx      context.Context
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
		runIDs:   runIDs,
		spinner:  NewDefaultSpinner(),
		progress: NewDefaultProgress(40),
		client:   client,
		ctx:      context.Background(),
	}
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
		case "ctrl+c", "esc", "q":
			return m, tea.Quit
		}

	case uploadedRunMsg:
		runID := m.runIDs[m.index]

		// Track failures
		if msg.err != nil {
			m.failed++
		}

		// Check if we're done
		if m.index >= len(m.runIDs)-1 {
			m.done = true
			var symbol string
			if msg.err != nil {
				symbol = StyledAlchemyError.String()
			} else {
				symbol = StyledAlchemySuccess.String()
			}
			return m, tea.Sequence(
				tea.Printf("%s %s", symbol, runID),
				tea.Quit,
			)
		}

		// Update progress bar
		m.index++
		progressCmd := m.progress.SetPercent(float64(m.index) / float64(len(m.runIDs)))

		var symbol string
		if msg.err != nil {
			symbol = StyledAlchemyError.String()
		} else {
			symbol = StyledAlchemySuccess.String()
		}

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
		succeeded := n - m.failed
		summary := fmt.Sprintf("Upload complete! %d succeeded", succeeded)
		if m.failed > 0 {
			summary += fmt.Sprintf(", %d failed", m.failed)
		}
		summary += ".\n"
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

// RunUploadTUI launches the upload progress TUI.
func RunUploadTUI(client *upload.Client, runIDs []string) error {
	return runUploadTUIWithClient(client, runIDs)
}

// runUploadTUIWithClient launches the upload progress TUI with any UploadClient implementation.
// This is exported for testing purposes.
func runUploadTUIWithClient(client upload.UploadClient, runIDs []string) error {
	if len(runIDs) == 0 {
		fmt.Println("No runs to upload")
		return nil
	}

	model := newUploadModel(client, runIDs)
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return err
	}

	return nil
}
