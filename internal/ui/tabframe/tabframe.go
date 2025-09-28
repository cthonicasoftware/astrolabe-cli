package tabframe

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	Highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}

	inactiveTabBorder = tabBorderWithBottom("┴", "─", "┴")
	activeTabBorder   = tabBorderWithBottom("┘", " ", "└")

	inactiveTabStyle = lipgloss.NewStyle().Border(inactiveTabBorder, true).BorderForeground(Highlight).Padding(0, 1)
	activeTabStyle   = inactiveTabStyle.Border(activeTabBorder, true)

	// Window that sits under the tabs (no top border, so tabs can “plug in”)
	windowStyle = lipgloss.NewStyle().
			BorderForeground(Highlight).
			Padding(2, 0). // vertical padding = 2, horizontal padding = 0
			Align(lipgloss.Center).
			Border(lipgloss.NormalBorder()).
			UnsetBorderTop()

	// Padding around the whole tabbed frame block
	framePad = lipgloss.NewStyle().Padding(1, 2)
)

func tabBorderWithBottom(left, middle, right string) lipgloss.Border {
	b := lipgloss.RoundedBorder()
	b.BottomLeft = left
	b.Bottom = middle
	b.BottomRight = right
	return b
}

// ---------- Single-pane (existing) ----------

func RenderTabbedFrame(tabs []string, active int, content string, termWidth int) string {
	tabRow := renderTabRow(tabs, active)

	innerW := clampInnerWidth(termWidth, lipgloss.Width(tabRow))
	win := windowStyle.Width(innerW).Render(content)

	var b strings.Builder
	b.WriteString(tabRow)
	b.WriteByte('\n')
	b.WriteString(win)
	return framePad.Render(b.String())
}

// ---------- Two-column helpers ----------

// TwoColOptions controls the split and minimums of the two columns.
type TwoColOptions struct {
	LeftRatio       float64 // 0.0-1.0; default 0.60
	MinLeft         int     // default 32
	MinRight        int     // default 28
	HorizontalGap   int     // space between columns; default 4
	ForceInnerWidth int     // optional: if >0, override inner width
}

func defaults(o TwoColOptions) TwoColOptions {
	if o.LeftRatio <= 0 || o.LeftRatio >= 1 {
		o.LeftRatio = 0.60
	}
	if o.MinLeft <= 0 {
		o.MinLeft = 32
	}
	if o.MinRight <= 0 {
		o.MinRight = 28
	}
	if o.HorizontalGap < 0 {
		o.HorizontalGap = 4
	}
	return o
}

// ComputeColumnWidths returns the content inner width (inside the window box)
// and the final left/right column widths based on the options and terminal width.
func ComputeColumnWidths(termWidth int, opts TwoColOptions) (inner, left, right int) {
	opts = defaults(opts)

	inner = opts.ForceInnerWidth
	if inner <= 0 {
		inner = clampInnerWidth(termWidth, termWidth) // best-effort inner width
	}
	// Split with ratio
	left = int(float64(inner-opts.HorizontalGap) * opts.LeftRatio)
	right = (inner - opts.HorizontalGap) - left

	// Enforce minimums
	if left < opts.MinLeft {
		left = opts.MinLeft
	}
	if right < opts.MinRight {
		right = opts.MinRight
	}
	// Clamp if overflow
	if left+opts.HorizontalGap+right > inner {
		right = inner - opts.HorizontalGap - left
		if right < 10 {
			right = 10
			left = inner - opts.HorizontalGap - right
		}
	}
	return
}

// RenderTwoColumnTabbedFrame composes a two-column content area under the tab row.
// It sizes and clamps both columns so they fit inside the window frame cleanly.
func RenderTwoColumnTabbedFrame(
	tabs []string,
	active int,
	leftContent, rightContent string,
	termWidth int,
	opts TwoColOptions,
) (rendered string, innerWidth, leftW, rightW int) {

	opts = defaults(opts)
	tabRow := renderTabRow(tabs, active)

	innerWidth = clampInnerWidth(termWidth, lipgloss.Width(tabRow))
	// compute column widths for this inner width
	opts.ForceInnerWidth = innerWidth
	_, leftW, rightW = ComputeColumnWidths(termWidth, opts)

	left := lipgloss.NewStyle().
		Width(leftW).
		MaxWidth(leftW).
		PaddingRight(opts.HorizontalGap).
		Render(leftContent)

	right := lipgloss.NewStyle().
		Width(rightW).
		MaxWidth(rightW).
		Render(rightContent)

	row := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	win := windowStyle.Width(innerWidth).Render(row)

	var b strings.Builder
	b.WriteString(tabRow)
	b.WriteByte('\n')
	b.WriteString(win)
	return framePad.Render(b.String()), innerWidth, leftW, rightW
}

// ---------- internals ----------

func renderTabRow(tabs []string, active int) string {
	if len(tabs) == 0 {
		return ""
	}
	if active < 0 {
		active = 0
	}
	if active >= len(tabs) {
		active = len(tabs) - 1
	}
	var rendered []string
	for i, t := range tabs {
		style := inactiveTabStyle
		if i == active {
			style = activeTabStyle
		}
		border, _, _, _, _ := style.GetBorder()
		isFirst, isLast, isActive := i == 0, i == len(tabs)-1, i == active

		switch {
		case isFirst && isActive:
			border.BottomLeft = "│"
		case isFirst && !isActive:
			border.BottomLeft = "├"
		case isLast && isActive:
			border.BottomRight = "│"
		case isLast && !isActive:
			border.BottomRight = "┤"
		default:
			border.BottomLeft = "┴"
			border.BottomRight = "┴"
		}
		style = style.Border(border)
		rendered = append(rendered, style.Render(t))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

// clampInnerWidth chooses a window inner width that fits the terminal and tab row.
// It subtracts the windowStyle’s horizontal frame so content won’t wrap awkwardly.
func clampInnerWidth(termWidth int, tabRowWidth int) int {
	if termWidth <= 0 {
		termWidth = 80
	}
	// Horizontal frame consumed by the window box
	frame := windowStyle.GetHorizontalFrameSize()
	// outer padding (framePad) adds space as well; subtract it
	outerPad := framePad.GetHorizontalFrameSize()

	maxInner := termWidth - frame - outerPad
	if maxInner < 10 {
		maxInner = 10
	}
	// Don’t exceed the tab row width either (nice visual alignment)
	if tabRowWidth > 0 {
		maxFromTabs := tabRowWidth - frame
		if maxFromTabs > 10 && maxInner > maxFromTabs {
			maxInner = maxFromTabs
		}
	}
	return maxInner
}
