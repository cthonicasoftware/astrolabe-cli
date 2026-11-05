package tui

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
)

// newHexApp returns a send-capable app whose SendBytes calls land in sent.
func newHexApp(t *testing.T, sendErr error) (*App, *[][]byte, *[]sentLine, *[]LiveViewPrefs) {
	t.Helper()
	app, lines, saved := newSendApp(t, nil)
	var sent [][]byte
	app.EnableSendBytes(func(data []byte) (time.Time, error) {
		sent = append(sent, data)
		return testAt, sendErr
	})
	return app, &sent, lines, saved
}

func TestAppHexKeyTogglesViewAndSaves(t *testing.T) {
	app, _, _, saved := newHexApp(t, nil)
	app.Update(lineMsg(FeedChunk{Data: "hi\r\n\x00", At: testAt}))

	if strings.Contains(app.vp.View(), "68 69 0d 0a 00") {
		t.Fatal("hex dump shown before toggling")
	}
	_, cmd := app.Update(keyRunes("x"))
	run(app, cmd)

	if !app.prefs.Hex {
		t.Fatal("x should turn hex view on")
	}
	view := app.vp.View()
	if !strings.Contains(view, "00000000  68 69 0d 0a 00") || !strings.Contains(view, "|hi...|") {
		t.Errorf("hex view missing raw bytes (\\r kept):\n%s", view)
	}
	if !strings.Contains(app.renderStatus(), "Bytes: 5") {
		t.Errorf("status = %q, want byte count", app.renderStatus())
	}
	if len(*saved) != 1 || !(*saved)[0].Hex {
		t.Errorf("saved prefs = %+v, want Hex on", *saved)
	}

	app.Update(keyRunes("x"))
	if app.prefs.Hex || !strings.Contains(app.vp.View(), "hi") || strings.Contains(app.vp.View(), "00000000") {
		t.Errorf("x again should return to text view:\n%s", app.vp.View())
	}
}

func TestAppHexViewRemembered(t *testing.T) {
	app := NewApp("test", make(chan FeedChunk))
	prefs := DefaultLiveViewPrefs()
	prefs.Hex = true
	app.SetPrefs(prefs, nil)
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app.Update(lineMsg(FeedChunk{Data: "A", At: testAt}))

	if !strings.Contains(app.vp.View(), "00000000  41") {
		t.Errorf("remembered hex pref not applied:\n%s", app.vp.View())
	}
	if !strings.Contains(app.input.Placeholder, "hex") {
		t.Errorf("placeholder = %q, want a hex hint", app.input.Placeholder)
	}
}

func TestAppHexDumpFitsNarrowViewport(t *testing.T) {
	for _, width := range []int{60, 75, 120} {
		app, _, _, _ := newHexApp(t, nil)
		app.Update(tea.WindowSizeMsg{Width: width, Height: 30})
		app.Update(keyRunes("x"))
		app.Update(lineMsg(FeedChunk{Data: strings.Repeat("z", 40), At: testAt}))

		perRow := hexBytesPerRow(app.vp.Width() - 1)
		rows := app.hexLog.Rows(perRow)
		if len(rows) < 2 || !strings.HasPrefix(rows[1].text, fmt.Sprintf("%08x  ", perRow)) {
			t.Errorf("width %d: second row = %+v, want offset %d", width, rows, perRow)
		}
		for _, row := range strings.Split(app.vp.View(), "\n") {
			if r := strings.TrimRight(row, " "); len(r) > app.vp.Width()-1 {
				t.Errorf("width %d: row %q wider than viewport %d", width, r, app.vp.Width())
			}
			if strings.HasPrefix(row, "00000000") && strings.Count(row, "7a") != perRow {
				t.Errorf("width %d: row %q not %d bytes (wrapped?)", width, row, perRow)
			}
		}
	}
}

func TestAppHexSendSendsExactBytesAndEchoes(t *testing.T) {
	app, sent, lines, _ := newHexApp(t, nil)
	app.Update(keyRunes("x"))
	app.Update(keyRunes("i"))
	typeText(app, "01 FF 0a")
	_, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	run(app, cmd)

	if len(*sent) != 1 || !bytes.Equal((*sent)[0], []byte{0x01, 0xff, 0x0a}) {
		t.Fatalf("SendBytes got %v, want 01 ff 0a", *sent)
	}
	if len(*lines) != 0 {
		t.Errorf("text Send also called: %+v", *lines)
	}
	if app.input.Value() != "" {
		t.Errorf("input not cleared: %q", app.input.Value())
	}
	if !strings.Contains(app.vp.View(), EchoPrefix+"hex 01 ff 0a") {
		t.Errorf("echo missing in hex view:\n%s", app.vp.View())
	}
	app.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	app.Update(keyRunes("x"))
	if !strings.Contains(app.vp.View(), EchoPrefix+"hex 01 ff 0a") {
		t.Errorf("echo missing in text view:\n%s", app.vp.View())
	}
}

func TestAppHexSendRejectsBadHex(t *testing.T) {
	app, sent, _, _ := newHexApp(t, nil)
	app.Update(keyRunes("x"))
	app.Update(keyRunes("i"))
	typeText(app, "01 f")
	_, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	run(app, cmd)

	if len(*sent) != 0 {
		t.Fatalf("bad hex was sent: %v", *sent)
	}
	if app.input.Value() != "01 f" || !strings.Contains(app.sendErr, "invalid hex") {
		t.Errorf("input=%q err=%q, want text kept and invalid hex error", app.input.Value(), app.sendErr)
	}
}

func TestAppHexSendFailureKeepsText(t *testing.T) {
	app, _, _, _ := newHexApp(t, errors.New("unplugged"))
	app.Update(keyRunes("x"))
	app.Update(keyRunes("i"))
	typeText(app, "aa")
	_, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	run(app, cmd)

	if app.input.Value() != "aa" || app.sendErr != "unplugged" {
		t.Errorf("input=%q err=%q", app.input.Value(), app.sendErr)
	}
}

func TestAppTextSendUnchangedOutsideHex(t *testing.T) {
	app, sent, lines, _ := newHexApp(t, nil)
	app.Update(keyRunes("i"))
	typeText(app, "01")
	_, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	run(app, cmd)

	if len(*sent) != 0 || len(*lines) != 1 || (*lines)[0] != (sentLine{"01", sources.LineEndingLF}) {
		t.Errorf("bytes=%v lines=%+v, want one text send", *sent, *lines)
	}
}

func TestAppHexInHelp(t *testing.T) {
	app, _, _, _ := newHexApp(t, nil)
	found := false
	for _, s := range app.HelpSections() {
		for _, k := range s.Keys {
			if k.Keys == "x" {
				found = true
			}
		}
	}
	if !found {
		t.Error("help does not list the x key")
	}
}
