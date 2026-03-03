package tui

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/runs"
)

var (
	runsTableContainerStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(ColorMuted).
				Padding(0, 1)

	runsDetailContainerStyle = lipgloss.NewStyle().
					BorderStyle(lipgloss.NormalBorder()).
					BorderForeground(ColorPrimary).
					Padding(1, 2)
)

const maxPayloadBytes int64 = 2 * 1024 * 1024

type runsViewMode int

const (
	modeTable runsViewMode = iota
	modeDetail
	modePayload
)

type runsLoadedMsg struct {
	result runs.Result
	err    error
}

type payloadLoadedMsg struct {
	path         string
	original     string
	content      string
	err          error
	truncated    bool
	totalBytes   int64
	readBytes    int64
	lines        int
	checkedPaths []string
}

type runsViewModel struct {
	cacheRoot string
	result    runs.Result
	err       error
	loading   bool

	table table.Model

	width  int
	height int

	mode                runsViewMode
	detailSummary       runs.Summary
	payloadViewport     viewport.Model
	payloadContent      string
	payloadLoading      bool
	payloadErr          error
	payloadTruncated    bool
	payloadTotalBytes   int64
	payloadReadBytes    int64
	payloadLineCount    int
	payloadPath         string
	payloadOriginal     string
	payloadCheckedPaths []string
}

// RunRunsViewer launches the runs viewer TUI.
func RunRunsViewer(status *StatusMessage) (*StatusMessage, error) {
	cfg := config.Load()
	model := &runsViewModel{
		cacheRoot:       cfg.OfflineCache,
		loading:         true,
		table:           newRunsTable(nil),
		mode:            modeTable,
		payloadViewport: viewport.New(0, 0),
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return status, err
	}

	if model, ok := finalModel.(*runsViewModel); ok {
		switch {
		case model.err != nil:
			status = NewStatusMessage(StatusError, "Load Runs Failed", model.err.Error())
		case len(model.result.Runs) == 0:
			status = NewStatusMessage(StatusInfo, "No Runs", "No cached runs available yet. Capture data to populate this view.")
		default:
			status = NewStatusMessage(StatusSuccess, "Runs Available", fmt.Sprintf("%d cached run(s) available.", len(model.result.Runs)))
		}
	}
	return status, nil
}

func (m *runsViewModel) Init() tea.Cmd {
	return loadRuns(m.cacheRoot)
}

func (m *runsViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeTable()
		m.resizePayloadViewport()
		return m, nil

	case runsLoadedMsg:
		m.loading = false
		m.err = msg.err
		m.result = msg.result
		if msg.err != nil {
			m.table = newRunsTable(nil)
			m.mode = modeTable
			return m, nil
		}

		selectedID := m.selectedRunID()
		rows := buildRunRows(msg.result.Runs)
		tbl := newRunsTable(rows)
		if selectedID != "" {
			if idx := findRunIndex(msg.result.Runs, selectedID); idx >= 0 {
				tbl.SetCursor(idx)
			}
		}
		m.table = tbl
		m.resizeTable()

		if m.mode == modeDetail || m.mode == modePayload {
			if summary := findSummaryByID(msg.result.Runs, m.detailSummary.ID); summary != nil {
				m.detailSummary = *summary
				if m.mode == modePayload {
					return m, m.reloadPayload()
				}
			} else {
				m.mode = modeTable
			}
		}
		return m, nil

	case payloadLoadedMsg:
		if m.mode != modePayload || msg.original != m.payloadOriginal {
			return m, nil
		}
		m.payloadPath = msg.path
		m.payloadCheckedPaths = msg.checkedPaths
		m.payloadLoading = false
		m.payloadErr = msg.err
		m.payloadTruncated = msg.truncated
		m.payloadTotalBytes = msg.totalBytes
		m.payloadReadBytes = msg.readBytes
		m.payloadLineCount = msg.lines
		if msg.err == nil {
			m.payloadContent = msg.content
			m.payloadViewport.SetContent(msg.content)
			m.payloadViewport.GotoTop()
		} else {
			m.payloadContent = ""
			m.payloadViewport.SetContent("")
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleRunsKey(msg)
	}

	switch m.mode {
	case modeTable:
		if m.loading || m.err != nil {
			return m, nil
		}
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	case modePayload:
		var cmd tea.Cmd
		m.payloadViewport, cmd = m.payloadViewport.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m *runsViewModel) View() string {
	if m.loading {
		content := []string{
			StyleTitle.Render("📊 Cached Runs"),
			"",
			"Loading runs...",
			StyleHelp.Render("Press q to return"),
		}
		return m.centerContent(strings.Join(content, "\n"))
	}

	if m.err != nil {
		content := []string{
			StyleTitle.Render("📊 Cached Runs"),
			"",
			StyleError.Render(fmt.Sprintf("Failed to load runs: %v", m.err)),
			StyleHelp.Render("Press r to retry • q to return"),
		}
		return m.centerContent(strings.Join(content, "\n"))
	}

	var sections []string
	sections = append(sections, StyleTitle.Render("📊 Cached Runs"))
	sections = append(sections, StyleHelp.Render(fmt.Sprintf("Cache root: %s", m.result.Root)))
	switch m.mode {
	case modeDetail:
		sections = append(sections, renderRunDetails(m.detailSummary))
		sections = append(sections, StyleHelp.Render("d: view data • esc/q: back"))
		return m.centerContent(strings.Join(sections, "\n\n"))
	case modePayload:
		sections = append(sections, m.renderPayloadView())
		sections = append(sections, StyleHelp.Render("↑/↓ scroll • pgup/pgdn • home/end • r: reload • esc/q: back"))
		return m.centerContent(strings.Join(sections, "\n\n"))
	}

	tableView := runsTableContainerStyle.Render(m.table.View())
	sections = append(sections, tableView)

	if len(m.result.Warnings) > 0 {
		var warns []string
		warns = append(warns, StyleError.Render("Warnings:"))
		for _, warn := range m.result.Warnings {
			warns = append(warns, fmt.Sprintf("  • %s - %s", warn.RunDir, warn.Reason))
		}
		sections = append(sections, strings.Join(warns, "\n"))
	}

	if len(m.result.Runs) == 0 {
		sections = append(sections, StyleHelp.Render("r: refresh • q/esc: return"))
	} else {
		sections = append(sections, StyleHelp.Render("↑/↓ navigate • enter: details • r refresh • q: return"))
	}

	return m.centerContent(strings.Join(sections, "\n\n"))
}

func (m *runsViewModel) handleRunsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeTable:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "r":
			m.loading = true
			m.err = nil
			return m, loadRuns(m.cacheRoot)
		case "enter":
			if m.loading || m.err != nil {
				return m, nil
			}
			if summary := m.selectedSummary(); summary != nil {
				m.mode = modeDetail
				m.detailSummary = *summary
			}
			return m, nil
		default:
			if m.loading || m.err != nil {
				return m, nil
			}
			var cmd tea.Cmd
			m.table, cmd = m.table.Update(msg)
			return m, cmd
		}
	case modeDetail:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.mode = modeTable
			return m, nil
		case "d":
			return m, m.openPayloadView()
		case "r":
			m.loading = true
			m.err = nil
			return m, loadRuns(m.cacheRoot)
		default:
			return m, nil
		}
	case modePayload:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.mode = modeDetail
			return m, nil
		case "r":
			return m, m.reloadPayload()
		}
		var cmd tea.Cmd
		m.payloadViewport, cmd = m.payloadViewport.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m *runsViewModel) resizePayloadViewport() {
	if m.width == 0 || m.height == 0 {
		return
	}
	width := m.width - 6
	if width < 20 {
		width = 20
	}
	height := m.height - 12
	if height < 6 {
		height = 6
	}
	if m.payloadViewport.Width != width {
		m.payloadViewport.Width = width
	}
	if m.payloadViewport.Height != height {
		m.payloadViewport.Height = height
	}
}

func (m *runsViewModel) resizeTable() {
	if m.width == 0 || m.height == 0 {
		return
	}

	tableWidth := m.width - 6
	if tableWidth < 60 {
		tableWidth = 60
	}

	tableHeight := m.height - 14
	if tableHeight < 6 {
		tableHeight = 6
	}

	m.table.SetWidth(tableWidth)
	m.table.SetHeight(tableHeight)
}

func (m *runsViewModel) openPayloadView() tea.Cmd {
	m.mode = modePayload
	m.payloadOriginal = m.detailSummary.PrimaryData
	m.payloadPath = ""
	m.payloadCheckedPaths = nil
	m.payloadLoading = true
	m.payloadErr = nil
	m.payloadTruncated = false
	m.payloadTotalBytes = 0
	m.payloadReadBytes = 0
	m.payloadLineCount = 0
	m.payloadContent = ""
	m.payloadViewport.SetContent("")
	m.payloadViewport.GotoTop()
	m.resizePayloadViewport()
	return loadPayload(m.detailSummary)
}

func (m *runsViewModel) reloadPayload() tea.Cmd {
	m.payloadLoading = true
	m.payloadErr = nil
	m.payloadTruncated = false
	m.payloadTotalBytes = 0
	m.payloadReadBytes = 0
	m.payloadLineCount = 0
	m.payloadContent = ""
	m.payloadViewport.GotoTop()
	m.payloadOriginal = m.detailSummary.PrimaryData
	return loadPayload(m.detailSummary)
}

func (m *runsViewModel) renderPayloadView() string {
	keyStyle := StyleKey.Copy().Width(14)
	valueStyle := StyleValue.Copy()

	infoLines := []string{
		fmt.Sprintf("%s %s", keyStyle.Render("Run ID:"), valueStyle.Render(m.detailSummary.ID)),
	}

	displayPath := m.payloadPath
	if displayPath == "" {
		if m.payloadOriginal != "" {
			displayPath = m.payloadOriginal
		} else if m.detailSummary.RunDir != "" {
			displayPath = filepath.Join(m.detailSummary.RunDir, "data.jsonl")
		} else {
			displayPath = "(unknown)"
		}
	}
	infoLines = append(infoLines, fmt.Sprintf("%s %s", keyStyle.Render("Data File:"), valueStyle.Render(displayPath)))
	if m.payloadOriginal != "" && m.payloadOriginal != displayPath {
		infoLines = append(infoLines, StyleMuted.Render(fmt.Sprintf("Manifest path: %s", m.payloadOriginal)))
	}

	if m.detailSummary.DataSizeBytes > 0 {
		infoLines = append(infoLines, fmt.Sprintf("%s %s", keyStyle.Render("Size:"), valueStyle.Render(formatBytes(m.detailSummary.DataSizeBytes))))
	}

	if m.payloadLineCount > 0 {
		infoLines = append(infoLines, fmt.Sprintf("%s %d", keyStyle.Render("Records:"), m.payloadLineCount))
	}

	if m.payloadTruncated {
		infoLines = append(infoLines, StyleWarning.Render(fmt.Sprintf("Showing first %s of %s", formatBytes(m.payloadReadBytes), formatBytes(m.payloadTotalBytes))))
	} else if m.payloadReadBytes > 0 && m.payloadTotalBytes > 0 {
		infoLines = append(infoLines, StyleMuted.Render(fmt.Sprintf("Loaded %s", formatBytes(m.payloadReadBytes))))
	}
	if m.payloadErr != nil && len(m.payloadCheckedPaths) > 0 {
		paths := make([]string, len(m.payloadCheckedPaths))
		for i, p := range m.payloadCheckedPaths {
			if strings.TrimSpace(p) == "" {
				paths[i] = "(unspecified)"
			} else {
				paths[i] = p
			}
		}
		infoLines = append(infoLines, StyleMuted.Render(fmt.Sprintf("Checked paths: %s", strings.Join(paths, ", "))))
	}

	infoBox := runsDetailContainerStyle.Render(strings.Join(infoLines, "\n"))

	var content string
	switch {
	case m.payloadLoading:
		content = StyleMuted.Render("Loading payload...")
	case m.payloadErr != nil:
		content = StyleError.Render(fmt.Sprintf("Failed to load payload: %v", m.payloadErr))
	case m.payloadContent == "":
		content = StyleMuted.Render("Payload is empty.")
	default:
		content = m.payloadViewport.View()
	}

	payloadBox := runsDetailContainerStyle.Render(content)

	return strings.Join([]string{infoBox, payloadBox}, "\n\n")
}

func payloadCandidates(summary runs.Summary) []string {
	add := func(slice []string, val string, seen map[string]struct{}) []string {
		if val == "" {
			return slice
		}
		if _, ok := seen[val]; ok {
			return slice
		}
		seen[val] = struct{}{}
		return append(slice, val)
	}

	seen := make(map[string]struct{})
	var candidates []string
	if summary.PrimaryData != "" {
		candidates = add(candidates, summary.PrimaryData, seen)
		if summary.RunDir != "" {
			candidates = add(candidates, filepath.Join(summary.RunDir, filepath.Base(summary.PrimaryData)), seen)
		}
	}
	if summary.RunDir != "" {
		candidates = add(candidates, filepath.Join(summary.RunDir, "data.jsonl"), seen)
	}
	return candidates
}

func (m *runsViewModel) centerContent(content string) string {
	if m.width == 0 || m.height == 0 {
		return content
	}
	return lipgloss.PlaceVertical(
		m.height,
		lipgloss.Center,
		lipgloss.PlaceHorizontal(m.width, lipgloss.Center, content),
	)
}

func loadPayload(summary runs.Summary) tea.Cmd {
	return func() tea.Msg {
		candidates := payloadCandidates(summary)
		if len(candidates) == 0 {
			candidates = []string{""}
		}

		for _, candidate := range candidates {
			if strings.TrimSpace(candidate) == "" {
				continue
			}

			info, err := os.Stat(candidate)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return payloadLoadedMsg{
					original:     summary.PrimaryData,
					path:         candidate,
					err:          err,
					checkedPaths: append([]string(nil), candidates...),
				}
			}

			if info.IsDir() {
				return payloadLoadedMsg{
					original:     summary.PrimaryData,
					path:         candidate,
					err:          fmt.Errorf("data path %q is a directory", candidate),
					checkedPaths: append([]string(nil), candidates...),
				}
			}

			limit := info.Size()
			if limit > maxPayloadBytes {
				limit = maxPayloadBytes
			}

			content, readBytes, lines, hitLimit, readErr := readPayloadPreview(candidate, limit)
			if readErr != nil {
				return payloadLoadedMsg{
					original:     summary.PrimaryData,
					path:         candidate,
					err:          readErr,
					checkedPaths: append([]string(nil), candidates...),
				}
			}

			truncated := hitLimit || info.Size() > limit

			return payloadLoadedMsg{
				original:     summary.PrimaryData,
				path:         candidate,
				content:      content,
				totalBytes:   info.Size(),
				readBytes:    readBytes,
				lines:        lines,
				truncated:    truncated,
				checkedPaths: append([]string(nil), candidates...),
			}
		}

		return payloadLoadedMsg{
			original:     summary.PrimaryData,
			path:         fallbackDataPath(summary, candidates),
			err:          fmt.Errorf("data file not found (checked %d path(s))", len(candidates)),
			checkedPaths: append([]string(nil), candidates...),
		}
	}
}

func readPayloadPreview(path string, limit int64) (string, int64, int, bool, error) {
	if limit <= 0 {
		return "", 0, 0, false, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return "", 0, 0, false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 256*1024)
	scanner.Buffer(buf, 2*1024*1024)

	var (
		builder   strings.Builder
		bytesRead int64
		lines     int
		hitLimit  bool
	)

	for scanner.Scan() {
		line := scanner.Bytes()
		lineSize := int64(len(line)) + 1 // account for newline
		if bytesRead+lineSize > limit {
			hitLimit = true
			break
		}

		bytesRead += lineSize
		lines++

		builder.WriteString(formatPayloadLine(lines, line))
		builder.WriteByte('\n')
	}

	if err := scanner.Err(); err != nil {
		return builder.String(), bytesRead, lines, hitLimit, err
	}

	return builder.String(), bytesRead, lines, hitLimit, nil
}

func formatPayloadLine(index int, raw []byte) string {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return fmt.Sprintf("[%d] decode error: %v", index, err)
	}

	ts := "<missing>"
	if val, ok := obj["ts"]; ok {
		ts = formatScalarValue(val)
	}

	payload := "<missing>"
	if val, ok := obj["payload"]; ok {
		payload = formatJSONValue(val)
	}

	return fmt.Sprintf("[%d] ts=%s payload=%s", index, ts, payload)
}

func formatScalarValue(v any) string {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case string:
		return val
	case fmt.Stringer:
		return val.String()
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return strings.Trim(string(data), "\"")
	}
}

func formatJSONValue(v any) string {
	if v == nil {
		return "null"
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}

func fallbackDataPath(summary runs.Summary, candidates []string) string {
	if summary.PrimaryData != "" {
		return summary.PrimaryData
	}
	if summary.RunDir != "" {
		return filepath.Join(summary.RunDir, "data.jsonl")
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func loadRuns(root string) tea.Cmd {
	return func() tea.Msg {
		result, err := runs.List(root)
		return runsLoadedMsg{result: result, err: err}
	}
}

func (m *runsViewModel) selectedSummary() *runs.Summary {
	if len(m.result.Runs) == 0 {
		return nil
	}
	cursor := m.table.Cursor()
	if cursor < 0 || cursor >= len(m.result.Runs) {
		cursor = 0
	}
	return &m.result.Runs[cursor]
}

func (m *runsViewModel) selectedRunID() string {
	summary := m.selectedSummary()
	if summary == nil {
		return ""
	}
	return summary.ID
}

func newRunsTable(rows []table.Row) table.Model {
	columns := []table.Column{
		{Title: "Run ID", Width: 24},
		{Title: "Started", Width: 19},
		{Title: "Duration", Width: 10},
		{Title: "Records", Width: 8},
		{Title: "Source", Width: 18},
		{Title: "Test Plan", Width: 18},
	}

	// sensible row count
	height := 10
	if n := len(rows); n > 0 {
		if n+2 < height {
			height = n + 2
		}
		if height < 6 {
			height = 6
		}
	}

	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
	)

	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorMuted).
		BorderBottom(true).
		Bold(false)
	styles.Selected = lipgloss.NewStyle().
		Foreground(ColorText).
		Background(ColorSecondary).
		Bold(true)
	styles.Cell = lipgloss.NewStyle().
		Foreground(ColorText)
	tbl.SetStyles(styles)
	tbl.Focus()
	return tbl
}

func buildRunRows(summaries []runs.Summary) []table.Row {
	rows := make([]table.Row, 0, len(summaries))
	for _, summary := range summaries {
		row := table.Row{
			summary.ID,
			summary.Started.Local().Format("2006-01-02 15:04:05"),
			runs.FormatDuration(summary.DurationSeconds, summary.Completed != nil),
			fmt.Sprintf("%d", summary.Records),
			runs.FormatSource(summary.Source),
			runs.FormatTestPlan(summary.Manifest.Test),
		}
		rows = append(rows, row)
	}
	return rows
}

func findRunIndex(summaries []runs.Summary, id string) int {
	for i, summary := range summaries {
		if summary.ID == id {
			return i
		}
	}
	return -1
}

func findSummaryByID(summaries []runs.Summary, id string) *runs.Summary {
	for i := range summaries {
		if summaries[i].ID == id {
			return &summaries[i]
		}
	}
	return nil
}

func renderRunDetails(summary runs.Summary) string {
	keyStyle := StyleKey.Copy().Width(14)
	valueStyle := StyleValue.Copy()
	lines := []string{
		StyleHeader.Render("Run Details"),
		fmt.Sprintf("%s %s", keyStyle.Render("Run ID:"), valueStyle.Render(summary.ID)),
		fmt.Sprintf("%s %s", keyStyle.Render("Started:"), valueStyle.Render(summary.Started.Local().Format("2006-01-02 15:04:05"))),
		fmt.Sprintf("%s %s", keyStyle.Render("Completed:"), valueStyle.Render(formatCompleted(summary.Completed))),
		fmt.Sprintf("%s %s", keyStyle.Render("Duration:"), valueStyle.Render(runs.FormatDuration(summary.DurationSeconds, summary.Completed != nil))),
		fmt.Sprintf("%s %s", keyStyle.Render("Source:"), valueStyle.Render(runs.FormatSource(summary.Source))),
		fmt.Sprintf("%s %s", keyStyle.Render("Records:"), valueStyle.Render(fmt.Sprintf("%d", summary.Records))),
		fmt.Sprintf("%s %s", keyStyle.Render("Test:"), valueStyle.Render(runs.FormatTestPlan(summary.Manifest.Test))),
	}

	if summary.Manifest.Test.Run != "" {
		lines = append(lines, fmt.Sprintf("%s %s", keyStyle.Render("Test Run:"), valueStyle.Render(summary.Manifest.Test.Run)))
	}

	if summary.Manifest.Operator != "" {
		lines = append(lines, fmt.Sprintf("%s %s", keyStyle.Render("Operator:"), valueStyle.Render(summary.Manifest.Operator)))
	}
	if summary.Manifest.Location != "" {
		lines = append(lines, fmt.Sprintf("%s %s", keyStyle.Render("Location:"), valueStyle.Render(summary.Manifest.Location)))
	}

	device := summary.Manifest.Device
	if device.ID != "" {
		lines = append(lines, fmt.Sprintf("%s %s", keyStyle.Render("Device ID:"), valueStyle.Render(device.ID)))
	}
	if device.Serial != "" {
		lines = append(lines, fmt.Sprintf("%s %s", keyStyle.Render("Device SN:"), valueStyle.Render(device.Serial)))
	}
	if device.Firmware != "" {
		fw := device.Firmware
		if device.FirmwareHash != "" {
			fw = fmt.Sprintf("%s (%s)", fw, device.FirmwareHash)
		}
		lines = append(lines, fmt.Sprintf("%s %s", keyStyle.Render("Firmware:"), valueStyle.Render(fw)))
	}
	if device.HardwareVersion != "" {
		lines = append(lines, fmt.Sprintf("%s %s", keyStyle.Render("Hardware:"), valueStyle.Render(device.HardwareVersion)))
	}

	if len(summary.Manifest.Tags) > 0 {
		lines = append(lines, fmt.Sprintf("%s %s", keyStyle.Render("Tags:"), valueStyle.Render(strings.Join(summary.Manifest.Tags, ", "))))
	}

	if len(summary.Manifest.Attributes) > 0 {
		lines = append(lines, keyStyle.Render("Attributes:"))
		for _, attr := range renderAttributes(summary.Manifest.Attributes) {
			lines = append(lines, fmt.Sprintf("  %s", attr))
		}
	}

	if summary.Capture.Notes != "" {
		lines = append(lines, fmt.Sprintf("%s %s", keyStyle.Render("Notes:"), valueStyle.Render(summary.Capture.Notes)))
	}

	if len(summary.Capture.Channels) > 0 {
		lines = append(lines, fmt.Sprintf("%s %s", keyStyle.Render("Channels:"), valueStyle.Render(strings.Join(summary.Capture.Channels, ", "))))
	}

	dataLine := fmt.Sprintf("%s %s", keyStyle.Render("Data:"), valueStyle.Render(summary.PrimaryData))
	if summary.DataSizeBytes > 0 {
		dataLine = fmt.Sprintf("%s %s", keyStyle.Render("Data:"), valueStyle.Render(fmt.Sprintf("%s (%s)", summary.PrimaryData, formatBytes(summary.DataSizeBytes))))
	}
	lines = append(lines, dataLine)
	lines = append(lines, fmt.Sprintf("%s %s", keyStyle.Render("Run Dir:"), valueStyle.Render(summary.RunDir)))

	return runsDetailContainerStyle.Render(strings.Join(lines, "\n"))
}

func renderAttributes(attrs map[string]string) []string {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s: %s", k, attrs[k]))
	}
	return lines
}

func formatCompleted(ts *time.Time) string {
	if ts == nil {
		return "-"
	}
	return ts.Local().Format("2006-01-02 15:04:05")
}

func formatBytes(size int64) string {
	const (
		_          = iota
		KB float64 = 1 << (10 * iota)
		MB
		GB
	)
	bytes := float64(size)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", bytes/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", bytes/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", bytes/KB)
	default:
		return fmt.Sprintf("%d B", size)
	}
}

