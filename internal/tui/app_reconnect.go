package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

// reconnectedShowFor is how long the status bar shows RECONNECTED after the
// link comes back.
const reconnectedShowFor = 3 * time.Second

// reconnectedExpiredMsg redraws the status bar once RECONNECTED has expired.
type reconnectedExpiredMsg struct{}

// EnableReconnect turns on the r toggle. set is called with the new state
// each time the operator flips it; it must not block.
func (a *App) EnableReconnect(set func(bool)) {
	a.setReconnect = set
}

// applyLink updates the reconnect state for a link event. Its marker line is
// added with the data (see appendData) so pausing keeps it in order.
func (a *App) applyLink(ev core.LinkEvent) tea.Cmd {
	switch ev.State {
	case core.LinkDown:
		a.reconnecting = true
		a.reconnectAttempt = ev.Attempts
		// Leave the field but keep its text: it is sent once the link is back.
		a.inputMode = false
		a.input.Blur()
		return nil
	case core.LinkUp:
		a.reconnecting = false
		a.reconnectAttempt = 0
		a.reconnectedUntil = a.now().Add(reconnectedShowFor)
		return tea.Tick(reconnectedShowFor, func(time.Time) tea.Msg { return reconnectedExpiredMsg{} })
	}
	return nil
}

// showReconnected reports whether the status bar still shows RECONNECTED.
func (a *App) showReconnected() bool {
	return !a.reconnectedUntil.IsZero() && a.now().Before(a.reconnectedUntil)
}

// linkMarker is the line shown in the live view for a link event at at.
func linkMarker(ev core.LinkEvent, at time.Time) string {
	switch ev.State {
	case core.LinkDown:
		reason := ""
		if ev.Err != "" {
			reason = " (" + ev.Err + ")"
		}
		return fmt.Sprintf("── link down %s%s ──", at.Local().Format("15:04:05"), reason)
	case core.LinkUp:
		gap := ev.Gap.Round(time.Millisecond)
		if ev.Gap >= time.Second {
			gap = ev.Gap.Round(100 * time.Millisecond)
		}
		on := ""
		if ev.Port != "" {
			on = " on " + ev.Port
		}
		return fmt.Sprintf("── link up after %s%s ──", gap, on)
	}
	return fmt.Sprintf("── link %s ──", ev.State)
}

// reconnectHint is the input row while the link is down.
func (a *App) reconnectHint() string {
	if a.reconnectAttempt > 0 {
		return fmt.Sprintf("Reconnecting… (attempt %d)", a.reconnectAttempt)
	}
	return "Reconnecting…"
}

// toggleReconnect flips reconnecting after a drop and remembers the choice.
func (a *App) toggleReconnect() (Screen, tea.Cmd) {
	a.prefs.Reconnect = !a.prefs.Reconnect
	a.setReconnect(a.prefs.Reconnect)
	return a, a.persistPrefs()
}
