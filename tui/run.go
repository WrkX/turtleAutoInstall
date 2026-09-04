package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/WrkX/tortoise-wow-portable/internal/portable"
	tea "github.com/charmbracelet/bubbletea"
)

type scriptStep struct {
	Label  string
	Script string
	Args   []string
	Env    map[string]string
}

type lineMsg string
type doneMsg struct{ err error }
type statusMsg Status
type tickMsg struct{}

type streamStartedMsg struct {
	lines  <-chan string
	done   <-chan error
	cancel func()
}

func startStream(root, script string, args []string) tea.Cmd {
	return startStreamEnv(root, script, args, nil)
}

func startStreamEnv(root, script string, args []string, extraEnv map[string]string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		lines := make(chan string, 64)
		done := make(chan error, 1)
		var cancelOnce sync.Once
		stop := func() {
			cancelOnce.Do(func() {
				cancel()
				cleanupPartialArtifacts(root)
			})
		}

		go func() {
			err := portable.Run(ctx, root, script, args, extraEnv, func(s string) {
				select {
				case lines <- s:
				case <-ctx.Done():
				}
			})
			done <- err
			close(lines)
		}()

		return streamStartedMsg{lines: lines, done: done, cancel: stop}
	}
}

func cleanupPartialArtifacts(root string) {
	cache := filepath.Join(root, "tools", ".cache")
	entries, err := os.ReadDir(cache)
	if err == nil {
		for _, entry := range entries {
			name := entry.Name()
			remove := strings.HasSuffix(name, ".partial") ||
				strings.HasSuffix(name, ".download") ||
				(strings.HasPrefix(name, "maps-url.txt.") && strings.HasSuffix(name, ".tmp")) ||
				name == "maps-extract" || name == "server-extract" ||
				name == "sql-extract" || strings.HasPrefix(name, "extract-")
			if remove {
				_ = os.RemoveAll(filepath.Join(cache, name))
			}
		}
	}

	credentialDir := filepath.Join(root, "data", "mysql-tmp")
	credentialEntries, err := os.ReadDir(credentialDir)
	if err != nil {
		return
	}
	for _, entry := range credentialEntries {
		name := entry.Name()
		if strings.HasPrefix(name, ".mysql-client-") && strings.HasSuffix(name, ".cnf") {
			_ = os.Remove(filepath.Join(credentialDir, name))
		}
	}
}

func awaitLine(lines <-chan string, done <-chan error) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-lines
		if !ok {
			return doneMsg{err: <-done}
		}
		return lineMsg(line)
	}
}

func refreshStatus(root string) tea.Cmd {
	return func() tea.Msg {
		return statusMsg(gatherStatus(root))
	}
}

func scheduleTick(sc screen) tea.Cmd {
	d := 2500 * time.Millisecond
	if sc == screenConsoles {
		d = 800 * time.Millisecond
	}
	return tea.Tick(d, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

func colorizeLine(s string) string {
	low := strings.ToLower(s)
	switch {
	case strings.Contains(low, "error:") || strings.Contains(low, "throw") ||
		strings.Contains(low, "failed") || strings.HasPrefix(low, "error"):
		return errStyle.Render(s)
	case strings.Contains(low, "warning") || strings.HasPrefix(low, "warn"):
		return warnStyle.Render(s)
	case strings.HasPrefix(low, "ok:") || strings.HasPrefix(low, "done"):
		return okStyle.Render(s)
	case strings.Contains(low, "download") || strings.Contains(low, "resolving") ||
		strings.Contains(low, "unpack"):
		return accentStyle.Render(s)
	default:
		return s
	}
}
