package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
)

const (
	settingParity      = 0
	settingDatabits    = 1
	settingStopbits    = 2
	settingFlowcontrol = 3
)

type advancedSettingsModel struct {
	sourceType string // "serial", "tcp", "file"
	focusMode  string // "fields" or "buttons"

	// Field/button focus
	focusedField  int // which setting field is focused
	focusedButton int // 0=Apply, 1=Cancel

	// Serial advanced settings - indices into sources package slices
	parityIndex      int
	dataBitsIndex    int
	stopBitsIndex    int
	flowControlIndex int

	// Result
	applied     bool
	shouldClose bool // true when user wants to close (esc, cancel, or apply)
	width       int
	height      int
}

// Helper functions for finding indices in sources package slices
func optionIndex(options []sources.SerialOption, code string) int {
	for i, opt := range options {
		if opt.Code == code {
			return i
		}
	}
	return -1
}

func intIndex(options []int, value int) int {
	for i, opt := range options {
		if opt == value {
			return i
		}
	}
	return -1
}

func NewAdvancedSettings(sourceType string, config sources.Config) *advancedSettingsModel {
	m := &advancedSettingsModel{
		sourceType:    sourceType,
		focusMode:     "fields",
		focusedField:  0,
		focusedButton: 0,
	}

	// Set indices based on current config (reusing sources package data)
	if sourceType == "serial" {
		m.parityIndex = max(optionIndex(sources.ParityOptions, config.Parity), 0)
		m.dataBitsIndex = max(intIndex(sources.DataBitsOptions, config.DataBits), 0)
		m.stopBitsIndex = max(optionIndex(sources.StopBitsOptions, config.StopBits), 0)
		m.flowControlIndex = max(optionIndex(sources.FlowControlOptions, config.FlowControl), 0)
	}

	return m
}

func (m *advancedSettingsModel) Update(msg tea.Msg) (*advancedSettingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			// Cancel without applying
			m.applied = false
			m.shouldClose = true
			return m, nil

		case "up", "k":
			switch m.focusMode {
			case "fields":
				if m.focusedField > 0 {
					m.focusedField--
				} else {
					// Wrap to buttons
					m.focusMode = "buttons"
					m.focusedButton = 0
				}
			case "buttons":
				// Switch back to fields (last field)
				m.focusMode = "fields"
				m.focusedField = 3 // Last field
			}

		case "down", "j":
			if m.focusMode == "fields" {
				maxField := 3 // 0=parity, 1=databits, 2=stopbits, 3=flowcontrol
				if m.focusedField < maxField {
					m.focusedField++
				} else {
					// Move to buttons
					m.focusMode = "buttons"
					m.focusedButton = 0
				}
			} else if m.focusMode == "buttons" {
				// Wrap to first field
				m.focusMode = "fields"
				m.focusedField = 0
			}

		case "left", "h":
			switch m.focusMode {
			case "buttons":
				// Wrap button navigation
				if m.focusedButton > 0 {
					m.focusedButton--
				} else {
					m.focusedButton = 1
				}
			case "fields":
				// Cycle option left
				m.cycleOption(-1)
			}

		case "right", "l", " ":
			switch m.focusMode {
			case "buttons":
				// Wrap button navigation
				if m.focusedButton < 1 {
					m.focusedButton++
				} else {
					m.focusedButton = 0
				}
			case "fields":
				// Cycle option right
				m.cycleOption(1)
			}

		case "enter":
			switch m.focusMode {
			case "buttons":
				if m.focusedButton == 0 {
					// Apply
					m.applied = true
					m.shouldClose = true
					return m, nil
				} else {
					// Cancel
					m.applied = false
					m.shouldClose = true
					return m, nil
				}
			case "fields":
				// Cycle option right
				m.cycleOption(1)
			}
		}
	}

	return m, nil
}

func (m *advancedSettingsModel) cycleOption(direction int) {
	switch m.focusedField {
	case settingParity:
		if direction > 0 {
			m.parityIndex = (m.parityIndex + 1) % len(sources.ParityOptions)
		} else {
			if m.parityIndex > 0 {
				m.parityIndex--
			} else {
				m.parityIndex = len(sources.ParityOptions) - 1
			}
		}
	case settingDatabits:
		if direction > 0 {
			m.dataBitsIndex = (m.dataBitsIndex + 1) % len(sources.DataBitsOptions)
		} else {
			if m.dataBitsIndex > 0 {
				m.dataBitsIndex--
			} else {
				m.dataBitsIndex = len(sources.DataBitsOptions) - 1
			}
		}
	case settingStopbits:
		if direction > 0 {
			m.stopBitsIndex = (m.stopBitsIndex + 1) % len(sources.StopBitsOptions)
		} else {
			if m.stopBitsIndex > 0 {
				m.stopBitsIndex--
			} else {
				m.stopBitsIndex = len(sources.StopBitsOptions) - 1
			}
		}
	case settingFlowcontrol:
		if direction > 0 {
			m.flowControlIndex = (m.flowControlIndex + 1) % len(sources.FlowControlOptions)
		} else {
			if m.flowControlIndex > 0 {
				m.flowControlIndex--
			} else {
				m.flowControlIndex = len(sources.FlowControlOptions) - 1
			}
		}
	}
}

func (m *advancedSettingsModel) View() string {
	var content strings.Builder

	// Title
	var title string
	switch m.sourceType {
	case "serial":
		title = "Advanced Serial Settings"
	case "tcp":
		title = "Advanced TCP Settings"
	case "file":
		title = "Advanced File Settings"
	default:
		title = "Advanced Settings"
	}

	content.WriteString(StyleTitle.Render(title))
	content.WriteString("\n\n")

	// Settings based on source type
	if m.sourceType == "serial" {
		m.renderSerialSettings(&content)
	}
	// TODO: Add TCP and File advanced settings when needed

	content.WriteString("\n")

	// Buttons
	m.renderButtons(&content)

	// Create bordered box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary).
		Padding(1, 2).
		Width(50)

	box := boxStyle.Render(content.String())

	// Help text
	var help strings.Builder
	help.WriteString("\n\n")
	help.WriteString(StyleHelp.Render(helpNavigateChange))

	// Combine box and help
	var s strings.Builder
	s.WriteString(box)
	s.WriteString(help.String())

	// Center everything
	centered := s.String()
	return lipgloss.PlaceVertical(m.height, lipgloss.Center,
		lipgloss.PlaceHorizontal(m.width, lipgloss.Center, centered))
}

func (m *advancedSettingsModel) renderSerialSettings(content *strings.Builder) {
	// Use sources package data directly
	settings := []struct {
		label string
		value string
	}{
		{"Parity:", sources.ParityOptions[m.parityIndex].Label},
		{"Data Bits:", fmt.Sprintf("%d", sources.DataBitsOptions[m.dataBitsIndex])},
		{"Stop Bits:", sources.StopBitsOptions[m.stopBitsIndex].Label},
		{"Flow Control:", sources.FlowControlOptions[m.flowControlIndex].Label},
	}

	for i, setting := range settings {
		isFocused := m.focusMode == "fields" && m.focusedField == i

		var cursorStr, labelStr, valueStr, indicatorStr string

		if isFocused {
			cursorStr = StyleCursor.Render(IconSelectedItem)
			labelStr = StyleWarning.UnsetWidth().Render(fmt.Sprintf("%-14s", setting.label))
			valueStr = StyleValue.Render(setting.value)
			indicatorStr = StyleMuted.Render(" ←/→")
		} else {
			cursorStr = "  "
			labelStr = StyleKey.UnsetWidth().Render(fmt.Sprintf("%-14s", setting.label))
			valueStr = StyleValue.Render(setting.value)
			indicatorStr = ""
		}

		content.WriteString(cursorStr + labelStr + valueStr + indicatorStr + "\n")
	}
}

func (m *advancedSettingsModel) renderButtons(content *strings.Builder) {
	applyStyle := StyleUnselected
	cancelStyle := StyleUnselected

	if m.focusMode == "buttons" {
		if m.focusedButton == 0 {
			applyStyle = StyleSelected
		} else {
			cancelStyle = StyleSelected
		}
	}

	applyBtn := applyStyle.Render("[ Apply ]")
	cancelBtn := cancelStyle.Render("[ Cancel ]")

	buttons := fmt.Sprintf("       %s  %s", applyBtn, cancelBtn)
	content.WriteString(buttons)
}

// ApplyToConfig applies the advanced settings to a config using sources package data
func (m *advancedSettingsModel) ApplyToConfig(config *sources.Config) {
	if m.sourceType == "serial" {
		config.Parity = sources.ParityOptions[m.parityIndex].Code
		config.DataBits = sources.DataBitsOptions[m.dataBitsIndex]
		config.StopBits = sources.StopBitsOptions[m.stopBitsIndex].Code
		config.FlowControl = sources.FlowControlOptions[m.flowControlIndex].Code
	}
}

// WasApplied returns true if the user clicked Apply
func (m *advancedSettingsModel) WasApplied() bool {
	return m.applied
}

// ShouldClose returns true if the dialog wants to close
func (m *advancedSettingsModel) ShouldClose() bool {
	return m.shouldClose
}

