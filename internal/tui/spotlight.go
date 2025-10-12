package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	// SpotlightMinIntensity is the minimum brightness for faded lines
	SpotlightMinIntensity = 0.3
	// SpotlightFocusRadius controls how wide the bright area is
	SpotlightFocusRadius = 0.4
	// ANSIEscapePrefix is the start of ANSI escape sequences
	ANSIEscapePrefix = "\x1b["
)

var faintLineStyle = lipgloss.NewStyle().Faint(true)

// SpotlightEffect applies a gradient fade effect to content based on scroll position
type SpotlightEffect struct {
	minIntensity float64
	focusRadius  float64
}

// NewSpotlightEffect creates a new spotlight effect with default settings
func NewSpotlightEffect() *SpotlightEffect {
	return &SpotlightEffect{
		minIntensity: SpotlightMinIntensity,
		focusRadius:  SpotlightFocusRadius,
	}
}

// NewSpotlightEffectCustom creates a new spotlight effect with custom settings
func NewSpotlightEffectCustom(minIntensity, focusRadius float64) *SpotlightEffect {
	return &SpotlightEffect{
		minIntensity: minIntensity,
		focusRadius:  focusRadius,
	}
}

// ScrollInfo contains information about the viewport's scroll position
type ScrollInfo struct {
	ScrollPercent float64 // 0.0 to 1.0
	AtTop         bool
	AtBottom      bool
}

// ApplyFade applies a gradient fade to the visible content based on scroll position
func (s *SpotlightEffect) ApplyFade(content string, scroll ScrollInfo) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}

	maxIndex := math.Max(float64(len(lines)-1), 1)

	// Determine focus position based on scroll state
	scrollPercent := ClampFloat(scroll.ScrollPercent, 0, 1)
	switch {
	case scroll.AtTop && !scroll.AtBottom:
		scrollPercent = 0
	case scroll.AtBottom && !scroll.AtTop:
		scrollPercent = 1
	case scroll.AtTop && scroll.AtBottom:
		scrollPercent = 1
	}

	focus := scrollPercent
	focusRadius := math.Max(s.focusRadius, 0.05)

	var result strings.Builder
	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		linePosition := ClampFloat(float64(i)/maxIndex, 0, 1)
		distance := math.Abs(linePosition - focus)
		normalized := ClampFloat(distance/focusRadius, 0, 1)
		focusIntensity := EaseOutCubic(1 - normalized)

		intensity := s.minIntensity + (1-s.minIntensity)*focusIntensity
		intensity = ClampFloat(intensity, s.minIntensity, 1)

		result.WriteString(s.applyColorIntensity(line, intensity))
	}

	return result.String()
}

// applyColorIntensity applies color intensity to a line using ANSI color codes
func (s *SpotlightEffect) applyColorIntensity(line string, intensity float64) string {
	if line == "" {
		return line
	}

	if intensity >= 0.99 {
		return line
	}

	// Preserve pre-existing colored output by falling back to a faint style
	if strings.Contains(line, ANSIEscapePrefix) {
		if intensity > 0.7 {
			return line
		}
		return faintLineStyle.Render(line)
	}

	// Map intensity to grayscale ANSI colors (232-255 are grayscale)
	// 232 = darkest, 255 = brightest (white)
	colorCode := max(min(232+int(math.Round(intensity*23)), 255), 232)

	return fmt.Sprintf("\x1b[38;5;%dm%s\x1b[0m", colorCode, line)
}
