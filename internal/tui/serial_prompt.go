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
	Port string
	Baud int
	TUI  bool
}

type serialPromptModel struct {
	step        int // 0=port, 1=baud, 2=tui
	ports       []string
	cursor      int
	selected    SerialConfig
	err         error
	baudRates   []int
	yesNoChoice int // 0=yes, 1=no
	width       int
	height      int
}

var commonBaudRates = []int{9600, 19200, 38400, 57600, 115200, 230400, 460800, 921600}

func NewSerialPrompt() tea.Model {
	ports, err := serial.GetPortsList()
	if err != nil {
		return &serialPromptModel{err: err}
	}

	sort.Strings(ports)

	return &serialPromptModel{
		step:      0,
		ports:     ports,
		baudRates: commonBaudRates,
		selected: SerialConfig{
			Baud: 115200,
			TUI:  true,
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
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			switch m.step {
			case 0: // port selection
				if m.cursor < len(m.ports)-1 {
					m.cursor++
				}
			case 1: // baud selection
				if m.cursor < len(m.baudRates)-1 {
					m.cursor++
				}
			case 2: // tui yes/no
				if m.cursor < 1 {
					m.cursor++
				}
			}

		case "enter", " ":
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
				m.cursor = 0 // yes for TUI
			case 2: // tui choice selected
				m.selected.TUI = (m.cursor == 0)
				return m, tea.Quit
			}
		}
	}

	return m, nil
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

	case 2: // TUI choice
		// Show summary
		boxContent.WriteString(StyleKey.Render("Port:") + " " + StyleValue.Render(m.selected.Port))
		boxContent.WriteString("\n")
		boxContent.WriteString(StyleKey.Render("Baud:") + " " + StyleValue.Render(fmt.Sprintf("%d", m.selected.Baud)))
		boxContent.WriteString("\n\n")

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
	s.WriteString(StyleHelp.Render("↑/↓: navigate • enter: select • q: quit"))

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
