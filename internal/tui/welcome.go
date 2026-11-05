package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type welcomeModel struct {
	cursor    int
	menuItems []menuItem
	width     int
	height    int
	status    *StatusMessage
}

type menuItem struct {
	icon     string
	label    string
	shortcut string
	action   string
}

var (
	// Logo uses error color for the distinctive pink/red
	welcomeLogoStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary).
				Bold(true)

	// welcomeTitle replaces the logo on terminals too small for it.
	welcomeTitle = welcomeLogoStyle.Render("✦ A S T R O L A B E ✦")
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

func NewWelcome(status *StatusMessage) Screen {
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

func (m *welcomeModel) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, func() tea.Msg {
				return NavigateMsg{To: ""}
			}

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

		case "enter", "space":
			action := m.menuItems[m.cursor].action
			switch action {
			case "capture":
				return m, func() tea.Msg {
					return NavigateMsg{To: ScreenCaptureTabs}
				}
			case "metadata":
				return m, func() tea.Msg {
					return NavigateMsg{To: ScreenMetadata}
				}
			case "view-runs":
				return m, func() tea.Msg {
					return NavigateMsg{To: ScreenRuns}
				}
			case "upload":
				return m, func() tea.Msg {
					return NavigateMsg{To: ScreenUpload}
				}
			case "config-connection":
				return m, func() tea.Msg {
					return NavigateMsg{To: ScreenConfig}
				}
			default:
				return m, func() tea.Msg {
					return NavigateMsg{To: ""}
				}
			}
		}
	}

	return m, nil
}

func (m *welcomeModel) View() string {
	// Menu items
	menuBlock := strings.Builder{}
	for i, item := range m.menuItems {
		icon := StyleIcon.Render(item.icon)
		label := item.label
		var line string

		if i == m.cursor {
			line = StyleMenuSelected.Render(fmt.Sprintf("%s %s %s", IconSelectedItem, item.icon, label))
		} else {
			line = StyleMenuItem.Render(fmt.Sprintf("  %s %s", icon, label))
		}

		menuBlock.WriteString(line)
		menuBlock.WriteString("\n")
	}

	footer := lipgloss.JoinVertical(lipgloss.Center,
		StyleHelp.Render("enter: select • q: quit • ?: help"),
		"",
		StyleHelp.Render("Astrolabe CLI, by Cthonica Software."))

	layout := func(top, header, status string) string {
		var s strings.Builder
		s.WriteString(top)
		for _, part := range []string{header, status, menuBlock.String(), footer} {
			if part == "" {
				continue
			}
			s.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, part))
			s.WriteString("\n\n")
		}
		return strings.TrimSuffix(s.String(), "\n\n")
	}
	fits := func(s string) bool {
		return (m.height <= 0 || lipgloss.Height(s) <= m.height) &&
			(m.width <= 0 || lipgloss.Width(s) <= m.width)
	}

	status := m.status.Render(m.width) // "" when there is no status

	// Shrink until the screen fits: drop the logo for a one-line title, then
	// the top gap, then trim the status body.
	content := layout("\n\n", welcomeLogoStyle.Render(logo), status)
	if !fits(content) {
		content = layout("\n\n", welcomeTitle, status)
	}
	if !fits(content) {
		content = layout("", welcomeTitle, status)
	}
	if !fits(content) && m.status != nil {
		room := m.height - (lipgloss.Height(layout("", welcomeTitle, scrollStandIn)) - 1)
		status = "" // RenderMax treats maxHeight <= 0 as no limit
		if room > 0 {
			status = m.status.RenderMax(m.width, room)
		}
		content = layout("", welcomeTitle, status)
	}

	return lipgloss.PlaceVertical(m.height, lipgloss.Center, content)
}

// HelpSections lists the menu's keys for the help dialog.
func (m *welcomeModel) HelpSections() []HelpSection {
	return []HelpSection{{
		Title: "Menu",
		Keys: []KeyHelp{
			{"↑/↓ k/j", "move"},
			{"enter/space", "open the selected item"},
			{"q/esc", "quit"},
		},
	}}
}
