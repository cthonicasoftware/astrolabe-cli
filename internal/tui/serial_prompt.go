package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.bug.st/serial"
)

// SerialConfig holds the user's serial port configuration choices
type SerialConfig struct {
	Port        string
	Baud        int
	Parity      string // "N", "O", "E", "M", "S"
	DataBits    int    // 5, 6, 7, 8
	StopBits    string // "1", "1.5", "2"
	FlowControl string // "none", "hardware", "software"
	TUI         bool
}

type serialPromptModel struct {
	step               int // 0=port, 1=baud, 2=advanced?, 3=advanced settings, 4=tui
	ports              []string
	cursor             int
	selected           SerialConfig
	err                error
	baudRates          []int
	yesNoChoice        int // 0=yes, 1=no
	width              int
	height             int
	skipAdvanced       bool // if true, skip step 3
	advancedFocused    int  // which advanced setting is focused (0-3)
	parityOptions      []string
	dataBitsOptions    []int
	stopBitsOptions    []string
	flowControlOptions []string
	parityIndex        int
	dataBitsIndex      int
	stopBitsIndex      int
	flowControlIndex   int
}

var commonBaudRates = []int{9600, 19200, 38400, 57600, 115200, 230400, 460800, 921600}

func NewSerialPrompt() tea.Model {
	ports, err := serial.GetPortsList()
	if err != nil {
		return &serialPromptModel{err: err}
	}

	sort.Strings(ports)

	return &serialPromptModel{
		step:               0,
		ports:              ports,
		baudRates:          commonBaudRates,
		parityOptions:      []string{"None", "Odd", "Even", "Mark", "Space"},
		dataBitsOptions:    []int{5, 6, 7, 8},
		stopBitsOptions:    []string{"1", "1.5", "2"},
		flowControlOptions: []string{"None", "Hardware (RTS/CTS)", "Software (XON/XOFF)"},
		parityIndex:        0, // None
		dataBitsIndex:      3, // 8
		stopBitsIndex:      0, // 1
		flowControlIndex:   0, // None
		selected: SerialConfig{
			Baud:        115200,
			Parity:      "N",
			DataBits:    8,
			StopBits:    "1",
			FlowControl: "none",
			TUI:         true,
		},
	}
}

func (m *serialPromptModel) Init() tea.Cmd {
	return nil
}

func (m *serialPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	if m.err != nil {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.step == 3 { // advanced settings
				if m.advancedFocused > 0 {
					m.advancedFocused--
				}
			} else {
				if m.cursor > 0 {
					m.cursor--
				}
			}

		case "down", "j":
			if m.step == 3 { // advanced settings
				if m.advancedFocused < 3 {
					m.advancedFocused++
				}
			} else {
				switch m.step {
				case 0: // port selection
					if m.cursor < len(m.ports)-1 {
						m.cursor++
					}
				case 1: // baud selection
					if m.cursor < len(m.baudRates)-1 {
						m.cursor++
					}
				case 2, 4: // advanced? or tui yes/no
					if m.cursor < 1 {
						m.cursor++
					}
				}
			}

		case "left", "h":
			if m.step == 3 { // advanced settings - cycle options left
				switch m.advancedFocused {
				case 0: // parity
					if m.parityIndex > 0 {
						m.parityIndex--
					} else {
						m.parityIndex = len(m.parityOptions) - 1
					}
				case 1: // data bits
					if m.dataBitsIndex > 0 {
						m.dataBitsIndex--
					} else {
						m.dataBitsIndex = len(m.dataBitsOptions) - 1
					}
				case 2: // stop bits
					if m.stopBitsIndex > 0 {
						m.stopBitsIndex--
					} else {
						m.stopBitsIndex = len(m.stopBitsOptions) - 1
					}
				case 3: // flow control
					if m.flowControlIndex > 0 {
						m.flowControlIndex--
					} else {
						m.flowControlIndex = len(m.flowControlOptions) - 1
					}
				}
			}

		case "right", "l", " ":
			if m.step == 3 { // advanced settings - cycle options right
				switch m.advancedFocused {
				case 0: // parity
					m.parityIndex = (m.parityIndex + 1) % len(m.parityOptions)
				case 1: // data bits
					m.dataBitsIndex = (m.dataBitsIndex + 1) % len(m.dataBitsOptions)
				case 2: // stop bits
					m.stopBitsIndex = (m.stopBitsIndex + 1) % len(m.stopBitsOptions)
				case 3: // flow control
					m.flowControlIndex = (m.flowControlIndex + 1) % len(m.flowControlOptions)
				}
			}

		case "esc":
			// Navigate back to previous step
			switch m.step {
			case 1: // Go back to port selection
				m.step = 0
				for i, port := range m.ports {
					if port == m.selected.Port {
						m.cursor = i
						break
					}
				}
			case 2: // Go back to baud selection
				m.step = 1
				for i, baud := range m.baudRates {
					if baud == m.selected.Baud {
						m.cursor = i
						break
					}
				}
			case 3: // Go back to advanced? prompt
				m.step = 2
				m.cursor = 0 // yes
			case 4: // Go back to appropriate step
				if m.skipAdvanced {
					m.step = 2
					m.cursor = 1 // no
				} else {
					m.step = 3
					m.advancedFocused = 0
				}
			}

		case "enter":
			switch m.step {
			case 0: // port selected
				if len(m.ports) > 0 {
					m.selected.Port = m.ports[m.cursor]
					m.step = 1
					// Find current baud rate in list
					for i, baud := range m.baudRates {
						if baud == m.selected.Baud {
							m.cursor = i
							break
						}
					}
				}
			case 1: // baud selected
				m.selected.Baud = m.baudRates[m.cursor]
				m.step = 2
				m.cursor = 0 // default to yes for advanced
			case 2: // advanced? selected
				if m.cursor == 0 { // yes - go to advanced settings
					m.skipAdvanced = false
					m.step = 3
					m.advancedFocused = 0
				} else { // no - skip to TUI choice
					m.skipAdvanced = true
					m.step = 4
					m.cursor = 0 // yes for TUI
				}
			case 3: // advanced settings - save and continue
				// Update selected config from indices
				m.selected.Parity = m.parityToCode(m.parityIndex)
				m.selected.DataBits = m.dataBitsOptions[m.dataBitsIndex]
				m.selected.StopBits = m.stopBitsOptions[m.stopBitsIndex]
				m.selected.FlowControl = m.flowControlToCode(m.flowControlIndex)
				m.step = 4
				m.cursor = 0 // yes for TUI
			case 4: // tui choice selected
				m.selected.TUI = (m.cursor == 0)
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

// Helper methods to convert display options to config codes
func (m *serialPromptModel) parityToCode(index int) string {
	codes := []string{"N", "O", "E", "M", "S"}
	return codes[index]
}

func (m *serialPromptModel) flowControlToCode(index int) string {
	codes := []string{"none", "hardware", "software"}
	return codes[index]
}

func (m *serialPromptModel) View() string {
	if m.err != nil {
		content := "\n" + StyleError.Render("✗ Error: "+m.err.Error()) + "\n\n" +
			StyleHelp.Render("Press q to quit.") + "\n"
		return lipgloss.PlaceVertical(m.height, lipgloss.Center,
			lipgloss.PlaceHorizontal(m.width, lipgloss.Center, content))
	}

	// Build the box content
	var boxContent strings.Builder

	// Title
	boxContent.WriteString(StyleTitle.Render("⚡ Serial Port Configuration"))
	boxContent.WriteString("\n\n")

	switch m.step {
	case 0: // Port selection
		boxContent.WriteString(StyleHeader.Render("Select a serial port:"))
		boxContent.WriteString("\n\n")

		if len(m.ports) == 0 {
			boxContent.WriteString(StyleError.Render("  No serial ports found!"))
			boxContent.WriteString("\n")
			boxContent.WriteString(StyleHelp.Render("  Press q to quit."))
		} else {
			for i, port := range m.ports {
				cursor := "  "
				style := StyleUnselected
				if m.cursor == i {
					cursor = StyleCursor.Render("❯ ")
					style = StyleSelected
				}
				boxContent.WriteString(cursor + style.Render(port))
				boxContent.WriteString("\n")
			}
		}

	case 1: // Baud rate selection
		// Show selected port
		boxContent.WriteString(StyleKey.Render("Port:") + " " + StyleValue.Render(m.selected.Port))
		boxContent.WriteString("\n\n")

		boxContent.WriteString(StyleHeader.Render("Select baud rate:"))
		boxContent.WriteString("\n\n")

		for i, baud := range m.baudRates {
			cursor := "  "
			style := StyleUnselected
			if m.cursor == i {
				cursor = StyleCursor.Render("❯ ")
				style = StyleSelected
			}
			boxContent.WriteString(cursor + style.Render(fmt.Sprintf("%d", baud)))
			boxContent.WriteString("\n")
		}

	case 2: // Advanced settings prompt
		boxContent.WriteString(StyleKey.Render("Port:") + " " + StyleValue.Render(m.selected.Port))
		boxContent.WriteString("\n")
		boxContent.WriteString(StyleKey.Render("Baud:") + " " + StyleValue.Render(fmt.Sprintf("%d", m.selected.Baud)))
		boxContent.WriteString("\n\n")

		boxContent.WriteString(StyleHeader.Render("Configure advanced settings?"))
		boxContent.WriteString("\n\n")

		choices := []string{"Yes", "No"}
		for i, choice := range choices {
			cursor := "  "
			style := StyleUnselected
			if m.cursor == i {
				cursor = StyleCursor.Render("❯ ")
				style = StyleSelected
			}
			boxContent.WriteString(cursor + style.Render(choice))
			boxContent.WriteString("\n")
		}

	case 3: // Advanced settings screen
		boxContent.WriteString(StyleKey.Render("Port:") + " " + StyleValue.Render(m.selected.Port))
		boxContent.WriteString("\n")
		boxContent.WriteString(StyleKey.Render("Baud:") + " " + StyleValue.Render(fmt.Sprintf("%d", m.selected.Baud)))
		boxContent.WriteString("\n\n")

		boxContent.WriteString(StyleHeader.Render("Advanced Settings:"))
		boxContent.WriteString("\n\n")

		// Build each setting line with consistent formatting
		settings := []struct {
			label string
			value string
		}{
			{"Parity:", m.parityOptions[m.parityIndex]},
			{"Data Bits:", fmt.Sprintf("%d", m.dataBitsOptions[m.dataBitsIndex])},
			{"Stop Bits:", m.stopBitsOptions[m.stopBitsIndex]},
			{"Flow Control:", m.flowControlOptions[m.flowControlIndex]},
		}

		// Create label styles without width constraints
		labelStyleUnfocused := lipgloss.NewStyle().Foreground(ColorMuted)
		labelStyleFocused := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)

		for i, setting := range settings {
			isFocused := m.advancedFocused == i

			// Build the line parts
			var cursorStr string
			var labelStr string
			var valueStr string
			var indicatorStr string

			if isFocused {
				cursorStr = StyleCursor.Render("❯ ")
				labelStr = labelStyleFocused.Render(fmt.Sprintf("%-14s", setting.label))
				valueStr = StyleValue.Render(setting.value)
				indicatorStr = StyleMuted.Render(" ←/→")
			} else {
				cursorStr = "  "
				labelStr = labelStyleUnfocused.Render(fmt.Sprintf("%-14s", setting.label))
				valueStr = StyleValue.Render(setting.value)
				indicatorStr = ""
			}

			boxContent.WriteString(cursorStr + labelStr + valueStr + indicatorStr + "\n")
		}

	case 4: // TUI choice
		// Show full summary
		boxContent.WriteString(StyleKey.Render("Port:") + " " + StyleValue.Render(m.selected.Port))
		boxContent.WriteString("\n")
		boxContent.WriteString(StyleKey.Render("Baud:") + " " + StyleValue.Render(fmt.Sprintf("%d", m.selected.Baud)))
		boxContent.WriteString("\n")
		if !m.skipAdvanced {
			boxContent.WriteString(StyleKey.Render("Parity:") + " " + StyleValue.Render(m.parityOptions[m.parityIndex]))
			boxContent.WriteString("\n")
			boxContent.WriteString(StyleKey.Render("Data Bits:") + " " + StyleValue.Render(fmt.Sprintf("%d", m.dataBitsOptions[m.dataBitsIndex])))
			boxContent.WriteString("\n")
			boxContent.WriteString(StyleKey.Render("Stop Bits:") + " " + StyleValue.Render(m.stopBitsOptions[m.stopBitsIndex]))
			boxContent.WriteString("\n")
			boxContent.WriteString(StyleKey.Render("Flow Control:") + " " + StyleValue.Render(m.flowControlOptions[m.flowControlIndex]))
			boxContent.WriteString("\n")
		}
		boxContent.WriteString("\n")

		boxContent.WriteString(StyleHeader.Render("Launch live TUI?"))
		boxContent.WriteString("\n\n")

		choices := []string{"Yes", "No"}
		for i, choice := range choices {
			cursor := "  "
			style := StyleUnselected
			if m.cursor == i {
				cursor = StyleCursor.Render("❯ ")
				style = StyleSelected
			}
			boxContent.WriteString(cursor + style.Render(choice))
			boxContent.WriteString("\n")
		}
	}

	// Create bordered box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2)

	box := boxStyle.Render(boxContent.String())

	// Build final content with help text
	var s strings.Builder
	s.WriteString(box)
	s.WriteString("\n\n")

	// Show different help text based on step
	var helpText string
	switch m.step {
	case 0:
		helpText = "↑/↓: navigate • enter: select • q: quit"
	case 3:
		helpText = "↑/↓: navigate • ←/→/space: change • enter: continue • esc: back • q: quit"
	default:
		helpText = "↑/↓: navigate • enter: select • esc: back • q: quit"
	}
	s.WriteString(StyleHelp.Render(helpText))

	content := s.String()
	return lipgloss.PlaceVertical(m.height, lipgloss.Center,
		lipgloss.PlaceHorizontal(m.width, lipgloss.Center, content))
}

// GetSerialConfig returns the selected configuration
func (m *serialPromptModel) GetSerialConfig() SerialConfig {
	return m.selected
}

// RunSerialPrompt runs the interactive prompt and returns the configuration
func RunSerialPrompt() (*SerialConfig, error) {
	p := tea.NewProgram(NewSerialPrompt(), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	model, ok := finalModel.(*serialPromptModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}

	if model.err != nil {
		return nil, model.err
	}

	config := model.GetSerialConfig()
	return &config, nil
}
