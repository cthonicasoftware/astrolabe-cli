package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Layout constants for viewport sizing
const (
	headerFooterHeight    = 8
	marginWidth           = 8
	minViewportWidth      = 40
	minViewportHeight     = 10
	tickInterval          = 250 * time.Millisecond
	spotlightMinIntensity = 0.3
	spotlightFocusRadius  = 0.4
)

const ansiEscapePrefix = "\x1b["

// keymap defines the key bindings for the application
type keymap struct {
	Quit  key.Binding
	Help  key.Binding
	Pause key.Binding
	Clear key.Binding
	Spot  key.Binding
}

func (k keymap) ShortHelp() []key.Binding { return []key.Binding{k.Quit, k.Pause, k.Spot, k.Help} }
func (k keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Quit, k.Pause, k.Spot, k.Clear, k.Help},
	}
}

var keys = keymap{
	Quit:  key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/ctrl+c", "quit")),
	Help:  key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
	Pause: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "pause/resume capture")),
	Clear: key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "clear buffer")),
	Spot:  key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "toggle spotlight")),
}

var faintLineStyle = lipgloss.NewStyle().Faint(true)

// App represents the live capture TUI application
type App struct {
	title            string
	vp               viewport.Model
	help             help.Model
	paused           bool
	feed             <-chan string
	lineBuffer       *LineBuffer
	lastTick         time.Time
	width            int
	height           int
	boxStyle         lipgloss.Style
	spotlightEnabled bool
}

// NewApp creates a new App instance
func NewApp(title string, feed <-chan string) App {
	vp := viewport.New(0, 0)
	vp.SetContent("")
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3
	vp.HighPerformanceRendering = false
	vp.Style = lipgloss.NewStyle()

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2)

	return App{
		title:            title,
		vp:               vp,
		help:             help.New(),
		feed:             feed,
		boxStyle:         boxStyle,
		lineBuffer:       NewLineBuffer(),
		spotlightEnabled: false,
	}
}

// Message types
type tickMsg time.Time
type lineMsg string

// Init initializes the app
func (a App) Init() tea.Cmd {
	return tea.Batch(a.tickCmd(), a.pullLine())
}

// Update handles messages and updates the model
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		return a.handleWindowResize(m)
	case tickMsg:
		return a.handleTick(m)
	case lineMsg:
		return a.handleLineData(m)
	case tea.KeyMsg:
		return a.handleKeyPress(m)
	}

	// Pass other messages to viewport (like mouse events)
	var cmd tea.Cmd
	a.vp, cmd = a.vp.Update(msg)
	return a, cmd
}

// handleWindowResize handles terminal window resize events
func (a App) handleWindowResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	a.width = msg.Width
	a.height = msg.Height

	// Calculate viewport dimensions
	frameWidth := a.boxStyle.GetHorizontalFrameSize()
	frameHeight := a.boxStyle.GetVerticalFrameSize()

	a.vp.Width = msg.Width - marginWidth - frameWidth - 1
	a.vp.Height = msg.Height - headerFooterHeight - frameHeight

	// Apply minimum dimensions
	if a.vp.Width < minViewportWidth {
		a.vp.Width = minViewportWidth
	}
	if a.vp.Height < minViewportHeight {
		a.vp.Height = minViewportHeight
	}

	// Reset position and re-wrap content
	a.vp.YPosition = 0
	a.updateViewportContent()
	a.vp.GotoBottom()

	return a, nil
}

// handleTick handles periodic tick events
func (a App) handleTick(msg tickMsg) (tea.Model, tea.Cmd) {
	a.lastTick = time.Time(msg)

	if !a.paused {
		return a, tea.Batch(a.tickCmd(), a.pullLine())
	}
	return a, a.tickCmd()
}

// handleLineData handles incoming line data
func (a App) handleLineData(msg lineMsg) (tea.Model, tea.Cmd) {
	if a.paused {
		return a, nil
	}

	// Check if user is at the bottom before adding new data
	wasAtBottom := a.vp.AtBottom()

	// Process incoming data
	a.lineBuffer.AddData(string(msg))

	// Update viewport with current state
	a.updateViewportContent()

	// Only auto-scroll to bottom if user was already at bottom
	if wasAtBottom {
		a.vp.GotoBottom()
	}

	return a, a.pullLine()
}

// handleKeyPress handles keyboard input
func (a App) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return a, tea.Quit
	case key.Matches(msg, keys.Help):
		a.help.ShowAll = !a.help.ShowAll
		return a, nil
	case key.Matches(msg, keys.Pause):
		return a.togglePause()
	case key.Matches(msg, keys.Clear):
		return a.clearBuffer()
	case key.Matches(msg, keys.Spot):
		return a.toggleSpotlight()
	}

	// Pass unhandled keys to viewport for scrolling
	var cmd tea.Cmd
	a.vp, cmd = a.vp.Update(msg)
	return a, cmd
}

// togglePause toggles the pause state
func (a App) togglePause() (tea.Model, tea.Cmd) {
	a.paused = !a.paused
	if !a.paused {
		return a, a.pullLine()
	}
	return a, nil
}

// clearBuffer clears the line buffer and viewport
func (a App) clearBuffer() (tea.Model, tea.Cmd) {
	a.lineBuffer.Clear()
	a.vp.SetContent("")
	return a, nil
}

// updateViewportContent updates the viewport with wrapped content
func (a *App) updateViewportContent() {
	wrapper := NewLineWrapper(a.vp.Width)
	wrappedContent := wrapper.Wrap(a.lineBuffer.GetDisplayLines())

	// Bottom-align shorter content so new lines appear at the base of the viewport
	if a.vp.Height > 0 && wrappedContent != "" {
		lineCount := countLines(wrappedContent)
		if lineCount < a.vp.Height {
			padding := strings.Repeat("\n", a.vp.Height-lineCount)
			wrappedContent = padding + wrappedContent
		}
	}

	a.vp.SetContent(wrappedContent)
}

// tickCmd returns a command that sends a tick message
func (a App) tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// pullLine pulls the next line from the feed channel
func (a *App) pullLine() tea.Cmd {
	return func() tea.Msg {
		if a.feed == nil {
			return nil
		}
		s, ok := <-a.feed
		if !ok {
			return nil
		}
		return lineMsg(s)
	}
}

// View renders the application UI
func (a App) View() string {
	var s strings.Builder

	// Title
	s.WriteString(StyleTitle.Render(a.title))
	s.WriteString("\n\n")

	// Status line
	s.WriteString(a.renderStatus())
	s.WriteString("\n\n")

	// Viewport in bordered box
	s.WriteString(a.renderViewport())
	s.WriteString("\n\n")

	// Help text
	s.WriteString(a.renderHelp())

	// Center content
	content := s.String()
	return lipgloss.PlaceVertical(a.height, lipgloss.Top,
		lipgloss.PlaceHorizontal(a.width, lipgloss.Center, content))
}

// renderStatus renders the status line with metadata
func (a App) renderStatus() string {
	statusText := fmt.Sprintf("Lines: %d", a.lineBuffer.LineCount())

	if a.paused {
		statusText = StyleWarning.Render("⏸ PAUSED") + " • " + statusText
	} else {
		statusText = StyleSuccess.Render("● LIVE") + " • " + statusText
	}

	if !a.lastTick.IsZero() {
		statusText += " • " + StyleMuted.Render(a.lastTick.Format("15:04:05"))
	}

	if a.spotlightEnabled {
		statusText += " • " + StyleMuted.Render("Spotlight on")
	} else {
		statusText += " • " + StyleMuted.Render("Spotlight off")
	}

	return statusText
}

// renderViewport renders the viewport in a bordered box with fade effect
func (a App) renderViewport() string {
	// Get the visible content from viewport
	visibleContent := a.vp.View()

	// Apply fade effect to visible lines when enabled
	fadedContent := visibleContent
	if a.spotlightEnabled {
		fadedContent = a.applyFadeEffect(visibleContent)
	}

	vpContent := lipgloss.NewStyle().
		Width(a.vp.Width).
		Height(a.vp.Height).
		Render(fadedContent)
	return a.boxStyle.Render(vpContent)
}

// applyFadeEffect applies a gradient fade to the visible content
func (a App) applyFadeEffect(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}

	maxIndex := math.Max(float64(len(lines)-1), 1)

	scrollPercent := clampFloat(a.vp.ScrollPercent(), 0, 1)
	switch {
	case a.vp.AtTop() && !a.vp.AtBottom():
		scrollPercent = 0
	case a.vp.AtBottom() && !a.vp.AtTop():
		scrollPercent = 1
	case a.vp.AtTop() && a.vp.AtBottom():
		scrollPercent = 1
	}

	focus := scrollPercent
	focusRadius := math.Max(spotlightFocusRadius, 0.05)

	var result strings.Builder
	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		linePosition := clampFloat(float64(i)/maxIndex, 0, 1)
		distance := math.Abs(linePosition - focus)
		normalized := clampFloat(distance/focusRadius, 0, 1)
		focusIntensity := easeOutCubic(1 - normalized)

		intensity := spotlightMinIntensity + (1-spotlightMinIntensity)*focusIntensity
		intensity = clampFloat(intensity, spotlightMinIntensity, 1)

		result.WriteString(a.applyColorIntensity(line, intensity))
	}

	return result.String()
}

// applyColorIntensity applies color intensity to a line using ANSI color codes
func (a App) applyColorIntensity(line string, intensity float64) string {
	if line == "" {
		return line
	}

	if intensity >= 0.99 {
		return line
	}

	// Preserve pre-existing colored output by falling back to a faint style
	if strings.Contains(line, ansiEscapePrefix) {
		if intensity > 0.7 {
			return line
		}
		return faintLineStyle.Render(line)
	}

	// Map intensity to grayscale ANSI colors (232-255 are grayscale)
	// 232 = darkest, 255 = brightest (white)
	colorCode := max(min(232+int(math.Round(intensity*23)), 255), 232)

	return fmt.Sprintf("\x1b[38;5;%dm%s\x1b[0m", colorCode, line)
}

func easeOutCubic(t float64) float64 {
	inv := 1 - t
	return 1 - (inv * inv * inv)
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func countLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

// renderHelp renders the help text
func (a App) renderHelp() string {
	if a.help.ShowAll {
		return StyleHelp.Render(a.help.View(keys))
	}
	helpText := "space: pause/resume • s: toggle spotlight • ↑/↓/pgup/pgdn: scroll • ctrl+l: clear • ?: help • q: quit"
	return StyleHelp.Render(helpText)
}

// toggleSpotlight toggles the spotlight fade effect
func (a App) toggleSpotlight() (tea.Model, tea.Cmd) {
	a.spotlightEnabled = !a.spotlightEnabled
	return a, nil
}
