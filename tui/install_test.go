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
	if !looksLikeRoot(root) {
		t.Fatal("portable.env should be enough to treat a folder as a root")
	}
}

func TestIsGoBuildBinary(t *testing.T) {
	if !isGoBuildBinary(`C:\Users\me\AppData\Local\Temp\go-build123\b001\exe\main.exe`) {
		t.Fatal("windows go run binary should be treated as ephemeral")
	}
	if !isGoBuildBinary("/tmp/go-build123/b001/exe/main") {
		t.Fatal("unix go run binary should be treated as ephemeral")
	}
	if isGoBuildBinary(`C:\Games\TortoiseWow\tortoise.exe`) {
		t.Fatal("installed exe should not look like a go-build binary")
	}
	if isGoBuildBinary("/opt/tortoise/tortoise") {
		t.Fatal("unix install path should not look like a go-build binary")
	}
}
