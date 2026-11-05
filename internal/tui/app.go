package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
)

// Layout constants for viewport sizing
const (
	marginWidth       = 8
	minViewportWidth  = 40
	minViewportHeight = 3 // keep small: a taller frame than the terminal loses rows
	tickInterval      = 250 * time.Millisecond
	maxSendLength     = 1024
	maxHistory        = 100

	// timestampLayout formats the per-line arrival time shown at the right
	// edge of the live view.
	timestampLayout = "15:04:05.000"
	// timestampGutter is the space kept free for a timestamp and the gap
	// before it, so long lines wrap short of it.
	timestampGutter = len(timestampLayout) + 1
)

// keymap defines the key bindings for the live view. Scroll and History
// only document keys the viewport and input field handle themselves.
type keymap struct {
	Quit    key.Binding
	Save    key.Binding
	Scroll  key.Binding
	History key.Binding
	Pause   key.Binding
	Clear   key.Binding
	Spot    key.Binding
	Time    key.Binding
	Input   key.Binding
	Ending  key.Binding
	Echo    key.Binding
	Send    key.Binding
	Leave   key.Binding
}

var keys = keymap{
	Quit:    key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "discard run and return")),
	Save:    key.NewBinding(key.WithKeys("ctrl+w"), key.WithHelp("ctrl+w", "stop & save run")),
	Scroll:  key.NewBinding(key.WithKeys("up", "down", "pgup", "pgdown"), key.WithHelp("↑/↓ pgup/pgdn", "scroll")),
	History: key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "recall earlier lines")),
	Pause:   key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "pause/resume capture")),
	Clear:   key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "clear buffer")),
	Spot:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "toggle spotlight")),
	Time:    key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "toggle timestamps")),
	Input:   key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "type to device")),
	Ending:  key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "cycle line ending")),
	Echo:    key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "toggle echo")),
	Send:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send line")),
	Leave:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "stop typing")),
}

// SendFunc writes a line plus line ending to the device and returns the time
// recorded for it. It may block.
type SendFunc func(line string, le sources.LineEnding) (time.Time, error)

// App represents the live capture TUI application
type App struct {
	title            string
	vp               viewport.Model
	paused           bool
	pending          []FeedChunk // chunks received while paused, applied on resume
	saveRequested    bool
	feed             <-chan FeedChunk
	lineBuffer       *LineBuffer
	lastTick         time.Time
	width            int
	height           int
	boxStyle         lipgloss.Style
	spotlightEnabled bool
	spotlight        *SpotlightEffect

	// Remembered view settings. savePrefs is nil when they do not persist.
	savePrefs func(LiveViewPrefs) error
	prefs     LiveViewPrefs

	// Send path. send is nil when the source cannot accept input.
	send         SendFunc
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
func NewApp(title string, feed <-chan FeedChunk) *App {
	vp := viewport.New()
	vp.SetContent("")
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3
	vp.Style = lipgloss.NewStyle()

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2)

	input := textinput.New()
	input.Prompt = "› "
	input.Placeholder = "type a line, enter to send, esc to stop"
	input.CharLimit = maxSendLength
	inputStyles := input.Styles()
	inputStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(ColorPrimary)
	inputStyles.Blurred.Prompt = inputStyles.Focused.Prompt
	input.SetStyles(inputStyles)

	return &App{
		title:            title,
		vp:               vp,
		paused:           false,
		saveRequested:    false,
		feed:             feed,
		boxStyle:         boxStyle,
		lineBuffer:       NewLineBuffer(),
		spotlightEnabled: false,
		spotlight:        NewSpotlightEffect(),
		prefs:            DefaultLiveViewPrefs(),
		input:            input,
	}
}

// SetPrefs applies remembered view settings. save is called with the new
// settings whenever the operator changes one; it may be nil.
func (a *App) SetPrefs(prefs LiveViewPrefs, save func(LiveViewPrefs) error) {
	a.prefs = prefs
	a.savePrefs = save
}

// EnableSend turns on the input field.
func (a *App) EnableSend(send SendFunc) {
	a.send = send
}

func (a *App) canSend() bool { return a.send != nil && !a.disconnected }

// Message types
type tickMsg time.Time
type lineMsg FeedChunk

// feedClosedMsg reports that the source stopped (e.g. device unplugged).
type feedClosedMsg struct{}

// sendResultMsg reports the outcome of an asynchronous Send.
type sendResultMsg struct {
	line string
	at   time.Time
	err  error
}

// prefsSaveErrMsg reports a failure to persist live-view preferences.
type prefsSaveErrMsg struct{ err error }

// Init initializes the app
func (a *App) Init() tea.Cmd {
	return tea.Batch(a.tickCmd(), a.pullLine())
}

// Update handles messages and updates the model
func (a *App) Update(msg tea.Msg) (Screen, tea.Cmd) {
	model, cmd := a.update(msg)
	// Any message can change the rows around the viewport (mode, errors,
	// settings), so re-fit the layout after each one.
	a.syncLayout()
	return model, cmd
}

func (a *App) update(msg tea.Msg) (Screen, tea.Cmd) {
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
		a.sendErr = "could not save settings: " + m.err.Error()
		return a, nil
	case tea.KeyPressMsg:
		if a.inputMode {
			return a.handleInputKey(m)
		}
		return a.handleKeyPress(m)
	case tea.PasteMsg:
		// Bubble Tea v2 delivers pastes separately from key presses.
		if !a.inputMode {
			return a, nil
		}
		var cmd tea.Cmd
		a.input, cmd = a.input.Update(m)
		return a, cmd
	}

	// Pass other messages to viewport (like mouse events)
	var cmd tea.Cmd
	a.vp, cmd = a.vp.Update(msg)
	return a, cmd
}

// handleWindowResize handles terminal window resize events
func (a *App) handleWindowResize(msg tea.WindowSizeMsg) (Screen, tea.Cmd) {
	a.width = msg.Width
	a.height = msg.Height

	// Width first: the status and help rows wrap to it, which sets how
	// much height is left for the viewport.
	a.vp.SetWidth(max(msg.Width-marginWidth-a.boxStyle.GetHorizontalFrameSize()-1, minViewportWidth))
	a.input.SetWidth(max(a.vp.Width()-lipgloss.Width(a.input.Prompt), 10))
	a.syncLayout()

	// Reset position and re-wrap content
	a.vp.YPosition = 0
	a.updateViewportContent()
	a.vp.GotoBottom()

	return a, nil
}

// handleTick handles periodic tick events
func (a *App) handleTick(msg tickMsg) (Screen, tea.Cmd) {
	a.lastTick = time.Time(msg)
	// Don't pull from the feed here: exactly one pullLine must be in flight at
	// a time (started in Init, re-armed by handleLineData). Extra concurrent
	// readers race on the channel and deliver chunks out of order.
	return a, a.tickCmd()
}

// handleLineData handles incoming line data
func (a *App) handleLineData(msg lineMsg) (Screen, tea.Cmd) {
	// Always drain the channel to prevent blocking the pipeline. While paused,
	// hold chunks so resuming shows them instead of silently dropping data.
	if a.paused {
		a.pending = append(a.pending, FeedChunk(msg))
	} else {
		a.appendData(FeedChunk(msg))
	}

	// Always pull the next line, even when paused, to prevent blocking
	return a, a.pullLine()
}

// handleFeedClosed marks the device as gone. The run can still be saved.
func (a *App) handleFeedClosed() (Screen, tea.Cmd) {
	a.disconnected = true
	a.inputMode = false
	a.input.Blur()
	return a, nil
}

// appendData adds chunks to the line buffer and refreshes the viewport,
// following the tail only if the user was already at the bottom.
func (a *App) appendData(chunks ...FeedChunk) {
	wasAtBottom := a.vp.AtBottom()
	for _, c := range chunks {
		a.lineBuffer.AddData(c.Data, c.At)
	}
	a.updateViewportContent()
	if wasAtBottom {
		a.vp.GotoBottom()
	}
}

// handleKeyPress handles keyboard input in view mode
func (a *App) handleKeyPress(msg tea.KeyPressMsg) (Screen, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return a.quit()
	case key.Matches(msg, keys.Save):
		return a.save()
	case key.Matches(msg, keys.Pause):
		return a.togglePause()
	case key.Matches(msg, keys.Clear):
		return a.clearBuffer()
	case key.Matches(msg, keys.Spot):
		return a.toggleSpotlight()
	case key.Matches(msg, keys.Time):
		return a.toggleTimestamps()
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
func (a *App) handleInputKey(msg tea.KeyPressMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.inputMode = false
		a.input.Blur()
		return a, nil
	case "enter":
		return a.submit()
	case "up":
		a.historyPrev()
		return a, nil
	case "down":
		a.historyNext()
		return a, nil
	case "pgup", "pgdown":
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
func (a *App) submit() (Screen, tea.Cmd) {
	if a.sending || !a.canSend() {
		return a, nil
	}
	line := a.input.Value()
	a.sending = true
	send, le := a.send, a.prefs.LineEnding
	return a, func() tea.Msg {
		at, err := send(line, le)
		return sendResultMsg{line: line, at: at, err: err}
	}
}

func (a *App) handleSendResult(msg sendResultMsg) (Screen, tea.Cmd) {
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
		a.lineBuffer.AddEcho(msg.line, msg.at)
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

// persistPrefs saves live-view settings off the UI loop.
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

func (a *App) quit() (Screen, tea.Cmd) {
	a.saveRequested = false
	return a, func() tea.Msg {
		return NavigateMsg{
			To:     ScreenWelcome,
			Status: NewStatusMessage(StatusInfo, "Discarded", "Capture exited without saving."),
		}
	}
}

func (a *App) save() (Screen, tea.Cmd) {
	a.saveRequested = true
	return a, func() tea.Msg {
		return NavigateMsg{
			To:     ScreenWelcome,
			Status: NewStatusMessage(StatusSuccess, "Saved", "Capture session saved successfully."),
		}
	}
}

// togglePause toggles the pause state
func (a *App) togglePause() (Screen, tea.Cmd) {
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
func (a *App) clearBuffer() (Screen, tea.Cmd) {
	a.lineBuffer.Clear()
	a.vp.SetContent("")
	return a, nil
}

// updateViewportContent updates the viewport with wrapped content. Each line
// is wrapped on its own so echoed sends can be styled after wrapping (the
// wrapper measures bytes and would miscount ANSI codes).
func (a *App) updateViewportContent() {
	// With timestamps on, text wraps short of a right-hand gutter so the
	// timestamp never pushes the line's text sideways.
	textWidth := a.vp.Width()
	if a.prefs.Timestamps && textWidth > 0 {
		textWidth = max(textWidth-timestampGutter, 1)
	}
	wrapper := NewLineWrapper(textWidth)
	echoStyle := lipgloss.NewStyle().Foreground(ColorSecondary)

	lines := a.lineBuffer.GetDisplayLines()
	parts := make([]string, len(lines))
	for i, line := range lines {
		rows := strings.Split(wrapper.Wrap([]string{line}), "\n")
		// Measure before styling: widths of styled text include ANSI codes.
		firstWidth := lipgloss.Width(rows[0])
		if a.lineBuffer.IsEcho(i) {
			for j := range rows {
				rows[j] = echoStyle.Render(rows[j])
			}
		}
		if a.prefs.Timestamps {
			rows[0] += a.timestampSuffix(a.lineBuffer.LineTime(i), firstWidth)
		}
		parts[i] = strings.Join(rows, "\n")
	}
	wrappedContent := strings.Join(parts, "\n")

	// Bottom-align shorter content so new lines appear at the base of the viewport
	if a.vp.Height() > 0 && wrappedContent != "" {
		lineCount := CountLines(wrappedContent)
		if lineCount < a.vp.Height() {
			padding := strings.Repeat("\n", a.vp.Height()-lineCount)
			wrappedContent = padding + wrappedContent
		}
	}

	a.vp.SetContent(wrappedContent)
}

// timestampSuffix right-aligns at in muted style after a row rowWidth cells
// wide. It ends one cell short of the viewport edge, like the wrapper.
func (a *App) timestampSuffix(at time.Time, rowWidth int) string {
	if at.IsZero() {
		return ""
	}
	start := a.vp.Width() - 1 - len(timestampLayout)
	pad := max(start-rowWidth, 1)
	return strings.Repeat(" ", pad) + StyleMuted.Render(at.Local().Format(timestampLayout))
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
		c, ok := <-a.feed
		if !ok {
			return feedClosedMsg{}
		}
		return lineMsg(c)
	}
}

// View renders the application UI
func (a *App) View() string {
	header, footer := a.renderChrome()
	content := header + a.renderViewport() + footer
	return lipgloss.PlaceVertical(a.height, lipgloss.Top,
		lipgloss.PlaceHorizontal(a.width, lipgloss.Center, content))
}

// renderChrome renders everything above and below the viewport box.
func (a *App) renderChrome() (header, footer string) {
	header = StyleTitle.Render(a.title) + "\n\n" + a.renderStatus() + "\n\n"
	footer = "\n\n"
	if a.send != nil {
		footer += a.renderInput() + "\n\n"
	}
	footer += a.renderHelp()
	return header, footer
}

// syncLayout sizes the viewport so the whole view fits the terminal. The
// rows around the box are measured, not assumed: status and help wrap on
// narrow terminals and change with mode, and a view taller than the
// terminal loses its top rows (the title).
func (a *App) syncLayout() {
	if a.height <= 0 {
		return
	}
	header, footer := a.renderChrome()
	chrome := lipgloss.Height(header+"x"+footer) - 1 // minus the stand-in box row
	h := max(a.height-chrome-a.boxStyle.GetVerticalFrameSize(), minViewportHeight)
	if h == a.vp.Height() {
		return
	}
	wasAtBottom := a.vp.AtBottom()
	a.vp.SetHeight(h)
	a.updateViewportContent()
	if wasAtBottom {
		a.vp.GotoBottom()
	}
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

	if a.prefs.Timestamps {
		statusText += " • " + StyleMuted.Render("Timestamps on")
	} else {
		statusText += " • " + StyleMuted.Render("Timestamps off")
	}

	if a.send != nil {
		echo := "echo off"
		if a.prefs.Echo {
			echo = "echo on"
		}
		statusText += " • " + StyleMuted.Render(string(a.prefs.LineEnding)+" • "+echo)
	}

	return a.fitBox(statusText)
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
	return a.fitBox(line)
}

// fitBox wraps s to the bordered box's width. View centers the layout on
// its widest row, so a row wider than the box would shift everything left.
func (a *App) fitBox(s string) string {
	if a.vp.Width() <= 0 {
		return s // not sized yet
	}
	return lipgloss.NewStyle().Width(a.vp.Width() + a.boxStyle.GetHorizontalFrameSize()).Render(s)
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
		Width(a.vp.Width()).
		Height(a.vp.Height()).
		Render(fadedContent)
	return a.boxStyle.Render(vpContent)
}

// renderHelp renders the help text
func (a *App) renderHelp() string {
	var helpText string
	switch {
	case a.inputMode:
		helpText = "enter: send • esc: stop typing • f1: help"
	case a.send != nil:
		helpText = "i: type • ctrl+w: save & quit • q: quit • ?: help"
	default:
		helpText = "space: pause • ctrl+w: save & quit • q: quit • ?: help"
	}
	return a.fitBox(StyleHelp.Render(helpText))
}

// TextFocused reports whether the send field has focus, so ? is typed
// rather than opening help.
func (a *App) TextFocused() bool { return a.inputMode }

// HelpSections lists the live view's keys for the help dialog.
func (a *App) HelpSections() []HelpSection {
	if a.inputMode {
		return []HelpSection{{
			Title: "Typing to the device",
			Keys:  keyHelpFrom(keys.Send, keys.History, keys.Leave, keys.Save),
		}, {
			Title: "Scrolling",
			Keys:  []KeyHelp{{"pgup/pgdn", "scroll"}},
		}}
	}
	sections := []HelpSection{{
		Title: "Live capture",
		Keys:  keyHelpFrom(keys.Save, keys.Quit, keys.Pause, keys.Time, keys.Spot, keys.Clear, keys.Scroll),
	}}
	if a.send != nil {
		sections = append(sections, HelpSection{
			Title: "Sending",
			Keys:  keyHelpFrom(keys.Input, keys.Ending, keys.Echo),
		})
	}
	return sections
}

// toggleSpotlight toggles the spotlight fade effect
func (a *App) toggleSpotlight() (Screen, tea.Cmd) {
	a.spotlightEnabled = !a.spotlightEnabled
	return a, nil
}

// toggleTimestamps shows or hides each line's arrival time and remembers
// the choice.
func (a *App) toggleTimestamps() (Screen, tea.Cmd) {
	a.prefs.Timestamps = !a.prefs.Timestamps
	wasAtBottom := a.vp.AtBottom()
	a.updateViewportContent()
	if wasAtBottom {
		a.vp.GotoBottom()
	}
	return a, a.persistPrefs()
}

// SaveRequested returns true if the user requested to save the run
func (a *App) SaveRequested() bool {
	return a.saveRequested
}
