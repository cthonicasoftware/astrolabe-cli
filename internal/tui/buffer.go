package tui

import (
	"strings"
	"time"
)

// MaxBufferedLines defines the maximum number of lines to keep in the buffer
const MaxBufferedLines = 1000

// EchoPrefix marks lines the operator sent, as shown in the live view.
const EchoPrefix = "→ "

// LineBuffer manages buffering and processing of incoming text data. Each
// line keeps the arrival time of its first byte, matching the ts the capture
// pipeline records for it in data.jsonl.
type LineBuffer struct {
	buffer    *strings.Builder
	partialAt time.Time // arrival time of the partial line in buffer
	lines     []string
	kind      []lineKind  // kind[i] says what lines[i] is
	at        []time.Time // at[i] is when lines[i] started arriving
}

// lineKind tells received lines from ones the live view adds.
type lineKind uint8

const (
	lineData   lineKind = iota // received from the device
	lineEcho                   // an echoed send
	lineMarker                 // a status marker, e.g. link down/up
)

// NewLineBuffer creates a new line buffer
func NewLineBuffer() *LineBuffer {
	return &LineBuffer{
		buffer: &strings.Builder{},
		lines:  []string{},
	}
}

// AddData adds raw data that arrived at at, processing complete lines.
func (lb *LineBuffer) AddData(data string, at time.Time) {
	// Strip carriage returns to prevent cursor positioning issues
	cleaned := strings.ReplaceAll(data, "\r", "")
	if cleaned == "" {
		return
	}
	if lb.buffer.Len() == 0 {
		lb.partialAt = at
	}
	lb.buffer.WriteString(cleaned)

	// Process complete lines (those ending with \n)
	bufferContent := lb.buffer.String()
	if !strings.Contains(bufferContent, "\n") {
		return
	}

	parts := strings.Split(bufferContent, "\n")

	// All parts except the last are complete lines. Only the first can have
	// started in an earlier chunk.
	for i := 0; i < len(parts)-1; i++ {
		lineAt := at
		if i == 0 {
			lineAt = lb.partialAt
		}
		if parts[i] != "" || i > 0 {
			lb.appendLine(parts[i], lineData, lineAt)
		}
	}

	// The last part is either empty (if ended with \n) or a partial line
	lb.buffer.Reset()
	if last := parts[len(parts)-1]; last != "" {
		lb.buffer.WriteString(last)
		lb.partialAt = at
	}

	lb.trim()
}

// AddEcho shows a line the operator sent at at. Any partial received line is
// ended first so the echo always sits on its own line; received data then
// continues on the next line.
func (lb *LineBuffer) AddEcho(line string, at time.Time) {
	lb.addOwnLine(EchoPrefix+line, lineEcho, at)
}

// AddMarker shows a status line, such as a link drop, at at. Like an echo it
// sits on its own line, but it is not a send.
func (lb *LineBuffer) AddMarker(text string, at time.Time) {
	lb.addOwnLine(text, lineMarker, at)
}

// addOwnLine ends any partial received line, then adds line on its own.
func (lb *LineBuffer) addOwnLine(line string, kind lineKind, at time.Time) {
	if lb.buffer.Len() > 0 {
		lb.appendLine(lb.buffer.String(), lineData, lb.partialAt)
		lb.buffer.Reset()
	}
	lb.appendLine(line, kind, at)
	lb.trim()
}

func (lb *LineBuffer) appendLine(line string, kind lineKind, at time.Time) {
	lb.lines = append(lb.lines, line)
	lb.kind = append(lb.kind, kind)
	lb.at = append(lb.at, at)
}

func (lb *LineBuffer) trim() {
	if len(lb.lines) > MaxBufferedLines {
		cut := len(lb.lines) - MaxBufferedLines
		lb.lines = lb.lines[cut:]
		lb.kind = lb.kind[cut:]
		lb.at = lb.at[cut:]
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
func (lb *LineBuffer) IsEcho(i int) bool { return lb.kindOf(i) == lineEcho }

// IsMarker reports whether display line i is a status marker.
func (lb *LineBuffer) IsMarker(i int) bool { return lb.kindOf(i) == lineMarker }

// kindOf returns display line i's kind; the partial line is data.
func (lb *LineBuffer) kindOf(i int) lineKind {
	if i >= 0 && i < len(lb.kind) {
		return lb.kind[i]
	}
	return lineData
}

// LineTime returns when display line i (as returned by GetDisplayLines)
// started arriving, or the zero time if i is out of range.
func (lb *LineBuffer) LineTime(i int) time.Time {
	switch {
	case i >= 0 && i < len(lb.at):
		return lb.at[i]
	case i == len(lb.at) && lb.buffer.Len() > 0:
		return lb.partialAt
	}
	return time.Time{}
}

// Clear resets the buffer
func (lb *LineBuffer) Clear() {
	lb.buffer.Reset()
	lb.partialAt = time.Time{}
	lb.lines = nil
	lb.kind = nil
	lb.at = nil
}

// LineCount returns the number of complete lines
func (lb *LineBuffer) LineCount() int {
	return len(lb.lines)
}
