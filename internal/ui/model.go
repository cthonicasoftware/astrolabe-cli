package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.bug.st/serial"

	"qa_cli/internal/capture"
)

type focusTarget int

const (
	focusTabs focusTarget = iota
	focusPort
	focusBaud
	focusRunID
	focusFirmware
	focusFormat
	focusUploadMode
	focusUploadToggle
	focusSubmit
)

type initFocusMsg struct{}

type captureResultMsg struct {
	result capture.Result
	err    error
}

type selectField struct {
	label    string
	options  []string
	selected int
}

type driverTab struct {
	name         string
	placeholder  string
	requiresBaud bool
}

type Model struct {
	tabs       []driverTab
	activeTab  int
	focusIndex int

	portInput       textinput.Model
	baudInput       textinput.Model
	runIDInput      textinput.Model
	firmwareInput   textinput.Model
	formatField     selectField
	uploadModeField selectField
	autoUpload      bool

	statusViewport viewport.Model
	logViewport    viewport.Model
	statusLines    []string
	logLines       []string

	capturing bool
	width     int
	height    int
	err       error
}

var (
	focusedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
	blurredStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	labelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Bold(true)
	titleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("239"))
	noStyle       = lipgloss.NewStyle()

	highlightColor   = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	inactiveTabStyle = lipgloss.NewStyle().Border(tabBorderWithBottom("┴", "─", "┴"), true).BorderForeground(highlightColor).Padding(0, 1)
	activeTabStyle   = inactiveTabStyle.Border(tabBorderWithBottom("┘", " ", "└"), true)
)

func tabBorderWithBottom(left, middle, right string) lipgloss.Border {
	border := lipgloss.RoundedBorder()
	border.BottomLeft = left
	border.Bottom = middle
	border.BottomRight = right
	return border
}

func NewModel() Model {
	ports, portErr := serial.GetPortsList()

	tabs := []driverTab{
		{name: "Serial", placeholder: "/dev/ttyUSB0", requiresBaud: true},
		{name: "TCP Socket", placeholder: "127.0.0.1:9000"},
		{name: "File Replay", placeholder: "capture.jsonl"},
	}

	portInput := textinput.New()
	portInput.Placeholder = tabs[0].placeholder
	portInput.CharLimit = 64
	portInput.Width = 32
	if len(ports) > 0 {
		portInput.SetValue(ports[0])
	}

	baudInput := textinput.New()
	baudInput.Placeholder = "115200"
	baudInput.CharLimit = 8
	baudInput.Width = 16
	baudInput.SetValue("115200")

	runIDInput := textinput.New()
	runIDInput.Placeholder = "run-id"
	runIDInput.CharLimit = 64
	runIDInput.Width = 32

	firmwareInput := textinput.New()
	firmwareInput.Placeholder = "firmware hash"
	firmwareInput.CharLimit = 64
	firmwareInput.Width = 32

	format := selectField{label: "Output Format", options: []string{"Normalized JSONL", "Protobuf"}}
	upload := selectField{label: "Upload Mode", options: []string{"Direct (API)", "Staged", "Offline"}}

	statusVP := viewport.New(60, 3)
	logVP := viewport.New(60, 5)

	m := Model{
		tabs:            tabs,
		activeTab:       0,
		focusIndex:      0,
		portInput:       portInput,
		baudInput:       baudInput,
		runIDInput:      runIDInput,
		firmwareInput:   firmwareInput,
		formatField:     format,
		uploadModeField: upload,
		autoUpload:      true,
		statusViewport:  statusVP,
		logViewport:     logVP,
	}

	m.statusLines = []string{"Use ↑/↓ or j/k to move fields. Tab/Shift+Tab cycle drivers."}
	if portErr != nil {
		m.logLines = append(m.logLines, fmt.Sprintf("Failed to enumerate serial ports: %v", portErr))
	} else if len(ports) == 0 {
		m.logLines = append(m.logLines, "No serial ports detected; enter one manually if needed.")
	} else {
		m.logLines = append(m.logLines, fmt.Sprintf("Detected %d serial port(s).", len(ports)))
	}
	m.syncStatus()
	m.syncLogs()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, func() tea.Msg { return initFocusMsg{} })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	m.portInput, cmd = m.portInput.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.baudInput, cmd = m.baudInput.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.runIDInput, cmd = m.runIDInput.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.firmwareInput, cmd = m.firmwareInput.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case initFocusMsg:
		return m, m.applyFocus()
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.statusViewport.Width = msg.Width
		m.logViewport.Width = msg.Width
		m.syncStatus()
		m.syncLogs()
		m.refreshViewportHeight()
	case captureResultMsg:
		m.capturing = false
		if msg.err != nil {
			m.err = msg.err
			m.appendStatus("Capture failed")
			m.appendLog(fmt.Sprintf("error: %v", msg.err))
		} else {
			m.err = nil
			m.appendStatus(fmt.Sprintf("Capture complete for run %s", msg.result.RunID))
			if msg.result.DataPath != "" {
				m.appendLog(fmt.Sprintf("data saved to %s", msg.result.DataPath))
			}
			if msg.result.MetadataPath != "" {
				m.appendLog(fmt.Sprintf("metadata saved to %s", msg.result.MetadataPath))
			}
		}
		m.refreshViewportHeight()
		return m, tea.Batch(cmds...)
	case tea.KeyMsg:
		key := msg.String()
		switch key {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "left", "h":
			if m.currentFocus() == focusTabs && len(m.tabs) > 0 {
				m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
				m.onTabChanged()
				return m, tea.Batch(cmds...)
			}
			if m.handleSelection(-1) {
				return m, tea.Batch(cmds...)
			}
		case "right", "l":
			if m.currentFocus() == focusTabs && len(m.tabs) > 0 {
				m.activeTab = (m.activeTab + 1) % len(m.tabs)
				m.onTabChanged()
				return m, tea.Batch(cmds...)
			}
			if m.handleSelection(1) {
				return m, tea.Batch(cmds...)
			}
		case "down", "j":
			if key == "j" && (m.focused(focusPort) || m.focused(focusBaud) || m.focused(focusRunID) || m.focused(focusFirmware)) {
				break
			}
			m.moveFocus(1)
			return m, tea.Batch(append(cmds, m.applyFocus())...)
		case "up", "k":
			if key == "k" && (m.focused(focusPort) || m.focused(focusBaud) || m.focused(focusRunID) || m.focused(focusFirmware)) {
				break
			}
			m.moveFocus(-1)
			return m, tea.Batch(append(cmds, m.applyFocus())...)
		case "tab":
			if len(m.tabs) > 0 {
				m.activeTab = (m.activeTab + 1) % len(m.tabs)
				m.onTabChanged()
				return m, tea.Batch(cmds...)
			}
		case "shift+tab":
			if len(m.tabs) > 0 {
				m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
				m.onTabChanged()
				return m, tea.Batch(cmds...)
			}
		case " ":
			if m.currentFocus() == focusUploadToggle {
				if m.uploadModeField.value() == "Offline" {
					m.appendStatus("Auto upload disabled in offline mode.")
					return m, tea.Batch(cmds...)
				}
				m.autoUpload = !m.autoUpload
				m.appendStatus(fmt.Sprintf("Auto upload %s", boolLabel(m.autoUpload)))
				return m, tea.Batch(cmds...)
			}
		case "enter":
			switch m.currentFocus() {
			case focusTabs:
				if len(m.tabs) > 0 {
					m.activeTab = (m.activeTab + 1) % len(m.tabs)
					m.onTabChanged()
				}
				return m, tea.Batch(cmds...)
			case focusUploadToggle:
				if m.uploadModeField.value() == "Offline" {
					m.appendStatus("Auto upload disabled in offline mode.")
					return m, tea.Batch(cmds...)
				}
				m.autoUpload = !m.autoUpload
				m.appendStatus(fmt.Sprintf("Auto upload %s", boolLabel(m.autoUpload)))
				return m, tea.Batch(cmds...)
			case focusSubmit:
				if m.capturing {
					return m, tea.Batch(cmds...)
				}
				cfg, err := m.buildConfig()
				if err != nil {
					m.err = err
					m.appendStatus("Configuration error")
					m.appendLog(err.Error())
					m.refreshViewportHeight()
					return m, tea.Batch(cmds...)
				}
				m.err = nil
				m.capturing = true
				m.appendStatus(fmt.Sprintf("Starting %s capture…", strings.ToLower(cfg.Driver)))
				m.refreshViewportHeight()
				return m, tea.Batch(append(cmds, startCaptureCommand(cfg))...)
			}
			m.moveFocus(1)
			return m, tea.Batch(append(cmds, m.applyFocus())...)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	form := m.formView()
	statusLabel := labelStyle.Render("Status")
	helpLine := helpStyle.Render("Left/Right to choose driver • Tab to navigate • Esc to quit")

	sections := []string{
		form,
		statusLabel,
		m.statusViewport.View(),
		labelStyle.Render("Log"),
		m.logViewport.View(),
		helpLine,
	}

	return strings.Join(sections, "\n")
}

func (m Model) formView() string {
	title := titleStyle.Render("QA Agent Capture")
	tabs := m.renderTabs()

	var leftParts []string
	var rightParts []string

	leftParts = append(leftParts, labelStyle.Render(m.portLabel()))
	if m.focused(focusPort) {
		m.portInput.PromptStyle = focusedStyle
		m.portInput.TextStyle = focusedStyle
	} else {
		m.portInput.PromptStyle = noStyle
		m.portInput.TextStyle = noStyle
	}
	leftParts = append(leftParts, m.portInput.View())

	if m.tabs[m.activeTab].requiresBaud {
		leftParts = append(leftParts, "")
		leftParts = append(leftParts, labelStyle.Render("Baud Rate"))
		if m.focused(focusBaud) {
			m.baudInput.PromptStyle = focusedStyle
			m.baudInput.TextStyle = focusedStyle
		} else {
			m.baudInput.PromptStyle = noStyle
			m.baudInput.TextStyle = noStyle
		}
		leftParts = append(leftParts, m.baudInput.View())
	}

	leftParts = append(leftParts, "")
	leftParts = append(leftParts, labelStyle.Render("Run ID"))
	if m.focused(focusRunID) {
		m.runIDInput.PromptStyle = focusedStyle
		m.runIDInput.TextStyle = focusedStyle
	} else {
		m.runIDInput.PromptStyle = noStyle
		m.runIDInput.TextStyle = noStyle
	}
	leftParts = append(leftParts, m.runIDInput.View())

	leftColumn := strings.Join(leftParts, "\n")
	leftColumn = lipgloss.NewStyle().PaddingRight(4).Render(leftColumn)

	if m.focused(focusFirmware) {
		m.firmwareInput.PromptStyle = focusedStyle
		m.firmwareInput.TextStyle = focusedStyle
	} else {
		m.firmwareInput.PromptStyle = noStyle
		m.firmwareInput.TextStyle = noStyle
	}
	rightParts = append(rightParts, labelStyle.Render("Firmware Hash"))
	rightParts = append(rightParts, m.firmwareInput.View())
	rightParts = append(rightParts, "")
	rightParts = append(rightParts, m.formatField.view(m.focused(focusFormat)))
	rightParts = append(rightParts, m.uploadModeField.view(m.focused(focusUploadMode)))
	rightParts = append(rightParts, m.autoUploadView())

	if m.err != nil {
		rightParts = append(rightParts, "")
		rightParts = append(rightParts, errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	rightColumn := lipgloss.NewStyle().Render(strings.Join(rightParts, "\n"))
	columns := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, rightColumn)

	tabFrame := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(highlightColor).
		Padding(1, 2).
		Render(columns)

	tabbedFrame := mergeTabsAndFrame(tabs, tabFrame)
	body := lipgloss.JoinVertical(lipgloss.Left, tabbedFrame, m.submitButtonView())

	sections := []string{title, body}
	return strings.Join(sections, "\n")
}

func (m *Model) applyFocus() tea.Cmd {
	var cmds []tea.Cmd

	if m.focused(focusPort) {
		cmd := m.portInput.Focus()
		m.portInput.PromptStyle = focusedStyle
		m.portInput.TextStyle = focusedStyle
		cmds = appendIfNotNil(cmds, cmd)
	} else {
		m.portInput.Blur()
		m.portInput.PromptStyle = noStyle
		m.portInput.TextStyle = noStyle
	}

	if m.focused(focusBaud) && m.tabs[m.activeTab].requiresBaud {
		cmd := m.baudInput.Focus()
		m.baudInput.PromptStyle = focusedStyle
		m.baudInput.TextStyle = focusedStyle
		cmds = appendIfNotNil(cmds, cmd)
	} else {
		m.baudInput.Blur()
		m.baudInput.PromptStyle = noStyle
		m.baudInput.TextStyle = noStyle
	}

	if m.focused(focusRunID) {
		cmd := m.runIDInput.Focus()
		m.runIDInput.PromptStyle = focusedStyle
		m.runIDInput.TextStyle = focusedStyle
		cmds = appendIfNotNil(cmds, cmd)
	} else {
		m.runIDInput.Blur()
		m.runIDInput.PromptStyle = noStyle
		m.runIDInput.TextStyle = noStyle
	}

	if m.focused(focusFirmware) {
		cmd := m.firmwareInput.Focus()
		m.firmwareInput.PromptStyle = focusedStyle
		m.firmwareInput.TextStyle = focusedStyle
		cmds = appendIfNotNil(cmds, cmd)
	} else {
		m.firmwareInput.Blur()
		m.firmwareInput.PromptStyle = noStyle
		m.firmwareInput.TextStyle = noStyle
	}

	return tea.Batch(cmds...)
}

func (m *Model) moveFocus(delta int) {
	order := m.focusOrder()
	if len(order) == 0 {
		return
	}

	m.focusIndex = (m.focusIndex + delta) % len(order)
	if m.focusIndex < 0 {
		m.focusIndex += len(order)
	}
}

func (m *Model) focusOrder() []focusTarget {
	order := []focusTarget{focusTabs, focusPort}
	if m.tabs[m.activeTab].requiresBaud {
		order = append(order, focusBaud)
	}
	order = append(order,
		focusRunID,
		focusFirmware,
		focusFormat,
		focusUploadMode,
		focusUploadToggle,
		focusSubmit,
	)
	return order
}

func (m *Model) currentFocus() focusTarget {
	order := m.focusOrder()
	if len(order) == 0 {
		return focusTabs
	}
	if m.focusIndex >= len(order) {
		m.focusIndex = len(order) - 1
	}
	if m.focusIndex < 0 {
		m.focusIndex = 0
	}
	return order[m.focusIndex]
}

func (m *Model) focused(target focusTarget) bool {
	return m.currentFocus() == target
}

func (m *Model) refreshViewportHeight() {
	if m.height == 0 {
		return
	}
	form := m.formView()
	headers := lipgloss.Height(labelStyle.Render("Status")) + lipgloss.Height(labelStyle.Render("Log"))
	footer := lipgloss.Height(helpStyle.Render("Left/Right to choose driver • Tab to navigate • Esc to quit"))
	used := lipgloss.Height(form) + headers + footer
	space := m.height - used
	if space < 2 {
		space = 2
	}
	statusHeight := 1
	if space > 2 {
		statusHeightCandidate := space / 3
		if statusHeightCandidate > 3 {
			statusHeightCandidate = 3
		}
		if statusHeightCandidate > statusHeight {
			statusHeight = statusHeightCandidate
		}
	}
	logHeight := space - statusHeight
	if logHeight < 1 {
		logHeight = 1
	}
	m.statusViewport.Height = statusHeight
	m.logViewport.Height = logHeight
}

func (m *Model) handleSelection(delta int) bool {
	switch m.currentFocus() {
	case focusFormat:
		prev := m.formatField.selected
		if delta > 0 {
			m.formatField.next()
		} else {
			m.formatField.prev()
		}
		if prev != m.formatField.selected {
			m.appendStatus(fmt.Sprintf("Output format set to %s", m.formatField.value()))
		}
		return true
	case focusUploadMode:
		prev := m.uploadModeField.selected
		if delta > 0 {
			m.uploadModeField.next()
		} else {
			m.uploadModeField.prev()
		}
		if prev != m.uploadModeField.selected {
			mode := m.uploadModeField.value()
			m.appendStatus(fmt.Sprintf("Upload mode set to %s", mode))
			if mode == "Offline" && m.autoUpload {
				m.autoUpload = false
				m.appendStatus("Auto upload disabled in offline mode.")
			}
		}
		return true
	}
	return false
}

func (m *Model) buildConfig() (capture.Config, error) {
	driver := m.tabs[m.activeTab].name
	cfg := capture.Config{
		Driver:       driver,
		OutputFormat: m.formatField.value(),
		UploadMode:   m.uploadModeField.value(),
		AutoUpload:   m.autoUpload && m.uploadModeField.value() != "Offline",
		OutputDir:    capture.DefaultOutputDir,
	}

	port := strings.TrimSpace(m.portInput.Value())
	if driver == "Serial" {
		if port == "" {
			return cfg, fmt.Errorf("serial port is required")
		}
		cfg.Port = port
		baudStr := strings.TrimSpace(m.baudInput.Value())
		if baudStr == "" {
			return cfg, fmt.Errorf("baud rate is required for serial captures")
		}
		baud, err := strconv.Atoi(baudStr)
		if err != nil {
			return cfg, fmt.Errorf("invalid baud rate %q", baudStr)
		}
		cfg.Baud = baud
	} else {
		cfg.Port = port
	}

	runID := strings.TrimSpace(m.runIDInput.Value())
	if runID == "" {
		runID = fmt.Sprintf("run-%s", time.Now().UTC().Format("20060102T150405"))
	}
	cfg.RunID = runID

	cfg.FirmwareHash = strings.TrimSpace(m.firmwareInput.Value())

	return cfg, nil
}

func (m *Model) appendStatus(msg string) {
	timestamp := time.Now().Format("15:04:05")
	formatted := fmt.Sprintf("[%s] %s", timestamp, msg)
	m.statusLines = append(m.statusLines, formatted)
	if len(m.statusLines) > 1 {
		m.statusLines = m.statusLines[len(m.statusLines)-1:]
	}
	m.syncStatus()
}

func (m *Model) appendLog(msg string) {
	timestamp := time.Now().Format("15:04:05")
	formatted := fmt.Sprintf("[%s] %s", timestamp, msg)
	m.logLines = append(m.logLines, formatted)
	m.syncLogs()
}

func (m *Model) syncStatus() {
	content := strings.Join(m.statusLines, "\n")
	if m.statusViewport.Width > 0 {
		content = lipgloss.NewStyle().Width(m.statusViewport.Width).Render(content)
	}
	m.statusViewport.SetContent(content)
	m.statusViewport.GotoBottom()
}

func (m *Model) syncLogs() {
	content := strings.Join(m.logLines, "\n")
	if m.logViewport.Width > 0 {
		content = lipgloss.NewStyle().Width(m.logViewport.Width).Render(content)
	}
	m.logViewport.SetContent(content)
	m.logViewport.GotoBottom()
}

func (m *Model) portLabel() string {
	switch m.tabs[m.activeTab].name {
	case "Serial":
		return "Serial Port"
	case "TCP Socket":
		return "Socket Endpoint"
	case "File Replay":
		return "Capture File"
	default:
		return "Port"
	}
}

func (m *Model) updatePortPlaceholder() {
	if len(m.tabs) == 0 {
		return
	}
	placeholder := m.tabs[m.activeTab].placeholder
	m.portInput.Placeholder = placeholder
	if m.portInput.Value() == "" && placeholder != "" {
		m.portInput.SetValue(placeholder)
	}
}

func (m *Model) autoUploadView() string {
	label := "Auto Upload"
	if m.uploadModeField.value() == "Offline" {
		label = "Auto Upload (disabled in offline mode)"
	}

	state := "Off"
	style := blurredStyle
	if m.autoUpload && m.uploadModeField.value() != "Offline" {
		state = "On"
	}

	if m.focused(focusUploadToggle) {
		style = focusedStyle
	}

	return fmt.Sprintf("%s: %s", labelStyle.Render(label), style.Render(state))
}

func (m Model) submitButtonView() string {
	label := "Start Capture"
	style := blurredStyle
	if m.capturing {
		label = "Capturing..."
		style = blurredStyle
	} else if m.focused(focusSubmit) {
		style = focusedStyle
	}

	return style.Render(fmt.Sprintf("[ %s ]", label))
}

func (m *Model) onTabChanged() {
	m.updatePortPlaceholder()
	m.appendStatus(fmt.Sprintf("Capture driver set to %s", m.tabs[m.activeTab].name))
	m.focusIndex = 0
	m.refreshViewportHeight()
}

func (m Model) renderTabs() string {
	var rendered []string
	for i, tab := range m.tabs {
		style := inactiveTabStyle
		if i == m.activeTab {
			style = activeTabStyle
		}
		if m.focused(focusTabs) && i == m.activeTab {
			style = style.Copy().Bold(true)
		}
		border, _, _, _, _ := style.GetBorder()
		isFirst := i == 0

		border.BottomLeft = "┴"
		border.BottomRight = "┴"
		if isFirst {
			border.BottomLeft = "╰"
		}
		style = style.Border(border)
		rendered = append(rendered, style.Render(tab.name))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

func mergeTabsAndFrame(tabs, frame string) string {
	if tabs == "" {
		return frame
	}
	if frame == "" {
		return tabs
	}

	tabLines := strings.Split(tabs, "\n")
	frameLines := strings.Split(frame, "\n")
	if len(tabLines) == 0 {
		return frame
	}
	if len(frameLines) == 0 {
		return tabs
	}

	tabLines[len(tabLines)-1] = overlayLine(tabLines[len(tabLines)-1], frameLines[0])
	mergedLines := append(tabLines, frameLines[1:]...)
	return strings.Join(mergedLines, "\n")
}

func overlayLine(topOverride, base string) string {
	topRunes := []rune(topOverride)
	baseRunes := []rune(base)

	max := len(baseRunes)
	if len(topRunes) > max {
		max = len(topRunes)
	}

	result := make([]rune, max)
	for i := 0; i < max; i++ {
		var topRune rune = ' '
		if i < len(topRunes) {
			topRune = topRunes[i]
		}
		var baseRune rune = ' '
		if i < len(baseRunes) {
			baseRune = baseRunes[i]
		}
		if topRune != ' ' {
			result[i] = topRune
			continue
		}
		result[i] = baseRune
	}

	return string(result)
}

func startCaptureCommand(cfg capture.Config) tea.Cmd {
	return func() tea.Msg {
		result, err := capture.Run(cfg)
		return captureResultMsg{result: result, err: err}
	}
}

func (f *selectField) next() {
	if len(f.options) == 0 {
		return
	}
	f.selected = (f.selected + 1) % len(f.options)
}

func (f *selectField) prev() {
	if len(f.options) == 0 {
		return
	}
	f.selected--
	if f.selected < 0 {
		f.selected = len(f.options) - 1
	}
}

func (f selectField) value() string {
	if len(f.options) == 0 {
		return ""
	}
	if f.selected < 0 || f.selected >= len(f.options) {
		return ""
	}
	return f.options[f.selected]
}

func (f selectField) view(focused bool) string {
	if len(f.options) == 0 {
		return fmt.Sprintf("%s: -", labelStyle.Render(f.label))
	}

	parts := make([]string, len(f.options))
	for i, opt := range f.options {
		if i == f.selected {
			if focused {
				parts[i] = focusedStyle.Copy().Underline(true).Render(opt)
			} else {
				parts[i] = selectedStyle.Render(opt)
			}
			continue
		}
		parts[i] = blurredStyle.Render(opt)
	}

	return fmt.Sprintf("%s: %s", labelStyle.Render(f.label), strings.Join(parts, "  "))
}

func boolLabel(v bool) string {
	if v {
		return "enabled"
	}
	return "disabled"
}

func appendIfNotNil(cmds []tea.Cmd, cmd tea.Cmd) []tea.Cmd {
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	return cmds
}
