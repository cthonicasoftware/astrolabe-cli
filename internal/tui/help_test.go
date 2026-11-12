package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// keyScreen records the keys it receives and can claim text focus.
type keyScreen struct {
	typing bool
	keys   []string
	other  int // non-key messages received
}

func (s *keyScreen) Init() tea.Cmd { return nil }
func (s *keyScreen) View() string  { return "screen" }
func (s *keyScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		s.keys = append(s.keys, k.String())
	} else {
		s.other++
	}
	return s, nil
}
func (s *keyScreen) TextFocused() bool { return s.typing }
func (s *keyScreen) HelpSections() []HelpSection {
	return []HelpSection{{Title: "Stub screen", Keys: []KeyHelp{{"x", "do the thing"}}}}
}

func newHelpRouter(screen Screen) *router {
	return &router{current: screen, width: 100, height: 30}
}

// v2 pastes are not key presses, so the help guard must catch them too.
func TestHelpSwallowsPaste(t *testing.T) {
	app, _, _ := newSendApp(t, nil)
	r := newHelpRouter(app)

	r.Update(keyRunes("i"))
	r.Update(tea.KeyPressMsg{Code: tea.KeyF1})
	r.Update(tea.PasteMsg{Content: "reboot"})
	if app.input.Value() != "" {
		t.Fatalf("paste reached hidden input while help open: %q", app.input.Value())
	}
}

func TestHelpOpensAndClosesWithoutReachingScreen(t *testing.T) {
	screen := &keyScreen{}
	r := newHelpRouter(screen)

	r.Update(keyRunes("?"))
	if !r.helpOpen {
		t.Fatal("? should open help")
	}
	view := r.View().Content
	for _, want := range []string{"Keyboard shortcuts", "Stub screen", "do the thing", "Everywhere", "esc: close"} {
		if !strings.Contains(view, want) {
			t.Errorf("help view missing %q", want)
		}
	}

	r.Update(keyRunes("x")) // swallowed while open
	r.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if r.helpOpen {
		t.Fatal("esc should close help")
	}
	if len(screen.keys) != 0 {
		t.Fatalf("screen got keys %v while help handled them", screen.keys)
	}
	if r.View().Content != "screen" {
		t.Fatal("screen view not restored after closing help")
	}
}

func TestHelpQuestionMarkTypesWhileTextFocused(t *testing.T) {
	screen := &keyScreen{typing: true}
	r := newHelpRouter(screen)

	r.Update(keyRunes("?"))
	if r.helpOpen {
		t.Fatal("? opened help while a text field had focus")
	}
	if len(screen.keys) != 1 || screen.keys[0] != "?" {
		t.Fatalf("screen keys = %v, want the ? typed", screen.keys)
	}

	r.Update(tea.KeyPressMsg{Code: tea.KeyF1})
	if !r.helpOpen {
		t.Fatal("f1 should open help even while typing")
	}
}

func TestHelpScreenKeepsGettingMessages(t *testing.T) {
	screen := &keyScreen{}
	r := newHelpRouter(screen)
	r.Update(keyRunes("?"))

	r.Update(tickMsg{})
	if screen.other != 1 {
		t.Fatal("non-key messages must still reach the screen while help is open")
	}
}

func TestHelpClosesOnNavigate(t *testing.T) {
	r := newHelpRouter(&keyScreen{})
	r.factories = map[ScreenID]ScreenFactory{
		ScreenWelcome: func(ScreenContext) (Screen, func(), error) { return &keyScreen{}, nil, nil },
	}
	r.Update(keyRunes("?"))
	r.Update(NavigateMsg{To: ScreenWelcome})
	if r.helpOpen {
		t.Fatal("help stayed open on a new screen")
	}
}

func TestLiveViewHelpFollowsMode(t *testing.T) {
	app, _, _ := newSendApp(t, nil)
	r := newHelpRouter(app)

	r.Update(keyRunes("?"))
	view := r.View().Content
	for _, want := range []string{"Live capture", "Sending", "toggle timestamps", "cycle line ending"} {
		if !strings.Contains(view, want) {
			t.Errorf("live view help missing %q", want)
		}
	}
	r.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	r.Update(keyRunes("i"))
	r.Update(keyRunes("?"))
	if r.helpOpen || app.input.Value() != "?" {
		t.Fatalf("while typing, ? should type: helpOpen=%v input=%q", r.helpOpen, app.input.Value())
	}
	r.Update(tea.KeyPressMsg{Code: tea.KeyF1})
	if !strings.Contains(r.View().Content, "Typing to the device") {
		t.Error("typing-mode help not shown")
	}
}

// A dialog taller than the terminal loses its top rows, so tall help
// switches to side-by-side sections.
func TestHelpDialogFitsSmallTerminal(t *testing.T) {
	app, _, _ := newSendApp(t, nil)
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		view := renderHelpDialog(app, size[0], size[1])
		if h := lipgloss.Height(view); h > size[1] {
			t.Errorf("%dx%d: dialog is %d rows", size[0], size[1], h)
		}
		if w := lipgloss.Width(view); w > size[0] {
			t.Errorf("%dx%d: dialog is %d cols", size[0], size[1], w)
		}
	}
}

func TestHelpOverlaysScreen(t *testing.T) {
	const w, h = 120, 40
	base := strings.Repeat(strings.Repeat("#", w)+"\n", h-1) + strings.Repeat("#", w)
	view := ansi.Strip(overlayHelp(base, &keyScreen{}, w, h))

	if got := lipgloss.Height(view); got != h {
		t.Errorf("overlay is %d rows, want %d", got, h)
	}
	if got := lipgloss.Width(view); got != w {
		t.Errorf("overlay is %d cols, want %d", got, w)
	}
	if !strings.Contains(view, "Keyboard shortcuts") {
		t.Error("dialog missing from overlay")
	}
	// The backdrop is the screen with its own styling replaced by the
	// muted color, so a styled base still comes out dimmed.
	styled := StyleTitle.Render("bright")
	if got := overlayHelp(styled, &keyScreen{}, w, h); strings.Contains(got, styled) {
		t.Error("screen behind the dialog kept its own styling; want it dimmed")
	}

	lines := strings.Split(view, "\n")
	if !strings.HasPrefix(lines[0], "####") || !strings.HasPrefix(lines[h/2], "#") {
		t.Error("screen underneath should stay visible around the dialog")
	}
}
