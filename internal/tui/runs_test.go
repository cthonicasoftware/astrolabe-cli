package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/cthonicasoftware/astrolabe-cli/internal/runs"
)

func TestBuildRunsTableColumnsFillWidth(t *testing.T) {
	const tableWidth = 120

	cols := buildRunsTableColumns(tableWidth)
	if len(cols) != len(runsTableColumnSpecs) {
		t.Fatalf("column count = %d, want %d", len(cols), len(runsTableColumnSpecs))
	}

	total := len(cols) * runsTableColumnPadding
	for _, col := range cols {
		total += col.Width
	}

	if total != tableWidth {
		t.Fatalf("total rendered width = %d, want %d", total, tableWidth)
	}
}

func TestBuildRunsTableColumnsShrinkToMinimums(t *testing.T) {
	const tableWidth = 80

	cols := buildRunsTableColumns(tableWidth)

	total := len(cols) * runsTableColumnPadding
	for i, col := range cols {
		total += col.Width
		if col.Width < runsTableColumnSpecs[i].minWidth {
			t.Fatalf("column %q width = %d, want >= %d", col.Title, col.Width, runsTableColumnSpecs[i].minWidth)
		}
	}

	if total != tableWidth {
		t.Fatalf("total rendered width = %d, want %d", total, tableWidth)
	}
}

// Bubble Tea v2 drops the bottom rows of a frame taller than the terminal,
// so every runs mode must fit, even for a run with lots of metadata.
func TestRunsViewFitsTerminal(t *testing.T) {
	summary := runs.Summary{
		ID:          "01M6MM281B2F28V2GB6V6TWYYQ",
		RunDir:      `C:\Users\someone\.astrolabe\runs\01M6MM281B2F28V2GB6V6TWYYQ`,
		Started:     time.Date(2026, 11, 4, 20, 9, 40, 0, time.UTC),
		Records:     199,
		PrimaryData: "data.jsonl",
	}
	summary.Manifest.Test.Run = "1"
	summary.Manifest.Operator = "Operator"
	summary.Manifest.Location = "laptop"
	summary.Manifest.Device.ID = "MS-1"
	summary.Manifest.Device.Serial = "123-4567"
	summary.Manifest.Device.Firmware = "1.2.3"
	summary.Manifest.Device.HardwareVersion = "B"
	summary.Manifest.Tags = []string{"basic", "burnin", "smoke"}
	summary.Manifest.Attributes = map[string]string{
		"baud": "115200", "port": "COM3", "remote": "true",
		"source_format": "jsonl", "source_kind": "serial", "tx_records": "0",
	}
	summary.Capture.Notes = "serial capture from COM3 @ 115200 baud"
	summary.Capture.Channels = []string{"serial"}

	for _, size := range [][2]int{{190, 37}, {80, 24}} {
		m := NewRunsViewer(t.TempDir()).(*runsViewModel)
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.Update(runsLoadedMsg{result: runs.Result{Root: t.TempDir(), Runs: []runs.Summary{summary}}})

		steps := []struct {
			name string
			do   func()
		}{
			{"table", func() {}},
			{"detail", func() {
				m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
				m.View() // sizes the detail viewport
				m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
				if m.detailViewport.YOffset() == 0 {
					t.Errorf("%dx%d detail view did not scroll on down", size[0], size[1])
				}
			}},
			{"payload", func() {
				m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
				m.Update(payloadLoadedMsg{original: m.payloadOriginal, content: strings.Repeat("line\n", 200), lines: 200})
			}},
		}
		for _, s := range steps {
			s.do()
			if got := lipgloss.Height(m.View()); got > size[1] {
				t.Errorf("%dx%d %s view is %d rows tall, terminal has %d", size[0], size[1], s.name, got, size[1])
			}
		}
	}
}
