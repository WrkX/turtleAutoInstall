package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type scriptStep struct {
	Label  string
	Script string
	Args   []string
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

func powershellBin() string {
	if runtime.GOOS == "windows" {
		return "powershell"
	}
	return "pwsh"
}

func runScriptCmd(root, script string, args []string) *exec.Cmd {
	path := filepath.Join(root, "tools", script)
	psArgs := append([]string{
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-File", path,
	}, args...)
	cmd := exec.Command(powershellBin(), psArgs...)
	cmd.Dir = root
	return cmd
}

func startStream(root, script string, args []string) tea.Cmd {
	return func() tea.Msg {
		cmd := runScriptCmd(root, script, args)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return doneMsg{err: err}
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return doneMsg{err: err}
		}
		if err := cmd.Start(); err != nil {
			return doneMsg{err: fmt.Errorf("%s: %w (need PowerShell to run tools\\*.ps1)", powershellBin(), err)}
		}

		lines := make(chan string, 64)
		done := make(chan error, 1)
		cancel := sync.OnceFunc(func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		})

		var wg sync.WaitGroup
		pump := func(r io.Reader) {
			defer wg.Done()
			sc := bufio.NewScanner(r)
			sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for sc.Scan() {
				lines <- sc.Text()
			}
		}
		wg.Add(2)
		go pump(stdout)
		go pump(stderr)
		go func() {
			wg.Wait()
			close(lines)
			done <- cmd.Wait()
			close(done)
		}()

		return streamStartedMsg{lines: lines, done: done, cancel: cancel}
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

func scheduleTick() tea.Cmd {
	return tea.Tick(2500*time.Millisecond, func(_ time.Time) tea.Msg {
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
