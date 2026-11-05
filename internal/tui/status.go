package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
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
	border color.Color
	icon   string
}{
	StatusInfo:    {ColorPrimary, IconStatusInfo},
	StatusSuccess: {ColorSuccess, IconStatusSuccess},
	StatusWarning: {ColorWarning, IconStatusWarning},
	StatusError:   {ColorError, IconStatusError},
}

// Render draws the status message within the provided width.
func (m *StatusMessage) Render(width int) string {
	return m.RenderMax(width, 0)
}

// RenderMax is Render limited to maxHeight rows (0 means no limit). Body
// lines that do not fit become an ellipsis, so the box keeps its border.
func (m *StatusMessage) RenderMax(width, maxHeight int) string {
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
		icon := strings.TrimSpace(palette.icon)
		titleLine := fmt.Sprintf("%s %s", icon, title)
		lines = append(lines, statusTitleStyle.Foreground(palette.border).Render(titleLine))
	}

	if body != "" {
		// Split body by newlines and render each line separately to preserve formatting
		bodyLines := strings.SplitSeq(body, "\n")
		for line := range bodyLines {
			if line != "" {
				lines = append(lines, statusBodyStyle.MaxWidth(available-4).Render(line))
			} else {
				lines = append(lines, "") // Preserve empty lines
			}
		}
	}

	// Each line is one row: body lines are truncated to width, not wrapped.
	if frame := statusBoxStyle.GetVerticalFrameSize(); maxHeight > 0 && len(lines)+frame > maxHeight {
		keep := max(maxHeight-frame-1, 1)
		if keep < len(lines) {
			lines = append(lines[:keep], StyleMuted.Render("…"))
		}
	}

	content := strings.Join(lines, "\n")
	box := statusBoxStyle.
		BorderForeground(palette.border).
		MaxWidth(available).
		Render(content)

	return box
}
