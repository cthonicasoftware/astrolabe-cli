package tui

import "strings"

// LineWrapper handles text wrapping logic
type LineWrapper struct {
	width int
}

// NewLineWrapper creates a new line wrapper with the given width
func NewLineWrapper(width int) *LineWrapper {
	return &LineWrapper{
		width: width,
	}
}

// Wrap wraps lines to fit within the configured width
func (lw *LineWrapper) Wrap(lines []string) string {
	if lw.width <= 0 {
		return strings.Join(lines, "\n")
	}

	// Use width - 1 to prevent viewport rendering artifacts at the edge
	wrapWidth := max(lw.width-1, 1)

	var wrapped strings.Builder
	for i, line := range lines {
		if i > 0 {
			wrapped.WriteString("\n")
		}

		if len(line) <= wrapWidth {
			wrapped.WriteString(line)
		} else {
			lw.wrapLineSimple(&wrapped, line, wrapWidth)
		}
	}

	return wrapped.String()
}

// wrapLineSimple wraps a single line
func (lw *LineWrapper) wrapLineSimple(builder *strings.Builder, line string, wrapWidth int) {
	remaining := line

	for len(remaining) > 0 {
		if len(remaining) <= wrapWidth {
			builder.WriteString(remaining)
			break
		}

		// Try to break at a space
		breakPoint := wrapWidth
		lastSpace := strings.LastIndex(remaining[:wrapWidth], " ")
		if lastSpace > 0 && lastSpace > wrapWidth/2 {
			breakPoint = lastSpace
		}

		builder.WriteString(remaining[:breakPoint])
		builder.WriteString("\n")

		// Skip the space if we broke at one
		if breakPoint < len(remaining) && remaining[breakPoint] == ' ' {
			remaining = remaining[breakPoint+1:]
		} else {
			remaining = remaining[breakPoint:]
		}
	}
}
