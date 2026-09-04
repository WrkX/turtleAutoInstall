package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m *model) openOverlay(name string) (tea.Model, tea.Cmd) {
	if name != "realmd" && name != "mangosd" {
		return m, nil
	}
	m.overlay = name
	m.reloadOverlay(true)
	return m, nil
}

func (m *model) closeOverlay() {
	m.overlay = ""
}

func (m *model) overlayContent(name string) string {
	running := false
	switch name {
	case "realmd":
		running = m.status.Realmd
	case "mangosd":
		running = m.status.Mangosd
	}
	if m.capture != nil && m.capture.Has(name) {
		body := strings.TrimRight(m.capture.Output(name), "\n")
		if body == "" {
			return dimStyle.Render("waiting for stdout…")
		}
		return body
	}
	if running {
		return dimStyle.Render("stdout isn't available — start from this TUI to capture it")
	}
	return dimStyle.Render(name + " is not running")
}

func (m *model) reloadOverlay(follow bool) {
	if m.overlay == "" {
		return
	}
	boxW := min(max(m.width-8, 40), 100)
	innerW := max(boxW-4, 20)
	boxH := min(max(m.height-8, 10), max(m.height/2, 12))
	m.overlayVp.Width = innerW
	m.overlayVp.Height = max(boxH-6, 6)
	atBottom := m.overlayVp.AtBottom()
	m.overlayVp.SetContent(m.overlayContent(m.overlay))
	if follow || atBottom {
		m.overlayVp.GotoBottom()
	}
}

func (m *model) onOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeOverlay()
		return m, nil
	case "q":
		return m.beginQuit()
	case "f1":
		return m.openOverlay("realmd")
	case "f2":
		return m.openOverlay("mangosd")
	}
	var cmd tea.Cmd
	m.overlayVp, cmd = m.overlayVp.Update(msg)
	return m, cmd
}

func (m *model) onMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.stopping || m.quitting {
		return m, nil
	}
	if m.screen != screenHome && m.screen != screenRun {
		return m, nil
	}
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		if name := m.status.pillAt(msg.X, msg.Y); name != "" {
			return m.openOverlay(name)
		}
		return m, nil
	}
	if m.overlay == "" {
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelUp {
		m.overlayVp.ScrollUp(3)
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelDown {
		m.overlayVp.ScrollDown(3)
		return m, nil
	}
	return m, nil
}

func (m model) overlayBox() string {
	name := m.overlay
	running := false
	switch name {
	case "realmd":
		running = m.status.Realmd
	case "mangosd":
		running = m.status.Mangosd
	}
	state := dimStyle.Render("stopped")
	if running {
		state = okStyle.Render("running")
	}
	if m.capture != nil && m.capture.Has(name) && running {
		state = okStyle.Render("live stdout")
	}
	head := selTitleStyle.Render(name) + "  " + state
	help := helpStyle.Render("f1 realmd  f2 mangosd  ↑↓ scroll  esc close  q quit and stop")

	boxW := min(max(m.width-8, 40), 100)
	innerW := max(boxW-4, 20)
	m.overlayVp.Width = innerW
	used := 6
	boxH := min(max(m.height-8, 10), max(m.height/2, 12))
	m.overlayVp.Height = max(boxH-used, 6)

	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colGold).
		Width(innerW + 2).
		Render(m.overlayVp.View())
	return lipgloss.JoinVertical(lipgloss.Left, head, body, help)
}

func stampOverlay(base, over string, width, height int) string {
	if over == "" || width < 1 || height < 1 {
		return base
	}
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}
	overLines := strings.Split(over, "\n")
	oh := len(overLines)
	ow := 0
	for _, line := range overLines {
		if w := lipgloss.Width(line); w > ow {
			ow = w
		}
	}
	y := max((height-oh)/2, 3)
	x := max((width-ow)/2, 0)
	for i, ol := range overLines {
		yi := y + i
		if yi < 0 || yi >= len(baseLines) {
			continue
		}
		baseLines[yi] = stampLine(baseLines[yi], ol, x, ow, width)
	}
	if height > 0 && len(baseLines) > height {
		baseLines = baseLines[:height]
	}
	return strings.Join(baseLines, "\n")
}

func stampLine(base, over string, x, ow, width int) string {
	if width <= 0 {
		return ""
	}
	if x < 0 {
		x = 0
	}
	for lipgloss.Width(over) < ow {
		over += " "
	}
	prefix := ansi.Cut(base, 0, x)
	for lipgloss.Width(prefix) < x {
		prefix += " "
	}
	suffix := ansi.TruncateLeft(base, x+ow, "")
	out := prefix + over + suffix
	if lipgloss.Width(out) > width {
		return ansi.Truncate(out, width, "")
	}
	return out
}
