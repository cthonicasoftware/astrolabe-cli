package tui

import (
	"bytes"
	"errors"
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

	if strings.Contains(app.vp.View(), "68 69 0d 0a") {
		t.Fatal("hex shown before toggling")
	}
	_, cmd := app.Update(keyRunes("x"))
	run(app, cmd)

	if !app.prefs.Hex {
		t.Fatal("x should turn hex view on")
	}
	row := rowContaining(t, app, "hi")
	if !strings.Contains(row, "68 69 0d 0a") {
		t.Errorf("text row should carry its raw bytes (\\r kept): %q", row)
	}
	if len(*saved) != 1 || !(*saved)[0].Hex {
		t.Errorf("saved prefs = %+v, want Hex on", *saved)
	}

	app.Update(keyRunes("x"))
	if app.prefs.Hex || strings.Contains(app.vp.View(), "68 69") {
		t.Errorf("x again should hide hex:\n%s", app.vp.View())
	}
}

// rowContaining returns the plain-text viewport row containing s.
func rowContaining(t *testing.T, app *App, s string) string {
	t.Helper()
	for _, r := range strings.Split(stripANSI(app.vp.View()), "\n") {
		if strings.Contains(r, s) {
			return r
		}
	}
	t.Fatalf("no row contains %q:\n%s", s, app.vp.View())
	return ""
}

func TestAppHexViewRemembered(t *testing.T) {
	app := NewApp("test", make(chan FeedChunk))
	prefs := DefaultLiveViewPrefs()
	prefs.Hex = true
	app.SetPrefs(prefs, nil)
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app.Update(lineMsg(FeedChunk{Data: "A", At: testAt}))

	if !strings.Contains(rowContaining(t, app, "A"), " 41") {
		t.Errorf("remembered hex pref not applied:\n%s", app.vp.View())
	}
	if !strings.Contains(app.input.Placeholder, "hex") {
		t.Errorf("placeholder = %q, want a hex hint", app.input.Placeholder)
	}
}

// The hex column sits on the right, muted, in a fixed column that ends where
// a timestamp starts (or one cell short of the edge without timestamps).
func TestAppHexColumnAlignsLikeTimestamps(t *testing.T) {
	for _, width := range []int{60, 75, 120} {
		for _, stamps := range []bool{false, true} {
			app, _, _, _ := newHexApp(t, nil)
			app.Update(tea.WindowSizeMsg{Width: width, Height: 40})
			app.prefs.Timestamps = stamps
			app.Update(keyRunes("x"))
			app.Update(lineMsg(FeedChunk{Data: "short\r\n" + strings.Repeat("z", 40) + "\n", At: testAt}))

			vpWidth := app.vp.Width()
			avail := vpWidth
			if stamps {
				avail -= timestampGutter
			}
			perRow := hexBytesPerRow(avail)
			hexStart := avail - hexColumnWidth(perRow) - 1

			short := rowContaining(t, app, "short")
			if got := strings.Index(short, "73 68"); got != hexStart {
				t.Errorf("w=%d ts=%v: hex starts at %d, want %d: %q", width, stamps, got, hexStart, short)
			}
			stamp := testAt.Local().Format(timestampLayout)
			if stamps && strings.Index(short, stamp) != vpWidth-1-len(timestampLayout) {
				t.Errorf("w=%d: timestamp moved: %q", width, short)
			}
			// 41 bytes of z-line take several hex rows; every row fits.
			for _, r := range strings.Split(stripANSI(app.vp.View()), "\n") {
				if w := len(strings.TrimRight(r, " ")); w > vpWidth-1 {
					t.Errorf("w=%d ts=%v: row %d wide > %d: %q", width, stamps, w, vpWidth-1, r)
				}
			}
			if !strings.Contains(app.vp.View(), StyleMuted.Render("73 68 6f 72")) &&
				!strings.Contains(app.vp.View(), StyleMuted.Render(formatHexRows([]byte("short\r\n"), perRow)[0])) {
				t.Errorf("w=%d: hex not in muted style", width)
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
