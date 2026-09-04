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

func TestReadFirstURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "maps-url.txt")
	body := "# comment\n\nhttps://example.com/maps.zip\nhttps://ignored.example/b.zip\n"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	got := readFirstURL(path)
	if got != "https://example.com/maps.zip" {
		t.Fatalf("got %q", got)
	}
}

func TestSaveLocalEnvSkipsEmptyOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "portable.local.env")
	body := "# keep me\nREALM_PORT=4000\nUNRELATED=value\n"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveLocalEnv(dir, map[string]string{
		"REALM_PORT": "",
		"WORLD_PORT": "",
	}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if strings.Contains(text, "REALM_PORT=") || strings.Contains(text, "WORLD_PORT=") {
		t.Fatalf("empty overrides persisted:\n%s", text)
	}
	if !strings.Contains(text, "UNRELATED=value") {
		t.Fatalf("unrelated local setting was lost:\n%s", text)
	}
}

func TestMapsZipURLPriority(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tools", ".cache"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "conf"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tools", ".cache", "maps-url.txt"), []byte("https://cache.example/maps.zip\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "conf", "maps-url.txt"), []byte("https://conf.example/maps.zip\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if got := mapsZipURL(root, map[string]string{}); got != "https://conf.example/maps.zip" {
		t.Fatalf("bundled conf URL = %q, want conf URL", got)
	}
	if got := mapsZipURL(root, map[string]string{"TORTOISE_WOW_MAPS_ZIP_URL": "https://override.example/maps.zip"}); got != "https://override.example/maps.zip" {
		t.Fatalf("override URL = %q, want override URL", got)
	}
	if got := mapsZipURL(root, map[string]string{"DATANODES_FILE_CODE": "abc123"}); got != "datanodes" {
		t.Fatalf("DataNodes URL = %q, want datanodes marker", got)
	}

	if err := os.Remove(filepath.Join(root, "tools", ".cache", "maps-url.txt")); err != nil {
		t.Fatal(err)
	}
	if got := mapsZipURL(root, map[string]string{}); got != "https://conf.example/maps.zip" {
		t.Fatalf("committed URL = %q, want committed URL", got)
	}
}
