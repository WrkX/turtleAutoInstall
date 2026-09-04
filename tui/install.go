package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	payload "github.com/WrkX/tortoise-wow-portable"
)

func resolveRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if looksLikeRoot(wd) {
		return wd, nil
	}
	if looksLikeRoot(filepath.Dir(wd)) {
		return filepath.Dir(wd), nil
	}

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find portable root (need portable.env next to tools\\)")
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		exe, _ = os.Executable()
	}
	dir := filepath.Dir(exe)
	if isGoBuildBinary(exe) {
		return "", fmt.Errorf("cannot find portable root (need portable.env next to tools\\)")
	}
	if looksLikeRoot(dir) {
		return dir, nil
	}
	if looksLikeRoot(filepath.Dir(dir)) {
		return filepath.Dir(dir), nil
	}
	return dir, nil
}

func looksLikeRoot(dir string) bool {
	if dir == "" || dir == "." || dir == string(filepath.Separator) {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "portable.env"))
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, "tools"))
	return err == nil
}

func isGoBuildBinary(exe string) bool {
	slash := strings.ReplaceAll(exe, "\\", "/")
	return strings.Contains(slash, "/go-build")
}

func bootstrapRoot() (string, error) {
	root, err := resolveRoot()
	if err != nil {
		return "", err
	}
	if err := payload.Ensure(root); err != nil {
		return "", fmt.Errorf("unpack launcher files: %w", err)
	}
	if !looksLikeRoot(root) {
		return "", fmt.Errorf("cannot find portable root (need portable.env next to tools\\)")
	}
	return root, nil
}
