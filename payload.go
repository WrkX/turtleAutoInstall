package payload

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

//go:embed portable.env
//go:embed conf/my.ini.template
//go:embed conf/maps-url.txt
var FS embed.FS

// Ensure writes default config next to a standalone tortoise.exe.
// PowerShell tools are not unpacked; the TUI runs those jobs in Go.
func Ensure(root string) error {
	if root == "" {
		return nil
	}
	devTree := isGitCheckout(root)
	sum, err := fingerprint()
	if err != nil {
		return err
	}
	stampPath := filepath.Join(root, "data", ".launcher-bundle")
	if !devTree {
		if prev, err := os.ReadFile(stampPath); err == nil && strings.TrimSpace(string(prev)) == sum {
			return nil
		}
	}
	if err := fs.WalkDir(FS, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasPrefix(path.Base(name), ".") {
			return nil
		}
		if strings.HasPrefix(name, "tools/") {
			return nil
		}
		dest := filepath.Join(root, filepath.FromSlash(name))
		if _, err := os.Stat(dest); err == nil {
			if name == "portable.env" {
				return nil
			}
			if name == "conf/maps-url.txt" && firstURLInFile(dest) != "" {
				return nil
			}
			if name != "conf/maps-url.txt" && devTree {
				return nil
			}
		}
		body, err := FS.ReadFile(name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		return os.WriteFile(dest, body, 0644)
	}); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(stampPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(stampPath, []byte(sum+"\n"), 0644)
}

func isGitCheckout(root string) bool {
	_, err := os.Stat(filepath.Join(root, ".git"))
	return err == nil
}

func fingerprint() (string, error) {
	h := sha256.New()
	err := fs.WalkDir(FS, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		body, err := FS.ReadFile(name)
		if err != nil {
			return err
		}
		_, _ = h.Write([]byte(name))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(body)
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func firstURLInFile(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			return line
		}
	}
	return ""
}
