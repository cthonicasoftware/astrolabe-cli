package tui

import (
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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

var (
	// layerBackdropStyle dims whatever sits under an open layer.
	layerBackdropStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	// layerStyle frames a screen's own layers (run details, data).
	layerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 2)
)

// Each layer deeper in a stack is inset this many columns and rows further
// on every side, so the layers below stay visible around it.
const (
	layerInsetX = 4
	layerInsetY = 1
)

// layerBounds is the largest outer size of a layer at depth (1 = first layer
// over the screen) in a width x height terminal.
func layerBounds(width, height, depth int) (w, h int) {
	return max(width-2*depth*layerInsetX, 20), max(height-2*depth*layerInsetY, 3)
}

// overlayLayer composites top as a layer centered over base in a width x
// height frame. base is repainted in one muted color so the layer stands
// out; its own colors would otherwise compete with it. Overlaying again
// stacks another layer, dimming every one below.
func overlayLayer(base, top string, width, height int) string {
	if width <= 0 {
		width = max(lipgloss.Width(base), lipgloss.Width(top))
	}
	if height <= 0 {
		height = max(lipgloss.Height(base), lipgloss.Height(top))
	}
	x := max(0, (width-lipgloss.Width(top))/2)
	y := max(0, (height-lipgloss.Height(top))/2)

	backdrop := layerBackdropStyle.Render(ansi.Strip(base))
	comp := lipgloss.NewCompositor(
		lipgloss.NewLayer(backdrop),
		lipgloss.NewLayer(top).X(x).Y(y).Z(1),
	)
	return lipgloss.NewCanvas(width, height).Compose(comp).Render()
}
