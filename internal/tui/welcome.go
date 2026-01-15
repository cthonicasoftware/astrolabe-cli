package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type welcomeModel struct {
	cursor         int
	menuItems      []menuItem
	width          int
	height         int
	selectedAction string
	status         *StatusMessage
}

type menuItem struct {
	icon     string
	label    string
	shortcut string
	action   string
}

type executeActionMsg struct {
	action string
}

var (
	// Logo uses error color for the distinctive pink/red
	welcomeLogoStyle = lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)
)

const logo = `
                                              ▄▄           ▄▄                                            
      ██              ██                    ▀███          ▄██                     ▄▄█▀▀▀█▄█████▀   ▀████▀
     ▄██▄             ██                      ██           ██                   ▄██▀     ▀█ ██       ██  
    ▄█▀██▄    ▄██▀████████▀███▄███  ▄██▀██▄   ██  ▄█▀██▄   ██▄████▄   ▄▄█▀██    ██▀       ▀ ██       ██  
   ▄█  ▀██    ██   ▀▀ ██    ██▀ ▀▀ ██▀   ▀██  ██ ██   ██   ██    ▀██ ▄█▀   ██   ██          ██       ██  
   ████████   ▀█████▄ ██    ██     ██     ██  ██  ▄█████   ██     ██ ██▀▀▀▀▀▀   ██▄         ██     ▄ ██  
  █▀      ██  █▄   ██ ██    ██     ██▄   ▄██  ██ ██   ██   ██▄   ▄██ ██▄    ▄   ▀██▄     ▄▀ ██    ▄█ ██  
▄███▄   ▄████▄██████▀ ▀████████▄    ▀█████▀ ▄████▄████▀██▄ █▀█████▀   ▀█████▀     ▀▀█████▀██████████████▄
˚　　　　✦　　　.　　.  　 ˚　.　　　　 　　.　　　　　　 ✦　　　.　　˚　 　　　　. ✦ 　   
　　.  　 　　　˚　　　　　*　　 　　✦　　　.　　.　　　✦　　˚ 　　　 　　˚　.　*　　. 　˚　　. 
`

func NewWelcome(status *StatusMessage) tea.Model {
	return &welcomeModel{
		cursor: 0,
		menuItems: []menuItem{
			{icon: IconMenuCapture, label: IconMenuSeparator + " Capture", shortcut: "", action: "capture"},
			{icon: IconMenuViewRuns, label: IconMenuSeparator + " View Runs", shortcut: "", action: "view-runs"},
			{icon: IconMenuUpload, label: IconMenuSeparator + " Upload Data", shortcut: "", action: "upload"},
			{icon: IconMenuMetadata, label: IconMenuSeparator + " Configure Metadata", shortcut: "", action: "metadata"},
			{icon: IconMenuConnection, label: IconMenuSeparator + " Configure Connection", shortcut: "", action: "config-connection"},
		},
		status: status,
	}
}

func (m *welcomeModel) Init() tea.Cmd {
	return nil
}

func (m *welcomeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case executeActionMsg:
		m.selectedAction = msg.action
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else {
				// Wrap to bottom
				m.cursor = len(m.menuItems) - 1
			}

		case "down", "j":
			if m.cursor < len(m.menuItems)-1 {
				m.cursor++
			} else {
				// Wrap to top
				m.cursor = 0
			}

		case "enter", " ":
			action := m.menuItems[m.cursor].action
			switch action {
			case "capture":
				// Launch capture source selection
				return m, func() tea.Msg {
					return executeActionMsg{action: "capture"}
				}
			case "metadata":
				return m, func() tea.Msg {
					return executeActionMsg{action: "metadata"}
				}
			case "view-runs":
				return m, func() tea.Msg {
					return executeActionMsg{action: "view-runs"}
				}
			case "upload":
				return m, func() tea.Msg {
					return executeActionMsg{action: "upload"}
				}
			case "config-connection":
				return m, func() tea.Msg {
					return executeActionMsg{action: "config-connection"}
				}
			default:
				// Not implemented yet
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m *welcomeModel) View() string {
	var s strings.Builder

	// Center the logo as a single block
	s.WriteString("\n\n")
	styledLogo := welcomeLogoStyle.Render(logo)
	centeredLogo := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, styledLogo)
	s.WriteString(centeredLogo)

	s.WriteString("\n\n")

	if m.status != nil {
		statusView := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, m.status.Render(m.width))
		s.WriteString(statusView)
		s.WriteString("\n\n")
	}

	// Menu items
	menuBlock := strings.Builder{}
	for i, item := range m.menuItems {
		icon := StyleIcon.Render(item.icon)
		label := item.label
		var line string

		if i == m.cursor {
			line = StyleMenuSelected.Render(fmt.Sprintf("%s %s %s", IconSelectedItem, item.icon, StyleMenuSelected.Render(label)))
		} else {
			line = StyleMenuItem.Render(fmt.Sprintf("  %s %s", icon, label))
		}

		menuBlock.WriteString(line)
		menuBlock.WriteString("\n")
	}

	// Center the menu
	centeredMenu := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, menuBlock.String())
	s.WriteString(centeredMenu)

	s.WriteString("\n\n")

	// Footer
	footer := StyleHelp.Render("Astrolabe CLI, by Cthonica Software.")
	centeredFooter := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, footer)
	s.WriteString(centeredFooter)

	// Fill remaining vertical space
	content := s.String()
	return lipgloss.PlaceVertical(m.height, lipgloss.Center, content)
}

// RunWelcome launches the welcome screen and returns the selected action
func RunWelcome(status *StatusMessage) (string, error) {
	p := tea.NewProgram(NewWelcome(status), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	model, ok := finalModel.(*welcomeModel)
	if !ok {
		return "", nil
	}

	return model.selectedAction, nil
}
