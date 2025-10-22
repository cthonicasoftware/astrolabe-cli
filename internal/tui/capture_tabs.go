package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.bug.st/serial"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/sources"
)

type captureTabsModel struct {
	// Tab state
	tabs      []string
	activeTab int

	// Focus state
	focusMode     string // "tabs", "fields", "buttons", "advanced"
	focusedField  int    // which field in active tab
	focusedButton int    // 0=Confirm, 1=Reset

	// Serial configuration
	serialConfig   sources.Config
	availablePorts []string
	baudRates      []int
	portCursor     int
	baudCursor     int

	// TCP configuration
	tcpHost      string
	tcpPort      string
	tcpCursor    int // 0=host, 1=port
	editingField bool

	// File configuration
	filePath   string
	fileCursor int

	// Advanced settings
	showAdvanced  bool
	advancedModel *advancedSettingsModel

	// Result
	confirmed      bool
	selectedSource string
	width          int
	height         int
}

func tabBorderWithBottom(left, middle, right string) lipgloss.Border {
	border := lipgloss.RoundedBorder()
	border.BottomLeft = left
	border.Bottom = middle
	border.BottomRight = right
	return border
}

var (
	inactiveTabBorder = tabBorderWithBottom("┴", "─", "┴")
	activeTabBorder   = tabBorderWithBottom("┘", " ", "└")
	inactiveTabStyle  = lipgloss.NewStyle().Border(inactiveTabBorder, true).BorderForeground(ColorPrimary).Padding(0, 1)
	//DO NOT CHANGE THE COLOR. IT LOOKS BAD!!!
	activeTabStyle = inactiveTabStyle.Border(activeTabBorder, true)
	windowStyle    = lipgloss.NewStyle().BorderForeground(ColorPrimary).Padding(2, 0).Align(lipgloss.Center).Border(lipgloss.NormalBorder()).UnsetBorderTop()
)

func NewCaptureTabs() tea.Model {
	// Get available serial ports
	ports, err := serial.GetPortsList()
	if err != nil || len(ports) == 0 {
		ports = []string{"No ports found"}
	}
	sort.Strings(ports)

	// Initialize with defaults
	defaultCfg := sources.DefaultConfig()

	m := &captureTabsModel{
		tabs: []string{
			IconMenuCapture + "Serial",
			IconMenuTCP + "TCP",
			IconMenuNewFile + "File",
		},
		activeTab:      0,
		focusMode:      "tabs",
		focusedField:   0,
		focusedButton:  0,
		serialConfig:   defaultCfg,
		availablePorts: ports,
		baudRates:      append([]int(nil), sources.CommonBaudRates...),
		portCursor:     0,
		baudCursor:     findBaudIndex(sources.CommonBaudRates, defaultCfg.Baud),
		tcpHost:        "localhost",
		tcpPort:        "9000",
		tcpCursor:      0,
		filePath:       "",
		fileCursor:     0,
	}

	return m
}

func findBaudIndex(baudRates []int, targetBaud int) int {
	for i, baud := range baudRates {
		if baud == targetBaud {
			return i
		}
	}
	return 0
}

func (m *captureTabsModel) Init() tea.Cmd {
	return nil
}

func (m *captureTabsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If advanced settings is showing, handle it first
	if m.showAdvanced && m.advancedModel != nil {
		newAdvanced, cmd := m.advancedModel.Update(msg)
		m.advancedModel = newAdvanced

		// Check if advanced settings wants to close
		if m.advancedModel.ShouldClose() {
			if m.advancedModel.WasApplied() {
				// Apply settings
				m.advancedModel.ApplyToConfig(&m.serialConfig)
			}
			// Close the dialog (whether applied or canceled)
			m.showAdvanced = false
			m.focusMode = "fields"
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.advancedModel != nil {
			m.advancedModel.width = msg.Width
			m.advancedModel.height = msg.Height
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	return m, nil
}

func (m *captureTabsModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle text input first when in editing mode
	if m.activeTab == 1 && m.focusMode == "fields" && m.editingField {
		key := msg.String()
		// Allow esc and enter to exit edit mode
		if key != "esc" && key != "enter" {
			return m.handleTextInput(key), nil
		}
		// esc or enter exits edit mode
		m.editingField = false
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "q", "esc":
		// Cancel and quit
		m.confirmed = false
		return m, tea.Quit

	case "a":
		// Open advanced settings for current tab
		if m.activeTab == 0 { // Serial tab
			m.showAdvanced = true
			m.advancedModel = NewAdvancedSettings("serial", m.serialConfig)
			m.advancedModel.width = m.width
			m.advancedModel.height = m.height
			m.focusMode = "advanced"
		}
		return m, nil

	case "tab", "shift+tab":
		// Navigate tabs
		if msg.String() == "tab" {
			m.activeTab = min(m.activeTab+1, len(m.tabs)-1)
		} else {
			m.activeTab = max(m.activeTab-1, 0)
		}
		// Reset field focus when switching tabs
		m.focusMode = "tabs"
		m.focusedField = 0
		return m, nil

	case "left", "h":
		return m.handleLeft(), nil

	case "right", "l":
		return m.handleRight(), nil

	case "up", "k":
		return m.handleUp(), nil

	case "down", "j":
		return m.handleDown(), nil

	case "enter", " ":
		return m.handleEnter()

	default:
		// Handle text input for TCP fields
		if m.activeTab == 1 && m.focusMode == "fields" && m.editingField {
			return m.handleTextInput(msg.String()), nil
		}
	}

	return m, nil
}

func (m *captureTabsModel) handleLeft() tea.Model {
	switch m.focusMode {
	case "tabs":
		m.activeTab = max(m.activeTab-1, 0)
	case "fields":
		m.handleFieldLeft()
	case "buttons":
		m.focusedButton = max(m.focusedButton-1, 0)
	}
	return m
}

func (m *captureTabsModel) handleRight() tea.Model {
	switch m.focusMode {
	case "tabs":
		m.activeTab = min(m.activeTab+1, len(m.tabs)-1)
	case "fields":
		m.handleFieldRight()
	case "buttons":
		m.focusedButton = min(m.focusedButton+1, 1)
	}
	return m
}

func (m *captureTabsModel) handleUp() tea.Model {
	switch m.focusMode {
	case "tabs":
		// Enter fields mode
		m.focusMode = "fields"
		m.focusedField = 0
	case "fields":
		m.handleFieldUp()
	case "buttons":
		// Go back to fields
		m.focusMode = "fields"
		m.focusedField = m.getMaxField()
	}
	return m
}

func (m *captureTabsModel) handleDown() tea.Model {
	switch m.focusMode {
	case "tabs":
		// Enter fields mode
		m.focusMode = "fields"
		m.focusedField = 0
	case "fields":
		if m.focusedField < m.getMaxField() {
			m.focusedField++
			m.editingField = false
		} else {
			// Move to buttons
			m.focusMode = "buttons"
			m.focusedButton = 0
		}
	}
	return m
}

func (m *captureTabsModel) handleFieldUp() {
	if m.focusedField > 0 {
		m.focusedField--
		m.editingField = false
	} else {
		// Go back to tabs
		m.focusMode = "tabs"
	}
}

func (m *captureTabsModel) handleFieldLeft() {
	switch m.activeTab {
	case 0: // Serial tab
		switch m.focusedField {
		case 0: // Port
			if len(m.availablePorts) > 0 && m.availablePorts[0] != "No ports found" {
				if m.portCursor == 0 {
					m.portCursor = len(m.availablePorts) - 1
				} else {
					m.portCursor--
				}
			}
		case 1: // Baud
			if len(m.baudRates) > 0 {
				if m.baudCursor == 0 {
					m.baudCursor = len(m.baudRates) - 1
				} else {
					m.baudCursor--
				}
			}
		}
	}
}

func (m *captureTabsModel) handleFieldRight() {
	switch m.activeTab {
	case 0: // Serial tab
		switch m.focusedField {
		case 0: // Port
			if len(m.availablePorts) > 0 && m.availablePorts[0] != "No ports found" {
				m.portCursor = (m.portCursor + 1) % len(m.availablePorts)
			}
		case 1: // Baud
			if len(m.baudRates) > 0 {
				m.baudCursor = (m.baudCursor + 1) % len(m.baudRates)
			}
		}
	}
}

func (m *captureTabsModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.focusMode {
	case "tabs":
		// Enter fields mode
		m.focusMode = "fields"
		m.focusedField = 0
	case "fields":
		m.handleFieldSelect()
	case "buttons":
		if m.focusedButton == 0 {
			// Confirm
			m.confirmed = true
			switch m.activeTab {
			case 0:
				m.selectedSource = "serial"
				// Set port from selected cursor
				if len(m.availablePorts) > 0 && m.availablePorts[0] != "No ports found" {
					m.serialConfig.Port = m.availablePorts[m.portCursor]
				}
				m.serialConfig.Baud = m.baudRates[m.baudCursor]
			case 1:
				m.selectedSource = "tcp"
			case 2:
				m.selectedSource = "file"
			}
			return m, tea.Quit
		} else {
			// Reset
			m.resetCurrentTab()
			return m, nil
		}
	}
	return m, nil
}

func (m *captureTabsModel) handleFieldSelect() {
	switch m.activeTab {
	case 0: // Serial
		switch m.focusedField {
		case 0: // Port - cycle through
			if len(m.availablePorts) > 0 && m.availablePorts[0] != "No ports found" {
				m.portCursor = (m.portCursor + 1) % len(m.availablePorts)
			}
		case 1: // Baud uses left/right arrows
		}
	case 1: // TCP
		// Toggle editing mode for text fields
		m.editingField = !m.editingField
	}
}

func (m *captureTabsModel) handleTextInput(key string) tea.Model {
	if m.activeTab != 1 {
		return m
	}

	switch key {
	case "backspace":
		if m.tcpCursor == 0 && len(m.tcpHost) > 0 {
			m.tcpHost = m.tcpHost[:len(m.tcpHost)-1]
		} else if m.tcpCursor == 1 && len(m.tcpPort) > 0 {
			m.tcpPort = m.tcpPort[:len(m.tcpPort)-1]
		}
	default:
		if len(key) == 1 {
			if m.tcpCursor == 0 {
				m.tcpHost += key
			} else if m.tcpCursor == 1 {
				// Only allow digits for port
				if key >= "0" && key <= "9" {
					m.tcpPort += key
				}
			}
		}
	}
	return m
}

func (m *captureTabsModel) getMaxField() int {
	switch m.activeTab {
	case 0: // Serial: Port, Baud
		return 1
	case 1: // TCP: Host, Port
		return 1
	case 2: // File: Path
		return 0
	}
	return 0
}

func (m *captureTabsModel) resetCurrentTab() {
	switch m.activeTab {
	case 0: // Serial
		defaultCfg := sources.DefaultConfig()
		m.serialConfig = defaultCfg
		m.portCursor = 0
		m.baudCursor = findBaudIndex(m.baudRates, defaultCfg.Baud)
	case 1: // TCP
		m.tcpHost = "localhost"
		m.tcpPort = "9000"
	case 2: // File
		m.filePath = ""
	}
}

func (m *captureTabsModel) View() string {
	// If advanced settings is showing, render it as overlay
	if m.showAdvanced && m.advancedModel != nil {
		return m.advancedModel.View()
	}

	// Build tabs
	var renderedTabs []string
	for i, t := range m.tabs {
		var style lipgloss.Style
		isFirst, isLast, isActive := i == 0, i == len(m.tabs)-1, i == m.activeTab
		if isActive {
			style = activeTabStyle
		} else {
			style = inactiveTabStyle
		}
		border, _, _, _, _ := style.GetBorder()
		if isFirst && isActive {
			border.BottomLeft = "│"
		} else if isFirst && !isActive {
			border.BottomLeft = "├"
		} else if isLast && isActive {
			border.BottomRight = "│"
		} else if isLast && !isActive {
			border.BottomRight = "┤"
		}
		style = style.Border(border)
		renderedTabs = append(renderedTabs, style.Render(t))
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	innerWidth := lipgloss.Width(row) - windowStyle.GetHorizontalFrameSize()
	if innerWidth < 0 {
		innerWidth = 0
	}

	// Build content window with configuration UI
	var windowContent strings.Builder
	windowContent.WriteString("\n")
	m.renderTabContent(&windowContent, innerWidth)
	windowContent.WriteString("\n")
	window := windowStyle.Width(innerWidth).Render(windowContent.String())

	// Combine tabs and window
	var tabbedBox strings.Builder
	tabbedBox.WriteString(row)
	tabbedBox.WriteString("\n")
	tabbedBox.WriteString(window)

	// Center the tabbed box horizontally
	centeredTabbedBox := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, tabbedBox.String())

	// Build final content
	var s strings.Builder
	s.WriteString(centeredTabbedBox)
	s.WriteString("\n")

	// Buttons
	m.renderButtons(&s)
	s.WriteString("\n\n")

	// Help text
	m.renderHelp(&s)

	// Center everything vertically
	content := s.String()
	return lipgloss.PlaceVertical(m.height, lipgloss.Center, content)
}

func (m *captureTabsModel) renderTabContent(content *strings.Builder, innerWidth int) {
	switch m.activeTab {
	case 0:
		m.renderSerialTab(content, innerWidth)
	case 1:
		m.renderTCPTab(content, innerWidth)
	case 2:
		m.renderFileTab(content, innerWidth)
	}
}

func (m *captureTabsModel) renderSerialTab(content *strings.Builder, innerWidth int) {
	// Port selection
	portLabel := "Port:"
	portValue := "No ports found"
	if len(m.availablePorts) > 0 && m.availablePorts[0] != "No ports found" {
		portValue = m.availablePorts[m.portCursor]
	}
	m.renderField(content, 0, portLabel, portValue, " (←/→ to change)", innerWidth)

	// Baud selection
	baudLabel := "Baud Rate:"
	baudValue := fmt.Sprintf("%d", m.baudRates[m.baudCursor])
	m.renderField(content, 1, baudLabel, baudValue, " (←/→ to change)", innerWidth)

	// Advanced settings hint
	content.WriteString("\n")
	hint := "  Press 'a' for advanced settings"
	hint = fitStringToWidth(hint, innerWidth)
	content.WriteString(StyleMuted.Render(hint))
	content.WriteString("\n")
}

func (m *captureTabsModel) renderTCPTab(content *strings.Builder, innerWidth int) {
	// Host input
	hostValue := m.tcpHost
	if m.focusMode == "fields" && m.focusedField == 0 && m.editingField {
		hostValue += "_"
	}
	m.renderField(content, 0, "Host:", hostValue, " (Enter to edit)", innerWidth)

	// Port input
	portValue := m.tcpPort
	if m.focusMode == "fields" && m.focusedField == 1 && m.editingField {
		portValue += "_"
	}
	m.renderField(content, 1, "Port:", portValue, " (Enter to edit)", innerWidth)
	content.WriteString("\n")
}

func (m *captureTabsModel) renderFileTab(content *strings.Builder, innerWidth int) {
	// File path input
	pathValue := m.filePath
	if pathValue == "" {
		pathValue = "(not set)"
	}
	if m.focusMode == "fields" && m.focusedField == 0 && m.editingField {
		pathValue += "_"
	}
	m.renderField(content, 0, "File Path:", pathValue, " (Enter to edit)", innerWidth)
	content.WriteString("\n")
}

func fitStringToWidth(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxWidth {
		return text
	}
	if maxWidth <= 3 {
		return strings.Repeat(".", maxWidth)
	}
	return string(runes[:maxWidth-3]) + "..."
}

func (m *captureTabsModel) renderField(content *strings.Builder, fieldIndex int, label, value, hint string, innerWidth int) {
	isFocused := m.focusMode == "fields" && m.focusedField == fieldIndex

	var cursorStr, labelStr, valueStr, hintStr string

	if isFocused {
		cursorStr = StyleCursor.Render("❯ ")
		labelStr = StyleWarning.Render(fmt.Sprintf("%-12s", label))
	} else {
		cursorStr = "  "
		labelStr = StyleKey.Render(fmt.Sprintf("%-12s", label))
	}

	cursorWidth := lipgloss.Width(cursorStr)
	labelWidth := lipgloss.Width(labelStr)
	remaining := innerWidth - cursorWidth - labelWidth
	if remaining < 0 {
		remaining = 0
	}

	valueText := fitStringToWidth(value, remaining)
	valueStr = StyleValue.Render(valueText)
	valueWidth := lipgloss.Width(valueStr)
	remaining -= valueWidth
	if remaining < 0 {
		remaining = 0
	}

	if isFocused && hint != "" && remaining > 0 {
		hintText := fitStringToWidth(hint, remaining)
		hintStr = StyleMuted.Render(hintText)
	} else {
		hintStr = ""
	}

	content.WriteString(cursorStr + labelStr + valueStr + hintStr + "\n")
}

func (m *captureTabsModel) renderButtons(s *strings.Builder) {
	confirmStyle := StyleUnselected
	resetStyle := StyleUnselected

	if m.focusMode == "buttons" {
		if m.focusedButton == 0 {
			confirmStyle = StyleSelected
		} else {
			resetStyle = StyleSelected
		}
	}

	confirmBtn := confirmStyle.Render("[ Confirm ]")
	resetBtn := resetStyle.Render("[ Reset ]")

	buttons := fmt.Sprintf("       %s  %s", confirmBtn, resetBtn)
	centered := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, buttons)
	s.WriteString(centered)
}

func (m *captureTabsModel) renderHelp(s *strings.Builder) {
	var helpText string
	switch m.focusMode {
	case "tabs":
		helpText = "←/→ or Tab: switch tabs • ↑/↓: enter fields • a: advanced • q: cancel"
	case "fields":
		helpText = "↑/↓: navigate fields • Enter: select/edit • ←/→: adjust options • a: advanced • q: cancel"
	case "buttons":
		helpText = "←/→: select button • Enter: confirm • ↑: back to fields • q: cancel"
	default:
		helpText = "Tab: switch tabs • ↑/↓: navigate • Enter: select • a: advanced • q: cancel"
	}

	help := StyleHelp.Render(helpText)
	s.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, help))
}

// RunCaptureTabs launches the capture source configuration interface
func RunCaptureTabs() (*CaptureConfig, error) {
	p := tea.NewProgram(NewCaptureTabs(), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	model, ok := finalModel.(*captureTabsModel)
	if !ok || !model.confirmed {
		return nil, nil
	}

	// Build configuration based on selected source
	config := &CaptureConfig{
		SourceType: model.selectedSource,
	}

	switch model.selectedSource {
	case "serial":
		config.SerialConfig = &model.serialConfig
	case "tcp":
		config.TCPHost = model.tcpHost
		config.TCPPort = model.tcpPort
	case "file":
		config.FilePath = model.filePath
	}

	return config, nil
}

// CaptureConfig holds the configuration for any capture source
type CaptureConfig struct {
	SourceType   string // "serial", "tcp", "file"
	SerialConfig *sources.Config
	TCPHost      string
	TCPPort      string
	FilePath     string
}
