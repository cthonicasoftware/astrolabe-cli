package tui

import (
	"strings"
	"testing"
	"time"

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
	if app.inputMode {
		t.Error("input mode kept while link is down")
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
	app.Update(keyRunes("i"))
	if app.inputMode {
		t.Error("i entered input mode while reconnecting")
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
	if s := app.renderStatus(); !strings.Contains(s, "Reconnect on") {
		t.Errorf("status = %q, want Reconnect on", s)
	}
	_, cmd := app.Update(keyRunes("r"))
	run(app, cmd)

	if len(got) != 1 || got[0] {
		t.Fatalf("setter calls = %v, want [false]", got)
	}
	if len(*saved) != 1 || (*saved)[0].Reconnect {
		t.Fatalf("saved = %+v, want one save with Reconnect off", *saved)
	}
	if s := app.renderStatus(); !strings.Contains(s, "Reconnect off") {
		t.Errorf("status = %q, want Reconnect off", s)
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
