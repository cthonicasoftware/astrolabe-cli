package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
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
	repo       config.MetadataRepository
	focusIndex int
	width      int
	height     int
	statusMsg  string
	errorMsg   string
	loadErr    error
	configPath string
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
func NewMetadataEditor(repo config.MetadataRepository, meta config.Metadata, path string, loadErr error) tea.Model {
	if repo == nil {
		repo = config.NewFileMetadataRepository(nil)
	}

	model := &metadataModel{
		inputs:     make([]textinput.Model, totalMetadataInputs),
		attributes: textarea.New(),
		repo:       repo,
		focusIndex: 0,
		configPath: path,
		loadErr:    loadErr,
	}

	for i := range totalMetadataInputs {
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
	model.attributes.SetValue(config.FormatAttributeLines(meta.Attributes))

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
		case "tab", "shift+tab", "up", "down":
			step := 1
			if msg.String() == "shift+tab" || msg.String() == "up" {
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
		msg := NewStatusMessage(StatusError, "Save Failed", m.errorMsg)
		content.WriteString(msg.Render(m.width))
		content.WriteString("\n")
	} else if m.statusMsg != "" {
		msg := NewStatusMessage(StatusSuccess, "Saved", m.statusMsg)
		content.WriteString(msg.Render(m.width))
		content.WriteString("\n")
	}

	help := " ↑/↓ tab/tabshift+tab: navigate • ctrl+s: save • esc: close"
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
	if m.repo == nil {
		return fmt.Errorf("metadata repository not configured")
	}

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

	tags, err := config.ParseTags(m.inputs[fieldTags].Value())
	if err != nil {
		return err
	}
	meta.Tags = tags

	attrs, err := config.ParseAttributes(m.attributes.Value())
	if err != nil {
		return err
	}
	meta.Attributes = attrs

	path, err := m.repo.Save(meta)
	if err != nil {
		return err
	}
	m.configPath = path
	return nil
}

// RunMetadataEditor launches the metadata configuration TUI.
func RunMetadataEditor(status *StatusMessage) (*StatusMessage, error) {
	repo := config.NewFileMetadataRepository(nil)

	meta, err := repo.Load()
	var path string
	if p, perr := repo.Path(); perr == nil {
		path = p
	}

	model := NewMetadataEditor(repo, meta, path, err)
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

