package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"
)

const (
	configFieldAPIURL = iota
	configFieldProjectID
	configFieldAuthToken
	configFieldOfflineCache
	configFieldMaxRetries
	totalConfigInputs
)

type configModel struct {
	inputs     []textinput.Model
	focusIndex int
	width      int
	height     int
	statusMsg  string
	errorMsg   string
	configPath string
}

type configField struct {
	label       string
	placeholder string
	key         string
}

var configFields = [totalConfigInputs]configField{
	configFieldAPIURL:       {label: "API URL", placeholder: "https://qa.yourcompany.com", key: "api_url"},
	configFieldProjectID:    {label: "Project ID", placeholder: "project-123", key: "project_id"},
	configFieldAuthToken:    {label: "Auth Token", placeholder: "your-api-token", key: "auth_token"},
	configFieldOfflineCache: {label: "Offline Cache", placeholder: "~/.astrolabe/runs", key: "offline_cache"},
	configFieldMaxRetries:   {label: "Max Retries", placeholder: "3", key: "upload.max_retries"},
}

// NewConfigEditor constructs the config TUI model pre-populated with existing values.
func NewConfigEditor() tea.Model {
	// Try to find existing config file
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		// Default to ~/.astrolabe/connection.yml
		home, _ := os.UserHomeDir()
		configPath = filepath.Join(home, ".astrolabe", "connection.yml")
	}

	model := &configModel{
		inputs:     make([]textinput.Model, totalConfigInputs),
		focusIndex: 0,
		configPath: configPath,
	}

	// Load existing config
	_ = viper.ReadInConfig()

	for i := range totalConfigInputs {
		ti := textinput.New()
		ti.Placeholder = configFields[i].placeholder
		ti.Prompt = ""
		ti.CharLimit = 512
		ti.Width = 60

		// Load existing value
		val := viper.GetString(configFields[i].key)
		if val == "" && i == configFieldMaxRetries {
			// Special handling for max_retries which is an int
			retries := viper.GetInt(configFields[i].key)
			if retries > 0 {
				val = strconv.Itoa(retries)
			}
		}
		ti.SetValue(val)

		// Mask auth token
		if i == configFieldAuthToken {
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '•'
		}

		model.inputs[i] = ti
	}

	model.setFocus(0)
	return model
}

func (m *configModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *configModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeInputs()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, func() tea.Msg {
				return NavigateMsg{To: ScreenWelcome, Status: nil}
			}
		case "tab", "shift+tab", "up", "down":
			step := 1
			if msg.String() == "shift+tab" || msg.String() == "up" {
				step = -1
			}
			m.cycleFocus(step)
			m.clearMessages()
			return m, nil
		case "enter":
			m.cycleFocus(1)
			m.clearMessages()
			return m, nil
		case "ctrl+s":
			m.clearMessages()
			if err := m.save(); err != nil {
				m.errorMsg = err.Error()
				return m, nil
			}
			statusMsg := NewStatusMessage(StatusSuccess, "Configuration Updated", fmt.Sprintf("Configuration saved to %s", m.configPath))
			return m, func() tea.Msg {
				return NavigateMsg{To: ScreenWelcome, Status: statusMsg}
			}
		case "ctrl+t":
			// Toggle auth token visibility
			if m.focusIndex == configFieldAuthToken {
				if m.inputs[configFieldAuthToken].EchoMode == textinput.EchoPassword {
					m.inputs[configFieldAuthToken].EchoMode = textinput.EchoNormal
					m.inputs[configFieldAuthToken].EchoCharacter = 0
				} else {
					m.inputs[configFieldAuthToken].EchoMode = textinput.EchoPassword
					m.inputs[configFieldAuthToken].EchoCharacter = '•'
				}
			}
			return m, nil
		}

		m.clearMessages()

		var cmd tea.Cmd
		m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	return m, cmd
}

func (m *configModel) View() string {
	var content strings.Builder

	title := StyleTitle.Render("⚙️  Astrolabe Configuration")
	content.WriteString(title)
	content.WriteString("\n\n")

	info := lipgloss.NewStyle().Foreground(ColorMuted).Render(
		"Configure connection and upload settings.",
	)
	content.WriteString(info)
	content.WriteString("\n\n")

	form := strings.Builder{}
	for i, field := range configFields {
		label := lipgloss.NewStyle().Foreground(ColorMuted).Render(fmt.Sprintf("%-15s", field.label+":"))
		inputView := m.inputs[i].View()
		if m.focusIndex == i {
			label = StyleHighlight.Render(fmt.Sprintf("%-15s", field.label+":"))
			inputView = StyleHighlight.Render(inputView)
		}
		form.WriteString(label + " " + inputView + "\n")
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2).
		Render(form.String())

	content.WriteString(box)
	content.WriteString("\n")

	if m.errorMsg != "" {
		msg := NewStatusMessage(StatusError, "Save Failed", m.errorMsg)
		content.WriteString(msg.Render(m.width))
		content.WriteString("\n")
	} else if m.statusMsg != "" {
		msg := NewStatusMessage(StatusSuccess, "Saved", m.statusMsg)
		content.WriteString(msg.Render(m.width))
		content.WriteString("\n")
	}

	help := " ↑/↓ tab/shift+tab: navigate • ctrl+t: toggle token visibility • ctrl+s: save • esc: close"
	content.WriteString(StyleHelp.Render(help))

	return lipgloss.PlaceVertical(m.height, lipgloss.Center,
		lipgloss.PlaceHorizontal(m.width, lipgloss.Center, content.String()))
}

func (m *configModel) resizeInputs() {
	if m.width == 0 {
		return
	}
	inputWidth := max(m.width-40, 30)
	for i := range m.inputs {
		m.inputs[i].Width = inputWidth
	}
}

func (m *configModel) cycleFocus(step int) {
	m.focusIndex = (m.focusIndex + step + totalConfigInputs) % totalConfigInputs
	m.setFocus(m.focusIndex)
}

func (m *configModel) setFocus(index int) {
	for i := range m.inputs {
		if i == index {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	m.focusIndex = index
}

func (m *configModel) clearMessages() {
	m.errorMsg = ""
	m.statusMsg = ""
}

func (m *configModel) save() error {
	// Build config map
	cfg := map[string]interface{}{
		"api_url":       strings.TrimSpace(m.inputs[configFieldAPIURL].Value()),
		"project_id":    strings.TrimSpace(m.inputs[configFieldProjectID].Value()),
		"auth_token":    strings.TrimSpace(m.inputs[configFieldAuthToken].Value()),
		"offline_cache": strings.TrimSpace(m.inputs[configFieldOfflineCache].Value()),
	}

	// Parse max_retries as int
	maxRetriesStr := strings.TrimSpace(m.inputs[configFieldMaxRetries].Value())
	if maxRetriesStr != "" {
		maxRetries, err := strconv.Atoi(maxRetriesStr)
		if err != nil {
			return fmt.Errorf("max retries must be a number: %w", err)
		}
		cfg["upload"] = map[string]interface{}{
			"max_retries": maxRetries,
		}
	}

	// Build YAML content
	var yamlContent strings.Builder
	yamlContent.WriteString("# Astrolabe Connection Configuration\n")
	yamlContent.WriteString("# Backend server connection and upload settings\n")
	yamlContent.WriteString("# Generated by astrolabe config edit\n\n")

	// Write top-level fields
	if apiURL := cfg["api_url"].(string); apiURL != "" {
		yamlContent.WriteString(fmt.Sprintf("api_url: %q\n", apiURL))
	}
	if projectID := cfg["project_id"].(string); projectID != "" {
		yamlContent.WriteString(fmt.Sprintf("project_id: %q\n", projectID))
	}
	if authToken := cfg["auth_token"].(string); authToken != "" {
		yamlContent.WriteString(fmt.Sprintf("auth_token: %q\n", authToken))
	}
	if offlineCache := cfg["offline_cache"].(string); offlineCache != "" {
		yamlContent.WriteString(fmt.Sprintf("offline_cache: %q\n", offlineCache))
	}

	// Write upload section
	if upload, ok := cfg["upload"].(map[string]interface{}); ok {
		yamlContent.WriteString("\nupload:\n")
		if maxRetries, ok := upload["max_retries"].(int); ok {
			yamlContent.WriteString(fmt.Sprintf("  max_retries: %d\n", maxRetries))
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	// Write config file
	if err := os.WriteFile(m.configPath, []byte(yamlContent.String()), 0o600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}
