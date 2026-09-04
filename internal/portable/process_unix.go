//go:build !windows

package portable

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

func hiddenCommand(exe string, args ...string) *exec.Cmd {
	return exec.Command(exe, args...)
}

func listenLoopback(port int) (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
}

func processImage(pid int) string {
	if pid <= 0 {
		return ""
	}
	link, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	if err != nil {
		return ""
	}
	return link
}

func processesByPath(exe string) []int {
	want := resolveExe(exe)
	if want == "" {
		return nil
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var ids []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if resolveExe(processImage(pid)) == want {
			ids = append(ids, pid)
		}
	}
	return ids
}

func listeningPID(port int) int {
	_ = port
	return 0
}
