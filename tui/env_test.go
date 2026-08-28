package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLocalEnvMergesKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "portable.local.env")
	if err := os.WriteFile(path, []byte("# keep me\nREALM_PORT=3724\nWORLD_PORT=8090\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveLocalEnv(dir, map[string]string{
		"REALM_PORT":      "4000",
		"MIN_RANDOM_BOTS": "40",
	}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !strings.Contains(got, "# keep me") {
		t.Fatalf("lost comment:\n%s", got)
	}
	if !strings.Contains(got, "REALM_PORT=4000") {
		t.Fatalf("did not update REALM_PORT:\n%s", got)
	}
	if !strings.Contains(got, "WORLD_PORT=8090") {
		t.Fatalf("dropped WORLD_PORT:\n%s", got)
	}
	if !strings.Contains(got, "MIN_RANDOM_BOTS=40") {
		t.Fatalf("did not append MIN_RANDOM_BOTS:\n%s", got)
	}
}
