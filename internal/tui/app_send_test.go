package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
)

type sentLine struct {
	line string
	le   sources.LineEnding
}

func newSendApp(t *testing.T, sendErr error) (*App, *[]sentLine, *[]SendPrefs) {
	t.Helper()
	var sent []sentLine
	var saved []SendPrefs
	app := NewApp("test", make(chan string))
	app.EnableSend(func(line string, le sources.LineEnding) error {
		sent = append(sent, sentLine{line, le})
		return sendErr
	}, DefaultSendPrefs(), func(p SendPrefs) error {
		saved = append(saved, p)
		return nil
	})
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return app, &sent, &saved
}

func keyRunes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

// run executes cmd and feeds its message back into the app, like Bubble Tea.
func run(app *App, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	if msg := cmd(); msg != nil {
		_, next := app.Update(msg)
		_ = next
	}
}

func typeText(app *App, s string) {
	for _, r := range s {
		app.Update(keyRunes(string(r)))
	}
}

func TestAppInputModeKeepsViewBindingsOutOfField(t *testing.T) {
	app, _, _ := newSendApp(t, nil)

	app.Update(keyRunes("i"))
	if !app.inputMode {
		t.Fatal("i should focus the input field")
	}

	// q and space must type, not quit or pause.
	_, cmd := app.Update(keyRunes("q"))
	if cmd != nil {
		if _, isNav := cmd().(NavigateMsg); isNav {
			t.Fatal("q in input mode navigated away")
		}
	}
	app.Update(keyRunes(" "))
	if app.paused {
		t.Fatal("space in input mode paused capture")
	}
	if got := app.input.Value(); got != "q " {
		t.Fatalf("input = %q, want %q", got, "q ")
	}

	app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if app.inputMode {
		t.Fatal("esc should leave input mode")
	}
}

func TestAppSendEchoesAndClears(t *testing.T) {
	app, sent, _ := newSendApp(t, nil)
	app.lineBuffer.AddData("partial") // device mid-line

	app.Update(keyRunes("i"))
	typeText(app, "AT")
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	run(app, cmd)

	if len(*sent) != 1 || (*sent)[0] != (sentLine{"AT", sources.LineEndingLF}) {
		t.Fatalf("sent = %+v, want one AT/LF", *sent)
	}
	if app.input.Value() != "" {
		t.Errorf("input not cleared after send: %q", app.input.Value())
	}
	if !app.inputMode {
		t.Error("input should stay focused after send")
	}

	lines := app.lineBuffer.GetDisplayLines()
	want := []string{"partial", EchoPrefix + "AT"}
	if len(lines) != len(want) || lines[0] != want[0] || lines[1] != want[1] {
		t.Fatalf("lines = %q, want %q", lines, want)
	}
	if !app.lineBuffer.IsEcho(1) || app.lineBuffer.IsEcho(0) {
		t.Error("echo flags wrong")
	}
}

func TestAppSendWithEchoOff(t *testing.T) {
	app, sent, _ := newSendApp(t, nil)
	app.Update(keyRunes("o")) // echo off
	app.Update(keyRunes("i"))
	typeText(app, "x")
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	run(app, cmd)

	if len(*sent) != 1 {
		t.Fatalf("sent = %+v", *sent)
	}
	if n := len(app.lineBuffer.GetDisplayLines()); n != 0 {
		t.Errorf("echo off still added %d lines", n)
	}
}

func TestAppEmptyEnterSendsLineEnding(t *testing.T) {
	app, sent, _ := newSendApp(t, nil)
	app.Update(keyRunes("i"))
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	run(app, cmd)
	if len(*sent) != 1 || (*sent)[0].line != "" {
		t.Fatalf("sent = %+v, want one empty line", *sent)
	}
}

func TestAppSendErrorKeepsText(t *testing.T) {
	app, _, _ := newSendApp(t, errors.New("port gone"))
	app.Update(keyRunes("i"))
	typeText(app, "retry")
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	run(app, cmd)

	if app.input.Value() != "retry" {
		t.Errorf("input = %q, want text kept for retry", app.input.Value())
	}
	if app.sendErr == "" {
		t.Error("send error not shown")
	}
	if len(app.lineBuffer.GetDisplayLines()) != 0 {
		t.Error("failed send should not echo")
	}
}

func TestAppIgnoresEnterWhileSending(t *testing.T) {
	app, _, _ := newSendApp(t, nil)
	app.Update(keyRunes("i"))
	_, first := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_, second := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if first == nil {
		t.Fatal("first enter should start a send")
	}
	if second != nil {
		t.Fatal("second enter while sending should be ignored to keep order")
	}
}

func TestAppHistory(t *testing.T) {
	app, _, _ := newSendApp(t, nil)
	app.Update(keyRunes("i"))
	for _, s := range []string{"one", "two"} {
		typeText(app, s)
		_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
		run(app, cmd)
	}
	typeText(app, "dra")

	app.Update(tea.KeyMsg{Type: tea.KeyUp})
	if app.input.Value() != "two" {
		t.Fatalf("up = %q, want two", app.input.Value())
	}
	app.Update(tea.KeyMsg{Type: tea.KeyUp})
	app.Update(tea.KeyMsg{Type: tea.KeyUp}) // already at oldest
	if app.input.Value() != "one" {
		t.Fatalf("up up = %q, want one", app.input.Value())
	}
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app.Update(tea.KeyMsg{Type: tea.KeyDown})
	if app.input.Value() != "dra" {
		t.Fatalf("down to draft = %q, want dra", app.input.Value())
	}
}

func TestAppLineEndingAndEchoPersist(t *testing.T) {
	app, _, saved := newSendApp(t, nil)

	_, cmd := app.Update(keyRunes("e"))
	run(app, cmd)
	_, cmd = app.Update(keyRunes("o"))
	run(app, cmd)

	if app.prefs.LineEnding != sources.LineEndingCRLF || app.prefs.Echo {
		t.Fatalf("prefs = %+v, want CRLF echo off", app.prefs)
	}
	if len(*saved) != 2 || (*saved)[1] != app.prefs {
		t.Fatalf("saved = %+v, want two saves ending in %+v", *saved, app.prefs)
	}
}

func TestAppFeedClosedDisablesInput(t *testing.T) {
	feed := make(chan string)
	close(feed)
	app := NewApp("test", feed)
	app.EnableSend(func(string, sources.LineEnding) error { return nil }, DefaultSendPrefs(), nil)
	app.Update(keyRunes("i"))

	run(app, app.pullLine())

	if !app.disconnected || app.inputMode {
		t.Fatalf("disconnected=%v inputMode=%v, want true/false", app.disconnected, app.inputMode)
	}
	app.Update(keyRunes("i"))
	if app.inputMode {
		t.Fatal("i should not focus input after disconnect")
	}
	// Saving must still work.
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	if cmd == nil {
		t.Fatal("ctrl+w should still save after disconnect")
	}
	if !app.SaveRequested() {
		t.Fatal("save not requested")
	}
}

func TestAppWithoutSendIgnoresSendKeys(t *testing.T) {
	app := NewApp("test", make(chan string))
	app.Update(keyRunes("i"))
	if app.inputMode {
		t.Fatal("read-only source should not enter input mode")
	}
	before := app.prefs
	app.Update(keyRunes("e"))
	if app.prefs != before {
		t.Fatal("e changed prefs without a send path")
	}
}
