package tabframe

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	fallbackTermWidth       = 80
	minContentWidth         = 10
	windowPaddingVertical   = 2
	windowPaddingHorizontal = 0
	framePaddingVertical    = 1
	framePaddingHorizontal  = 2
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
			Padding(windowPaddingVertical, windowPaddingHorizontal).
			Align(lipgloss.Center).
			Border(lipgloss.NormalBorder()).
			UnsetBorderTop()

	// Padding around the whole tabbed frame block
	framePad = lipgloss.NewStyle().Padding(framePaddingVertical, framePaddingHorizontal)
)

func tabBorderWithBottom(left, middle, right string) lipgloss.Border {
	b := lipgloss.RoundedBorder()
	b.BottomLeft = left
	b.Bottom = middle
	b.BottomRight = right
	return b
}

// Single-pane

func RenderTabbedFrame(tabs []string, active int, content string, termWidth int) string {
	frame := New(tabs, active, termWidth)
	return frame.Render(content)
}

type Frame struct {
	tabRow     string
	innerWidth int
}

func New(tabs []string, active, termWidth int) Frame {
	tabRow := renderTabRow(tabs, active)
	inner := clampInnerWidth(termWidth, lipgloss.Width(tabRow))
	return Frame{tabRow: tabRow, innerWidth: inner}
}

func (f Frame) InnerWidth() int {
	return f.innerWidth
}

func (f Frame) Render(content string) string {
	win := windowStyle.Width(f.innerWidth).Render(content)
	var b strings.Builder
	b.WriteString(f.tabRow)
	b.WriteByte('\n')
	b.WriteString(win)
	return framePad.Render(b.String())
}

// internals

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
		termWidth = fallbackTermWidth
	}
	// Horizontal frame consumed by the window box
	frame := windowStyle.GetHorizontalFrameSize()
	// outer padding (framePad) adds space as well; subtract it
	outerPad := framePad.GetHorizontalFrameSize()

	maxInner := max(termWidth-frame-outerPad, minContentWidth)
	// Don’t exceed the tab row width either (nice visual alignment)
	if tabRowWidth > 0 {
		maxFromTabs := tabRowWidth - frame
		if maxFromTabs > minContentWidth && maxInner > maxFromTabs {
			maxInner = maxFromTabs
		}
	}
	return maxInner
}
