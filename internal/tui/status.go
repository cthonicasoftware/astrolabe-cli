package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// StatusKind determines the styling applied to a status message.
type StatusKind int

const (
	StatusInfo StatusKind = iota
	StatusSuccess
	StatusWarning
	StatusError
)

// StatusMessage represents a bordered notification rendered inside the TUI.
type StatusMessage struct {
	Kind  StatusKind
	Title string
	Body  string
}

// NewStatusMessage constructs a new status message.
func NewStatusMessage(kind StatusKind, title, body string) *StatusMessage {
	return &StatusMessage{Kind: kind, Title: title, Body: body}
}

var (
	statusBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			Margin(0, 0)

	statusTitleStyle = lipgloss.NewStyle().
				Bold(true)

	statusBodyStyle = lipgloss.NewStyle().
			Foreground(ColorText)
)

var statusPalette = map[StatusKind]struct {
	border lipgloss.Color
	icon   string
}{
	StatusInfo:    {ColorPrimary, "\uf05a"},
	StatusSuccess: {ColorSuccess, "\uf058"},
	StatusWarning: {ColorWarning, "\uf071"},
	StatusError:   {ColorError, "\uebfb"},
}

// Render draws the status message within the provided width.
func (m *StatusMessage) Render(width int) string {
	if m == nil {
		return ""
	}

	palette := statusPalette[m.Kind]
	if width <= 0 {
		width = 60
	}
	available := clamp(width, 30, 80)

	title := strings.TrimSpace(m.Title)
	body := strings.TrimSpace(m.Body)

	var lines []string
	if title != "" {
		icon := palette.icon
		titleLine := fmt.Sprintf("%s %s", icon, title)
		lines = append(lines, statusTitleStyle.Foreground(palette.border).Render(titleLine))
	}

	if body != "" {
		// Split body by newlines and render each line separately to preserve formatting
		bodyLines := strings.Split(body, "\n")
		for _, line := range bodyLines {
			if line != "" {
				lines = append(lines, statusBodyStyle.Copy().MaxWidth(available-4).Render(line))
			} else {
				lines = append(lines, "") // Preserve empty lines
			}
		}
	}

	content := strings.Join(lines, "\n")
	box := statusBoxStyle.Copy().
		BorderForeground(palette.border).
		MaxWidth(available).
		Render(content)

	return box
}

// TODO: Move to utils.go
func clamp(val, minVal, maxVal int) int {
	return int(math.Max(float64(minVal), math.Min(float64(maxVal), float64(val))))
}
