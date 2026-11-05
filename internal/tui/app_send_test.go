package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
)

type sentLine struct {
	line string
	le   sources.LineEnding
}

func newSendApp(t *testing.T, sendErr error) (*App, *[]sentLine, *[]LiveViewPrefs) {
	t.Helper()
	var sent []sentLine
	var saved []LiveViewPrefs
	app := NewApp("test", make(chan FeedChunk))
	app.SetPrefs(DefaultLiveViewPrefs(), func(p LiveViewPrefs) error {
		saved = append(saved, p)
		return nil
	})
	app.EnableSend(func(line string, le sources.LineEnding) (time.Time, error) {
		sent = append(sent, sentLine{line, le})
		return testAt, sendErr
	})
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return app, &sent, &saved
}

func keyRunes(s string) tea.KeyPressMsg {
	r := []rune(s)
	return tea.KeyPressMsg{Code: r[0], Text: s}
}

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

	app.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if app.inputMode {
		t.Fatal("esc should leave input mode")
	}
}

// Bubble Tea v2 delivers pasted text as tea.PasteMsg, not key presses.
func TestAppPasteGoesToInputField(t *testing.T) {
	app, sent, _ := newSendApp(t, nil)

	app.Update(tea.PasteMsg{Content: "ignored"})
	if app.input.Value() != "" {
		t.Fatalf("paste outside input mode reached the field: %q", app.input.Value())
	}

	app.Update(keyRunes("i"))
	app.Update(tea.PasteMsg{Content: "AT+GMR"})
	_, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	run(app, cmd)

	if len(*sent) != 1 || (*sent)[0].line != "AT+GMR" {
		t.Fatalf("sent = %+v, want AT+GMR", *sent)
	}
}

func TestAppSendEchoesAndClears(t *testing.T) {
	app, sent, _ := newSendApp(t, nil)
	app.lineBuffer.AddData("partial", testAt) // device mid-line

	app.Update(keyRunes("i"))
	typeText(app, "AT")
	_, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	if got := app.lineBuffer.LineTime(1); !got.Equal(testAt) {
		t.Errorf("echo time = %v, want the time Send returned (%v)", got, testAt)
	}
}

func TestAppTimestampToggleRightAlignsAndPersists(t *testing.T) {
	app, _, saved := newSendApp(t, nil)
	app.appendData(FeedChunk{Data: "hello\n", At: testAt})

	if strings.Contains(app.vp.View(), timestampText(testAt)) {
		t.Fatal("timestamp shown before toggling on")
	}

	_, cmd := app.Update(keyRunes("t"))
	run(app, cmd)
	if !app.prefs.Timestamps {
		t.Fatal("t should turn timestamps on")
	}
	if len(*saved) != 1 || !(*saved)[0].Timestamps {
		t.Fatalf("saved = %+v, want timestamps on persisted", *saved)
	}

	var row string
	for _, r := range strings.Split(app.vp.View(), "\n") {
		if strings.Contains(r, "hello") {
			row = r
		}
	}
	if !strings.HasPrefix(row, "hello") {
		t.Fatalf("row = %q, want text still at the left edge", row)
	}
	plain := strings.TrimRight(stripANSI(row), " ")
	if !strings.HasSuffix(plain, timestampText(testAt)) {
		t.Fatalf("row = %q, want timestamp at the right", plain)
	}
	if got, want := len(plain), app.vp.Width()-1; got != want {
		t.Errorf("row width = %d, want timestamp ending at column %d", got, want)
	}
}

func TestAppTimestampToggleWithoutSendPath(t *testing.T) {
	app := NewApp("test", make(chan FeedChunk))
	app.Update(keyRunes("t"))
	if !app.prefs.Timestamps {
		t.Fatal("t should toggle timestamps on read-only sources too")
	}
}

// View centers on its widest row, so a status or help line wider than the
// bordered box shifts the whole layout left.
func TestAppRowsFitBoxWidth(t *testing.T) {
	app, _, _ := newSendApp(t, nil)
	box := lipgloss.Width(app.renderViewport())

	check := func(mode string) {
		t.Helper()
		for name, s := range map[string]string{
			"status": app.renderStatus(),
			"input":  app.renderInput(),
			"help":   app.renderHelp(),
		} {
			if w := lipgloss.Width(s); w > box {
				t.Errorf("%s: %s is %d wide, box is %d", mode, name, w, box)
			}
		}
	}

	check("view")
	app.Update(keyRunes("i"))
	check("typing")
}

// Bubble Tea drops the top rows of a view taller than the terminal, which
// hides the title.
func TestAppViewFitsTerminalHeight(t *testing.T) {
	for _, width := range []int{60, 80, 120, 200} {
		app, _, _ := newSendApp(t, nil)
		app.Update(tea.WindowSizeMsg{Width: width, Height: 30})
		for _, mode := range []string{"view", "typing"} {
			if mode == "typing" {
				app.Update(keyRunes("i"))
			}
			if h := lipgloss.Height(app.View()); h > 30 {
				t.Errorf("width %d, %s: view is %d rows, terminal is 30", width, mode, h)
			}
		}
	}
}

func timestampText(at time.Time) string { return at.Local().Format(timestampLayout) }

// stripANSI removes SGR escape sequences.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func TestAppSendWithEchoOff(t *testing.T) {
	app, sent, _ := newSendApp(t, nil)
	app.Update(keyRunes("o")) // echo off
	app.Update(keyRunes("i"))
	typeText(app, "x")
	_, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	_, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	run(app, cmd)
	if len(*sent) != 1 || (*sent)[0].line != "" {
		t.Fatalf("sent = %+v, want one empty line", *sent)
	}
}

func TestAppSendErrorKeepsText(t *testing.T) {
	app, _, _ := newSendApp(t, errors.New("port gone"))
	app.Update(keyRunes("i"))
	typeText(app, "retry")
	_, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	_, first := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	_, second := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
		_, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		run(app, cmd)
	}
	typeText(app, "dra")

	app.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if app.input.Value() != "two" {
		t.Fatalf("up = %q, want two", app.input.Value())
	}
	app.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	app.Update(tea.KeyPressMsg{Code: tea.KeyUp}) // already at oldest
	if app.input.Value() != "one" {
		t.Fatalf("up up = %q, want one", app.input.Value())
	}
	app.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	app.Update(tea.KeyPressMsg{Code: tea.KeyDown})
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
	feed := make(chan FeedChunk)
	close(feed)
	app := NewApp("test", feed)
	app.EnableSend(func(string, sources.LineEnding) (time.Time, error) { return testAt, nil })
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
	_, cmd := app.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+w should still save after disconnect")
	}
	if !app.SaveRequested() {
		t.Fatal("save not requested")
	}
}

func TestAppWithoutSendIgnoresSendKeys(t *testing.T) {
	app := NewApp("test", make(chan FeedChunk))
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
