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
    Pause: key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "pause/resume")),
    Clear: key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "clear log")),
}

type App struct {
    title     string
    vp        viewport.Model
    help      help.Model
    paused    bool
    feed      <-chan string
    lines     []string
    lastTick  time.Time
}

var (
    borderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
    titleStyle  = lipgloss.NewStyle().Bold(true)
    dimStyle    = lipgloss.NewStyle().Faint(true)
)

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
        a.vp.Width = m.Width - 4
        a.vp.Height = m.Height - 6
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
        }
        return a, a.pullLine()

    case tea.KeyMsg:
        switch {
        case key.Matches(m, keys.Quit):
            return a, tea.Quit
        case key.Matches(m, keys.Help):
            a.help.ShowAll = !a.help.ShowAll
        case key.Matches(m, keys.Pause):
            a.paused = !a.paused
        case key.Matches(m, keys.Clear):
            a.lines = nil
            a.vp.SetContent("")
        }
    }
    var cmd tea.Cmd
    a.vp, cmd = a.vp.Update(msg)
    return a, cmd
}

func (a App) View() string {
    header := titleStyle.Render(a.title) + "  " + dimStyle.Render(fmt.Sprintf("tick: %s  paused: %v  lines: %d", a.lastTick.Format("15:04:05"), a.paused, len(a.lines)))
    box := borderStyle.Render(a.vp.View())
    footer := a.help.View(keys)
    return lipgloss.JoinVertical(lipgloss.Left, header, box, footer)
}
