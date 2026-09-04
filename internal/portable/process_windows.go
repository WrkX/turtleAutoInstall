//go:build windows

package portable

import (
	"io"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func hiddenCommand(exe string, args ...string) *exec.Cmd {
	cmd := exec.Command(exe, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
	return cmd
}

func listenLoopback(port int) (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
}

func processImage(pid int) string {
	if pid <= 0 {
		return ""
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	n := uint32(windows.MAX_PATH)
	buf := make([]uint16, n)
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		n = 32768
		buf = make([]uint16, n)
		if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
			return ""
		}
	}
	return windows.UTF16ToString(buf)
}

func processesByPath(exe string) []int {
	want := resolveExe(exe)
	if want == "" {
		return nil
	}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snap, &entry); err != nil {
		return nil
	}
	var ids []int
	for {
		pid := int(entry.ProcessID)
		if resolveExe(processImage(pid)) == want {
			ids = append(ids, pid)
		}
		if err := windows.Process32Next(snap, &entry); err != nil {
			break
		}
	}
	return ids
}

func listeningPID(port int) int {
	out, err := exec.Command("netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return 0
	}
	re := regexp.MustCompile(`(?im)^\s*TCP\s+\S+:` + strconv.Itoa(port) + `\s+\S+\s+LISTENING\s+(\d+)\s*$`)
	m := re.FindStringSubmatch(string(out))
	if len(m) < 2 {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(m[1]))
	if err != nil {
		return 0
	}
	return n
}
