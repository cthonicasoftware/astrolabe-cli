package tui

import "strings"

// ClampFloat constrains a float64 value between min and max
func ClampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// EaseOutCubic applies cubic easing to a value (0-1)
func EaseOutCubic(t float64) float64 {
	inv := 1 - t
	return 1 - (inv * inv * inv)
}

// CountLines returns the number of lines in a string
func CountLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}
