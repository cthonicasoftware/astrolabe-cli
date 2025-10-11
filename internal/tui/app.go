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
	title    string
	vp       viewport.Model
	help     help.Model
	paused   bool
	feed     <-chan string
	lines    []string
	lastTick time.Time
	width    int
	height   int
}

func NewApp(title string, feed <-chan string) App {
	vp := viewport.New(0, 0)
	vp.SetContent("")
	return App{
		title: title,
		vp:    vp,
		help:  help.New(),
		feed:  feed,
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

		// Calculate viewport size accounting for:
		// - Border (2 chars horizontal, 2 vertical)
		// - Padding (4 chars horizontal, 2 vertical)
		// - Header/footer space
		a.vp.Width = m.Width - 10
		a.vp.Height = m.Height - 10
		if a.vp.Width < 40 {
			a.vp.Width = 40
		}
		if a.vp.Height < 10 {
			a.vp.Height = 10
		}
		return a, nil

	case tickMsg:
		a.lastTick = time.Time(m)
		if !a.paused {
			return a, tea.Batch(tickCmd(), a.pullLine())
		}
		return a, tickCmd()

	case lineMsg:
		if !a.paused {
			a.lines = append(a.lines, string(m))
			if len(a.lines) > 1000 {
				a.lines = a.lines[len(a.lines)-1000:]
			}
			a.vp.SetContent(strings.Join(a.lines, "\n"))
			a.vp.GotoBottom()
			// Only pull next line if not paused
			return a, a.pullLine()
		}
		// If paused, don't pull more lines
		return a, nil

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
			a.vp.SetContent("")
			return a, nil
		}
	}
	var cmd tea.Cmd
	a.vp, cmd = a.vp.Update(msg)
	return a, cmd
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

	// Create bordered box for viewport
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2)

	box := boxStyle.Render(a.vp.View())
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
