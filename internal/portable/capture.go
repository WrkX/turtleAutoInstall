package portable

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

const daemonCaptureBytes = 256 * 1024

type captureCtxKey struct{}

// DaemonCapture holds live stdout/stderr from daemons started by this process.
type DaemonCapture struct {
	mu    sync.Mutex
	seq   map[string]int
	items map[string]*capturedDaemon
}

type capturedDaemon struct {
	gen  int
	pid  int
	buf  []byte
	done bool
}

// NewDaemonCapture returns an empty capture store for a TUI session.
func NewDaemonCapture() *DaemonCapture {
	return &DaemonCapture{
		seq:   map[string]int{},
		items: map[string]*capturedDaemon{},
	}
}

// WithCapture attaches a capture store to a job context.
func WithCapture(ctx context.Context, cap *DaemonCapture) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if cap == nil {
		return ctx
	}
	return context.WithValue(ctx, captureCtxKey{}, cap)
}

func captureFrom(ctx context.Context) *DaemonCapture {
	if ctx == nil {
		return nil
	}
	cap, _ := ctx.Value(captureCtxKey{}).(*DaemonCapture)
	return cap
}

// Has reports whether this session started name and still has a buffer for it.
func (c *DaemonCapture) Has(name string) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.items[name]
	return ok
}

// Output returns captured stdout/stderr for name.
func (c *DaemonCapture) Output(name string) string {
	if c == nil {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	d := c.items[name]
	if d == nil {
		return ""
	}
	return string(d.buf)
}

// Start launches exe hidden, capturing combined stdout and stderr.
func (c *DaemonCapture) Start(name, exe, workDir string, args ...string) (int, error) {
	if c == nil {
		return 0, fmt.Errorf("daemon capture is nil")
	}
	cmd := hiddenCommand(exe, args...)
	cmd.Dir = workDir
	pr, pw, err := os.Pipe()
	if err != nil {
		return 0, err
	}
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		_ = pr.Close()
		_ = pw.Close()
		return 0, fmt.Errorf("failed to start %s: %w", exe, err)
	}
	_ = pw.Close()

	c.mu.Lock()
	c.seq[name]++
	gen := c.seq[name]
	c.items[name] = &capturedDaemon{gen: gen, pid: cmd.Process.Pid}
	c.mu.Unlock()

	go func() {
		defer pr.Close()
		c.ingest(name, gen, pr)
		_ = cmd.Wait()
		c.mu.Lock()
		if d := c.items[name]; d != nil && d.gen == gen {
			d.done = true
		}
		c.mu.Unlock()
	}()
	return cmd.Process.Pid, nil
}

func (c *DaemonCapture) ingest(name string, gen int, r io.Reader) {
	chunk := make([]byte, 8192)
	var acc []byte
	for {
		n, err := r.Read(chunk)
		if n > 0 {
			acc = append(acc, chunk[:n]...)
			acc = normalizeCapture(acc)
			if len(acc) > daemonCaptureBytes {
				acc = acc[len(acc)-daemonCaptureBytes:]
				if i := strings.IndexByte(string(acc), '\n'); i >= 0 && i+1 < len(acc) {
					acc = acc[i+1:]
				}
			}
			c.store(name, gen, acc)
		}
		if err != nil {
			return
		}
	}
}

func (c *DaemonCapture) store(name string, gen int, buf []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	d := c.items[name]
	if d == nil || d.gen != gen {
		return
	}
	d.buf = append(d.buf[:0], buf...)
}

func normalizeCapture(buf []byte) []byte {
	s := string(buf)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return []byte(s)
}

func (j *Job) startRealmDaemon(name, exe, workDir string) (int, error) {
	if cap := captureFrom(j.Ctx); cap != nil {
		return cap.Start(name, exe, workDir)
	}
	return j.startHidden(exe, workDir)
}
