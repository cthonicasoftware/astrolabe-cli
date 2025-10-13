package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/config"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/runs"
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

type runsLoadedMsg struct {
	result runs.Result
	err    error
}

type runsViewModel struct {
	cacheRoot string
	result    runs.Result
	err       error
	loading   bool

	table table.Model

	width  int
	height int

	showDetail    bool
	detailSummary runs.Summary
}

// RunRunsViewer launches the runs viewer TUI.
func RunRunsViewer() error {
	cfg := config.Load()
	model := &runsViewModel{
		cacheRoot: cfg.OfflineCache,
		loading:   true,
		table:     newRunsTable(nil),
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m *runsViewModel) Init() tea.Cmd {
	return loadRuns(m.cacheRoot)
}

func (m *runsViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case runsLoadedMsg:
		m.loading = false
		m.err = msg.err
		m.result = msg.result
		if msg.err != nil {
			m.table = newRunsTable(nil)
			m.showDetail = false
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

		if m.showDetail {
			if summary := findSummaryByID(msg.result.Runs, m.detailSummary.ID); summary != nil {
				m.detailSummary = *summary
			} else {
				m.showDetail = false
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			if m.showDetail {
				m.showDetail = false
				return m, nil
			}
			return m, tea.Quit
		case "r":
			m.loading = true
			m.err = nil
			return m, loadRuns(m.cacheRoot)
		}
		if m.showDetail {
			switch msg.String() {
			case "enter", " ", "tab", "shift+tab":
				// Ignore to stay on detail view
				return m, nil
			}
			return m, nil
		}
		if m.loading || m.err != nil {
			return m, nil
		}
		if msg.String() == "enter" {
			if summary := m.selectedSummary(); summary != nil {
				m.showDetail = true
				m.detailSummary = *summary
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}

	if m.loading || m.err != nil {
		return m, nil
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *runsViewModel) View() string {
	if m.loading {
		content := []string{
			StyleTitle.Render("📊 Cached Runs"),
			"",
			"Loading runs...",
			StyleHelp.Render("Press q to return"),
		}
		return strings.Join(content, "\n")
	}

	if m.err != nil {
		content := []string{
			StyleTitle.Render("📊 Cached Runs"),
			"",
			StyleError.Render(fmt.Sprintf("Failed to load runs: %v", m.err)),
			StyleHelp.Render("Press r to retry • q to return"),
		}
		return strings.Join(content, "\n")
	}

	var sections []string
	sections = append(sections, StyleTitle.Render("📊 Cached Runs"))
	sections = append(sections, StyleHelp.Render(fmt.Sprintf("Cache root: %s", m.result.Root)))

	if m.showDetail {
		sections = append(sections, renderRunDetails(m.detailSummary))
		sections = append(sections, StyleHelp.Render("esc: back • q: return"))
		return strings.Join(sections, "\n\n")
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

	return strings.Join(sections, "\n\n")
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

	height := 8
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
