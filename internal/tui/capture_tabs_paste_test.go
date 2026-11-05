package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestTCPHostPasteDropsWhitespace(t *testing.T) {
	m := NewCaptureTabs().(*captureTabsModel)
	m.activeTab = TabIndexTCP
	m.tcpCursor = TCPFieldHost
	m.editingField = true
	m.tcpHost = ""

	m.Update(tea.PasteMsg{Content: " 192.168.1.10\r\n\t"})
	if m.tcpHost != "192.168.1.10" {
		t.Fatalf("tcpHost = %q, want 192.168.1.10", m.tcpHost)
	}
}
