package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLooksLikeRoot(t *testing.T) {
	root := t.TempDir()
	if looksLikeRoot(root) {
		t.Fatal("empty dir should not look like a portable root")
	}
	if err := os.WriteFile(filepath.Join(root, "portable.env"), []byte("X=1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if looksLikeRoot(root) {
		t.Fatal("portable.env alone is not enough")
	}
	if err := os.Mkdir(filepath.Join(root, "tools"), 0755); err != nil {
		t.Fatal(err)
	}
	if !looksLikeRoot(root) {
		t.Fatal("expected portable.env + tools to look like a root")
	}
}

func TestIsGoBuildBinary(t *testing.T) {
	if !isGoBuildBinary(`C:\Users\me\AppData\Local\Temp\go-build123\b001\exe\main.exe`) {
		t.Fatal("go run binary should be treated as ephemeral")
	}
	if isGoBuildBinary(`C:\Games\TortoiseWow\tortoise.exe`) {
		t.Fatal("installed exe should not look like a go-build binary")
	}
}
