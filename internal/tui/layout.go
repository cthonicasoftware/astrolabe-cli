package tui

import (
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

// scrollStandIn marks where a scroll area goes in a layout passed to
// sizeScrollArea. It renders as one row.
const scrollStandIn = "x"

// sizeScrollArea sizes vp so the whole screen fits the terminal. layout is
// the screen rendered with scrollStandIn in place of the viewport, so every
// other row (titles, borders, help) is measured rather than assumed: Bubble
// Tea v2 drops the bottom rows of a frame taller than the terminal.
//
// With fill the area takes all spare rows and maxWidth columns; otherwise it
// hugs its content and scrolls only when the content does not fit.
func sizeScrollArea(vp *viewport.Model, maxWidth, termHeight int, layout string, fill bool) {
	if termHeight <= 0 || maxWidth <= 0 {
		return
	}
	avail := max(termHeight-(lipgloss.Height(layout)-1), 1)
	if fill {
		vp.SetWidth(maxWidth)
		vp.SetHeight(avail)
		return
	}
	content := vp.GetContent()
	width := min(lipgloss.Width(content), maxWidth)
	vp.SetWidth(width)
	wrapped := lipgloss.NewStyle().Width(width).Render(content)
	vp.SetHeight(min(lipgloss.Height(wrapped), avail))
}
