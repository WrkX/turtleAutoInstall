package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const consoleTailBytes = 256 * 1024

var consoleDaemons = []string{"realmd", "mangosd"}

func realmLogCandidates(root, name string) []string {
	logs := filepath.Join(root, "logs")
	switch strings.ToLower(name) {
	case "realmd":
		return []string{
			filepath.Join(logs, "Realmd.log"),
			filepath.Join(logs, "realmd.log"),
		}
	case "mangosd":
		return []string{
			filepath.Join(logs, "server.log"),
			filepath.Join(logs, "Server.log"),
		}
	default:
		return nil
	}
}

func realmLogPath(root, name string) string {
	for _, path := range realmLogCandidates(root, name) {
		if fileExists(path) {
			return path
		}
	}
	if cands := realmLogCandidates(root, name); len(cands) > 0 {
		return cands[0]
	}
	return ""
}

func realmLogExists(root, name string) bool {
	for _, path := range realmLogCandidates(root, name) {
		if fileExists(path) {
			return true
		}
	}
	return false
}

func tailFile(path string, maxBytes int) string {
	if path == "" {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	size := info.Size()
	start := int64(0)
	if size > int64(maxBytes) {
		start = size - int64(maxBytes)
	}
	if _, err := f.Seek(start, 0); err != nil {
		return ""
	}
	buf := make([]byte, size-start)
	n, err := f.Read(buf)
	if n == 0 {
		if err != nil {
			return ""
		}
		return ""
	}
	text := string(buf[:n])
	if start > 0 {
		if i := strings.IndexByte(text, '\n'); i >= 0 && i+1 < len(text) {
			text = text[i+1:]
		}
		return fmt.Sprintf("… earlier log omitted …\n%s", text)
	}
	return text
}

func realmConsoleContent(root, name string) string {
	path := realmLogPath(root, name)
	body := strings.TrimRight(tailFile(path, consoleTailBytes), "\n")
	if body == "" {
		if path == "" {
			return dimStyle.Render("no log file yet")
		}
		return dimStyle.Render("waiting for " + filepath.Base(path))
	}
	return body
}

func (m *model) openConsoles() (tea.Model, tea.Cmd) {
	m.screen = screenConsoles
	m.consoleTab = 1
	if !m.status.Mangosd && m.status.Realmd {
		m.consoleTab = 0
	}
	m.layout()
	m.reloadConsoles(true)
	return m, nil
}

func (m *model) reloadConsoles(follow bool) {
	if m.consoleTab < 0 || m.consoleTab >= len(consoleDaemons) {
		m.consoleTab = 0
	}
	name := consoleDaemons[m.consoleTab]
	atBottom := m.vp.AtBottom()
	m.vp.SetContent(realmConsoleContent(m.root, name))
	if follow || atBottom {
		m.vp.GotoBottom()
	}
}

func (m *model) onConsolesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenHome
		return m, nil
	case "tab", "right", "l":
		m.consoleTab = (m.consoleTab + 1) % len(consoleDaemons)
		m.reloadConsoles(true)
		return m, nil
	case "shift+tab", "left", "h":
		m.consoleTab = (m.consoleTab + len(consoleDaemons) - 1) % len(consoleDaemons)
		m.reloadConsoles(true)
		return m, nil
	case "1":
		m.consoleTab = 0
		m.reloadConsoles(true)
		return m, nil
	case "2":
		m.consoleTab = 1
		m.reloadConsoles(true)
		return m, nil
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m model) consolesView() string {
	header := renderHeader(m.status, m.width)
	name := consoleDaemons[m.consoleTab]
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
	tabs := make([]string, 0, len(consoleDaemons))
	for i, d := range consoleDaemons {
		label := d
		if i == m.consoleTab {
			label = selTitleStyle.Render(d)
		} else {
			label = dimStyle.Render(d)
		}
		tabs = append(tabs, label)
	}
	head := strings.Join(tabs, dimStyle.Render("  ·  ")) + "  " + state
	help := helpStyle.Render("tab switch  1 realmd  2 mangosd  ↑↓ scroll  esc back")

	used := lipgloss.Height(header) + lipgloss.Height(head) + lipgloss.Height(help) + 2
	vpH := max(m.height-used, 6)
	m.vp.Width = max(m.width-4, 16)
	m.vp.Height = vpH

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colBorder).
		Width(max(m.width-2, 20)).
		Render(m.vp.View())

	return lipgloss.JoinVertical(lipgloss.Left, header, head, box, help)
}
