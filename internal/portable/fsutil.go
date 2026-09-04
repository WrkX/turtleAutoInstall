package portable

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func unzip(src, dest string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	root := filepath.Clean(dest) + string(os.PathSeparator)
	for _, f := range r.File {
		target := filepath.Join(dest, f.Name)
		clean := filepath.Clean(target)
		if clean != filepath.Clean(dest) && !strings.HasPrefix(clean, root) {
			return fmt.Errorf("zip path escapes dest: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(clean, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(clean), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(clean, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		closeErr := out.Close()
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

func dirHasFiles(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(entries) > 0
}
