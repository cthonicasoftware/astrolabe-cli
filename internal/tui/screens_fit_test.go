package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/runs"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/cthonicasoftware/astrolabe-cli/internal/upload"
)

// Bubble Tea v2 drops rows (and columns) of a frame larger than the
// terminal, so every screen must fit common terminal sizes.
func TestScreensFitTerminal(t *testing.T) {
	sized := func(s Screen, w, h int) Screen {
		s, _ = s.Update(tea.WindowSizeMsg{Width: w, Height: h})
		return s
	}
	manyPorts := func() []string {
		var p []string
		for i := 1; i <= 12; i++ {
			p = append(p, fmt.Sprintf("COM%d", i))
		}
		return p
	}

	cases := map[string]func(w, h int) string{
		"welcome": func(w, h int) string { return sized(NewWelcome(nil), w, h).View() },
		"welcome+error": func(w, h int) string {
			body := strings.Repeat("Something went wrong talking to the device on COM3.\n", 6)
			return sized(NewWelcome(NewStatusMessage(StatusError, "Capture failed", body)), w, h).View()
		},
		"config": func(w, h int) string { return sized(NewConfigEditor(), w, h).View() },
		"metadata": func(w, h int) string {
			return sized(NewMetadataEditor(&stubMetadataPort{path: "/tmp/meta.json"}, MetadataValues{}, "/tmp/meta.json", nil), w, h).View()
		},
		"capture-serial": func(w, h int) string {
			s := sized(NewCaptureTabs(), w, h)
			s, _ = s.Update(serialPortsLoadedMsg{ports: manyPorts()})
			return s.View()
		},
		"capture-tcp": func(w, h int) string {
			m := sized(NewCaptureTabs(), w, h).(*captureTabsModel)
			m.activeTab = TabIndexTCP
			m.updateTabContent()
			return m.View()
		},
		"capture-advanced": func(w, h int) string {
			m := sized(NewCaptureTabs(), w, h).(*captureTabsModel)
			m.showAdvanced = true
			m.advancedModel = NewAdvancedSettings(SourceTypeSerial, m.serialConfig)
			return m.View()
		},
		"capture-info": func(w, h int) string {
			m := sized(NewCaptureTabs(), w, h).(*captureTabsModel)
			m.showInfo = true
			m.infoModel = NewSourceInfo(SourceTypeSerial, w, h)
			var ports []serialPortInfo
			for _, p := range manyPorts() {
				ports = append(ports, serialPortInfo{name: p, description: "USB Serial Device"})
			}
			var nics []networkInterfaceInfo
			for i := range 6 {
				nics = append(nics, networkInterfaceInfo{name: fmt.Sprintf("eth%d", i), address: "192.168.1.10", status: "up"})
			}
			m.infoModel.Update(sourceInfoLoadedMsg{sourceType: SourceTypeSerial, serialPorts: ports, networkInterfaces: nics})
			return m.View()
		},
		"upload": func(w, h int) string {
			return sized(NewUploadModel(upload.NewMockUploadClient(), []string{"run-1", "run-2"}), w, h).View()
		},
		"upload-failed": func(w, h int) string {
			ids := []string{"run-1", "run-2", "run-3", "run-4", "run-5"}
			s := sized(NewUploadModel(upload.NewMockUploadClient(), ids), w, h)
			for _, id := range ids {
				s, _ = s.Update(uploadedRunMsg{runID: id, err: errors.New("presign failed: 503 service unavailable from the server")})
			}
			return s.View()
		},
		"live": func(w, h int) string {
			app := NewApp("Live Capture", nil)
			app.EnableSend(func(string, sources.LineEnding) (time.Time, error) { return time.Time{}, nil })
			return sized(app, w, h).View()
		},
		"live+reconnect": func(w, h int) string {
			return reconnectLiveApp(w, h, false).View()
		},
		"live+reconnecting": func(w, h int) string {
			return reconnectLiveApp(w, h, true).View()
		},
		"capture-info-error": func(w, h int) string {
			m := NewSourceInfo(SourceTypeSerial, w, h)
			long := strings.Repeat("enumerating /dev/serial/by-id/usb-FTDI_device failed: permission denied\n", 30)
			m.Update(sourceInfoLoadedMsg{sourceType: SourceTypeSerial, err: errors.New(long)})
			return m.View()
		},
		"help-over-live": func(w, h int) string {
			app := NewApp("Live Capture", nil)
			app.EnableSend(func(string, sources.LineEnding) (time.Time, error) { return time.Time{}, nil })
			return renderHelpDialog(sized(app, w, h), w, h)
		},
	}

	for _, size := range [][2]int{{80, 24}, {120, 30}} {
		w, h := size[0], size[1]
		for name, render := range cases {
			view := render(w, h)
			if got := lipgloss.Height(view); got > h {
				t.Errorf("%dx%d %s: %d rows tall", w, h, name, got)
			}
			if got := lipgloss.Width(view); got > w {
				t.Errorf("%dx%d %s: %d columns wide", w, h, name, got)
			}
		}
	}
}

// reconnectLiveApp is a sendable live view with reconnect enabled and the
// clock showing, optionally with the link down. Its status bar may wrap at
// 80 columns; the frame must still fit.
func reconnectLiveApp(w, h int, reconnecting bool) *App {
	app := NewApp("Live Capture", nil)
	app.EnableSend(func(string, sources.LineEnding) (time.Time, error) { return time.Time{}, nil })
	app.EnableReconnect(func(bool) {})
	app.Update(tea.WindowSizeMsg{Width: w, Height: h})
	app.Update(tickMsg(time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)))
	if reconnecting {
		app.Update(lineMsg(FeedChunk{At: time.Now(), Link: &core.LinkEvent{State: core.LinkDown, Err: "EOF"}}))
	}
	return app
}

func manyRuns() runs.Result {
	res := runs.Result{Root: "/tmp/cache"}
	for i := range 30 {
		res.Runs = append(res.Runs, runs.Summary{ID: fmt.Sprintf("run-%02d", i), RunDir: fmt.Sprintf("/tmp/cache/run-%02d", i)})
	}
	for i := range 3 {
		res.Warnings = append(res.Warnings, runs.Warning{RunDir: fmt.Sprintf("bad-%d", i), Reason: "manifest unreadable"})
	}
	return res
}

// Short terminals: the status box and runs table must shrink, not push
// the frame past the bottom edge.
func TestScreensFitShortTerminal(t *testing.T) {
	body := strings.Repeat("Something went wrong talking to the device on COM3.\n", 6)
	for h := 8; h <= 24; h++ {
		plain, _ := NewWelcome(nil).Update(tea.WindowSizeMsg{Width: 80, Height: h})
		// Below the menu's own height nothing can fit; only the status
		// must not add rows on top of it.
		limit := max(h, lipgloss.Height(plain.View()))
		s, _ := NewWelcome(NewStatusMessage(StatusError, "Capture failed", body)).Update(tea.WindowSizeMsg{Width: 80, Height: h})
		if got := lipgloss.Height(s.View()); got > limit {
			t.Errorf("welcome+error at 80x%d: %d rows (limit %d)", h, got, limit)
		}
	}
	// The runs table needs 84 columns at its narrowest, so test at 100.
	// Below 18 rows the title, root, help and a 3-row table can't all fit.
	for h := 18; h <= 40; h++ {
		s, _ := NewRunsViewer("").Update(tea.WindowSizeMsg{Width: 100, Height: h})
		s, _ = s.Update(runsLoadedMsg{result: manyRuns()})
		if got := lipgloss.Height(s.View()); got > h {
			t.Errorf("runs at 100x%d: %d rows", h, got)
		}
	}
}

func TestStatusRenderMaxHonoursSmallHeights(t *testing.T) {
	msg := NewStatusMessage(StatusError, "Capture failed", "line one\nline two\nline three\nline four")
	for maxH := 1; maxH <= 8; maxH++ {
		if got := lipgloss.Height(msg.RenderMax(60, maxH)); got > maxH {
			t.Errorf("RenderMax(60, %d) = %d rows", maxH, got)
		}
	}
}
