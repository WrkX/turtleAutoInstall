package portable

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func pidFile(root, name string) string {
	return filepath.Join(root, "data", name+".pid")
}

// ReadPID returns the recorded pid for name, or 0 if missing/invalid.
func ReadPID(root, name string) int {
	return readPID(root, name)
}

func readPID(root, name string) int {
	raw := strings.TrimSpace(readTrimmed(pidFile(root, name)))
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func writePID(root, name string, id int) error {
	return writeMarker(pidFile(root, name), strconv.Itoa(id))
}

func removePID(root, name string) {
	_ = os.Remove(pidFile(root, name))
}

func resolveExe(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return strings.ToLower(filepath.Clean(abs))
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return processImage(pid) != ""
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func killPID(pid int) {
	if pid <= 0 {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Kill()
}

func waitGone(pids []int, timeout time.Duration) []int {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var alive []int
		for _, id := range pids {
			if processAlive(id) {
				alive = append(alive, id)
			}
		}
		if len(alive) == 0 {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
		pids = alive
	}
	return pids
}

func (j *Job) startHidden(exe, workDir string, args ...string) (int, error) {
	cmd := hiddenCommand(exe, args...)
	cmd.Dir = workDir
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start %s: %w", exe, err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	return pid, nil
}

func PathIsRunning(exe string) bool {
	return exe != "" && len(processesByPath(exe)) > 0
}

func uniqueInts(ids []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func tcpPortInUse(port int) bool {
	ln, err := listenLoopback(port)
	if err != nil {
		return true
	}
	_ = ln.Close()
	return false
}

func testPortablePID(pid int, exe string) bool {
	if pid <= 0 {
		return false
	}
	want := resolveExe(exe)
	got := resolveExe(processImage(pid))
	return want != "" && got == want
}
