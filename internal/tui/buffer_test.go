package tui

import (
	"reflect"
	"strings"
	"testing"
)

func TestNewLineBuffer(t *testing.T) {
	lb := NewLineBuffer()
	if lb == nil {
		t.Fatal("NewLineBuffer() returned nil")
	}
	if lb.LineCount() != 0 {
		t.Errorf("New buffer should have 0 lines, got %d", lb.LineCount())
	}
}

func TestLineBuffer_AddData(t *testing.T) {
	tests := []struct {
		name          string
		inputs        []string
		expectedLines []string
		expectedCount int
	}{
		{
			name:          "single complete line",
			inputs:        []string{"hello\n"},
			expectedLines: []string{"hello"},
			expectedCount: 1,
		},
		{
			name:          "multiple complete lines",
			inputs:        []string{"line1\nline2\nline3\n"},
			expectedLines: []string{"line1", "line2", "line3"},
			expectedCount: 3,
		},
		{
			name:          "partial line",
			inputs:        []string{"partial"},
			expectedLines: []string{"partial"},
			expectedCount: 0,
		},
		{
			name:          "partial then complete",
			inputs:        []string{"part", "ial\n"},
			expectedLines: []string{"partial"},
			expectedCount: 1,
		},
		{
			name:          "mixed complete and partial",
			inputs:        []string{"line1\nline2\npartial"},
			expectedLines: []string{"line1", "line2", "partial"},
			expectedCount: 2,
		},
		{
			name:          "empty string",
			inputs:        []string{""},
			expectedLines: []string{},
			expectedCount: 0,
		},
		{
			name:          "only newlines",
			inputs:        []string{"\n\n\n"},
			expectedLines: []string{"", ""},
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb := NewLineBuffer()
			for _, input := range tt.inputs {
				lb.AddData(input)
			}

			displayLines := lb.GetDisplayLines()
			if !reflect.DeepEqual(displayLines, tt.expectedLines) {
				t.Errorf("GetDisplayLines() = %v, want %v", displayLines, tt.expectedLines)
			}

			if lb.LineCount() != tt.expectedCount {
				t.Errorf("LineCount() = %d, want %d", lb.LineCount(), tt.expectedCount)
			}
		})
	}
}

func TestLineBuffer_CarriageReturnStripping(t *testing.T) {
	lb := NewLineBuffer()
	lb.AddData("hello\r\nworld\r\n")

	expectedLines := []string{"hello", "world"}
	displayLines := lb.GetDisplayLines()

	if !reflect.DeepEqual(displayLines, expectedLines) {
		t.Errorf("Carriage returns not stripped properly: got %v, want %v",
			displayLines, expectedLines)
	}
}

func TestLineBuffer_MaxBufferedLines(t *testing.T) {
	lb := NewLineBuffer()

	// Add more than MaxBufferedLines
	for i := 0; i < MaxBufferedLines+100; i++ {
		lb.AddData("line\n")
	}

	if lb.LineCount() != MaxBufferedLines {
		t.Errorf("Buffer should be limited to %d lines, got %d",
			MaxBufferedLines, lb.LineCount())
	}
}

func TestLineBuffer_GetDisplayLines(t *testing.T) {
	t.Run("complete lines only", func(t *testing.T) {
		lb := NewLineBuffer()
		lb.AddData("line1\nline2\n")

		expected := []string{"line1", "line2"}
		result := lb.GetDisplayLines()

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("GetDisplayLines() = %v, want %v", result, expected)
		}
	})

	t.Run("with partial line", func(t *testing.T) {
		lb := NewLineBuffer()
		lb.AddData("line1\npartial")

		expected := []string{"line1", "partial"}
		result := lb.GetDisplayLines()

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("GetDisplayLines() = %v, want %v", result, expected)
		}
	})

	t.Run("only partial line", func(t *testing.T) {
		lb := NewLineBuffer()
		lb.AddData("partial")

		expected := []string{"partial"}
		result := lb.GetDisplayLines()

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("GetDisplayLines() = %v, want %v", result, expected)
		}
	})

	t.Run("empty buffer", func(t *testing.T) {
		lb := NewLineBuffer()

		expected := []string{}
		result := lb.GetDisplayLines()

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("GetDisplayLines() = %v, want %v", result, expected)
		}
	})
}

func TestLineBuffer_Clear(t *testing.T) {
	lb := NewLineBuffer()
	lb.AddData("line1\nline2\npartial")

	lb.Clear()

	if lb.LineCount() != 0 {
		t.Errorf("After Clear(), LineCount() = %d, want 0", lb.LineCount())
	}

	displayLines := lb.GetDisplayLines()
	if len(displayLines) != 0 {
		t.Errorf("After Clear(), GetDisplayLines() = %v, want empty slice", displayLines)
	}
}

func TestLineBuffer_LineCount(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "no lines",
			input:    "",
			expected: 0,
		},
		{
			name:     "one complete line",
			input:    "line\n",
			expected: 1,
		},
		{
			name:     "partial line not counted",
			input:    "partial",
			expected: 0,
		},
		{
			name:     "three complete lines",
			input:    "line1\nline2\nline3\n",
			expected: 3,
		},
		{
			name:     "lines with partial at end",
			input:    "line1\nline2\npartial",
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb := NewLineBuffer()
			lb.AddData(tt.input)

			if lb.LineCount() != tt.expected {
				t.Errorf("LineCount() = %d, want %d", lb.LineCount(), tt.expected)
			}
		})
	}
}

func TestLineBuffer_IncrementalAdds(t *testing.T) {
	lb := NewLineBuffer()

	// Simulate streaming data
	lb.AddData("hello ")
	lb.AddData("world")
	lb.AddData("\n")
	lb.AddData("second")
	lb.AddData(" line\n")

	expected := []string{"hello world", "second line"}
	result := lb.GetDisplayLines()

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Incremental adds: got %v, want %v", result, expected)
	}
}

func TestLineBuffer_EdgeCases(t *testing.T) {
	t.Run("single newline", func(t *testing.T) {
		lb := NewLineBuffer()
		lb.AddData("\n")

		// A single newline creates an empty line, but the first empty line is skipped
		// due to the condition: if parts[i] != "" || i > 0
		if lb.LineCount() != 0 {
			t.Errorf("Single newline should create 0 lines (first empty skipped), got %d", lb.LineCount())
		}
	})

	t.Run("trailing newline", func(t *testing.T) {
		lb := NewLineBuffer()
		lb.AddData("line1\n")
		lb.AddData("line2\n")

		// Should have exactly 2 lines, no empty line at the end
		expected := []string{"line1", "line2"}
		result := lb.GetDisplayLines()

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Trailing newline handling: got %v, want %v", result, expected)
		}
	})

	t.Run("long continuous stream", func(t *testing.T) {
		lb := NewLineBuffer()

		// Add a very long line without newlines
		longLine := strings.Repeat("a", 10000)
		lb.AddData(longLine)

		// Should still be in partial buffer
		if lb.LineCount() != 0 {
			t.Errorf("Long partial line should not be counted, got %d lines", lb.LineCount())
		}

		// Complete the line
		lb.AddData("\n")

		if lb.LineCount() != 1 {
			t.Errorf("After completing long line, should have 1 line, got %d", lb.LineCount())
		}

		displayLines := lb.GetDisplayLines()
		if len(displayLines) != 1 || displayLines[0] != longLine {
			t.Error("Long line not preserved correctly")
		}
	})
}
