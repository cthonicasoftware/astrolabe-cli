package cliout

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/tui"
)

// Printer handles styled CLI output with consistent formatting
// Reuses the TUI color palette and styles for consistency
type Printer struct {
	writer      io.Writer
	jsonMode    bool
	noColor     bool
	enableIcons bool
}

// NewPrinter creates a new CLI output printer
func NewPrinter(w io.Writer, jsonMode bool) *Printer {
	// Check if colors should be disabled
	noColor := os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"

	// Enable icons by default (can be disabled with NO_ICONS env var)
	enableIcons := os.Getenv("NO_ICONS") == ""

	return &Printer{
		writer:      w,
		jsonMode:    jsonMode,
		noColor:     noColor,
		enableIcons: enableIcons,
	}
}

// DefaultPrinter creates a printer that writes to stdout
func DefaultPrinter(jsonMode bool) *Printer {
	return NewPrinter(os.Stdout, jsonMode)
}

// render applies style if colors are enabled
func (p *Printer) render(text string, styleFn func(...string) string) string {
	if p.noColor {
		return text
	}
	return styleFn(text)
}

// StyledHeader returns header-styled text while honoring color settings.
func (p *Printer) StyledHeader(text string) string {
	return p.render(text, tui.StyleHeader.Render)
}

// StyledMuted returns muted-styled text while honoring color settings.
func (p *Printer) StyledMuted(text string) string {
	return p.render(text, tui.StyleMuted.Render)
}

// StyledItem returns emphasized item-styled text while honoring color settings.
func (p *Printer) StyledItem(text string) string {
	return p.render(text, tui.StyleWarning.Render)
}

// StyledIcon returns an icon string that honors color and icon toggles.
func (p *Printer) StyledIcon(icon string) string {
	if !p.enableIcons {
		return ""
	}
	icon = strings.TrimSpace(icon)
	if icon == "" {
		return ""
	}
	return p.render(icon, tui.StyleIcon.Render)
}

// Info prints an informational message
func (p *Printer) Info(msg string) {
	if p.jsonMode {
		p.printJSON("info", msg, nil)
		return
	}
	icon := ""
	if p.enableIcons {
		icon = tui.StyleIcon.Render(tui.IconStatusInfo) + " "
	}
	fmt.Fprintf(p.writer, "%s%s\n", icon, p.render(msg, tui.StyleSubheader.Render))
}

// Success prints a success message
func (p *Printer) Success(msg string) {
	if p.jsonMode {
		p.printJSON("success", msg, nil)
		return
	}
	icon := ""
	if p.enableIcons {
		icon = tui.StyleSuccess.Render(tui.IconStatusSuccess) + " "
	}
	fmt.Fprintf(p.writer, "%s%s\n", icon, p.render(msg, tui.StyleSuccess.Render))
}

// Error prints an error message
func (p *Printer) Error(msg string) {
	if p.jsonMode {
		p.printJSON("error", msg, nil)
		return
	}
	icon := ""
	if p.enableIcons {
		icon = tui.StyleError.Render(tui.IconStatusError) + " "
	}
	fmt.Fprintf(p.writer, "%s%s\n", icon, p.render(msg, tui.StyleError.Render))
}

// Warning prints a warning message
func (p *Printer) Warning(msg string) {
	if p.jsonMode {
		p.printJSON("warning", msg, nil)
		return
	}
	icon := ""
	if p.enableIcons {
		icon = tui.StyleWarning.Render(tui.IconStatusWarning) + " "
	}
	fmt.Fprintf(p.writer, "%s%s\n", icon, p.render(msg, tui.StyleWarning.Render))
}

// Step prints a step/progress message with arrow indicator
func (p *Printer) Step(msg string) {
	if p.jsonMode {
		p.printJSON("step", msg, nil)
		return
	}
	icon := ""
	if p.enableIcons {
		icon = tui.StyleCursor.Render(tui.IconSelectedItem) + " "
	}
	fmt.Fprintf(p.writer, "%s%s\n", icon, p.render(msg, tui.StyleSubheader.Render))
}

// KeyValue prints a key-value pair with consistent styling
func (p *Printer) KeyValue(key, value string) {
	if p.jsonMode {
		p.printJSON("kv", "", map[string]string{key: value})
		return
	}
	keyStyled := p.render(key+":", tui.StyleKey.Render)
	valueStyled := p.render(value, tui.StyleValue.Render)
	fmt.Fprintf(p.writer, "  %s %s\n", keyStyled, valueStyled)
}

// Header prints a section header
func (p *Printer) Header(text string) {
	if p.jsonMode {
		p.printJSON("header", text, nil)
		return
	}
	fmt.Fprintf(p.writer, "\n%s\n", p.render(text, tui.StyleHeader.Render))
}

// Muted prints dimmed/muted text
func (p *Printer) Muted(msg string) {
	if p.jsonMode {
		return // Skip muted messages in JSON mode
	}
	fmt.Fprintf(p.writer, "%s\n", p.render(msg, tui.StyleMuted.Render))
}

// Blank prints a blank line
func (p *Printer) Blank() {
	if p.jsonMode {
		return // Skip blank lines in JSON mode
	}
	fmt.Fprintln(p.writer)
}

// Print raw text without styling
func (p *Printer) Print(msg string) {
	fmt.Fprint(p.writer, msg)
}

// Println prints raw text with newline
func (p *Printer) Println(msg string) {
	fmt.Fprintln(p.writer, msg)
}

// printJSON outputs structured JSON for machine consumption
func (p *Printer) printJSON(level, message string, data interface{}) {
	evt := struct {
		Level   string      `json:"level"`
		Message string      `json:"message"`
		Data    interface{} `json:"data,omitempty"`
	}{
		Level:   level,
		Message: message,
		Data:    data,
	}

	_ = json.NewEncoder(p.writer).Encode(evt)
}
