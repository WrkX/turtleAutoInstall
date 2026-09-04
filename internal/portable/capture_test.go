package portable

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDaemonCaptureRecordsStdout(t *testing.T) {
	cap := NewDaemonCapture()
	var (
		pid int
		err error
	)
	if runtime.GOOS == "windows" {
		pid, err = cap.Start("t", "cmd.exe", t.TempDir(), "/c", "echo hello-capture")
	} else {
		pid, err = cap.Start("t", "/bin/echo", t.TempDir(), "hello-capture")
	}
	if err != nil {
		t.Fatal(err)
	}
	if pid <= 0 {
		t.Fatalf("pid=%d", pid)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(cap.Output("t"), "hello-capture") {
			if !cap.Has("t") {
				t.Fatal("expected Has after output")
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("output=%q", cap.Output("t"))
}

func TestCaptureContextRoundTrip(t *testing.T) {
	if captureFrom(nil) != nil {
		t.Fatal("nil context should have no capture")
	}
	cap := NewDaemonCapture()
	ctx := WithCapture(nil, cap)
	if captureFrom(ctx) != cap {
		t.Fatal("capture did not round-trip")
	}
	if WithCapture(ctx, nil) != ctx {
		t.Fatal("nil capture should leave context unchanged")
	}
}
