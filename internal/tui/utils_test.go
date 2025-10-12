package tui

import (
	"math"
	"testing"
)

func TestClampFloat(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		min      float64
		max      float64
		expected float64
	}{
		{
			name:     "value within range",
			value:    5.0,
			min:      0.0,
			max:      10.0,
			expected: 5.0,
		},
		{
			name:     "value below minimum",
			value:    -5.0,
			min:      0.0,
			max:      10.0,
			expected: 0.0,
		},
		{
			name:     "value above maximum",
			value:    15.0,
			min:      0.0,
			max:      10.0,
			expected: 10.0,
		},
		{
			name:     "value equals minimum",
			value:    0.0,
			min:      0.0,
			max:      10.0,
			expected: 0.0,
		},
		{
			name:     "value equals maximum",
			value:    10.0,
			min:      0.0,
			max:      10.0,
			expected: 10.0,
		},
		{
			name:     "negative range",
			value:    -5.0,
			min:      -10.0,
			max:      -1.0,
			expected: -5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClampFloat(tt.value, tt.min, tt.max)
			if result != tt.expected {
				t.Errorf("ClampFloat(%v, %v, %v) = %v, want %v",
					tt.value, tt.min, tt.max, result, tt.expected)
			}
		})
	}
}

func TestEaseOutCubic(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{
			name:     "at start (t=0)",
			input:    0.0,
			expected: 0.0,
		},
		{
			name:     "at end (t=1)",
			input:    1.0,
			expected: 1.0,
		},
		{
			name:     "at midpoint (t=0.5)",
			input:    0.5,
			expected: 0.875, // 1 - 0.5^3 = 0.875
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EaseOutCubic(tt.input)
			if math.Abs(result-tt.expected) > 0.0001 {
				t.Errorf("EaseOutCubic(%v) = %v, want %v",
					tt.input, result, tt.expected)
			}
		})
	}

	// Test that easing function is monotonic increasing
	t.Run("monotonic increasing", func(t *testing.T) {
		prev := EaseOutCubic(0.0)
		for i := 1; i <= 10; i++ {
			input := float64(i) / 10.0
			current := EaseOutCubic(input)
			if current < prev {
				t.Errorf("EaseOutCubic is not monotonic: f(%v)=%v < f(%v)=%v",
					input, current, float64(i-1)/10.0, prev)
			}
			prev = current
		}
	})
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "empty string",
			content:  "",
			expected: 0,
		},
		{
			name:     "single line no newline",
			content:  "hello",
			expected: 1,
		},
		{
			name:     "single line with newline",
			content:  "hello\n",
			expected: 2,
		},
		{
			name:     "two lines",
			content:  "hello\nworld",
			expected: 2,
		},
		{
			name:     "three lines with trailing newline",
			content:  "line1\nline2\nline3\n",
			expected: 4,
		},
		{
			name:     "multiple newlines",
			content:  "\n\n\n",
			expected: 4,
		},
		{
			name:     "single newline",
			content:  "\n",
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CountLines(tt.content)
			if result != tt.expected {
				t.Errorf("CountLines(%q) = %v, want %v",
					tt.content, result, tt.expected)
			}
		})
	}
}
