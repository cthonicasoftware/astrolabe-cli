package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type keymap struct {
	Quit  key.Binding
	Help  key.Binding
	Pause key.Binding
	Clear key.Binding
}

func (k keymap) ShortHelp() []key.Binding { return []key.Binding{k.Quit, k.Pause, k.Help} }
func (k keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Quit, k.Pause, k.Clear, k.Help},
	}
}

var keys = keymap{
	Quit:  key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/ctrl+c", "quit")),
	Help:  key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
	Pause: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "pause/resume capture")),
	Clear: key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "clear buffer")),
}

type App struct {
	title      string
	vp         viewport.Model
	help       help.Model
	paused     bool
	feed       <-chan string
	lines      []string
	lineBuffer *strings.Builder // Buffer for partial lines (pointer to avoid copy issues)
	lastTick   time.Time
	width      int
	height     int
	boxStyle   lipgloss.Style
}

func NewApp(title string, feed <-chan string) App {
	vp := viewport.New(0, 0)
	vp.SetContent("")
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3

	// Disable high performance rendering to prevent border artifacts
	vp.HighPerformanceRendering = false

	// Apply a style to the viewport to ensure clean rendering
	vp.Style = lipgloss.NewStyle()

	// Create box style once
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2)

	return App{
		title:      title,
		vp:         vp,
		help:       help.New(),
		feed:       feed,
		boxStyle:   boxStyle,
		lineBuffer: &strings.Builder{},
	}
}

type tickMsg time.Time
type lineMsg string

func tickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (a App) Init() tea.Cmd {
	return tea.Batch(tickCmd(), a.pullLine())
}

func (a *App) pullLine() tea.Cmd {
	return func() tea.Msg {
		if a.feed == nil {
			return nil
		}
		s, ok := <-a.feed
		if !ok {
			return nil
		}
		return lineMsg(s)
	}
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = m.Width
		a.height = m.Height

		// Calculate viewport size to fill the available space
		// Account for:
		// - Box frame (border + padding)
		// - Title (2 lines)
		// - Status (2 lines)
		// - Help text (2 lines)
		// - Some margin for centering
		const headerFooterHeight = 8
		const marginWidth = 8

		// Get the exact frame size from the box style
		frameWidth := a.boxStyle.GetHorizontalFrameSize()
		frameHeight := a.boxStyle.GetVerticalFrameSize()

		a.vp.Width = m.Width - marginWidth - frameWidth - 1
		a.vp.Height = m.Height - headerFooterHeight - frameHeight

		// Set minimums
		if a.vp.Width < 40 {
			a.vp.Width = 40
		}
		if a.vp.Height < 10 {
			a.vp.Height = 10
		}

		// Reset YPosition to prevent viewport from rendering outside bounds
		a.vp.YPosition = 0

		// Re-wrap content when window size changes
		if len(a.lines) > 0 {
			wrappedContent := a.wrapLines(a.lines, a.vp.Width)
			a.vp.SetContent(wrappedContent)
			a.vp.GotoBottom()
		}

		return a, nil

	case tickMsg:
		a.lastTick = time.Time(m)
		if !a.paused {
			return a, tea.Batch(tickCmd(), a.pullLine())
		}
		return a, tickCmd()

	case lineMsg:

		if a.paused {
			return a, nil
		}
		// Add incoming data to buffer, stripping carriage returns to prevent cursor positioning issues
		cleaned := strings.ReplaceAll(string(m), "\r", "")
		a.lineBuffer.WriteString(cleaned)

		// Process complete lines (those ending with \n)
		bufferContent := a.lineBuffer.String()
		if strings.Contains(bufferContent, "\n") {
			// Split on newlines
			parts := strings.Split(bufferContent, "\n")

			// All parts except the last are complete lines
			for i := 0; i < len(parts)-1; i++ {
				if parts[i] != "" || i > 0 { // Keep empty lines except the very first
					a.lines = append(a.lines, parts[i])
				}
			}

			// The last part is either empty (if ended with \n) or a partial line
			a.lineBuffer.Reset()
			if parts[len(parts)-1] != "" {
				a.lineBuffer.WriteString(parts[len(parts)-1])
			}

			// Trim to max 1000 lines
			if len(a.lines) > 1000 {
				a.lines = a.lines[len(a.lines)-1000:]
			}
		}

		// Always update viewport to show current state (including partial line)
		displayLines := make([]string, len(a.lines))
		copy(displayLines, a.lines)

		// Add partial line if there is one
		if a.lineBuffer.Len() > 0 {
			displayLines = append(displayLines, a.lineBuffer.String())
		}

		wrappedContent := a.wrapLines(displayLines, a.vp.Width)
		a.vp.SetContent(wrappedContent)
		a.vp.GotoBottom()

		// Only pull next line if not paused
		return a, a.pullLine()

	case tea.KeyMsg:
		switch {
		case key.Matches(m, keys.Quit):
			return a, tea.Quit
		case key.Matches(m, keys.Help):
			a.help.ShowAll = !a.help.ShowAll
			return a, nil
		case key.Matches(m, keys.Pause):
			a.paused = !a.paused
			// If resuming from pause, restart pulling lines
			if !a.paused {
				return a, a.pullLine()
			}
			return a, nil
		case key.Matches(m, keys.Clear):
			a.lines = nil
			a.lineBuffer.Reset()
			a.vp.SetContent("")
			return a, nil
		}
	}
	var cmd tea.Cmd
	a.vp, cmd = a.vp.Update(msg)
	return a, cmd
}

// wrapLines wraps long lines to fit within the given width
func (a App) wrapLines(lines []string, width int) string {
	if width <= 0 {
		return strings.Join(lines, "\n")
	}

	// Use width - 1 to prevent viewport rendering artifacts at the edge
	wrapWidth := width - 1
	if wrapWidth < 1 {
		wrapWidth = 1
	}

	var wrapped strings.Builder
	for i, line := range lines {
		if i > 0 {
			wrapped.WriteString("\n")
		}

		// If line fits, add it as-is
		if len(line) <= wrapWidth {
			wrapped.WriteString(line)
			continue
		}

		// Wrap long lines
		remaining := line
		for len(remaining) > 0 {
			if len(remaining) <= wrapWidth {
				wrapped.WriteString(remaining)
				break
			}

			// Try to break at a space
			breakPoint := wrapWidth
			lastSpace := strings.LastIndex(remaining[:wrapWidth], " ")
			if lastSpace > 0 && lastSpace > wrapWidth/2 { // Don't break too early
				breakPoint = lastSpace
			}

			wrapped.WriteString(remaining[:breakPoint])
			wrapped.WriteString("\n")

			// Skip the space if we broke at one
			if breakPoint < len(remaining) && remaining[breakPoint] == ' ' {
				remaining = remaining[breakPoint+1:]
			} else {
				remaining = remaining[breakPoint:]
			}
		}
	}

	return wrapped.String()
}

func (a App) View() string {
	var s strings.Builder

	// Title section
	s.WriteString(StyleTitle.Render(a.title))
	s.WriteString("\n\n")

	// Status line with metadata
	statusText := fmt.Sprintf("Lines: %d", len(a.lines))
	if a.paused {
		statusText = StyleWarning.Render("⏸ PAUSED") + " • " + statusText
	} else {
		statusText = StyleSuccess.Render("● LIVE") + " • " + statusText
	}
	if !a.lastTick.IsZero() {
		statusText += " • " + StyleMuted.Render(a.lastTick.Format("15:04:05"))
	}
	s.WriteString(statusText)
	s.WriteString("\n\n")

	// Render viewport in bordered box with explicit size constraints
	vpContent := lipgloss.NewStyle().
		Width(a.vp.Width).
		Height(a.vp.Height).
		Render(a.vp.View())
	box := a.boxStyle.Render(vpContent)
	s.WriteString(box)
	s.WriteString("\n\n")

	// Help text
	if a.help.ShowAll {
		s.WriteString(StyleHelp.Render(a.help.View(keys)))
	} else {
		helpText := "space: pause/resume • ↑/↓/pgup/pgdn: scroll • ctrl+l: clear • ?: help • q: quit"
		s.WriteString(StyleHelp.Render(helpText))
	}

	content := s.String()
	return lipgloss.PlaceVertical(a.height, lipgloss.Top,
		lipgloss.PlaceHorizontal(a.width, lipgloss.Center, content))
}
