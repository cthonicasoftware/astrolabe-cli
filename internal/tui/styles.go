package tui

import (
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

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

	IconMenuCapture    = " " // nf-fa-bar_chart
	IconMenuListPorts  = " " // nf-fa-usb
	IconMenuMetadata   = " " // nf-fa-id_badge
	IconMenuViewRuns   = " " // nf-fa-database
	IconMenuUpload     = " " // nf-fa-upload
	IconMenuConfig     = " " // nf-fa-cog
	IconMenuConnection = "󱘖 " // nf-md-connection
	IconMenuNewFile    = " " // nf-fa-file_text
	IconSelectedItem   = "❯ "
	IconMenuSeparator  = "\ue621" // nf-indentation line

	IconTitlePorts = IconMenuListPorts
)

const (
	// Status indicators
	IconAlchemyComplete = "🜏 "
	IconAlchemyUpload   = "🜍 "
	IconAlchemySuccess  = ""
	IconAlchemyWarning  = "⌽ "
	IconAlchemyError    = "⊗ "
	IconAlchemySettings = "⚖ "

	// Operational states
	IconAlchemyActive     = "🜃 " // Fire - operations in progress
	IconAlchemyProcess    = "⚗ " // Alembic - data transformation
	IconAlchemyCached     = "🜄 " // Earth - stored locally
	IconAlchemyStreaming  = "🜂 " // Water - live data flow
	IconAlchemyInfo       = "🜔 " // Quintessence - information/knowledge
	IconAlchemyDownload   = "🝱 " // Precipitate - download operations
	IconAlchemyConnected  = "🜁 " // Air - connection active
	IconAlchemyValidation = "🜨 " // Retort - validation/verification
)

// Progress bar configuration
// Custom gradient that matches our color palette (cyan → purple)
var (
	ProgressGradientStart   = lipgloss.Color("#00D9FF") // Cyan
	ProgressGradientEnd     = lipgloss.Color("#7D56F4") // Purple
	DefaultProgressGradient = progress.WithGradient(string(ProgressGradientStart), string(ProgressGradientEnd))
)

// NewDefaultProgress creates a progress bar with consistent styling across the application.
func NewDefaultProgress(width int) progress.Model {
	p := progress.New(
		DefaultProgressGradient,
		progress.WithWidth(width),
		progress.WithoutPercentage(),
	)
	return p
}

// NewDefaultSpinner creates a spinner with consistent styling across the application.
func NewDefaultSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorPrimary)
	return s
}

// Styled alchemy symbols for status display
var (
	// Status indicators
	StyledAlchemySuccess = lipgloss.NewStyle().Foreground(ColorSuccess).SetString(IconAlchemySuccess)
	StyledAlchemyError   = lipgloss.NewStyle().Foreground(ColorError).SetString(IconAlchemyError)
	StyledAlchemyWarning = lipgloss.NewStyle().Foreground(ColorWarning).SetString(IconAlchemyWarning)

	// Operational states
	StyledAlchemyActive     = lipgloss.NewStyle().Foreground(ColorWarning).SetString(IconAlchemyActive)     // Yellow for active ops
	StyledAlchemyProcess    = lipgloss.NewStyle().Foreground(ColorPrimary).SetString(IconAlchemyProcess)    // Cyan for processing
	StyledAlchemyCached     = lipgloss.NewStyle().Foreground(ColorMuted).SetString(IconAlchemyCached)       // Gray for cached
	StyledAlchemyStreaming  = lipgloss.NewStyle().Foreground(ColorPrimary).SetString(IconAlchemyStreaming)  // Cyan for streaming
	StyledAlchemyInfo       = lipgloss.NewStyle().Foreground(ColorSecondary).SetString(IconAlchemyInfo)     // Purple for info
	StyledAlchemyConnected  = lipgloss.NewStyle().Foreground(ColorSuccess).SetString(IconAlchemyConnected)  // Green for connected
	StyledAlchemyValidation = lipgloss.NewStyle().Foreground(ColorPrimary).SetString(IconAlchemyValidation) // Cyan for validation
)
