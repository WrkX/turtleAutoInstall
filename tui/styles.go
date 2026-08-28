package main

import "github.com/charmbracelet/lipgloss"

var (
	colGold   = lipgloss.Color("178")
	colGoldHi = lipgloss.Color("221")
	colCream  = lipgloss.Color("229")
	colMoss   = lipgloss.Color("108")
	colOk     = lipgloss.Color("71")
	colBad    = lipgloss.Color("167")
	colWarn   = lipgloss.Color("180")
	colMuted  = lipgloss.Color("243")
	colFaint  = lipgloss.Color("240")
	colText   = lipgloss.Color("252")
	colAccent = lipgloss.Color("114")
	colBorder = lipgloss.Color("136")
	colDanger = lipgloss.Color("174")
)

var (
	brandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colGoldHi)

	tagStyle = lipgloss.NewStyle().
			Foreground(colMoss).
			Italic(true)

	subStyle = lipgloss.NewStyle().Foreground(colMuted)

	cardTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colGold).
			MarginBottom(0)

	helpStyle = lipgloss.NewStyle().Foreground(colMuted)

	okStyle     = lipgloss.NewStyle().Foreground(colOk)
	badStyle    = lipgloss.NewStyle().Foreground(colBad)
	warnStyle   = lipgloss.NewStyle().Foreground(colWarn)
	dimStyle    = lipgloss.NewStyle().Foreground(colMuted)
	faintStyle  = lipgloss.NewStyle().Foreground(colFaint)
	accentStyle = lipgloss.NewStyle().Foreground(colAccent)
	textStyle   = lipgloss.NewStyle().Foreground(colText)
	errStyle    = lipgloss.NewStyle().Foreground(colBad)

	nameStyle = lipgloss.NewStyle().Foreground(colCream).Width(9)

	selTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(colGoldHi)
	selDescStyle  = lipgloss.NewStyle().Foreground(colMoss)
	itemStyle     = lipgloss.NewStyle().Foreground(colText)
	offStyle      = lipgloss.NewStyle().Foreground(colFaint)
	dangerStyle   = lipgloss.NewStyle().Foreground(colDanger)

	runningTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colGoldHi)

	confirmBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colGold).
			Padding(1, 2)

	headerRuleStyle = lipgloss.NewStyle().Foreground(colBorder)
)

func card(title, body string, width int, border lipgloss.Color) string {
	if width < 16 {
		width = 16
	}
	content := cardTitleStyle.Render(title) + "\n" + body
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(width - 2).
		Render(content)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}
