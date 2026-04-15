package main

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("97"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	directionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("36"))
	stopNameStyle  = lipgloss.NewStyle().Bold(true)
	uuidStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	redBoldStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	yellowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	greenStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
	lateStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))

	footerBorderStyle = lipgloss.NewStyle().
				BorderTop(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("240")).
				PaddingTop(0)

	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))

	favoriteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220")) // gold ★
	selectedStyle = lipgloss.NewStyle().Background(lipgloss.Color("235")) // subtle row highlight
	cursorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))  // green >
)
