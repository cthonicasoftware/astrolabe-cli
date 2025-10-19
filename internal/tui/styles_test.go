package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestMockUploadProgressPreview renders the styled progress bar at several
// points in a simulated upload so developers can eyeball the gradient and
// padding when iterating on styles. Run with:
//
//	go test ./internal/tui -run TestMockUploadProgressPreview -v
func TestMockUploadProgressPreview(t *testing.T) {
	t.Parallel()

	runIDs := []string{
		"run-orange-0001",
		"run-orange-0002",
		"run-orange-0003",
		"run-orange-0004",
	}

	lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(os.Stderr))
	lipgloss.SetColorProfile(termenv.TrueColor)

	progressBar := progress.New(
		DefaultProgressGradient,
		progress.WithWidth(34),
		progress.WithoutPercentage(),
		progress.WithColorProfile(termenv.TrueColor),
	)

	var preview strings.Builder
	preview.WriteString("Mock upload progress preview:\n")

	for i := 0; i <= len(runIDs); i++ {
		percent := float64(i) / float64(len(runIDs))
		progressLine := progressBar.ViewAs(percent)

		var status string
		switch {
		case i == 0:
			status = "queued all runs"
		case i == len(runIDs):
			status = "finished uploading"
		default:
			status = fmt.Sprintf("completed %s", runIDs[i-1])
		}

		preview.WriteString(fmt.Sprintf("  - %6.2f%% %s :: %s\n", percent*100, status, progressLine))
	}

	// Emit the preview so a developer can see the gradient in test output.
	t.Log("\n" + preview.String())

	// Sanity-check that the progress bar renders something visible in the middle.
	midpoint := progressBar.ViewAs(0.5)
	if strings.TrimSpace(midpoint) == "" {
		t.Fatal("expected progress bar to render at 50%, got empty output")
	}
}
