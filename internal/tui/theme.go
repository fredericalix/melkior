package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	primaryColor   = lipgloss.Color("#7D56F4")
	secondaryColor = lipgloss.Color("#F76D6D")
	successColor   = lipgloss.Color("#04B575")
	warningColor   = lipgloss.Color("#FFA500")
	errorColor     = lipgloss.Color("#FF0000")
	mutedColor     = lipgloss.Color("#626262")
	bgColor        = lipgloss.Color("#1A1B26")
	fgColor        = lipgloss.Color("#C0CAF5")

	// Status colors
	statusUpColor       = lipgloss.Color("#04B575")
	statusDownColor     = lipgloss.Color("#FF0000")
	statusDegradedColor = lipgloss.Color("#FFA500")
	statusUnknownColor  = lipgloss.Color("#626262")

	// Base styles
	baseStyle = lipgloss.NewStyle().
			Background(bgColor).
			Foreground(fgColor)

	// Header styles
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	// Tab styles
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fgColor).
			Background(primaryColor).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Padding(0, 2)

	tabGapStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Table styles
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(mutedColor)

	tableCellStyle = lipgloss.NewStyle().
			PaddingRight(2)

	selectedRowStyle = lipgloss.NewStyle().
				Background(primaryColor).
				Foreground(fgColor).
				Bold(true)

	// Footer styles
	footerStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)

	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#2A2B3C")).
			Padding(0, 1)

	// Help styles
	helpKeyStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	helpTextStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Log styles
	logTimestampStyle = lipgloss.NewStyle().
				Foreground(mutedColor)

	logEventStyle = lipgloss.NewStyle().
			Foreground(fgColor)

	// Details pane styles
	labelKeyStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	labelValueStyle = lipgloss.NewStyle().
			Foreground(fgColor)

	// Border styles
	focusedBorderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor)

	unfocusedBorderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(mutedColor)

	// Error styles
	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	// Success styles
	successStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	// Warning styles
	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true)
)

// GetStatusColor returns the color for a status
func GetStatusColor(status string) lipgloss.Color {
	switch status {
	case "UP":
		return statusUpColor
	case "DOWN":
		return statusDownColor
	case "DEGRADED":
		return statusDegradedColor
	default:
		return statusUnknownColor
	}
}

// GetStatusStyle returns the style for a status
func GetStatusStyle(status string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(GetStatusColor(status)).Bold(true)
}

// GetTypeColor returns the color for a node type
func GetTypeColor(nodeType string) lipgloss.Color {
	switch nodeType {
	case "BAREMETAL":
		return lipgloss.Color("#FF6B6B")
	case "VM":
		return lipgloss.Color("#4ECDC4")
	case "CONTAINER":
		return lipgloss.Color("#95E1D3")
	default:
		return mutedColor
	}
}

// GetTypeStyle returns the style for a node type
func GetTypeStyle(nodeType string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(GetTypeColor(nodeType))
}