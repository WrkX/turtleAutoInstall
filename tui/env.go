package main

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func loadEnv(root string) map[string]string {
	out := map[string]string{}
	for _, name := range []string{"portable.env", "portable.local.env"} {
		f, err := os.Open(filepath.Join(root, name))
		if err != nil {
			continue
		}
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
		_ = f.Close()
	}
	return out
}

func envOr(m map[string]string, key, def string) string {
	if v := m[key]; v != "" {
		return v
	}
	return def
}

func readTag(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func saveLocalEnv(root string, updates map[string]string) error {
	path := filepath.Join(root, "portable.local.env")
	data, err := os.ReadFile(path)
	var lines []string
	if err == nil && len(data) > 0 {
		lines = strings.Split(strings.TrimRight(string(data), "\r\n"), "\n")
	} else {
		lines = []string{"# Local overrides for tortoise-wow portable. Gitignored."}
	}

	seen := map[string]bool{}
	out := make([]string, 0, len(lines)+len(updates))
	for _, line := range lines {
		trim := strings.TrimSpace(strings.TrimPrefix(line, "\r"))
		if trim == "" || strings.HasPrefix(trim, "#") {
			out = append(out, strings.TrimRight(line, "\r"))
			continue
		}
		k, _, ok := strings.Cut(trim, "=")
		if !ok {
			out = append(out, strings.TrimRight(line, "\r"))
			continue
		}
		k = strings.TrimSpace(k)
		if v, yes := updates[k]; yes {
			out = append(out, k+"="+v)
			seen[k] = true
			continue
		}
		out = append(out, strings.TrimRight(line, "\r"))
	}

	var missing []string
	for k := range updates {
		if !seen[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		if len(out) > 0 && out[len(out)-1] != "" {
			out = append(out, "")
		}
		for _, k := range missing {
			out = append(out, k+"="+updates[k])
		}
	}

	body := strings.Join(out, "\n") + "\n"
	return os.WriteFile(path, []byte(body), 0644)
}
