package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTailFileOmitsHead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.log")
	body := strings.Repeat("a\n", 20) + "TAIL\n"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	got := tailFile(path, 12)
	if !strings.Contains(got, "TAIL") {
		t.Fatalf("missing tail: %q", got)
	}
	if !strings.Contains(got, "earlier log omitted") {
		t.Fatalf("expected omission marker: %q", got)
	}
}

func TestRealmLogPathPrefersExisting(t *testing.T) {
	root := t.TempDir()
	logs := filepath.Join(root, "logs")
	if err := os.Mkdir(logs, 0755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(logs, "Realmd.log")
	if err := os.WriteFile(want, []byte("auth ok\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := realmLogPath(root, "realmd"); got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	if !realmLogExists(root, "realmd") || realmLogExists(root, "mangosd") {
		t.Fatal("expected only realmd log to exist")
	}
}

func TestConsolesDisabledWithoutLogs(t *testing.T) {
	a := menu[menuIndex("consoles")]
	if reason := a.disabled(Status{}); reason == "" {
		t.Fatal("consoles should be disabled before first start")
	}
	if reason := a.disabled(Status{Mangosd: true}); reason != "" {
		t.Fatalf("running mangosd should enable consoles: %s", reason)
	}
	if reason := a.disabled(Status{HasRealmLogs: true}); reason != "" {
		t.Fatalf("existing logs should enable consoles: %s", reason)
	}
}
