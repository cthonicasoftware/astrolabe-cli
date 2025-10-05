package tui

import "github.com/charmbracelet/lipgloss"

// Color Palette - centralized color definitions
var (
	ColorPrimary   = lipgloss.Color("#00D9FF") // Cyan - main highlights, cursor
	ColorSecondary = lipgloss.Color("#7D56F4") // Purple - headers
	ColorSuccess   = lipgloss.Color("#04B575") // Green - selected items, success states
	ColorWarning   = lipgloss.Color("#FFD700") // Yellow - warnings, active states
	ColorError     = lipgloss.Color("#FF5F87") // Pink - errors, logo
	ColorMuted     = lipgloss.Color("#626262") // Gray - unselected, help text
	ColorText      = lipgloss.Color("#FFFFFF") // White - normal text
)

// Common Styles - reusable across all TUI components
var (
	// Text styles
	StyleBold = lipgloss.NewStyle().
			Bold(true)

	StyleItalic = lipgloss.NewStyle().
			Italic(true)

	StyleDim = lipgloss.NewStyle().
			Faint(true)

	// Component styles
	StyleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)

	StyleTitle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			MarginTop(1).
			MarginBottom(1)

	StyleHeader = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true).
			MarginTop(1)

	StyleSubheader = lipgloss.NewStyle().
			Foreground(ColorText).
			Bold(true)

	// Interactive element styles
	StyleCursor = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	StyleSelected = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true).
			PaddingLeft(1)

	StyleUnselected = lipgloss.NewStyle().
			Foreground(ColorMuted).
			PaddingLeft(1)

	StyleHighlight = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	// Status styles
	StyleSuccess = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	StyleError = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true).
			MarginTop(1)

	StyleWarning = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	StyleMuted = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Help text
	StyleHelp = lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginTop(1).
			Italic(true)

	// Key-value display
	StyleKey = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Width(8)

	StyleValue = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	// Icon styles
	StyleIcon = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)
)
