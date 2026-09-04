package portable

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Job struct {
	Root     string
	Env      map[string]string
	ExtraEnv map[string]string
	Log      func(string)
	Ctx      context.Context
	client   *http.Client
}

func newJob(ctx context.Context, root string, extra map[string]string, log func(string)) *Job {
	if ctx == nil {
		ctx = context.Background()
	}
	if extra == nil {
		extra = map[string]string{}
	}
	return &Job{
		Root:     root,
		Env:      LoadEnv(root),
		ExtraEnv: extra,
		Log:      log,
		Ctx:      ctx,
	}
}

func (j *Job) log(s string) {
	if j != nil && j.Log != nil && s != "" {
		j.Log(s)
	}
}

func (j *Job) logf(format string, args ...any) {
	j.log(fmt.Sprintf(format, args...))
}

func (j *Job) warn(s string) {
	j.log("WARNING: " + s)
}

func (j *Job) errIfDone() error {
	if j == nil || j.Ctx == nil {
		return nil
	}
	select {
	case <-j.Ctx.Done():
		return j.Ctx.Err()
	default:
		return nil
	}
}

func (j *Job) get(name, def string) string {
	return envOr(j.Env, name, def)
}

func (j *Job) getInt(name, def string) (int, error) {
	raw := j.get(name, def)
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not an integer", name, raw)
	}
	return n, nil
}

func (j *Job) cacheDir() string {
	return filepath.Join(j.Root, "tools", ".cache")
}

func (j *Job) mkdirAll(path string) error {
	return os.MkdirAll(path, 0755)
}

func (j *Job) httpClient() *http.Client {
	if j.client == nil {
		jar, _ := cookiejar.New(nil)
		j.client = &http.Client{Jar: jar}
	}
	return j.client
}
