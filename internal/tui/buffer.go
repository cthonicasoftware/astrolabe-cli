package tui

import "strings"

// MaxBufferedLines defines the maximum number of lines to keep in the buffer
const MaxBufferedLines = 1000

// EchoPrefix marks lines the operator sent, as shown in the live view.
const EchoPrefix = "→ "

// LineBuffer manages buffering and processing of incoming text data
type LineBuffer struct {
	buffer *strings.Builder
	lines  []string
	echo   []bool // echo[i] reports whether lines[i] is an echoed send
}

// NewLineBuffer creates a new line buffer
func NewLineBuffer() *LineBuffer {
	return &LineBuffer{
		buffer: &strings.Builder{},
		lines:  []string{},
	}
}

// AddData adds raw data to the buffer, processing complete lines
func (lb *LineBuffer) AddData(data string) {
	// Strip carriage returns to prevent cursor positioning issues
	cleaned := strings.ReplaceAll(data, "\r", "")
	lb.buffer.WriteString(cleaned)

	// Process complete lines (those ending with \n)
	bufferContent := lb.buffer.String()
	if !strings.Contains(bufferContent, "\n") {
		return
	}

	parts := strings.Split(bufferContent, "\n")

	// All parts except the last are complete lines
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] != "" || i > 0 {
			lb.appendLine(parts[i], false)
		}
	}

	// The last part is either empty (if ended with \n) or a partial line
	lb.buffer.Reset()
	if parts[len(parts)-1] != "" {
		lb.buffer.WriteString(parts[len(parts)-1])
	}

	lb.trim()
}

// AddEcho shows a line the operator sent. Any partial received line is
// ended first so the echo always sits on its own line; received data then
// continues on the next line.
func (lb *LineBuffer) AddEcho(line string) {
	if lb.buffer.Len() > 0 {
		lb.appendLine(lb.buffer.String(), false)
		lb.buffer.Reset()
	}
	lb.appendLine(EchoPrefix+line, true)
	lb.trim()
}

func (lb *LineBuffer) appendLine(line string, echo bool) {
	lb.lines = append(lb.lines, line)
	lb.echo = append(lb.echo, echo)
}

func (lb *LineBuffer) trim() {
	if len(lb.lines) > MaxBufferedLines {
		cut := len(lb.lines) - MaxBufferedLines
		lb.lines = lb.lines[cut:]
		lb.echo = lb.echo[cut:]
	}
}

// GetDisplayLines returns all complete lines plus any partial line
func (lb *LineBuffer) GetDisplayLines() []string {
	displayLines := make([]string, len(lb.lines))
	copy(displayLines, lb.lines)

	// Add partial line if there is one
	if lb.buffer.Len() > 0 {
		displayLines = append(displayLines, lb.buffer.String())
	}

	return displayLines
}

// IsEcho reports whether display line i (as returned by GetDisplayLines) is
// an echoed send. The trailing partial line is never an echo.
func (lb *LineBuffer) IsEcho(i int) bool {
	return i >= 0 && i < len(lb.echo) && lb.echo[i]
}

// Clear resets the buffer
func (lb *LineBuffer) Clear() {
	lb.buffer.Reset()
	lb.lines = nil
	lb.echo = nil
}

// LineCount returns the number of complete lines
func (lb *LineBuffer) LineCount() int {
	return len(lb.lines)
}
