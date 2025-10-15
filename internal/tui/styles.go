package tui

import "github.com/charmbracelet/lipgloss"

const menuWidth = 44
const styleKeyWidth = 8
const marginSize = 1

// Color Palette - centralized color definitions
var (
	ColorPrimary   = lipgloss.Color("#00D9FF") // Cyan - main highlights, cursor
	ColorSecondary = lipgloss.Color("#7D56F4") // Purple - headers
	ColorSuccess   = lipgloss.Color("#04B575") // Green - selected items, success states
	ColorWarning   = lipgloss.Color("#FFD700") // Yellow - warnings, active states
	ColorError     = lipgloss.Color("#FF5F87") // Pink - errors, logo
	ColorMuted     = lipgloss.Color("#626262") // Gray - unselected, help text
	ColorText      = lipgloss.Color("#FFFFFF") // White - normal text
	ColorHighlight = lipgloss.Color("#313244") // Slate highlight for selections
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

	StyleSelected = lipgloss.NewStyle().Bold(true).Italic(true).Foreground(ColorWarning).Background(ColorHighlight)

	// Component styles
	StyleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)

	StyleTitle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			MarginTop(marginSize).
			MarginBottom(marginSize)

	StyleHeader = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true).
			MarginTop(marginSize)

	StyleSubheader = lipgloss.NewStyle().
			Foreground(ColorText).
			Bold(true)

	// Interactive element styles
	StyleCursor = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	StyleUnselected = lipgloss.NewStyle().
			Foreground(ColorMuted).
			PaddingLeft(marginSize)

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
			MarginTop(marginSize)

	StyleWarning = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	StyleMuted = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Help text

	StyleMenuItem = lipgloss.NewStyle().
			Foreground(ColorText).
			Padding(0, 2).
			Width(menuWidth)

	StyleMenuSelected = lipgloss.NewStyle().
				Background(ColorHighlight).
				Foreground(ColorWarning).
				Bold(true).
				Width(menuWidth)

	StyleHelp = lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginTop(marginSize).
			Italic(true)

	// Key-value display
	StyleKey = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Width(styleKeyWidth)

	StyleValue = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	// Icon styles
	StyleIcon = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)
)

// Icon Palette - Nerd Font glyphs
// Update these values as needed
const (
	IconStatusInfo    = " " // nf-fa-info_circle
	IconStatusSuccess = " " // nf-fa-check
	IconStatusWarning = " " // nf-fa-exclamation_triangle
	IconStatusError   = " " // nf-fa-times

	IconMenuCapture   = " " // nf-fa-bar_chart
	IconMenuListPorts = " " // nf-fa-usb
	IconMenuMetadata  = " " // nf-fa-id_badge
	IconMenuViewRuns  = " " // nf-fa-database
	IconMenuUpload    = " " // nf-fa-upload
	IconMenuConfig    = " " // nf-fa-cog
	IconMenuNewFile   = " " // nf-fa-file_text
	IconSelectedItem  = "❯ "
	IconMenuSeparator = "\ue621" // nf-indentation line

	IconTitlePorts = IconMenuListPorts
)

const (
	IconAlchemyComplete = "🜏 "
	IconAlchemyUpload   = "🜍 "
	IconAlchemySuccess  = ""
	IconAlchemyWarning  = "⌽ "
	IconAlchemyError    = "⊗ "
	IconAlchemySettings = "⚖ "
)
