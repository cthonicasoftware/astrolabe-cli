package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/cthonicasoftware/astrolabe-cli/internal/runs"
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

// Screen is a routable child model. Only the router is a tea.Model; screens
// render plain strings and the router wraps them in a tea.View.
type Screen interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Screen, tea.Cmd)
	View() string
}

// ScreenFactory constructs a screen on demand.
// The returned func() is a cleanup hook called when the router navigates away.
// Return nil for the func() if no cleanup is needed.
// Return a non-nil error to navigate back to welcome with a StatusError.
type ScreenFactory func(ctx ScreenContext) (Screen, func(), error)

// RouterConfig holds router dependencies.
type RouterConfig struct {
	InitialScreen     ScreenID
	InitialStatus     *StatusMessage
	CapturePort       CaptureSessionPort
	MetadataPort      MetadataPort
	UploadPort        UploadPort
	LiveViewPrefsPort LiveViewPrefsPort // optional; nil uses defaults and does not persist
	RunsCacheRoot     string
}

type router struct {
	cfg       RouterConfig
	factories map[ScreenID]ScreenFactory
	current   Screen
	cleanup   func()
	width     int
	height    int
	// helpOpen shows the help dialog over the current screen. The screen
	// still gets every non-key message, so captures keep running.
	helpOpen bool
}

func newRouter(cfg RouterConfig) *router {
	status := cfg.InitialStatus
	factories := buildFactories(cfg)
	screen, cleanup, err := factories[cfg.InitialScreen](ScreenContext{Status: status})
	if err != nil {
		// Fall back to welcome on error
		screen, cleanup, _ = factories[ScreenWelcome](ScreenContext{})
	}
	cfg.InitialStatus = status
	return &router{
		cfg:       cfg,
		factories: factories,
		current:   screen,
		cleanup:   cleanup,
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

	case tea.KeyPressMsg:
		// Global quit on ctrl+c
		if msg.String() == "ctrl+c" {
			r.runCleanup()
			return r, tea.Quit
		}
		if r.helpOpen {
			// The dialog takes all keys; the screen never sees them.
			if closesHelp(msg) {
				r.helpOpen = false
			}
			return r, nil
		}
		if opensHelp(msg, r.current) {
			r.helpOpen = true
			return r, nil
		}

	case tea.PasteMsg:
		// v2 pastes are not key presses; the open dialog swallows them too.
		if r.helpOpen {
			return r, nil
		}
	}

	newCurrent, cmd := r.current.Update(msg)
	r.current = newCurrent
	return r, cmd
}

func (r *router) View() tea.View {
	v := tea.NewView(r.render())
	v.AltScreen = true
	return v
}

// render returns the frame content as a string.
func (r *router) render() string {
	if r.helpOpen {
		return renderHelpDialog(r.current, r.width, r.height)
	}
	return r.current.View()
}

func (r *router) navigate(msg NavigateMsg) tea.Cmd {
	// Clean up current screen
	r.runCleanup()
	r.helpOpen = false // help described the screen being left

	// Quit if navigating to empty screen (welcome quit path)
	if msg.To == "" {
		return tea.Quit
	}

	factory, ok := r.factories[msg.To]
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

func buildFactories(cfg RouterConfig) map[ScreenID]ScreenFactory {
	return map[ScreenID]ScreenFactory{
		ScreenWelcome: func(ctx ScreenContext) (Screen, func(), error) {
			return NewWelcome(ctx.Status), nil, nil
		},
		ScreenConfig: func(ctx ScreenContext) (Screen, func(), error) {
			return NewConfigEditor(), nil, nil
		},
		ScreenCaptureTabs: func(ctx ScreenContext) (Screen, func(), error) {
			return NewCaptureTabs(), nil, nil
		},
		ScreenCaptureLive: func(ctx ScreenContext) (Screen, func(), error) {
			if cfg.CapturePort == nil {
				return nil, nil, fmt.Errorf("capture not configured")
			}
			captureCfg, ok := ctx.Args.(CaptureConfig)
			if !ok {
				return nil, nil, fmt.Errorf("capture live: missing or invalid CaptureConfig in Args")
			}
			// Prefs load first: the source starts with the remembered
			// reconnect setting.
			prefs, savePrefs := loadLiveViewPrefs(cfg.LiveViewPrefsPort)
			captureCfg.Reconnect = prefs.Reconnect
			session, err := cfg.CapturePort.Start(context.Background(), captureCfg)
			if err != nil {
				return nil, nil, fmt.Errorf("start capture: %w", err)
			}
			appModel := NewApp("Live Capture", session.Feed())
			appModel.SetPrefs(prefs, savePrefs)
			if session.CanSend() {
				appModel.EnableSend(session.Send)
				appModel.EnableSendBytes(session.SendBytes)
			}
			appModel.EnableReconnect(session.SetReconnect)
			cleanup := func() {
				if appModel.SaveRequested() {
					session.RequestSave()
				}
				_ = session.Stop()
			}
			return appModel, cleanup, nil
		},
		ScreenMetadata: func(ctx ScreenContext) (Screen, func(), error) {
			if cfg.MetadataPort == nil {
				return nil, nil, fmt.Errorf("metadata not configured")
			}
			meta, loadErr := cfg.MetadataPort.Load()
			path, _ := cfg.MetadataPort.Path()
			return NewMetadataEditor(cfg.MetadataPort, meta, path, loadErr), nil, nil
		},
		ScreenRuns: func(ctx ScreenContext) (Screen, func(), error) {
			return NewRunsViewer(cfg.RunsCacheRoot), nil, nil
		},
		ScreenUpload: func(ctx ScreenContext) (Screen, func(), error) {
			if cfg.UploadPort == nil {
				return nil, nil, fmt.Errorf("upload not configured")
			}
			runIDs, err := runs.FindPending(cfg.RunsCacheRoot)
			if err != nil {
				return nil, nil, fmt.Errorf("find pending runs: %w", err)
			}
			if len(runIDs) == 0 {
				return nil, nil, fmt.Errorf("no pending runs: all runs have been uploaded")
			}
			return NewUploadModel(cfg.UploadPort, runIDs), nil, nil
		},
	}
}

// NewRouterForTest exposes router construction for white-box tests.
// loadLiveViewPrefs reads remembered live-view settings. Load errors fall back to
// defaults; a capture should never fail because a prefs file is bad.
func loadLiveViewPrefs(port LiveViewPrefsPort) (LiveViewPrefs, func(LiveViewPrefs) error) {
	if port == nil {
		return DefaultLiveViewPrefs(), nil
	}
	prefs, err := port.Load()
	if err != nil {
		prefs = DefaultLiveViewPrefs()
	}
	return prefs, port.Save
}

func NewRouterForTest(cfg RouterConfig) *router {
	return newRouter(cfg)
}

func (r *router) runCleanup() {
	if r.cleanup != nil {
		r.cleanup()
		r.cleanup = nil
	}
}

// RunTUI launches the single Bubble Tea program for the entire application lifetime.
func RunTUI(cfg RouterConfig) error {
	r := newRouter(cfg)
	_, err := tea.NewProgram(r).Run()
	return err
}

// standalone runs a single Screen as its own program (inline, no alt screen).
type standalone struct{ s Screen }

func (p standalone) Init() tea.Cmd { return p.s.Init() }

func (p standalone) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	p.s, cmd = p.s.Update(msg)
	return p, cmd
}

func (p standalone) View() tea.View { return tea.NewView(p.s.View()) }

// RunScreen runs s as a standalone Bubble Tea program.
func RunScreen(s Screen) error {
	_, err := tea.NewProgram(standalone{s: s}).Run()
	return err
}
