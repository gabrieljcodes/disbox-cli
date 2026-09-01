package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Brand Colors
	ColorPrimary   = lipgloss.Color("#00F5FF") // Neon Cyan
	ColorSecondary = lipgloss.Color("#00E5A3") // Emerald Green
	ColorAccent    = lipgloss.Color("#9D4EDD") // Vibrant Purple
	ColorWarning   = lipgloss.Color("#FFB703") // Amber
	ColorDanger    = lipgloss.Color("#EF233C") // Crimson
	ColorMuted     = lipgloss.Color("#6C757D") // Gray
	ColorBgDark    = lipgloss.Color("#0B0F19") // Dark Slate
	ColorPanel     = lipgloss.Color("#161B26") // Panel Dark
	ColorBorder    = lipgloss.Color("#272E3F") // Border Gray
	ColorText      = lipgloss.Color("#F8F9FA") // Off-White

	// Typography & Containers
	StyleApp = lipgloss.NewStyle().
			Padding(0, 1).
			Background(ColorBgDark).
			Foreground(ColorText)

	StyleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	StyleLogo = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Background(lipgloss.Color("#132338")).
			Padding(0, 1).
			MarginRight(1)

	StyleTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Background(ColorPanel).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1)

	StyleTabInactive = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Background(ColorBgDark).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorder).
				Padding(0, 1)

	StylePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Background(ColorPanel).
			Padding(0, 1)

	StyleBadgeGreen = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(ColorSecondary).
			Padding(0, 1)

	StyleBadgeBlue = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(ColorPrimary).
			Padding(0, 1)

	StyleBadgeYellow = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#000000")).
				Background(ColorWarning).
				Padding(0, 1)

	StyleBadgeRed = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(ColorDanger).
			Padding(0, 1)

	StyleHelp = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleKey = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	StyleValue = lipgloss.NewStyle().
			Foreground(ColorText)

	StyleSuccess = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	StyleError = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)

	StyleRowSelected = lipgloss.NewStyle().
				Background(lipgloss.Color("#1E293B")).
				Foreground(ColorPrimary).
				Bold(true)
)

func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func FormatSpeed(bps int64) string {
	return FormatBytes(bps) + "/s"
}

func FormatETA(seconds int64) string {
	if seconds <= 0 {
		return "—"
	}
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm %ds", seconds/60, seconds%60)
	}
	return fmt.Sprintf("%dh %dm", seconds/3600, (seconds%3600)/60)
}
