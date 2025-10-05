package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.bug.st/serial/enumerator"
)

type portDetail struct {
	Name         string
	IsUSB        bool
	VID          string
	PID          string
	SerialNumber string
	Product      string
}

// getDeviceName returns a friendly name based on VID:PID by reading from USB IDs database
func getDeviceName(vid, pid string) string {
	// Try to read from the system USB IDs database
	vendorName := lookupVendor(vid)
	productName := lookupProduct(vid, pid)

	if productName != "" {
		return productName
	} else if vendorName != "" {
		return vendorName
	}

	return ""
}

// lookupVendor looks up vendor name from USB IDs database
func lookupVendor(vid string) string {
	// Common locations for USB IDs database on Linux
	paths := []string{
		"/usr/share/hwdata/usb.ids",
		"/usr/share/misc/usb.ids",
		"/var/lib/usbutils/usb.ids",
	}

	for _, path := range paths {
		if vendor := searchUSBIDs(path, vid, ""); vendor != "" {
			return vendor
		}
	}
	return ""
}

// lookupProduct looks up product name from USB IDs database
func lookupProduct(vid, pid string) string {
	paths := []string{
		"/usr/share/hwdata/usb.ids",
		"/usr/share/misc/usb.ids",
		"/var/lib/usbutils/usb.ids",
	}

	for _, path := range paths {
		if product := searchUSBIDs(path, vid, pid); product != "" {
			return product
		}
	}
	return ""
}

// searchUSBIDs searches the USB IDs file for vendor or product
func searchUSBIDs(filepath, vid, pid string) string {
	// This is a simplified implementation
	// In production, you might want to use a proper USB IDs parser library
	// or cache the database in memory for better performance

	// For now, we'll keep a small curated list of common development boards
	// This is more maintainable than a huge hardcoded map
	devices := map[string]map[string]string{
		"2341": { // Arduino
			"":     "Arduino",
			"0043": "Arduino Uno",
			"0001": "Arduino Uno (Rev1)",
			"0243": "Arduino Uno (Rev3)",
			"8036": "Arduino Leonardo",
		},
		"239a": { // Adafruit
			"":     "Adafruit",
			"8014": "Feather M0",
			"80f4": "Feather RP2040",
			"8011": "Feather 32u4",
		},
		"1a86": { // QinHeng Electronics
			"":     "QinHeng Electronics",
			"7523": "CH340 USB-Serial",
		},
		"0403": { // FTDI
			"":     "FTDI",
			"6001": "FT232 USB-Serial",
		},
		"10c4": { // Silicon Labs
			"":     "Silicon Labs",
			"ea60": "CP210x UART Bridge",
		},
		"0483": { // STMicroelectronics
			"":     "STMicroelectronics",
			"374b": "ST-LINK/V2.1",
		},
		"16c0": { // PJRC (Teensy)
			"":     "PJRC",
			"0483": "Teensy USB Serial",
		},
		"cafe": { // PJRC (Teensy)
			"":     "PJRC Teensy",
			"4011": "Teensy 4.1",
		},
	}

	if vendorDevices, ok := devices[vid]; ok {
		if pid == "" {
			// Looking for vendor name only
			return vendorDevices[""]
		}
		// Looking for specific product
		if product, ok := vendorDevices[pid]; ok {
			return product
		}
		// Return vendor name if product not found
		return vendorDevices[""]
	}

	return ""
}

type listPortsModel struct {
	ports  []*portDetail
	width  int
	height int
	err    error
}

func NewListPorts() tea.Model {
	// Use enumerator to get detailed port information
	portDetails, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return &listPortsModel{err: err}
	}

	// Convert to our internal format
	var ports []*portDetail
	for _, p := range portDetails {
		detail := &portDetail{
			Name:         p.Name,
			IsUSB:        p.IsUSB,
			VID:          p.VID,
			PID:          p.PID,
			SerialNumber: p.SerialNumber,
			Product:      p.Product,
		}
		ports = append(ports, detail)
	}

	return &listPortsModel{
		ports: ports,
	}
}

func (m *listPortsModel) Init() tea.Cmd {
	return nil
}

func (m *listPortsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc", "enter":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *listPortsModel) View() string {
	var s strings.Builder

	// Title
	s.WriteString(StyleTitle.Render("📡 Available Serial Ports"))
	s.WriteString("\n\n")

	if m.err != nil {
		s.WriteString(StyleError.Render("✗ Error: " + m.err.Error()))
		s.WriteString("\n\n")
		s.WriteString(StyleHelp.Render("Press q or esc to close"))
		content := s.String()
		return lipgloss.PlaceVertical(m.height, lipgloss.Center,
			lipgloss.PlaceHorizontal(m.width, lipgloss.Center, content))
	}

	if len(m.ports) == 0 {
		s.WriteString(StyleWarning.Render("No serial ports found!"))
		s.WriteString("\n\n")
		s.WriteString(StyleMuted.Render("Make sure your device is connected"))
		s.WriteString("\n\n")
		s.WriteString(StyleHelp.Render("Press q or esc to close"))
		content := s.String()
		return lipgloss.PlaceVertical(m.height, lipgloss.Center,
			lipgloss.PlaceHorizontal(m.width, lipgloss.Center, content))
	}

	// Create a box for the port list
	var portList strings.Builder
	portList.WriteString(StyleHeader.Render("Detected Ports:"))
	portList.WriteString("\n\n")

	for i, port := range m.ports {
		// Port name header
		icon := StyleIcon.Render("▸")
		portName := StyleValue.Render(port.Name)
		portList.WriteString(fmt.Sprintf("  %s %s\n", icon, portName))

		// Show detailed information if it's a USB device
		if port.IsUSB {
			// Try to get a friendly device name
			var deviceName string
			if port.Product != "" {
				deviceName = port.Product
			} else if port.VID != "" && port.PID != "" {
				deviceName = getDeviceName(port.VID, port.PID)
			}

			if deviceName != "" {
				portList.WriteString(fmt.Sprintf("    %s %s\n", StyleKey.Render("Device:"), StyleSuccess.Render(deviceName)))
			}

			if port.VID != "" && port.PID != "" {
				portList.WriteString(fmt.Sprintf("    %s %s\n", StyleKey.Render("VID:PID:"), StyleMuted.Render(port.VID+":"+port.PID)))
			}
			if port.SerialNumber != "" {
				portList.WriteString(fmt.Sprintf("    %s %s\n", StyleKey.Render("Serial:"), StyleMuted.Render(port.SerialNumber)))
			}
		} else {
			portList.WriteString(fmt.Sprintf("    %s\n", StyleMuted.Render("Non-USB device")))
		}

		// Add spacing between ports
		if i < len(m.ports)-1 {
			portList.WriteString("\n")
		}
	}

	portList.WriteString("\n\n")
	portList.WriteString(StyleKey.Render("Total:") + " " + StyleValue.Render(fmt.Sprintf("%d", len(m.ports))))
	portList.WriteString("\n\n")

	// Usage example
	exampleTitle := StyleMuted.Render("Usage Example:")
	exampleCmd := StyleSubheader.Render(fmt.Sprintf("qa-agent capture serial --port %s --baud 115200", m.ports[0].Name))
	portList.WriteString(exampleTitle + "\n")
	portList.WriteString("  " + exampleCmd)

	// Create bordered box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2)

	box := boxStyle.Render(portList.String())
	s.WriteString(box)

	s.WriteString("\n\n")
	s.WriteString(StyleHelp.Render("Press q or esc to close"))

	content := s.String()
	return lipgloss.PlaceVertical(m.height, lipgloss.Center,
		lipgloss.PlaceHorizontal(m.width, lipgloss.Center, content))
}

// RunListPorts launches the list ports view
func RunListPorts() error {
	p := tea.NewProgram(NewListPorts(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
