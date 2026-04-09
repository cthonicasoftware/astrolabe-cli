package tui

import (
	"strings"
	"testing"
)

func TestNewLineWrapper(t *testing.T) {
	lw := NewLineWrapper(80)
	if lw == nil {
		t.Fatal("NewLineWrapper() returned nil")
	}
	if lw.width != 80 {
		t.Errorf("Width = %d, want 80", lw.width)
	}
}

func TestLineWrapper_Wrap(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		lines    []string
		expected string
	}{
		{
			name:     "lines fit within width",
			width:    20,
			lines:    []string{"hello", "world"},
			expected: "hello\nworld",
		},
		{
			name:     "single line no wrapping needed",
			width:    20,
			lines:    []string{"hello"},
			expected: "hello",
		},
		{
			name:     "empty lines",
			width:    20,
			lines:    []string{},
			expected: "",
		},
		{
			name:     "line exactly at width",
			width:    6, // Effective width is 5 (width-1)
			lines:    []string{"hello"},
			expected: "hello",
		},
		{
			name:     "line needs wrapping",
			width:    11, // Effective width is 10
			lines:    []string{"hello world"},
			expected: "hello worl\nd", // Breaks at position 10, not at space (since 5 > 5 is false)
		},
		{
			name:     "multiple lines with wrapping",
			width:    11,
			lines:    []string{"hello world", "foo bar baz"},
			expected: "hello worl\nd\nfoo bar\nbaz",
		},
		{
			name:     "long word without spaces",
			width:    6,
			lines:    []string{"helloworld"},
			expected: "hello\nworld",
		},
		{
			name:     "zero width uses no wrapping",
			width:    0,
			lines:    []string{"hello", "world"},
			expected: "hello\nworld",
		},
		{
			name:     "negative width uses no wrapping",
			width:    -10,
			lines:    []string{"hello", "world"},
			expected: "hello\nworld",
		},
		{
			name:     "preserve empty lines",
			width:    20,
			lines:    []string{"hello", "", "world"},
			expected: "hello\n\nworld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lw := NewLineWrapper(tt.width)
			result := lw.Wrap(tt.lines)
			if result != tt.expected {
				t.Errorf("Wrap() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestLineWrapper_WordBreaking(t *testing.T) {
	t.Run("breaks at space when available", func(t *testing.T) {
		lw := NewLineWrapper(16) // Effective width: 15
		input := []string{"this is a long line"}

		result := lw.Wrap(input)
		lines := strings.Split(result, "\n")

		// Should break at spaces, not mid-word
		for _, line := range lines {
			if len(line) > 15 {
				t.Errorf("Line %q exceeds width of 15", line)
			}
		}

		// Verify breaking occurs at reasonable points
		// With the condition lastSpace > wrapWidth/2, breaks should be intelligent
		if len(lines) < 2 {
			t.Error("Long line should be wrapped")
		}
	})

	t.Run("breaks mid-word when no good space", func(t *testing.T) {
		lw := NewLineWrapper(11) // Effective width: 10
		input := []string{"verylongwordwithoutspaces"}

		result := lw.Wrap(input)
		lines := strings.Split(result, "\n")

		// Should break into chunks of roughly 10 chars
		for _, line := range lines[:len(lines)-1] {
			if len(line) != 10 {
				t.Errorf("Line %q should be exactly 10 chars, got %d", line, len(line))
			}
		}
	})

	t.Run("skips space at break point", func(t *testing.T) {
		lw := NewLineWrapper(10)         // Effective width: 9
		input := []string{"hello world"} // 11 chars total, will wrap

		result := lw.Wrap(input)
		expected := "hello\nworld" // Space at position 5 should be used as break point and skipped

		if result != expected {
			t.Errorf("Wrap() = %q, want %q (space should be skipped)", result, expected)
		}
	})

	t.Run("preserves spaces not at break points", func(t *testing.T) {
		lw := NewLineWrapper(21) // Effective width: 20
		input := []string{"hello  world  test"}

		result := lw.Wrap(input)
		// Should preserve double spaces
		if !strings.Contains(result, "  ") {
			t.Error("Should preserve multiple consecutive spaces")
		}
	})
}

func TestLineWrapper_EdgeCases(t *testing.T) {
	t.Run("single character width", func(t *testing.T) {
		lw := NewLineWrapper(2) // Effective width: 1
		input := []string{"hello"}

		result := lw.Wrap(input)
		lines := strings.Split(result, "\n")

		// Should break into single characters
		if len(lines) != 5 {
			t.Errorf("Should have 5 lines (one per char), got %d", len(lines))
		}

		for i, line := range lines {
			if len(line) != 1 {
				t.Errorf("Line %d should be 1 char, got %d: %q", i, len(line), line)
			}
		}
	})

	t.Run("very wide width", func(t *testing.T) {
		lw := NewLineWrapper(10000)
		input := []string{"short line", "another short"}

		result := lw.Wrap(input)
		expected := "short line\nanother short"

		if result != expected {
			t.Errorf("Wide width should not affect short lines: got %q, want %q", result, expected)
		}
	})

	t.Run("single empty line", func(t *testing.T) {
		lw := NewLineWrapper(20)
		input := []string{""}

		result := lw.Wrap(input)
		if result != "" {
			t.Errorf("Empty line should produce empty string, got %q", result)
		}
	})

	t.Run("line with only spaces", func(t *testing.T) {
		lw := NewLineWrapper(20)
		input := []string{"     "}

		result := lw.Wrap(input)
		if result != "     " {
			t.Errorf("Spaces should be preserved, got %q", result)
		}
	})
}

func TestLineWrapper_MultipleLines(t *testing.T) {
	lw := NewLineWrapper(16) // Effective width: 15

	input := []string{
		"short",
		"this is a longer line that needs wrapping",
		"medium line",
	}

	result := lw.Wrap(input)
	lines := strings.Split(result, "\n")

	// Verify no line exceeds the width
	for i, line := range lines {
		if len(line) > 15 {
			t.Errorf("Line %d (%q) exceeds width of 15: has %d chars", i, line, len(line))
		}
	}

	// Verify we have more than 3 lines (due to wrapping)
	if len(lines) <= 3 {
		t.Errorf("Expected wrapping to create more than 3 lines, got %d", len(lines))
	}

	// Verify original line order is preserved
	reconstructed := strings.Join(lines, " ")
	originalJoined := strings.Join(input, " ")

	// Remove extra spaces that might have been introduced at wrap points
	cleanReconstructed := strings.Join(strings.Fields(reconstructed), " ")
	cleanOriginal := strings.Join(strings.Fields(originalJoined), " ")

	if cleanReconstructed != cleanOriginal {
		t.Errorf("Content not preserved after wrapping:\ngot:  %q\nwant: %q",
			cleanReconstructed, cleanOriginal)
	}
}

func TestLineWrapper_LongLineWithMixedContent(t *testing.T) {
	lw := NewLineWrapper(21) // Effective width: 20

	input := []string{
		"word anotherverylongwordthatcannotbreak short",
	}

	result := lw.Wrap(input)
	lines := strings.Split(result, "\n")

	// Verify no line exceeds width
	for i, line := range lines {
		if len(line) > 20 {
			t.Errorf("Line %d (%q) exceeds width of 20: has %d chars", i, line, len(line))
		}
	}

	// Verify content is preserved
	reconstructed := strings.ReplaceAll(result, "\n", "")
	original := strings.ReplaceAll(input[0], " ", "")

	// Account for skipped spaces at break points
	if !strings.Contains(original, strings.ReplaceAll(reconstructed, " ", "")) {
		t.Error("Content lost during wrapping")
	}
}
