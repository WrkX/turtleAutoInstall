package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

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

func (m *model) openConsoles() (tea.Model, tea.Cmd) {
	name := "mangosd"
	if !m.status.Mangosd && m.status.Realmd {
		name = "realmd"
	}
	return m.openOverlay(name)
}
