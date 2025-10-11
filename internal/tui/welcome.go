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

// Styles are now centralized in styles.go

var (
	// Logo uses error color for the distinctive pink/red
	welcomeLogoStyle = lipgloss.NewStyle().
		Foreground(ColorError).
		Bold(true)
)

const logo = `
  ██████╗   █████╗        █████╗  ██████╗ ███████╗███╗   ██╗████████╗
██╔═══██╗██╔══██╗      ██╔══██╗██╔════╝ ██╔════╝████╗  ██║╚══██╔══╝
██║   ██║███████║█████╗███████║██║  ███╗█████╗  ██╔██╗ ██║   ██║
██║▄▄ ██║██╔══██║╚════╝██╔══██║██║   ██║██╔══╝  ██║╚██╗██║   ██║
╚██████╔╝██║  ██║      ██║  ██║╚██████╔╝███████╗██║ ╚████║   ██║
 ╚══▀▀═╝ ╚═╝  ╚═╝      ╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝  ╚═══╝   ╚═╝
`

func NewWelcome() tea.Model {
	return &welcomeModel{
		cursor: 0,
		menuItems: []menuItem{
			{icon: "📡", label: "Capture Serial", shortcut: "", action: "capture"},
			{icon: "🔍", label: "List Ports", shortcut: "", action: "list-ports"},
			{icon: "📊", label: "View Runs", shortcut: "", action: "view-runs"},
			{icon: "☁️ ", label: "Upload Data", shortcut: "", action: "upload"},
			{icon: "⚙️ ", label: "Configuration", shortcut: "", action: "config"},
			{icon: "📄", label: "New File", shortcut: "", action: "new"},
		},
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
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.menuItems)-1 {
				m.cursor++
			}

		case "enter", " ":
			action := m.menuItems[m.cursor].action
			switch action {
			case "capture":
				// Launch serial capture prompt
				return m, func() tea.Msg {
					return executeActionMsg{action: "capture"}
				}
			case "list-ports":
				// Launch list ports
				return m, func() tea.Msg {
					return executeActionMsg{action: "list-ports"}
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
	styledLogo := welcomeLogoStyle.Render(strings.TrimSpace(logo))
	centeredLogo := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, styledLogo)
	s.WriteString(centeredLogo)

	s.WriteString("\n\n")

	// Menu items
	menuBlock := strings.Builder{}
	for i, item := range m.menuItems {
		var line string

		if i == m.cursor {
			// Selected item
			cursor := StyleHighlight.Render("❯ ")
			icon := StyleIcon.Render(item.icon)
			label := StyleHighlight.Render(item.label)
			line = fmt.Sprintf("%s%s %s", cursor, icon, label)
		} else {
			// Unselected item
			cursor := "  "
			icon := StyleIcon.Render(item.icon)
			label := StyleSubheader.Render(item.label)
			line = fmt.Sprintf("%s%s %s", cursor, icon, label)
		}

		menuBlock.WriteString(line)
		menuBlock.WriteString("\n")
	}

	// Center the menu
	centeredMenu := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, menuBlock.String())
	s.WriteString(centeredMenu)

	s.WriteString("\n\n")

	// Footer
	footer := StyleHelp.Render("QA command line tool")
	centeredFooter := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, footer)
	s.WriteString(centeredFooter)

	// Fill remaining vertical space
	content := s.String()
	return lipgloss.PlaceVertical(m.height, lipgloss.Center, content)
}

// RunWelcome launches the welcome screen and returns the selected action
func RunWelcome() (string, error) {
	p := tea.NewProgram(NewWelcome(), tea.WithAltScreen())
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
