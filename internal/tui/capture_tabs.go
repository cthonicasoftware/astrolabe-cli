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
	tabs       []string
	tabContent []string // Pre-rendered content for each tab
	activeTab  int

	// Focus state
	focusMode     string // FocusModeTabs, FocusModeFields, FocusModeButtons, FocusModeAdvanced
	focusedField  int    // which field in active tab
	focusedButton int    // ButtonIndexConfirm or ButtonIndexReset

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

	// SCPI/VISA configuration (stub for future implementation)
	scpiAddress string
	scpiCursor  int

	// Advanced settings
	showAdvanced  bool
	advancedModel *advancedSettingsModel

	// Source info dialog
	showInfo  bool
	infoModel *sourceInfoModel

	// Buttons
	buttonConfirm string
	buttonCancel  string

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

// Tab indices
const (
	TabIndexSerial = 0
	TabIndexTCP    = 1
	TabIndexSCPI   = 2
	TabCount       = 3
)

// Field indices for Serial tab
const (
	SerialFieldPort  = 0
	SerialFieldBaud  = 1
	SerialFieldCount = 2
)

// Field indices for TCP tab
const (
	TCPFieldHost  = 0
	TCPFieldPort  = 1
	TCPFieldCount = 2
)

// Field indices for SCPI/VISA tab
const (
	SCPIFieldAddress = 0
	SCPIFieldCount   = 1
)

// Button indices
const (
	ButtonIndexConfirm = 0
	ButtonIndexReset   = 1
)

// Focus modes
const (
	FocusModeTabs     = "tabs"
	FocusModeFields   = "fields"
	FocusModeButtons  = "buttons"
	FocusModeAdvanced = "advanced"
	FocusModeInfo     = "info"
)

// Source type identifiers
const (
	SourceTypeSerial = "serial"
	SourceTypeTCP    = "tcp"
	SourceTypeSCPI   = "scpi"
)

// UI text constants
// TODO: Centralize all Help Warn and Placholder strings for app-wide consistency
const (
	HelpSeparator     = "•"
	HelpEnterEditMode = " (Enter to edit)"
	HelpArrowsChange  = " ←/→"
	WarnNoPortsFound  = "No ports found"
	PlaceholderNotSet = "(not set)"
)

// Field formatting
const (
	LabelWidth    = 14
	CursorPadding = "  "
)

func NewCaptureTabs() tea.Model {
	// Get available serial ports
	ports, err := serial.GetPortsList()
	if err != nil || len(ports) == 0 {
		ports = []string{WarnNoPortsFound}
	}
	sort.Strings(ports)

	// Initialize with defaults
	defaultCfg := sources.DefaultConfig()

	m := &captureTabsModel{
		tabs: []string{
			IconMenuCapture + "Serial",
			IconMenuTCP + "TCP",
			IconMenuConnection + "SCPI/VISA",
		},
		tabContent:     make([]string, TabCount),
		activeTab:      TabIndexSerial,
		focusMode:      FocusModeTabs,
		focusedField:   0,
		focusedButton:  ButtonIndexConfirm,
		serialConfig:   defaultCfg,
		availablePorts: ports,
		baudRates:      append([]int(nil), sources.CommonBaudRates...),
		portCursor:     SerialFieldPort,
		baudCursor:     findBaudIndex(sources.CommonBaudRates, defaultCfg.Baud),
		tcpHost:        "localhost",
		tcpPort:        "9000",
		tcpCursor:      TCPFieldHost,
		scpiAddress:    "",
		scpiCursor:     SCPIFieldAddress,
	}

	// Initialize tab content
	m.updateTabContent()

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
	// When the info dialog is visible, route all messages to it first.
	if m.showInfo && m.infoModel != nil {
		if sizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = sizeMsg.Width
			m.height = sizeMsg.Height
		}
		newInfo, cmd := m.infoModel.Update(msg)
		m.infoModel = newInfo
		if m.infoModel.ShouldClose() {
			m.showInfo = false
			m.focusMode = FocusModeFields
			m.infoModel = nil
		}
		return m, cmd
	}

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
			m.focusMode = FocusModeFields
			m.updateTabContent() // Refresh content after applying settings
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.infoModel != nil {
			m.infoModel.width = msg.Width
			m.infoModel.height = msg.Height
		}
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
	if m.activeTab == TabIndexTCP && m.focusMode == FocusModeFields && m.editingField {
		key := msg.String()
		// Allow esc and enter to exit edit mode
		if key != "esc" && key != "enter" {
			return m.handleTextInput(key), nil
		}
		// esc or enter exits edit mode
		m.editingField = false
		m.updateTabContent() // Refresh to remove cursor
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
		if m.activeTab == TabIndexSerial {
			m.showAdvanced = true
			m.advancedModel = NewAdvancedSettings(SourceTypeSerial, m.serialConfig)
			m.advancedModel.width = m.width
			m.advancedModel.height = m.height
			m.focusMode = FocusModeAdvanced
		}
		return m, nil

	case "i", "p":
		sourceType := m.getSourceTypeForActiveTab()
		if sourceType != "" {
			m.showInfo = true
			m.infoModel = NewSourceInfo(sourceType, m.width, m.height)
			m.focusMode = FocusModeInfo
			return m, nil
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
		m.focusMode = FocusModeTabs
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
	}

	return m, nil
}

func (m *captureTabsModel) handleLeft() tea.Model {
	switch m.focusMode {
	case FocusModeTabs:
		m.activeTab = max(m.activeTab-1, 0)
	case FocusModeFields:
		m.handleFieldLeft()
	case FocusModeButtons:
		m.focusedButton = max(m.focusedButton-1, ButtonIndexConfirm)
	}
	return m
}

func (m *captureTabsModel) handleRight() tea.Model {
	switch m.focusMode {
	case FocusModeTabs:
		m.activeTab = min(m.activeTab+1, len(m.tabs)-1)
	case FocusModeFields:
		m.handleFieldRight()
	case FocusModeButtons:
		m.focusedButton = min(m.focusedButton+1, ButtonIndexReset)
	}
	return m
}

func (m *captureTabsModel) handleUp() tea.Model {
	switch m.focusMode {
	case FocusModeTabs:
		// Enter fields mode
		m.focusMode = FocusModeFields
		m.focusedField = 0
	case FocusModeFields:
		m.handleFieldUp()
	case FocusModeButtons:
		// Go back to fields
		m.focusMode = FocusModeFields
		m.focusedField = m.getMaxField()
	}
	return m
}

func (m *captureTabsModel) handleDown() tea.Model {
	switch m.focusMode {
	case FocusModeTabs:
		// Enter fields mode
		m.focusMode = FocusModeFields
		m.focusedField = 0
	case FocusModeFields:
		if m.focusedField < m.getMaxField() {
			m.focusedField++
			m.editingField = false
		} else {
			// Move to buttons
			m.focusMode = FocusModeButtons
			m.focusedButton = ButtonIndexConfirm
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
		m.focusMode = FocusModeTabs
	}
}

func (m *captureTabsModel) handleFieldLeft() {
	switch m.activeTab {
	case TabIndexSerial:
		switch m.focusedField {
		case SerialFieldPort:
			if len(m.availablePorts) > 0 && m.availablePorts[0] != WarnNoPortsFound {
				if m.portCursor == 0 {
					m.portCursor = len(m.availablePorts) - 1
				} else {
					m.portCursor--
				}
				m.updateTabContent()
			}
		case SerialFieldBaud:
			if len(m.baudRates) > 0 {
				if m.baudCursor == 0 {
					m.baudCursor = len(m.baudRates) - 1
				} else {
					m.baudCursor--
				}
				m.updateTabContent()
			}
		}
	}
}

func (m *captureTabsModel) handleFieldRight() {
	switch m.activeTab {
	case TabIndexSerial:
		switch m.focusedField {
		case SerialFieldPort:
			if len(m.availablePorts) > 0 && m.availablePorts[0] != WarnNoPortsFound {
				m.portCursor = (m.portCursor + 1) % len(m.availablePorts)
				m.updateTabContent()
			}
		case SerialFieldBaud:
			if len(m.baudRates) > 0 {
				m.baudCursor = (m.baudCursor + 1) % len(m.baudRates)
				m.updateTabContent()
			}
		}
	}
}

func (m *captureTabsModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.focusMode {
	case FocusModeTabs:
		// Enter fields mode
		m.focusMode = FocusModeFields
		m.focusedField = 0
	case FocusModeFields:
		m.handleFieldSelect()
	case FocusModeButtons:
		if m.focusedButton == ButtonIndexConfirm {
			// Confirm
			m.confirmed = true
			switch m.activeTab {
			case TabIndexSerial:
				m.selectedSource = SourceTypeSerial
				// Set port from selected cursor
				if len(m.availablePorts) > 0 && m.availablePorts[0] != WarnNoPortsFound {
					m.serialConfig.Port = m.availablePorts[m.portCursor]
				}
				m.serialConfig.Baud = m.baudRates[m.baudCursor]
			case TabIndexTCP:
				m.selectedSource = SourceTypeTCP
			case TabIndexSCPI:
				// SCPI/VISA not implemented yet - do nothing
				// User will stay on this tab and can navigate away
				return m, nil
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
	case TabIndexSerial:
		switch m.focusedField {
		case SerialFieldPort:
			if len(m.availablePorts) > 0 && m.availablePorts[0] != WarnNoPortsFound {
				m.portCursor = (m.portCursor + 1) % len(m.availablePorts)
				m.updateTabContent()
			}
		case SerialFieldBaud:
			// Baud uses left/right arrows
		}
	case TabIndexTCP:
		// Toggle editing mode for text fields
		m.editingField = !m.editingField
		// Sync tcpCursor with focusedField
		m.tcpCursor = m.focusedField
	}
}

func (m *captureTabsModel) handleTextInput(key string) tea.Model {
	if m.activeTab != TabIndexTCP {
		return m
	}

	switch key {
	case "backspace":
		if m.tcpCursor == TCPFieldHost && len(m.tcpHost) > 0 {
			m.tcpHost = m.tcpHost[:len(m.tcpHost)-1]
			m.updateTabContent()
		} else if m.tcpCursor == TCPFieldPort && len(m.tcpPort) > 0 {
			m.tcpPort = m.tcpPort[:len(m.tcpPort)-1]
			m.updateTabContent()
		}
	default:
		if len(key) == 1 {
			if m.tcpCursor == TCPFieldHost {
				m.tcpHost += key
				m.updateTabContent()
			} else if m.tcpCursor == TCPFieldPort {
				// Only allow digits for port
				if key >= "0" && key <= "9" {
					m.tcpPort += key
					m.updateTabContent()
				}
			}
		}
	}
	return m
}

func (m *captureTabsModel) getSourceTypeForActiveTab() string {
	switch m.activeTab {
	case TabIndexSerial:
		return SourceTypeSerial
	case TabIndexTCP:
		return SourceTypeTCP
	case TabIndexSCPI:
		return SourceTypeSCPI
	default:
		return ""
	}
}

func (m *captureTabsModel) getMaxField() int {
	switch m.activeTab {
	case TabIndexSerial:
		return SerialFieldCount - 1 // Return max index, not count
	case TabIndexTCP:
		return TCPFieldCount - 1 // Return max index, not count
	case TabIndexSCPI:
		return SCPIFieldCount - 1 // Return max index, not count
	}
	return 0
}

func (m *captureTabsModel) resetCurrentTab() {
	switch m.activeTab {
	case TabIndexSerial:
		defaultCfg := sources.DefaultConfig()
		m.serialConfig = defaultCfg
		m.portCursor = SerialFieldPort
		m.baudCursor = findBaudIndex(m.baudRates, defaultCfg.Baud)
	case TabIndexTCP:
		m.tcpHost = "localhost"
		m.tcpPort = "9000"
	case TabIndexSCPI:
		m.scpiAddress = ""
	}
	m.updateTabContent()
}

// updateTabContent refreshes the content for all tabs based on current state.
// This follows the Single Responsibility Principle by separating content generation from rendering.
func (m *captureTabsModel) updateTabContent() {
	m.tabContent[TabIndexSerial] = m.buildSerialContent()
	m.tabContent[TabIndexTCP] = m.buildTCPContent()
	m.tabContent[TabIndexSCPI] = m.buildSCPIContent()
}

// buildSerialContent generates the serial tab content.
// Separated method for maintainability (Single Responsibility).
func (m *captureTabsModel) buildSerialContent() string {
	var content strings.Builder
	content.WriteString("\n")

	// Port field
	portLabel := "Port:"
	portValue := WarnNoPortsFound
	if len(m.availablePorts) > 0 && m.availablePorts[0] != WarnNoPortsFound {
		portValue = m.availablePorts[m.portCursor]
	}
	m.buildField(&content, SerialFieldPort, portLabel, portValue, HelpArrowsChange)

	// Baud field
	baudLabel := "Baud Rate:"
	baudValue := fmt.Sprintf("%d", m.baudRates[m.baudCursor])
	m.buildField(&content, SerialFieldBaud, baudLabel, baudValue, HelpArrowsChange)

	// Advanced settings hint
	content.WriteString("\n")
	content.WriteString("\n")

	return content.String()
}

// buildTCPContent generates the TCP tab content.
func (m *captureTabsModel) buildTCPContent() string {
	var content strings.Builder
	content.WriteString("\n")

	// Host field
	hostLabel := "Host:"
	hostValue := m.tcpHost
	if m.focusMode == FocusModeFields && m.focusedField == TCPFieldHost && m.editingField {
		hostValue += "_"
	}
	m.buildField(&content, TCPFieldHost, hostLabel, hostValue, HelpEnterEditMode)

	// Port field
	portLabel := "Port:"
	portValue := m.tcpPort
	if m.focusMode == FocusModeFields && m.focusedField == TCPFieldPort && m.editingField {
		portValue += "_"
	}
	m.buildField(&content, TCPFieldPort, portLabel, portValue, HelpEnterEditMode)
	content.WriteString("\n")

	return content.String()
}

// buildSCPIContent generates the SCPI/VISA tab content (stub for future implementation).
func (m *captureTabsModel) buildSCPIContent() string {
	var content strings.Builder
	content.WriteString("\n")

	// Coming soon message
	content.WriteString(StyleHeader.Render("SCPI/VISA Instrument Support"))
	content.WriteString("\n\n")
	content.WriteString(StyleMuted.Render("Coming soon - instrument integration via SCPI/VISA"))
	content.WriteString("\n\n")
	content.WriteString(StyleKey.Render("Planned features:"))
	content.WriteString("\n")
	content.WriteString("  " + StyleIcon.Render(IconStatusInfo) + " DMM (Digital Multimeter)\n")
	content.WriteString("  " + StyleIcon.Render(IconStatusInfo) + " Oscilloscope\n")
	content.WriteString("  " + StyleIcon.Render(IconStatusInfo) + " Signal Generator\n")
	content.WriteString("\n")

	return content.String()
}

// buildField constructs a field line with cursor, label, value, and hint.
// This method provides a consistent field rendering interface (Interface Segregation).
func (m *captureTabsModel) buildField(content *strings.Builder, fieldIndex int, label, value, hint string) {
	isFocused := m.focusMode == FocusModeFields && m.focusedField == fieldIndex

	var cursorStr, labelStr, valueStr, hintStr string

	if isFocused {
		cursorStr = StyleSelected.Render(IconSelectedItem)
		labelStr = StyleSelected.Render(fmt.Sprintf("%-*s", LabelWidth, label))
	} else {
		cursorStr = StyleUnselected.Render(CursorPadding)
		labelStr = StyleUnselected.Render(fmt.Sprintf("%-*s", LabelWidth, label))
	}

	valueStr = StyleValue.Render(value)

	if isFocused && hint != "" {
		hintStr = StyleMuted.Render(hint)
	} else {
		hintStr = ""
	}

	content.WriteString(cursorStr + labelStr + valueStr + hintStr + "\n")
}

func (m *captureTabsModel) View() string {
	if m.showInfo && m.infoModel != nil {
		return m.infoModel.View()
	}

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
	//TODO: Investigate why adding to tabWidth breaks top of border.
	// need to pad with "─" about the row content?
	tabWidth := lipgloss.Width(row) - windowStyle.GetHorizontalFrameSize()

	//Build window
	var windowContent strings.Builder
	windowContent.WriteString("\n")
	m.renderTabContent(&windowContent, tabWidth)
	windowContent.WriteString("\n")

	m.renderButtons(&windowContent, tabWidth)

	window := windowStyle.Width(tabWidth).Render(windowContent.String())

	// combine tabs and window
	var tabbedBox strings.Builder
	tabbedBox.WriteString(row)
	tabbedBox.WriteString("\n")
	tabbedBox.WriteString(window)

	centeredTabbedBox := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, tabbedBox.String())

	var content strings.Builder
	content.WriteString(centeredTabbedBox)
	content.WriteString("\n\n")

	// Help text
	m.renderHelp(&content)

	return lipgloss.PlaceVertical(m.height, lipgloss.Center, content.String())
}

func (m *captureTabsModel) renderTabContent(content *strings.Builder, innerWidth int) {
	switch m.activeTab {
	case 0:
		m.renderSerialTab(content, innerWidth)
	case 1:
		m.renderTCPTab(content, innerWidth)
	case 2:
		m.renderSCPITab(content, innerWidth)
	}
}

func (m *captureTabsModel) renderSerialTab(content *strings.Builder, innerWidth int) {
	// Port selection
	portLabel := "Port:"
	portValue := WarnNoPortsFound
	if len(m.availablePorts) > 0 && m.availablePorts[0] != WarnNoPortsFound {
		portValue = m.availablePorts[m.portCursor]
	}
	m.renderField(content, 0, portLabel, portValue, HelpArrowsChange, innerWidth)

	// Baud selection
	baudLabel := "Baud Rate:"
	baudValue := fmt.Sprintf("%d", m.baudRates[m.baudCursor])
	m.renderField(content, 1, baudLabel, baudValue, HelpArrowsChange, innerWidth)

	content.WriteString("\n")
	s := content.String()
	lipgloss.PlaceHorizontal(innerWidth, lipgloss.Left, s)
}

func (m *captureTabsModel) renderTCPTab(content *strings.Builder, innerWidth int) {
	// Host input
	hostLabel := "Host:"
	hostValue := m.tcpHost
	if m.focusMode == "fields" && m.focusedField == 0 && m.editingField {
		hostValue += "_"
	}
	m.renderField(content, 0, hostLabel, hostValue, HelpEnterEditMode, innerWidth)

	// Port input
	portLabel := "Port:"
	portValue := m.tcpPort
	if m.focusMode == "fields" && m.focusedField == 1 && m.editingField {
		portValue += "_"
	}
	m.renderField(content, 1, portLabel, portValue, HelpEnterEditMode, innerWidth)

	content.WriteString("\n")
}

func (m *captureTabsModel) renderSCPITab(content *strings.Builder, innerWidth int) {
	// SCPI/VISA stub - show coming soon message
	message := StyleHeader.Render("SCPI/VISA Instrument Support")
	content.WriteString(lipgloss.PlaceHorizontal(innerWidth, lipgloss.Center, message))
	content.WriteString("\n\n")

	comingSoon := StyleMuted.Render("Coming soon - instrument integration via SCPI/VISA")
	content.WriteString(lipgloss.PlaceHorizontal(innerWidth, lipgloss.Center, comingSoon))
	content.WriteString("\n\n")

	features := StyleKey.Render("Planned features:")
	content.WriteString(lipgloss.PlaceHorizontal(innerWidth, lipgloss.Left, features))
	content.WriteString("\n")

	items := []string{
		"  " + StyleIcon.Render(IconStatusInfo) + " DMM (Digital Multimeter)",
		"  " + StyleIcon.Render(IconStatusInfo) + " Oscilloscope",
		"  " + StyleIcon.Render(IconStatusInfo) + " Signal Generator",
	}
	for _, item := range items {
		content.WriteString(lipgloss.PlaceHorizontal(innerWidth, lipgloss.Left, item))
		content.WriteString("\n")
	}
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
		cursorStr = StyleCursor.Render(IconSelectedItem)
		labelStr = StyleWarning.UnsetWidth().Render(fmt.Sprintf("%-*s", LabelWidth, label))
	} else {
		cursorStr = CursorPadding
		labelStr = StyleKey.UnsetWidth().Render(fmt.Sprintf("%-*s", LabelWidth, label))
	}

	cursorWidth := lipgloss.Width(cursorStr)
	labelWidth := lipgloss.Width(labelStr)
	remaining := max(innerWidth-cursorWidth-labelWidth, 0)

	valueStr = StyleValue.Render(value)
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

	line := cursorStr + labelStr + valueStr + hintStr
	if innerWidth > 0 {
		content.WriteString(lipgloss.PlaceHorizontal(innerWidth, lipgloss.Left, line))
	} else {
		content.WriteString(line)
	}
	content.WriteString("\n")
}

func (m *captureTabsModel) renderButtons(s *strings.Builder, width int) {
	confirmStyle := StyleUnselected
	resetStyle := StyleUnselected

	if m.focusMode == FocusModeButtons {
		if m.focusedButton == ButtonIndexConfirm {
			confirmStyle = StyleSelected
		} else {
			resetStyle = StyleSelected
		}
	}

	confirmBtn := confirmStyle.Render("[ Confirm ]")
	resetBtn := resetStyle.Render("[ Reset ]")

	buttons := fmt.Sprintf("%s  %s", confirmBtn, resetBtn)
	centered := lipgloss.PlaceHorizontal(width, lipgloss.Center, buttons)
	s.WriteString(centered)
}

func (m *captureTabsModel) renderHelp(s *strings.Builder) {
	var helpText string
	switch m.focusMode {
	case FocusModeTabs:
		helpText = "←/→ or Tab: switch tabs • ↑/↓: enter fields • a: advanced • q: cancel"
	case FocusModeFields:
		helpText = "↑/↓: navigate • Enter: select/edit • ←/→: adjust • i/p: info • a: advanced • q: cancel"
	case FocusModeButtons:
		helpText = "←/→: select button • Enter: confirm • ↑: back to fields • q: cancel"
	case FocusModeInfo:
		helpText = "esc/q: close info • ↑/↓: resume navigation"
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
	case SourceTypeSerial:
		config.SerialConfig = &model.serialConfig
	case SourceTypeTCP:
		config.TCPHost = model.tcpHost
		config.TCPPort = model.tcpPort
	case SourceTypeSCPI:
		config.SCPIAddress = model.scpiAddress
	}

	return config, nil
}

// CaptureConfig holds the configuration for any capture source
type CaptureConfig struct {
	SourceType   string // SourceTypeSerial, SourceTypeTCP, or SourceTypeSCPI
	SerialConfig *sources.Config
	TCPHost      string
	TCPPort      string
	SCPIAddress  string // For future SCPI/VISA implementation
}
