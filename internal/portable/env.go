package portable

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func LoadEnv(root string) map[string]string {
	out := map[string]string{}
	for _, name := range []string{"portable.env", "portable.local.env"} {
		mergeEnvFile(out, filepath.Join(root, name))
	}
	return out
}

func mergeEnvFile(out map[string]string, path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
}

func envOr(m map[string]string, key, def string) string {
	if v := strings.TrimSpace(m[key]); v != "" {
		return v
	}
	return def
}

func readFirstURL(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			return line
		}
	}
	return ""
}
