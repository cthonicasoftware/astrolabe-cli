package tui

import "strings"

// MaxBufferedLines defines the maximum number of lines to keep in the buffer
const MaxBufferedLines = 1000

// LineBuffer manages buffering and processing of incoming text data
type LineBuffer struct {
	buffer *strings.Builder
	lines  []string
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
			lb.lines = append(lb.lines, parts[i])
		}
	}

	// The last part is either empty (if ended with \n) or a partial line
	lb.buffer.Reset()
	if parts[len(parts)-1] != "" {
		lb.buffer.WriteString(parts[len(parts)-1])
	}

	// Trim to max buffer size
	if len(lb.lines) > MaxBufferedLines {
		lb.lines = lb.lines[len(lb.lines)-MaxBufferedLines:]
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

// Clear resets the buffer
func (lb *LineBuffer) Clear() {
	lb.buffer.Reset()
	lb.lines = nil
}

// LineCount returns the number of complete lines
func (lb *LineBuffer) LineCount() int {
	return len(lb.lines)
}
