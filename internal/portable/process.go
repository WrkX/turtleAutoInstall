package portable

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// mangosd/realmd read the console from stdin. A NUL/closed stdin is EOF, which
// they treat as "server shutdown" a few seconds after boot. Keep a write-end
// open for the life of each hidden daemon.
var heldStdin sync.Map

func holdStdin(pid int, w *os.File) {
	if pid <= 0 || w == nil {
		return
	}
	if old, ok := heldStdin.LoadAndDelete(pid); ok {
		_ = old.(*os.File).Close()
	}
	heldStdin.Store(pid, w)
}

func releaseHeldStdin(pid int) {
	if pid <= 0 {
		return
	}
	if w, ok := heldStdin.LoadAndDelete(pid); ok {
		_ = w.(*os.File).Close()
	}
}

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
	releaseHeldStdin(pid)
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
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		return 0, err
	}
	cmd.Stdin = stdinR
	if err := cmd.Start(); err != nil {
		_ = stdinR.Close()
		_ = stdinW.Close()
		return 0, fmt.Errorf("failed to start %s: %w", exe, err)
	}
	_ = stdinR.Close()
	holdStdin(cmd.Process.Pid, stdinW)
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
