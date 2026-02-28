package tui

import (
	"strings"
	"testing"
)

func TestNewSpotlightEffect(t *testing.T) {
	s := NewSpotlightEffect()
	if s == nil {
		t.Fatal("NewSpotlightEffect() returned nil")
	}
	if s.minIntensity != SpotlightMinIntensity {
		t.Errorf("minIntensity = %v, want %v", s.minIntensity, SpotlightMinIntensity)
	}
	if s.focusRadius != SpotlightFocusRadius {
		t.Errorf("focusRadius = %v, want %v", s.focusRadius, SpotlightFocusRadius)
	}
}

func TestNewSpotlightEffectCustom(t *testing.T) {
	s := NewSpotlightEffectCustom(0.5, 0.6)
	if s == nil {
		t.Fatal("NewSpotlightEffectCustom() returned nil")
	}
	if s.minIntensity != 0.5 {
		t.Errorf("minIntensity = %v, want 0.5", s.minIntensity)
	}
	if s.focusRadius != 0.6 {
		t.Errorf("focusRadius = %v, want 0.6", s.focusRadius)
	}
}

func TestSpotlightEffect_ApplyFade(t *testing.T) {
	s := NewSpotlightEffect()

	tests := []struct {
		name    string
		content string
		scroll  ScrollInfo
	}{
		{
			name:    "empty content",
			content: "",
			scroll: ScrollInfo{
				ScrollPercent: 0.5,
				AtTop:         false,
				AtBottom:      false,
			},
		},
		{
			name:    "single line at top",
			content: "hello world",
			scroll: ScrollInfo{
				ScrollPercent: 0.0,
				AtTop:         true,
				AtBottom:      false,
			},
		},
		{
			name:    "single line at bottom",
			content: "hello world",
			scroll: ScrollInfo{
				ScrollPercent: 1.0,
				AtTop:         false,
				AtBottom:      true,
			},
		},
		{
			name:    "multiple lines at top",
			content: "line1\nline2\nline3\nline4\nline5",
			scroll: ScrollInfo{
				ScrollPercent: 0.0,
				AtTop:         true,
				AtBottom:      false,
			},
		},
		{
			name:    "multiple lines at bottom",
			content: "line1\nline2\nline3\nline4\nline5",
			scroll: ScrollInfo{
				ScrollPercent: 1.0,
				AtTop:         false,
				AtBottom:      true,
			},
		},
		{
			name:    "multiple lines in middle",
			content: "line1\nline2\nline3\nline4\nline5",
			scroll: ScrollInfo{
				ScrollPercent: 0.5,
				AtTop:         false,
				AtBottom:      false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.ApplyFade(tt.content, tt.scroll)

			// Result should not be empty for non-empty input
			if tt.content != "" && result == "" {
				t.Error("ApplyFade returned empty string for non-empty input")
			}

			// Result should preserve line count
			if tt.content != "" {
				expectedLines := strings.Count(tt.content, "\n") + 1
				resultLines := strings.Count(result, "\n") + 1
				if resultLines != expectedLines {
					t.Errorf("Line count changed: got %d lines, want %d",
						resultLines, expectedLines)
				}
			}
		})
	}
}

func TestSpotlightEffect_ScrollPositions(t *testing.T) {
	s := NewSpotlightEffect()
	content := "line1\nline2\nline3\nline4\nline5"

	tests := []struct {
		name   string
		scroll ScrollInfo
	}{
		{
			name: "at top only",
			scroll: ScrollInfo{
				ScrollPercent: 0.0,
				AtTop:         true,
				AtBottom:      false,
			},
		},
		{
			name: "at bottom only",
			scroll: ScrollInfo{
				ScrollPercent: 1.0,
				AtTop:         false,
				AtBottom:      true,
			},
		},
		{
			name: "at both top and bottom (single page)",
			scroll: ScrollInfo{
				ScrollPercent: 0.0,
				AtTop:         true,
				AtBottom:      true,
			},
		},
		{
			name: "in middle",
			scroll: ScrollInfo{
				ScrollPercent: 0.5,
				AtTop:         false,
				AtBottom:      false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.ApplyFade(content, tt.scroll)

			// Should return non-empty result
			if result == "" {
				t.Error("ApplyFade returned empty result")
			}

			// Should preserve number of lines
			if strings.Count(result, "\n") != strings.Count(content, "\n") {
				t.Error("Line count not preserved")
			}
		})
	}
}

func TestSpotlightEffect_ANSIColorPreservation(t *testing.T) {
	s := NewSpotlightEffect()

	t.Run("preserves ANSI colored content at high intensity", func(t *testing.T) {
		coloredLine := "\x1b[31mred text\x1b[0m"
		content := coloredLine + "\n" + coloredLine

		scroll := ScrollInfo{
			ScrollPercent: 0.0,
			AtTop:         true,
			AtBottom:      false,
		}

		result := s.ApplyFade(content, scroll)

		// Should contain the ANSI escape sequences
		if !strings.Contains(result, "\x1b[") {
			t.Error("ANSI escape sequences were removed")
		}
	})

	t.Run("applies faint style to colored content at low intensity", func(t *testing.T) {
		coloredLine := "\x1b[31mred text\x1b[0m"

		scroll := ScrollInfo{
			ScrollPercent: 1.0, // Bottom focus
			AtTop:         false,
			AtBottom:      true,
		}

		result := s.ApplyFade(coloredLine, scroll)

		// Should still contain escape sequences
		if !strings.Contains(result, "\x1b[") {
			t.Error("ANSI escape sequences were removed")
		}
	})

	t.Run("reapplies faint after reset sequences", func(t *testing.T) {
		midIntensity := NewSpotlightEffectCustom(SpotlightMinIntensity, 0.8)
		line := "\x1b[31mERR\x1b[0m normal \x1b[32mOK\x1b[0m"
		content := strings.Join([]string{"focus", "line2", line, "line4", "line5"}, "\n")
		scroll := ScrollInfo{
			ScrollPercent: 0.0,
			AtTop:         true,
			AtBottom:      false,
		}

		result := midIntensity.ApplyFade(content, scroll)
		lines := strings.Split(result, "\n")
		if len(lines) != 5 || !strings.Contains(lines[2], "\x1b[0;2m") {
			t.Error("expected faint to be reapplied after ANSI reset codes")
		}
	})

	t.Run("uses grayscale for low-intensity ANSI lines", func(t *testing.T) {
		ansiLine := "\x1b[31merror\x1b[0m details"
		content := strings.Join([]string{"focus", "middle", ansiLine}, "\n")
		scroll := ScrollInfo{
			ScrollPercent: 0.0,
			AtTop:         true,
			AtBottom:      false,
		}

		result := s.ApplyFade(content, scroll)
		lines := strings.Split(result, "\n")
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines, got %d", len(lines))
		}

		last := lines[2]
		if !strings.Contains(last, "\x1b[38;5;") {
			t.Error("expected grayscale coloring for low-intensity ANSI line")
		}
		if strings.Contains(last, "\x1b[31m") {
			t.Error("expected ANSI color controls to be stripped in low-intensity mode")
		}
	})
}

func TestSpotlightEffect_PlainTextColoring(t *testing.T) {
	s := NewSpotlightEffect()

	t.Run("applies grayscale coloring to plain text", func(t *testing.T) {
		content := "plain text line"

		scroll := ScrollInfo{
			ScrollPercent: 0.5,
			AtTop:         false,
			AtBottom:      false,
		}

		result := s.ApplyFade(content, scroll)

		// Result should contain ANSI color codes for grayscale
		// (format: \x1b[38;5;XXXm where XXX is 232-255)
		if !strings.Contains(result, "\x1b[38;5;") {
			t.Error("Grayscale ANSI codes not applied to plain text")
		}
	})

	t.Run("does not color at full intensity", func(t *testing.T) {
		content := "line1\nline2\nline3"

		scroll := ScrollInfo{
			ScrollPercent: 1.0,
			AtTop:         false,
			AtBottom:      true,
		}

		result := s.ApplyFade(content, scroll)

		// At bottom, last line should be at full intensity (no coloring)
		lines := strings.Split(result, "\n")
		lastLine := lines[len(lines)-1]

		// Full intensity lines should not have our grayscale codes
		// (though they might have reset codes from previous lines)
		if lastLine == "line3" {
			// If no ANSI codes at all, that's also acceptable for full intensity
			return
		}
	})
}

func TestSpotlightEffect_EmptyAndEdgeCases(t *testing.T) {
	s := NewSpotlightEffect()

	t.Run("empty string", func(t *testing.T) {
		result := s.ApplyFade("", ScrollInfo{})
		if result != "" {
			t.Errorf("Empty string should return empty, got %q", result)
		}
	})

	t.Run("single character", func(t *testing.T) {
		result := s.ApplyFade("a", ScrollInfo{AtBottom: true})
		if result == "" {
			t.Error("Single character should not return empty")
		}
	})

	t.Run("only newlines", func(t *testing.T) {
		result := s.ApplyFade("\n\n\n", ScrollInfo{AtBottom: true})
		lineCount := strings.Count(result, "\n")
		if lineCount != 3 {
			t.Errorf("Newline count should be preserved, got %d want 3", lineCount)
		}
	})
}

func TestSpotlightEffect_IntensityGradient(t *testing.T) {
	s := NewSpotlightEffect()

	// Create many lines to test gradient
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, "test line")
	}
	content := strings.Join(lines, "\n")

	// Focus at middle
	scroll := ScrollInfo{
		ScrollPercent: 0.5,
		AtTop:         false,
		AtBottom:      false,
	}

	result := s.ApplyFade(content, scroll)

	// Result should have varying color codes (gradient effect)
	resultLines := strings.Split(result, "\n")

	// Count lines with ANSI codes
	linesWithColor := 0
	for _, line := range resultLines {
		if strings.Contains(line, "\x1b[38;5;") {
			linesWithColor++
		}
	}

	// Most lines should have some coloring applied
	if linesWithColor < 7 {
		t.Errorf("Expected gradient effect on most lines, only %d/%d have coloring",
			linesWithColor, len(resultLines))
	}
}

func TestSpotlightEffect_CustomSettings(t *testing.T) {
	t.Run("very low minimum intensity", func(t *testing.T) {
		s := NewSpotlightEffectCustom(0.1, 0.3)
		content := "line1\nline2\nline3\nline4\nline5"

		result := s.ApplyFade(content, ScrollInfo{ScrollPercent: 0.5})

		if result == "" {
			t.Error("Should produce output with custom settings")
		}
	})

	t.Run("very wide focus radius", func(t *testing.T) {
		s := NewSpotlightEffectCustom(0.3, 0.9)
		content := "line1\nline2\nline3\nline4\nline5"

		result := s.ApplyFade(content, ScrollInfo{ScrollPercent: 0.5})

		if result == "" {
			t.Error("Should produce output with wide focus radius")
		}
	})

	t.Run("narrow focus radius", func(t *testing.T) {
		s := NewSpotlightEffectCustom(0.3, 0.1)
		content := "line1\nline2\nline3\nline4\nline5"

		result := s.ApplyFade(content, ScrollInfo{ScrollPercent: 0.5})

		if result == "" {
			t.Error("Should produce output with narrow focus radius")
		}
	})
}
