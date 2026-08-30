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
		mergeEnvFile(out, filepath.Join(root, name))
	}
	return out
}

// loadLocalEnv is used by the Settings form. Defaults from portable.env are
// deliberately not copied into editable fields: an empty field means that no
// local override will be written.
func loadLocalEnv(root string) map[string]string {
	out := map[string]string{}
	mergeEnvFile(out, filepath.Join(root, "portable.local.env"))
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
	if v := m[key]; v != "" {
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

func mapsZipURL(root string, env map[string]string) string {
	if v := envOr(env, "TORTOISE_WOW_MAPS_ZIP_URL", ""); v != "" {
		return v
	}
	if envOr(env, "DATANODES_FILE_CODE", "") != "" {
		return "datanodes"
	}
	if u := readFirstURL(filepath.Join(root, "tools", ".cache", "maps-url.txt")); u != "" {
		return u
	}
	return readFirstURL(filepath.Join(root, "conf", "maps-url.txt"))
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
			// Empty form fields are not overrides. Drop an existing managed key
			// when it is cleared instead of persisting an empty assignment.
			if strings.TrimSpace(v) == "" {
				continue
			}
			out = append(out, k+"="+strings.TrimSpace(v))
			seen[k] = true
			continue
		}
		out = append(out, strings.TrimRight(line, "\r"))
	}

	var missing []string
	for k := range updates {
		if !seen[k] && strings.TrimSpace(updates[k]) != "" {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		if len(out) > 0 && out[len(out)-1] != "" {
			out = append(out, "")
		}
		for _, k := range missing {
			out = append(out, k+"="+strings.TrimSpace(updates[k]))
		}
	}

	body := strings.Join(out, "\n") + "\n"
	return os.WriteFile(path, []byte(body), 0644)
}
