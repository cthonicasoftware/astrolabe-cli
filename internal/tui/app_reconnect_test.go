package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

func linkChunk(ev core.LinkEvent) lineMsg {
	return lineMsg(FeedChunk{At: testAt, Link: &ev})
}

func lastLine(app *App) string {
	lines := app.lineBuffer.GetDisplayLines()
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

func TestApp_LinkDownShowsReconnectingAndBlocksSend(t *testing.T) {
	app, _, _ := newSendApp(t, nil)
	app.Update(keyRunes("i"))
	typeText(app, "AT+R")

	app.Update(linkChunk(core.LinkEvent{State: core.LinkDown, Err: "EOF"}))

	if s := app.renderStatus(); !strings.Contains(s, "RECONNECTING") {
		t.Errorf("status = %q, want RECONNECTING", s)
	}
	if app.canSend() {
		t.Error("canSend() = true while reconnecting")
	}
	if !app.inputMode {
		t.Error("input mode dropped while link is down; typing should continue")
	}
	if app.input.Value() != "AT+R" {
		t.Errorf("draft = %q, want it kept", app.input.Value())
	}
	if l := lastLine(app); !strings.Contains(l, "link down") || !strings.Contains(l, "EOF") {
		t.Errorf("last line = %q, want link down marker with reason", l)
	}
	if in := app.renderInput(); !strings.Contains(in, "Reconnecting…") {
		t.Errorf("input line = %q, want Reconnecting…", in)
	}
}

func TestApp_ReconnectingNotTypingMayEnterInputButNotSend(t *testing.T) {
	app, sent, _ := newSendApp(t, nil)
	app.Update(linkChunk(core.LinkEvent{State: core.LinkDown, Err: "EOF"}))
	if app.inputMode {
		t.Fatal("link down entered input mode on its own")
	}
	app.Update(keyRunes("i"))
	if !app.inputMode {
		t.Fatal("i did not enter input mode while reconnecting")
	}
	typeText(app, "hi")
	_, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	run(app, cmd)
	if len(*sent) != 0 || app.input.Value() != "hi" {
		t.Errorf("sent=%v draft=%q, want nothing sent and draft kept", *sent, app.input.Value())
	}
}

func TestApp_ReconnectingKeepsTypingKeysAsText(t *testing.T) {
	app, sent, saved := newSendApp(t, nil)
	app.EnableReconnect(func(bool) {})
	app.Update(keyRunes("i"))
	app.Update(linkChunk(core.LinkEvent{State: core.LinkDown, Err: "EOF"}))
	before := app.prefs.Reconnect

	_, c1 := app.Update(keyRunes("q"))
	_, c2 := app.Update(keyRunes("r"))
	for _, c := range []tea.Cmd{c1, c2} {
		if c == nil {
			continue
		}
		if _, ok := c().(NavigateMsg); ok {
			t.Fatal("typing q/r while reconnecting navigated away")
		}
	}
	if app.input.Value() != "qr" {
		t.Errorf("draft = %q, want qr", app.input.Value())
	}
	if app.prefs.Reconnect != before || len(*saved) != 0 {
		t.Errorf("prefs changed: reconnect %v->%v, saves=%d", before, app.prefs.Reconnect, len(*saved))
	}

	_, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	run(app, cmd)
	if len(*sent) != 0 || app.input.Value() != "qr" {
		t.Errorf("sent=%v draft=%q, want nothing sent and draft kept", *sent, app.input.Value())
	}
	in := app.renderInput()
	if !strings.Contains(in, "Reconnecting…") || !strings.Contains(in, "qr") {
		t.Errorf("input row = %q, want draft and Reconnecting… hint", in)
	}
}

func TestApp_FeedClosedLeavesInputMode(t *testing.T) {
	app, _, _ := newSendApp(t, nil)
	app.Update(keyRunes("i"))
	app.Update(linkChunk(core.LinkEvent{State: core.LinkDown}))
	app.handleFeedClosed()
	if app.inputMode || app.reconnecting || !app.disconnected {
		t.Errorf("inputMode=%v reconnecting=%v disconnected=%v", app.inputMode, app.reconnecting, app.disconnected)
	}
}

func TestApp_LinkUpShowsReconnectedThenClears(t *testing.T) {
	app, _, _ := newSendApp(t, nil)
	now := testAt
	app.now = func() time.Time { return now }

	app.Update(linkChunk(core.LinkEvent{State: core.LinkDown, Err: "EOF"}))
	_, cmd := app.Update(linkChunk(core.LinkEvent{
		State: core.LinkUp, Attempts: 3, Gap: 5200 * time.Millisecond, Port: "/dev/ttyUSB1",
	}))
	if cmd == nil {
		t.Error("link up returned no command; want a redraw tick")
	}

	if s := app.renderStatus(); !strings.Contains(s, "RECONNECTED") {
		t.Errorf("status = %q, want RECONNECTED", s)
	}
	if !app.canSend() {
		t.Error("canSend() = false after link up")
	}
	if l := lastLine(app); !strings.Contains(l, "link up after 5.2s on /dev/ttyUSB1") {
		t.Errorf("last line = %q, want link up marker", l)
	}

	now = now.Add(3*time.Second + time.Millisecond)
	if s := app.renderStatus(); strings.Contains(s, "RECONNECTED") || !strings.Contains(s, "LIVE") {
		t.Errorf("status after window = %q, want LIVE", s)
	}
}

func TestApp_LinkUpMarkerWithoutPort(t *testing.T) {
	app, _, _ := newSendApp(t, nil)
	app.Update(linkChunk(core.LinkEvent{State: core.LinkUp, Gap: 5200 * time.Millisecond}))
	if l := lastLine(app); !strings.HasSuffix(strings.TrimSpace(l), "link up after 5.2s ──") {
		t.Errorf("last line = %q", l)
	}
}

func TestApp_FeedClosedWhileReconnectingShowsDisconnected(t *testing.T) {
	app, _, _ := newSendApp(t, nil)
	app.Update(linkChunk(core.LinkEvent{State: core.LinkDown}))
	app.Update(feedClosedMsg{})

	s := app.renderStatus()
	if !strings.Contains(s, "DISCONNECTED") || strings.Contains(s, "RECONNECTING") {
		t.Errorf("status = %q, want DISCONNECTED and not RECONNECTING", s)
	}
}

func TestApp_ReconnectKeyTogglesAndSaves(t *testing.T) {
	app, _, saved := newSendApp(t, nil)
	before := app.prefs
	app.Update(keyRunes("r"))
	if app.prefs != before || len(*saved) != 0 {
		t.Fatal("r acted without EnableReconnect")
	}

	var got []bool
	app.EnableReconnect(func(on bool) { got = append(got, on) })
	if s := app.renderStatus(); !strings.Contains(s, "↻ on") {
		t.Errorf("status = %q, want ↻ on", s)
	}
	_, cmd := app.Update(keyRunes("r"))
	run(app, cmd)

	if len(got) != 1 || got[0] {
		t.Fatalf("setter calls = %v, want [false]", got)
	}
	if len(*saved) != 1 || (*saved)[0].Reconnect {
		t.Fatalf("saved = %+v, want one save with Reconnect off", *saved)
	}
	if s := app.renderStatus(); !strings.Contains(s, "↻ off") {
		t.Errorf("status = %q, want ↻ off", s)
	}
}

func TestDefaultLiveViewPrefsReconnectOn(t *testing.T) {
	if !DefaultLiveViewPrefs().Reconnect {
		t.Error("default prefs have reconnect off")
	}
}

func TestLineBufferAddMarker(t *testing.T) {
	lb := NewLineBuffer()
	lb.AddData("partial", testAt)
	lb.AddMarker("── mark ──", testAt.Add(time.Second))
	lines := lb.GetDisplayLines()
	if len(lines) != 2 || lines[0] != "partial" || lines[1] != "── mark ──" {
		t.Fatalf("lines = %q", lines)
	}
	if !lb.IsMarker(1) || lb.IsEcho(1) || lb.IsMarker(0) {
		t.Error("marker flags wrong")
	}
	if !lb.LineTime(1).Equal(testAt.Add(time.Second)) {
		t.Errorf("marker time = %v", lb.LineTime(1))
	}
}
