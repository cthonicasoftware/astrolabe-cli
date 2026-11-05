package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// keyScreen records the keys it receives and can claim text focus.
type keyScreen struct {
	typing bool
	keys   []string
	other  int // non-key messages received
}

func (s *keyScreen) Init() tea.Cmd { return nil }
func (s *keyScreen) View() string  { return "screen" }
func (s *keyScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
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

func newHelpRouter(screen tea.Model) *router {
	return &router{current: screen, width: 100, height: 30}
}

func TestHelpOpensAndClosesWithoutReachingScreen(t *testing.T) {
	screen := &keyScreen{}
	r := newHelpRouter(screen)

	r.Update(keyRunes("?"))
	if !r.helpOpen {
		t.Fatal("? should open help")
	}
	view := r.View()
	for _, want := range []string{"Keyboard shortcuts", "Stub screen", "do the thing", "Everywhere", "esc: close"} {
		if !strings.Contains(view, want) {
			t.Errorf("help view missing %q", want)
		}
	}

	r.Update(keyRunes("x")) // swallowed while open
	r.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if r.helpOpen {
		t.Fatal("esc should close help")
	}
	if len(screen.keys) != 0 {
		t.Fatalf("screen got keys %v while help handled them", screen.keys)
	}
	if r.View() != "screen" {
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

	r.Update(tea.KeyMsg{Type: tea.KeyF1})
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
		ScreenWelcome: func(ScreenContext) (tea.Model, func(), error) { return &keyScreen{}, nil, nil },
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
	view := r.View()
	for _, want := range []string{"Live capture", "Sending", "toggle timestamps", "cycle line ending"} {
		if !strings.Contains(view, want) {
			t.Errorf("live view help missing %q", want)
		}
	}
	r.Update(tea.KeyMsg{Type: tea.KeyEsc})

	r.Update(keyRunes("i"))
	r.Update(keyRunes("?"))
	if r.helpOpen || app.input.Value() != "?" {
		t.Fatalf("while typing, ? should type: helpOpen=%v input=%q", r.helpOpen, app.input.Value())
	}
	r.Update(tea.KeyMsg{Type: tea.KeyF1})
	if !strings.Contains(r.View(), "Typing to the device") {
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
