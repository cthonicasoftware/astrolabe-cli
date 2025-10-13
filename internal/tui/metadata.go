package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/config"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
)

const (
	fieldOperator = iota
	fieldLocation
	fieldDeviceID
	fieldDeviceSerial
	fieldDeviceFirmware
	fieldDeviceFirmwareHash
	fieldDeviceHardware
	fieldTestPlan
	fieldTestVariant
	fieldTestRun
	fieldTags
	totalMetadataInputs
	focusAttributes = totalMetadataInputs
)

type metadataModel struct {
	inputs     []textinput.Model
	attributes textarea.Model
	focusIndex int
	width      int
	height     int
	statusMsg  string
	errorMsg   string
	loadErr    error
	configPath string
	lastSaved  config.Metadata
}

type metadataField struct {
	label       string
	placeholder string
}

var metadataFields = [totalMetadataInputs]metadataField{
	fieldOperator:           {label: "Operator", placeholder: "operator name"},
	fieldLocation:           {label: "Location", placeholder: "bench or lab"},
	fieldDeviceID:           {label: "Device ID", placeholder: "device identifier"},
	fieldDeviceSerial:       {label: "Device Serial", placeholder: "serial number"},
	fieldDeviceFirmware:     {label: "Firmware", placeholder: "firmware version"},
	fieldDeviceFirmwareHash: {label: "Firmware Hash", placeholder: "commit hash"},
	fieldDeviceHardware:     {label: "Hardware Rev", placeholder: "hardware revision"},
	fieldTestPlan:           {label: "Test Plan", placeholder: "plan identifier"},
	fieldTestVariant:        {label: "Test Variant", placeholder: "variant name"},
	fieldTestRun:            {label: "Test Run", placeholder: "run id"},
	fieldTags:               {label: "Tags", placeholder: "tag1, tag2"},
}

// NewMetadataEditor constructs the metadata TUI model pre-populated with existing values.
func NewMetadataEditor(meta config.Metadata, path string, loadErr error) tea.Model {
	model := &metadataModel{
		inputs:     make([]textinput.Model, totalMetadataInputs),
		attributes: textarea.New(),
		focusIndex: 0,
		configPath: path,
		loadErr:    loadErr,
		lastSaved:  meta,
	}

	for i := 0; i < totalMetadataInputs; i++ {
		ti := textinput.New()
		ti.Placeholder = metadataFields[i].placeholder
		ti.Prompt = ""
		ti.CharLimit = 256
		ti.Width = 40
		model.inputs[i] = ti
	}

	model.inputs[fieldOperator].SetValue(meta.Operator)
	model.inputs[fieldLocation].SetValue(meta.Location)
	model.inputs[fieldDeviceID].SetValue(meta.Device.ID)
	model.inputs[fieldDeviceSerial].SetValue(meta.Device.Serial)
	model.inputs[fieldDeviceFirmware].SetValue(meta.Device.Firmware)
	model.inputs[fieldDeviceFirmwareHash].SetValue(meta.Device.FirmwareHash)
	model.inputs[fieldDeviceHardware].SetValue(meta.Device.HardwareVersion)
	model.inputs[fieldTestPlan].SetValue(meta.Test.Plan)
	model.inputs[fieldTestVariant].SetValue(meta.Test.Variant)
	model.inputs[fieldTestRun].SetValue(meta.Test.Run)
	model.inputs[fieldTags].SetValue(strings.Join(meta.Tags, ", "))

	model.attributes.Placeholder = "key=value (one per line)"
	model.attributes.Prompt = ""
	model.attributes.SetHeight(5)
	model.attributes.SetWidth(40)
	model.attributes.SetValue(formatAttributeLines(meta.Attributes))

	model.setFocus(0)

	return model
}

func (m *metadataModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *metadataModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeInputs()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "tab", "shift+tab":
			step := 1
			if msg.String() == "shift+tab" {
				step = -1
			}
			m.cycleFocus(step)
			m.clearMessages()
			return m, nil
		case "enter":
			if m.focusIndex != focusAttributes {
				m.cycleFocus(1)
				m.clearMessages()
				return m, nil
			}
			// Allow textarea to handle enter.
		case "ctrl+s":
			m.clearMessages()
			if err := m.save(); err != nil {
				m.errorMsg = err.Error()
			} else {
				m.statusMsg = fmt.Sprintf("Metadata saved to %s", m.configPath)
			}
			return m, nil
		}

		m.clearMessages()

		if m.focusIndex < len(m.inputs) {
			var cmd tea.Cmd
			m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
			return m, cmd
		}

		var cmd tea.Cmd
		m.attributes, cmd = m.attributes.Update(msg)
		return m, cmd
	}

	if m.focusIndex < len(m.inputs) {
		var cmd tea.Cmd
		m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.attributes, cmd = m.attributes.Update(msg)
	return m, cmd
}

func (m *metadataModel) View() string {
	var content strings.Builder

	title := StyleTitle.Render("📝 Run Metadata")
	content.WriteString(title)
	content.WriteString("\n")

	if m.loadErr != nil {
		content.WriteString(StyleError.Render(fmt.Sprintf("Failed to load existing metadata: %v", m.loadErr)))
		content.WriteString("\n\n")
	}

	form := strings.Builder{}
	for i, field := range metadataFields {
		label := lipgloss.NewStyle().Foreground(ColorMuted).Render(fmt.Sprintf("%-15s", field.label+":"))
		inputView := m.inputs[i].View()
		if m.focusIndex == i {
			label = StyleHighlight.Render(fmt.Sprintf("%-15s", field.label+":"))
			inputView = StyleHighlight.Render(inputView)
		}
		form.WriteString(label + " " + inputView + "\n")
	}

	attrLabel := "Attributes:"
	attrStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	if m.focusIndex == focusAttributes {
		attrLabel = StyleHighlight.Render(attrLabel)
	} else {
		attrLabel = attrStyle.Render(attrLabel)
	}

	textareaView := m.attributes.View()
	if m.focusIndex == focusAttributes {
		textareaView = StyleHighlight.Render(textareaView)
	}

	form.WriteString("\n" + attrLabel + "\n")
	form.WriteString(textareaView + "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2).
		Render(form.String())

	content.WriteString(box)
	content.WriteString("\n")

	if m.errorMsg != "" {
		content.WriteString(StyleError.Render("✗ " + m.errorMsg))
		content.WriteString("\n")
	} else if m.statusMsg != "" {
		content.WriteString(StyleSuccess.Render("✓ " + m.statusMsg))
		content.WriteString("\n")
	}

	help := "tab/shift+tab: navigate • ctrl+s: save • esc: close"
	content.WriteString(StyleHelp.Render(help))

	return lipgloss.PlaceVertical(m.height, lipgloss.Center,
		lipgloss.PlaceHorizontal(m.width, lipgloss.Center, content.String()))
}

func (m *metadataModel) resizeInputs() {
	if m.width == 0 {
		return
	}
	inputWidth := max(m.width-40, 24)
	for i := range m.inputs {
		m.inputs[i].Width = inputWidth
	}
	m.attributes.SetWidth(inputWidth)
}

func (m *metadataModel) cycleFocus(step int) {
	m.focusIndex = (m.focusIndex + step + totalMetadataInputs + 1) % (totalMetadataInputs + 1)
	m.setFocus(m.focusIndex)
}

func (m *metadataModel) setFocus(index int) {
	for i := range m.inputs {
		if i == index {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	if index == focusAttributes {
		m.attributes.Focus()
	} else {
		m.attributes.Blur()
	}
	m.focusIndex = index
}

func (m *metadataModel) clearMessages() {
	m.errorMsg = ""
	m.statusMsg = ""
}

func (m *metadataModel) save() error {
	meta := config.Metadata{
		Operator: m.inputs[fieldOperator].Value(),
		Location: m.inputs[fieldLocation].Value(),
		Device: core.DeviceInfo{
			ID:              m.inputs[fieldDeviceID].Value(),
			Serial:          m.inputs[fieldDeviceSerial].Value(),
			Firmware:        m.inputs[fieldDeviceFirmware].Value(),
			FirmwareHash:    m.inputs[fieldDeviceFirmwareHash].Value(),
			HardwareVersion: m.inputs[fieldDeviceHardware].Value(),
		},
		Test: core.TestInfo{
			Plan:    strings.TrimSpace(m.inputs[fieldTestPlan].Value()),
			Variant: m.inputs[fieldTestVariant].Value(),
			Run:     m.inputs[fieldTestRun].Value(),
		},
	}

	if meta.Test.Plan == "" {
		meta.Test.Plan = "unspecified"
	}

	tags, err := parseTags(m.inputs[fieldTags].Value())
	if err != nil {
		return err
	}
	meta.Tags = tags

	attrs, err := parseAttributes(m.attributes.Value())
	if err != nil {
		return err
	}
	meta.Attributes = attrs

	path, err := config.SaveMetadata(meta)
	if err != nil {
		return err
	}
	m.configPath = path
	m.lastSaved = meta
	return nil
}

func parseTags(input string) ([]string, error) {
	if strings.TrimSpace(input) == "" {
		return nil, nil
	}
	raw := strings.Split(input, ",")
	tags := make([]string, 0, len(raw))
	for _, tag := range raw {
		t := strings.TrimSpace(tag)
		if t == "" {
			return nil, fmt.Errorf("tags cannot contain empty values")
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func parseAttributes(input string) (map[string]string, error) {
	result := map[string]string{}
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid attribute %q (expected key=value)", line)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, fmt.Errorf("attribute key missing in %q", line)
		}
		if value == "" {
			return nil, fmt.Errorf("attribute %q has empty value", key)
		}
		result[key] = value
	}
	return result, nil
}

func formatAttributeLines(attrs map[string]string) string {
	if len(attrs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", k, attrs[k]))
	}
	return strings.Join(lines, "\n")
}

// RunMetadataEditor launches the metadata configuration TUI.
func RunMetadataEditor(status *StatusMessage) (*StatusMessage, error) {
	meta, err := config.LoadMetadata()
	var path string
	if p, perr := config.MetadataPath(); perr == nil {
		path = p
	}

	model := NewMetadataEditor(meta, path, err)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, runErr := p.Run()
	if runErr != nil {
		return status, runErr
	}

	editor, ok := finalModel.(*metadataModel)
	if ok && editor != nil && editor.statusMsg != "" {
		status = NewStatusMessage(StatusSuccess, "Metadata Saved", editor.statusMsg)
	}
	return status, nil
}
