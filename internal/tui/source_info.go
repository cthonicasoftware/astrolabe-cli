package tui

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

const (
	infoBoxWidth     = 64
	infoPortTestBaud = 9600
)

type sourceInfoModel struct {
	sourceType  string // "serial", "tcp", "scpi"
	width       int
	height      int
	shouldClose bool

	// Serial-specific info
	serialPorts []serialPortInfo

	// TCP-specific info
	networkInterfaces []networkInterfaceInfo

	loadErr error
}

type serialPortInfo struct {
	name        string
	description string
	connected   bool
	inUse       bool
}

type networkInterfaceInfo struct {
	name    string
	address string
	status  string
}

// NewSourceInfo builds the contextual diagnostics dialog for the given source type.
func NewSourceInfo(sourceType string, width, height int) *sourceInfoModel {
	m := &sourceInfoModel{
		sourceType: sourceType,
		width:      width,
		height:     height,
	}

	m.loadData()
	return m
}

func (m *sourceInfoModel) loadData() {
	switch sourceType := m.sourceType; sourceType {
	case SourceTypeSerial:
		m.serialPorts, m.loadErr = collectSerialPorts()
	case SourceTypeTCP:
		m.networkInterfaces, m.loadErr = collectNetworkInterfaces()
	case SourceTypeSCPI:
		// No data yet – reserved for future implementation.
	default:
		m.loadErr = fmt.Errorf("unsupported source type: %s", sourceType)
	}
}

func collectSerialPorts() ([]serialPortInfo, error) {
	portNames, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}

	detailed, _ := enumerator.GetDetailedPortsList() // best effort
	detailByName := map[string]*enumerator.PortDetails{}
	for _, port := range detailed {
		detailByName[port.Name] = port
	}

	var ports []serialPortInfo
	for _, name := range portNames {
		info := serialPortInfo{
			name:      name,
			connected: true,
		}

		if detail := detailByName[name]; detail != nil {
			switch {
			case detail.Product != "":
				info.description = detail.Product
			case detail.VID != "" && detail.PID != "":
				info.description = fmt.Sprintf("VID %s PID %s", strings.ToUpper(detail.VID), strings.ToUpper(detail.PID))
			case detail.IsUSB:
				info.description = "USB Device"
			}
		}

		inUse, statusText := testSerialPort(name)
		info.inUse = inUse

		if info.description == "" {
			info.description = statusText
		} else if statusText != "" {
			info.description = fmt.Sprintf("%s — %s", info.description, statusText)
		}

		ports = append(ports, info)
	}

	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].name < ports[j].name
	})

	return ports, nil
}

func testSerialPort(name string) (bool, string) {
	mode := &serial.Mode{
		BaudRate: infoPortTestBaud,
	}

	port, err := serial.Open(name, mode)
	if err != nil {
		var portErr *serial.PortError
		if errors.As(err, &portErr) {
			switch portErr.Code() {
			case serial.PortBusy:
				return true, "Port busy"
			case serial.PermissionDenied:
				return true, "Permission denied"
			}
		}

		switch {
		case errors.Is(err, syscall.EBUSY):
			return true, "Port busy"
		case errors.Is(err, syscall.EACCES):
			return true, "Permission denied"
		default:
			return true, sanitizeSerialError(err)
		}
	}
	defer port.Close()

	return false, "Available"
}

func sanitizeSerialError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if idx := strings.LastIndex(msg, ": "); idx >= 0 && idx < len(msg)-2 {
		msg = msg[idx+2:]
	}
	return strings.TrimSpace(msg)
}

func collectNetworkInterfaces() ([]networkInterfaceInfo, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var results []networkInterfaceInfo
	for _, iface := range ifaces {
		status := "down"
		if iface.Flags&net.FlagUp != 0 {
			status = "up"
		}

		addrs, err := iface.Addrs()
		if err != nil {
			results = append(results, networkInterfaceInfo{
				name:    iface.Name,
				address: "error retrieving addresses",
				status:  status,
			})
			continue
		}

		if len(addrs) == 0 {
			results = append(results, networkInterfaceInfo{
				name:    iface.Name,
				address: "(no address)",
				status:  status,
			})
			continue
		}

		var addresses []string
		for _, addr := range addrs {
			addresses = append(addresses, addr.String())
		}

		results = append(results, networkInterfaceInfo{
			name:    iface.Name,
			address: strings.Join(addresses, ", "),
			status:  status,
		})
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].name < results[j].name
	})

	return results, nil
}

func (m *sourceInfoModel) Update(msg tea.Msg) (*sourceInfoModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.shouldClose = true
			return m, nil
		case "ctrl+c":
			m.shouldClose = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *sourceInfoModel) View() string {
	var body strings.Builder

	title := m.title()
	body.WriteString(StyleTitle.Render(title))
	body.WriteString("\n\n")

	if m.loadErr != nil {
		body.WriteString(StyleError.Render(fmt.Sprintf("Unable to load diagnostics: %v", m.loadErr)))
		body.WriteString("\n\n")
		body.WriteString(StyleHelp.Render("esc: close"))
		return m.wrap(body.String())
	}

	switch m.sourceType {
	case SourceTypeSerial:
		m.renderSerialInfo(&body)
	case SourceTypeTCP:
		m.renderNetworkInfo(&body)
	case SourceTypeSCPI:
		m.renderSCPIPlaceholder(&body)
	default:
		body.WriteString(StyleWarning.Render("Unsupported source diagnostics"))
		body.WriteString("\n")
	}

	body.WriteString("\n")
	body.WriteString(StyleHelp.Render("esc/q: close"))

	return m.wrap(body.String())
}

func (m *sourceInfoModel) title() string {
	switch m.sourceType {
	case SourceTypeSerial:
		return "Serial Port Information"
	case SourceTypeTCP:
		return "Network Interfaces"
	case SourceTypeSCPI:
		return "SCPI/VISA Info"
	default:
		return "Source Diagnostics"
	}
}

func (m *sourceInfoModel) wrap(content string) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary).
		Padding(1, 2).
		Width(infoBoxWidth)

	box := boxStyle.Render(content)
	return lipgloss.PlaceVertical(m.height, lipgloss.Center,
		lipgloss.PlaceHorizontal(m.width, lipgloss.Center, box))
}

func (m *sourceInfoModel) renderSerialInfo(s *strings.Builder) {
	if len(m.serialPorts) == 0 {
		s.WriteString(StyleWarning.Render("No serial ports detected"))
		s.WriteString("\n")
		return
	}

	for i, port := range m.serialPorts {
		statusIcon := IconAlchemyConnected
		statusStyle := StyleSuccess
		statusText := "Available"

		if port.inUse {
			statusIcon = IconStatusWarning
			statusStyle = StyleWarning
			statusText = "In Use"
		}

		s.WriteString(fmt.Sprintf("%s %s %s\n", statusIcon, StyleValue.Render(port.name), statusStyle.Render(statusText)))
		if port.description != "" {
			s.WriteString("   " + StyleMuted.Render(port.description) + "\n")
		}

		if i < len(m.serialPorts)-1 {
			s.WriteString("\n")
		}
	}
}

func (m *sourceInfoModel) renderNetworkInfo(s *strings.Builder) {
	if len(m.networkInterfaces) == 0 {
		s.WriteString(StyleWarning.Render("No interfaces detected"))
		s.WriteString("\n")
		return
	}

	for i, iface := range m.networkInterfaces {
		icon := IconAlchemyInfo
		statusStyle := StyleMuted

		switch strings.ToLower(iface.status) {
		case "up":
			icon = IconAlchemyActive
			statusStyle = StyleSuccess
		case "down":
			icon = IconStatusWarning
			statusStyle = StyleWarning
		}

		s.WriteString(fmt.Sprintf("%s %s %s\n", icon, StyleValue.Render(iface.name), statusStyle.Render(strings.ToUpper(iface.status))))
		if iface.address != "" {
			s.WriteString("   " + StyleMuted.Render(iface.address) + "\n")
		}

		if i < len(m.networkInterfaces)-1 {
			s.WriteString("\n")
		}
	}
}

func (m *sourceInfoModel) renderSCPIPlaceholder(s *strings.Builder) {
	s.WriteString(StyleHeader.Render("Coming Soon"))
	s.WriteString("\n\n")
	s.WriteString(StyleMuted.Render("Instrument identification via *IDN? query - planned feature."))
	s.WriteString("\n")
	s.WriteString(StyleMuted.Render("Connect oscilloscopes, DMMs, and more via SCPI/VISA."))
	s.WriteString("\n")
	// TODO: Implement SCPI diagnostics when VISA transport support lands.
}

// ShouldClose reports whether the dialog should close.
func (m *sourceInfoModel) ShouldClose() bool {
	return m.shouldClose
}
