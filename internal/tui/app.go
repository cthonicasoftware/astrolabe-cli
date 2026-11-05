package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
)

// Layout constants for viewport sizing
const (
	headerFooterHeight = 8
	inputRowHeight     = 2 // input line plus the blank line above it
	marginWidth        = 8
	minViewportWidth   = 40
	minViewportHeight  = 10
	tickInterval       = 250 * time.Millisecond
	maxSendLength      = 1024
	maxHistory         = 100
)

// keymap defines the key bindings for the application
type keymap struct {
	Quit   key.Binding
	Save   key.Binding
	Help   key.Binding
	Pause  key.Binding
	Clear  key.Binding
	Spot   key.Binding
	Input  key.Binding
	Ending key.Binding
	Echo   key.Binding
	Send   key.Binding
	Leave  key.Binding
}

func (k keymap) ShortHelp() []key.Binding { return []key.Binding{k.Save, k.Pause, k.Spot, k.Help} }
func (k keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Quit, k.Save, k.Pause, k.Spot, k.Clear, k.Help},
		{k.Input, k.Send, k.Leave, k.Ending, k.Echo},
	}
}

var keys = keymap{
	Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/ctrl+c", "quit without saving")),
	Save:   key.NewBinding(key.WithKeys("ctrl+w"), key.WithHelp("ctrl+w", "stop & save run")),
	Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
	Pause:  key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "pause/resume capture")),
	Clear:  key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "clear buffer")),
	Spot:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "toggle spotlight")),
	Input:  key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "type to device")),
	Ending: key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "cycle line ending")),
	Echo:   key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "toggle echo")),
	Send:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send line")),
	Leave:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "stop typing")),
}

// SendFunc writes a line plus line ending to the device. It may block.
type SendFunc func(line string, le sources.LineEnding) error

// App represents the live capture TUI application
type App struct {
	title            string
	vp               viewport.Model
	help             help.Model
	paused           bool
	pending          []string // chunks received while paused, applied on resume
	saveRequested    bool
	feed             <-chan string
	lineBuffer       *LineBuffer
	lastTick         time.Time
	width            int
	height           int
	boxStyle         lipgloss.Style
	spotlightEnabled bool
	spotlight        *SpotlightEffect

	// Send path. send is nil when the source cannot accept input.
	send         SendFunc
	savePrefs    func(SendPrefs) error
	prefs        SendPrefs
	input        textinput.Model
	inputMode    bool
	sending      bool
	history      []string
	historyIdx   int    // == len(history) when not browsing
	draft        string // text typed before browsing history
	disconnected bool
	sendErr      string
}

// NewApp creates a new App instance
func NewApp(title string, feed <-chan string) *App {
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

	input := textinput.New()
	input.Prompt = "› "
	input.Placeholder = "type a line, enter to send, esc to stop"
	input.CharLimit = maxSendLength
	input.PromptStyle = lipgloss.NewStyle().Foreground(ColorPrimary)

	return &App{
		title:            title,
		vp:               vp,
		help:             help.New(),
		paused:           false,
		saveRequested:    false,
		feed:             feed,
		boxStyle:         boxStyle,
		lineBuffer:       NewLineBuffer(),
		spotlightEnabled: false,
		spotlight:        NewSpotlightEffect(),
		prefs:            DefaultSendPrefs(),
		input:            input,
	}
}

// EnableSend turns on the input field. savePrefs may be nil.
func (a *App) EnableSend(send SendFunc, prefs SendPrefs, savePrefs func(SendPrefs) error) {
	a.send = send
	a.prefs = prefs
	a.savePrefs = savePrefs
}

func (a *App) canSend() bool { return a.send != nil && !a.disconnected }

// Message types
type tickMsg time.Time
type lineMsg string

// feedClosedMsg reports that the source stopped (e.g. device unplugged).
type feedClosedMsg struct{}

// sendResultMsg reports the outcome of an asynchronous Send.
type sendResultMsg struct {
	line string
	err  error
}

// prefsSaveErrMsg reports a failure to persist send preferences.
type prefsSaveErrMsg struct{ err error }

// Init initializes the app
func (a *App) Init() tea.Cmd {
	return tea.Batch(a.tickCmd(), a.pullLine())
}

// Update handles messages and updates the model
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		return a.handleWindowResize(m)
	case tickMsg:
		return a.handleTick(m)
	case lineMsg:
		return a.handleLineData(m)
	case feedClosedMsg:
		return a.handleFeedClosed()
	case sendResultMsg:
		return a.handleSendResult(m)
	case prefsSaveErrMsg:
		a.sendErr = "could not save send settings: " + m.err.Error()
		return a, nil
	case tea.KeyMsg:
		if a.inputMode {
			return a.handleInputKey(m)
		}
		return a.handleKeyPress(m)
	}

	// Pass other messages to viewport (like mouse events)
	var cmd tea.Cmd
	a.vp, cmd = a.vp.Update(msg)
	return a, cmd
}

// handleWindowResize handles terminal window resize events
func (a *App) handleWindowResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	a.width = msg.Width
	a.height = msg.Height

	// Calculate viewport dimensions
	frameWidth := a.boxStyle.GetHorizontalFrameSize()
	frameHeight := a.boxStyle.GetVerticalFrameSize()

	reserved := headerFooterHeight
	if a.send != nil {
		reserved += inputRowHeight
	}

	a.vp.Width = msg.Width - marginWidth - frameWidth - 1
	a.vp.Height = msg.Height - reserved - frameHeight

	// Apply minimum dimensions
	if a.vp.Width < minViewportWidth {
		a.vp.Width = minViewportWidth
	}
	if a.vp.Height < minViewportHeight {
		a.vp.Height = minViewportHeight
	}

	a.input.Width = max(a.vp.Width-lipgloss.Width(a.input.Prompt), 10)

	// Reset position and re-wrap content
	a.vp.YPosition = 0
	a.updateViewportContent()
	a.vp.GotoBottom()

	return a, nil
}

// handleTick handles periodic tick events
func (a *App) handleTick(msg tickMsg) (tea.Model, tea.Cmd) {
	a.lastTick = time.Time(msg)
	// Don't pull from the feed here: exactly one pullLine must be in flight at
	// a time (started in Init, re-armed by handleLineData). Extra concurrent
	// readers race on the channel and deliver chunks out of order.
	return a, a.tickCmd()
}

// handleLineData handles incoming line data
func (a *App) handleLineData(msg lineMsg) (tea.Model, tea.Cmd) {
	// Always drain the channel to prevent blocking the pipeline. While paused,
	// hold chunks so resuming shows them instead of silently dropping data.
	if a.paused {
		a.pending = append(a.pending, string(msg))
	} else {
		a.appendData(string(msg))
	}

	// Always pull the next line, even when paused, to prevent blocking
	return a, a.pullLine()
}

// handleFeedClosed marks the device as gone. The run can still be saved.
func (a *App) handleFeedClosed() (tea.Model, tea.Cmd) {
	a.disconnected = true
	a.inputMode = false
	a.input.Blur()
	return a, nil
}

// appendData adds chunks to the line buffer and refreshes the viewport,
// following the tail only if the user was already at the bottom.
func (a *App) appendData(chunks ...string) {
	wasAtBottom := a.vp.AtBottom()
	for _, c := range chunks {
		a.lineBuffer.AddData(c)
	}
	a.updateViewportContent()
	if wasAtBottom {
		a.vp.GotoBottom()
	}
}

// handleKeyPress handles keyboard input in view mode
func (a *App) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return a.quit()
	case key.Matches(msg, keys.Save):
		return a.save()
	case key.Matches(msg, keys.Help):
		a.help.ShowAll = !a.help.ShowAll
		return a, nil
	case key.Matches(msg, keys.Pause):
		return a.togglePause()
	case key.Matches(msg, keys.Clear):
		return a.clearBuffer()
	case key.Matches(msg, keys.Spot):
		return a.toggleSpotlight()
	case key.Matches(msg, keys.Input) && a.canSend():
		a.inputMode = true
		return a, a.input.Focus()
	case key.Matches(msg, keys.Ending) && a.send != nil:
		a.prefs.LineEnding = a.prefs.LineEnding.Next()
		return a, a.persistPrefs()
	case key.Matches(msg, keys.Echo) && a.send != nil:
		a.prefs.Echo = !a.prefs.Echo
		return a, a.persistPrefs()
	}

	// Pass unhandled keys to viewport for scrolling
	var cmd tea.Cmd
	a.vp, cmd = a.vp.Update(msg)
	return a, cmd
}

// handleInputKey handles keys while the input field has focus. Bare keys
// like q and space go to the field; only chords and control keys act.
func (a *App) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		a.inputMode = false
		a.input.Blur()
		return a, nil
	case tea.KeyEnter:
		return a.submit()
	case tea.KeyUp:
		a.historyPrev()
		return a, nil
	case tea.KeyDown:
		a.historyNext()
		return a, nil
	case tea.KeyPgUp, tea.KeyPgDown:
		var cmd tea.Cmd
		a.vp, cmd = a.vp.Update(msg)
		return a, cmd
	}
	if key.Matches(msg, keys.Save) {
		return a.save()
	}

	var cmd tea.Cmd
	a.input, cmd = a.input.Update(msg)
	return a, cmd
}

// submit sends the current field text. Only one send runs at a time so
// lines reach the device in the order they were typed.
func (a *App) submit() (tea.Model, tea.Cmd) {
	if a.sending || !a.canSend() {
		return a, nil
	}
	line := a.input.Value()
	a.sending = true
	send, le := a.send, a.prefs.LineEnding
	return a, func() tea.Msg {
		return sendResultMsg{line: line, err: send(line, le)}
	}
}

func (a *App) handleSendResult(msg sendResultMsg) (tea.Model, tea.Cmd) {
	a.sending = false
	a.pushHistory(msg.line)
	if msg.err != nil {
		// Keep the text so the operator can retry.
		a.sendErr = msg.err.Error()
		return a, nil
	}
	a.sendErr = ""
	a.input.SetValue("")
	if a.prefs.Echo {
		wasAtBottom := a.vp.AtBottom()
		a.lineBuffer.AddEcho(msg.line)
		a.updateViewportContent()
		if wasAtBottom {
			a.vp.GotoBottom()
		}
	}
	return a, nil
}

func (a *App) pushHistory(line string) {
	if line != "" && (len(a.history) == 0 || a.history[len(a.history)-1] != line) {
		a.history = append(a.history, line)
		if len(a.history) > maxHistory {
			a.history = a.history[len(a.history)-maxHistory:]
		}
	}
	a.historyIdx = len(a.history)
	a.draft = ""
}

func (a *App) historyPrev() {
	if a.historyIdx == 0 {
		return
	}
	if a.historyIdx == len(a.history) {
		a.draft = a.input.Value()
	}
	a.historyIdx--
	a.input.SetValue(a.history[a.historyIdx])
	a.input.CursorEnd()
}

func (a *App) historyNext() {
	if a.historyIdx >= len(a.history) {
		return
	}
	a.historyIdx++
	if a.historyIdx == len(a.history) {
		a.input.SetValue(a.draft)
	} else {
		a.input.SetValue(a.history[a.historyIdx])
	}
	a.input.CursorEnd()
}

// persistPrefs saves send settings off the UI loop.
func (a *App) persistPrefs() tea.Cmd {
	if a.savePrefs == nil {
		return nil
	}
	save, prefs := a.savePrefs, a.prefs
	return func() tea.Msg {
		if err := save(prefs); err != nil {
			return prefsSaveErrMsg{err: err}
		}
		return nil
	}
}

func (a *App) quit() (tea.Model, tea.Cmd) {
	a.saveRequested = false
	return a, func() tea.Msg {
		return NavigateMsg{
			To:     ScreenWelcome,
			Status: NewStatusMessage(StatusInfo, "Discarded", "Capture exited without saving."),
		}
	}
}

func (a *App) save() (tea.Model, tea.Cmd) {
	a.saveRequested = true
	return a, func() tea.Msg {
		return NavigateMsg{
			To:     ScreenWelcome,
			Status: NewStatusMessage(StatusSuccess, "Saved", "Capture session saved successfully."),
		}
	}
}

// togglePause toggles the pause state
func (a *App) togglePause() (tea.Model, tea.Cmd) {
	a.paused = !a.paused
	// handleLineData keeps pulling while paused, so resuming needs no new
	// reader. Apply anything held while paused.
	if !a.paused && len(a.pending) > 0 {
		a.appendData(a.pending...)
		a.pending = nil
	}
	return a, nil
}

// clearBuffer clears the line buffer and viewport
func (a *App) clearBuffer() (tea.Model, tea.Cmd) {
	a.lineBuffer.Clear()
	a.vp.SetContent("")
	return a, nil
}

// updateViewportContent updates the viewport with wrapped content. Each line
// is wrapped on its own so echoed sends can be styled after wrapping (the
// wrapper measures bytes and would miscount ANSI codes).
func (a *App) updateViewportContent() {
	wrapper := NewLineWrapper(a.vp.Width)
	echoStyle := lipgloss.NewStyle().Foreground(ColorSecondary)

	lines := a.lineBuffer.GetDisplayLines()
	parts := make([]string, len(lines))
	for i, line := range lines {
		wrapped := wrapper.Wrap([]string{line})
		if a.lineBuffer.IsEcho(i) {
			wrapped = echoStyle.Render(wrapped)
		}
		parts[i] = wrapped
	}
	wrappedContent := strings.Join(parts, "\n")

	// Bottom-align shorter content so new lines appear at the base of the viewport
	if a.vp.Height > 0 && wrappedContent != "" {
		lineCount := CountLines(wrappedContent)
		if lineCount < a.vp.Height {
			padding := strings.Repeat("\n", a.vp.Height-lineCount)
			wrappedContent = padding + wrappedContent
		}
	}

	a.vp.SetContent(wrappedContent)
}

// tickCmd returns a command that sends a tick message
func (a *App) tickCmd() tea.Cmd {
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
			return feedClosedMsg{}
		}
		return lineMsg(s)
	}
}

// View renders the application UI
func (a *App) View() string {
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

	if a.send != nil {
		s.WriteString(a.renderInput())
		s.WriteString("\n\n")
	}

	// Help text
	s.WriteString(a.renderHelp())

	// Center content
	content := s.String()
	return lipgloss.PlaceVertical(a.height, lipgloss.Top,
		lipgloss.PlaceHorizontal(a.width, lipgloss.Center, content))
}

// renderStatus renders the status line with metadata
func (a *App) renderStatus() string {
	statusText := fmt.Sprintf("Lines: %d", a.lineBuffer.LineCount())

	switch {
	case a.disconnected:
		statusText = StyleError.Render(" DISCONNECTED") + " • " + statusText
	case a.paused:
		statusText = StyleWarning.Render(" PAUSED") + " • " + statusText
	default:
		statusText = StyleSuccess.Render("  LIVE ") + " • " + statusText
	}

	if !a.lastTick.IsZero() {
		statusText += " • " + StyleMuted.Render(a.lastTick.Format("15:04:05"))
	}

	if a.spotlightEnabled {
		statusText += " • " + StyleMuted.Render("Spotlight on")
	} else {
		statusText += " • " + StyleMuted.Render("Spotlight off")
	}

	if a.send != nil {
		echo := "echo off"
		if a.prefs.Echo {
			echo = "echo on"
		}
		statusText += " • " + StyleMuted.Render(string(a.prefs.LineEnding)+" • "+echo)
	}

	return statusText
}

// renderInput renders the send field, or a hint when it is not focused.
func (a *App) renderInput() string {
	var line string
	switch {
	case a.disconnected:
		line = StyleMuted.Render("Device disconnected. ctrl+w saves the run.")
	case a.inputMode:
		line = a.input.View()
		if a.sending {
			line += " " + StyleMuted.Render("sending…")
		}
	default:
		line = StyleMuted.Render("press i to type to the device")
	}
	if a.sendErr != "" {
		line += "  " + StyleError.Render(a.sendErr)
	}
	return lipgloss.NewStyle().Width(a.vp.Width + a.boxStyle.GetHorizontalFrameSize()).Render(line)
}

// renderViewport renders the viewport in a bordered box with fade effect
func (a *App) renderViewport() string {
	// Get the visible content from viewport
	visibleContent := a.vp.View()

	// Apply fade effect to visible lines when enabled
	fadedContent := visibleContent
	if a.spotlightEnabled {
		scroll := ScrollInfo{
			ScrollPercent: a.vp.ScrollPercent(),
			AtTop:         a.vp.AtTop(),
			AtBottom:      a.vp.AtBottom(),
		}
		fadedContent = a.spotlight.ApplyFade(visibleContent, scroll)
	}

	vpContent := lipgloss.NewStyle().
		Width(a.vp.Width).
		Height(a.vp.Height).
		Render(fadedContent)
	return a.boxStyle.Render(vpContent)
}

// renderHelp renders the help text
func (a *App) renderHelp() string {
	if a.help.ShowAll {
		return StyleHelp.Render(a.help.View(keys))
	}
	var helpText string
	switch {
	case a.inputMode:
		helpText = "enter: send • esc: stop typing • ↑/↓: history • pgup/pgdn: scroll • ctrl+w: save & quit"
	case a.send != nil:
		helpText = "i: type • e: line ending • o: echo • ctrl+w: save & quit • space: pause • s: spotlight • ↑/↓: scroll • ctrl+l: clear • q: quit"
	default:
		helpText = "ctrl+w: save & quit • space: pause/resume • s: spotlight • ↑/↓: scroll • ctrl+l: clear • q: quit without saving"
	}
	return StyleHelp.Render(helpText)
}

// toggleSpotlight toggles the spotlight fade effect
func (a *App) toggleSpotlight() (tea.Model, tea.Cmd) {
	a.spotlightEnabled = !a.spotlightEnabled
	return a, nil
}

// SaveRequested returns true if the user requested to save the run
func (a *App) SaveRequested() bool {
	return a.saveRequested
}
