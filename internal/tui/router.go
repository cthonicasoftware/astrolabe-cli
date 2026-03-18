package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// ScreenID identifies a routable screen.
type ScreenID string

const (
	ScreenWelcome     ScreenID = "welcome"
	ScreenConfig      ScreenID = "config"
	ScreenMetadata    ScreenID = "metadata"
	ScreenRuns        ScreenID = "runs"
	ScreenUpload      ScreenID = "upload"
	ScreenCaptureTabs ScreenID = "capture-tabs"
	ScreenCaptureLive ScreenID = "capture-live"
)

// NavigateMsg is emitted by screens to request a transition.
type NavigateMsg struct {
	To     ScreenID
	Status *StatusMessage
	Args   any
}

// ScreenContext is passed to every ScreenFactory at construction time.
type ScreenContext struct {
	Status *StatusMessage
	Args   any
}

// ScreenFactory constructs a screen on demand.
// The returned func() is a cleanup hook called when the router navigates away.
// Return nil for the func() if no cleanup is needed.
// Return a non-nil error to navigate back to welcome with a StatusError.
type ScreenFactory func(ctx ScreenContext) (tea.Model, func(), error)

// RouterConfig holds all screen factories and external dependencies.
type RouterConfig struct {
	InitialScreen ScreenID
	InitialStatus *StatusMessage
	Factories     map[ScreenID]ScreenFactory
	CapturePort   CaptureSessionPort // may be nil until capture is wired
}

type router struct {
	cfg     RouterConfig
	current tea.Model
	cleanup func()
	width   int
	height  int
}

func newRouter(cfg RouterConfig) *router {
	status := cfg.InitialStatus
	screen, cleanup, err := cfg.Factories[cfg.InitialScreen](ScreenContext{Status: status})
	if err != nil {
		// Fall back to welcome on error
		screen, cleanup, _ = cfg.Factories[ScreenWelcome](ScreenContext{})
	}
	return &router{
		cfg:     cfg,
		current: screen,
		cleanup: cleanup,
	}
}

func (r *router) Init() tea.Cmd {
	return r.current.Init()
}

func (r *router) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = msg.Width
		r.height = msg.Height
		newCurrent, cmd := r.current.Update(msg)
		r.current = newCurrent
		return r, cmd

	case NavigateMsg:
		return r, r.navigate(msg)

	case tea.KeyMsg:
		// Global quit on ctrl+c
		if msg.Type == tea.KeyCtrlC {
			return r, tea.Quit
		}
	}

	newCurrent, cmd := r.current.Update(msg)
	r.current = newCurrent
	return r, cmd
}

func (r *router) View() string {
	return r.current.View()
}

func (r *router) navigate(msg NavigateMsg) tea.Cmd {
	// Clean up current screen
	if r.cleanup != nil {
		r.cleanup()
		r.cleanup = nil
	}

	// Quit if navigating to empty screen (welcome quit path)
	if msg.To == "" {
		return tea.Quit
	}

	factory, ok := r.cfg.Factories[msg.To]
	if !ok {
		// Unknown screen — go back to welcome with error
		return r.navigate(NavigateMsg{
			To:     ScreenWelcome,
			Status: NewStatusMessage(StatusError, "Navigation Error", "Unknown screen: "+string(msg.To)),
		})
	}

	screen, cleanup, err := factory(ScreenContext{Status: msg.Status, Args: msg.Args})
	if err != nil {
		// Factory error — go back to welcome with error status
		return r.navigate(NavigateMsg{
			To:     ScreenWelcome,
			Status: NewStatusMessage(StatusError, "Error", err.Error()),
		})
	}

	r.current = screen
	r.cleanup = cleanup

	return tea.Batch(
		screen.Init(),
		func() tea.Msg {
			return tea.WindowSizeMsg{Width: r.width, Height: r.height}
		},
	)
}

// RunTUI launches the single Bubble Tea program for the entire application lifetime.
func RunTUI(cfg RouterConfig) error {
	r := newRouter(cfg)
	_, err := tea.NewProgram(r, tea.WithAltScreen()).Run()
	return err
}
