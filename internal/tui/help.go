package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// KeyHelp is one row of the help dialog: the keys and what they do.
type KeyHelp struct {
	Keys string
	Desc string
}

// HelpSection is a titled group of rows in the help dialog.
type HelpSection struct {
	Title string
	Keys  []KeyHelp
}

// HelpProvider is implemented by screens that list their keys in the help
// dialog. Sections should describe the screen's current mode.
type HelpProvider interface {
	HelpSections() []HelpSection
}

// TextFocuser is implemented by screens that can have a text field focused.
// While one is, ? is typed into the field; f1 still opens help.
type TextFocuser interface {
	TextFocused() bool
}

// keyHelpFrom turns bindings into dialog rows using their help text.
func keyHelpFrom(bindings ...key.Binding) []KeyHelp {
	rows := make([]KeyHelp, 0, len(bindings))
	for _, b := range bindings {
		h := b.Help()
		rows = append(rows, KeyHelp{Keys: h.Key, Desc: h.Desc})
	}
	return rows
}

// globalHelp lists the keys the router handles on every screen.
var globalHelp = HelpSection{
	Title: "Everywhere",
	Keys: []KeyHelp{
		{"?/f1", "show this help"},
		{"ctrl+c", "quit Astrolabe"},
	},
}

// opensHelp reports whether msg should open the help dialog on screen.
func opensHelp(msg tea.KeyPressMsg, screen Screen) bool {
	switch msg.String() {
	case "f1":
		return true
	case "?":
		tf, ok := screen.(TextFocuser)
		return !ok || !tf.TextFocused()
	}
	return false
}

// closesHelp reports whether msg closes an open help dialog.
func closesHelp(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "esc", "?", "f1", "q":
		return true
	}
	return false
}

var (
	helpDialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2)
	helpKeyStyle     = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	helpSectionStyle = lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true)
)

// helpColumnGap separates side-by-side columns in the help dialog.
const helpColumnGap = "    "

// renderHelpDialog draws the help dialog box sized to fit a width x height
// screen. Sections go in as few columns as fit the terminal's height, since
// rows past the bottom would push the dialog's top off screen.
func renderHelpDialog(screen Screen, width, height int) string {
	var sections []HelpSection
	if hp, ok := screen.(HelpProvider); ok {
		sections = hp.HelpSections()
	}
	sections = append(sections, globalHelp)

	title := StyleTitle.UnsetMargins().Render("Keyboard shortcuts")
	footer := StyleHelp.UnsetMargins().Render("esc: close")
	render := func(body string) string {
		return helpDialogStyle.Render(title + "\n\n" + body + "\n\n" + footer)
	}

	var best string
	for cols := 1; cols <= len(sections); cols++ {
		dialog := render(helpColumns(sections, cols))
		fitsWidth := width <= 0 || lipgloss.Width(dialog) <= width
		if best == "" || fitsWidth {
			best = dialog
		}
		if height <= 0 || lipgloss.Height(dialog) <= height {
			break
		}
		if !fitsWidth {
			break // more columns only get wider
		}
	}
	return best
}

// overlayHelp composites the help dialog as a layer centered over the
// screen's own view, so the screen stays visible around the dialog.
func overlayHelp(base string, screen Screen, width, height int) string {
	return overlayLayer(base, renderHelpDialog(screen, width, height), width, height)
}

// helpColumns lays sections out in n columns, in order, keeping columns
// about equally tall. Keys line up within each column.
func helpColumns(sections []HelpSection, n int) string {
	rows := func(s HelpSection) int { return len(s.Keys) + 2 } // title + gap
	total := 0
	for _, s := range sections {
		total += rows(s)
	}
	target := (total + n - 1) / n

	var columns [][]HelpSection
	var current []HelpSection
	used := 0
	for i, s := range sections {
		remainingCols := n - len(columns)
		remainingSecs := len(sections) - i
		if len(current) > 0 && (used+rows(s) > target || remainingSecs < remainingCols) {
			columns = append(columns, current)
			current, used = nil, 0
		}
		current = append(current, s)
		used += rows(s)
	}
	columns = append(columns, current)

	rendered := make([]string, 0, 2*len(columns))
	for i, col := range columns {
		if i > 0 {
			rendered = append(rendered, helpColumnGap)
		}
		rendered = append(rendered, renderHelpColumn(col))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

func renderHelpColumn(sections []HelpSection) string {
	keyWidth := 0
	for _, s := range sections {
		for _, k := range s.Keys {
			keyWidth = max(keyWidth, lipgloss.Width(k.Keys))
		}
	}
	blocks := make([]string, len(sections))
	for i, s := range sections {
		var b strings.Builder
		b.WriteString(helpSectionStyle.Render(s.Title))
		for _, k := range s.Keys {
			b.WriteString("\n")
			b.WriteString(helpKeyStyle.Width(keyWidth + 2).Render(k.Keys))
			b.WriteString(k.Desc)
		}
		blocks[i] = b.String()
	}
	return strings.Join(blocks, "\n\n")
}
