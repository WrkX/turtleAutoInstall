package payload

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureBootstrapsStandaloneFolder(t *testing.T) {
	root := t.TempDir()
	if err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"portable.env",
		"conf/my.ini.template",
		"conf/maps-url.txt",
		"tools/_common.ps1",
		"tools/setup.ps1",
		"tools/update.ps1",
		"data/.launcher-bundle",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "tools", ".parse-check.ps1")); !os.IsNotExist(err) {
		t.Fatal("dot scripts should stay out of a standalone install")
	}
}

func TestEnsureDoesNotClobberCheckoutScripts(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	tools := filepath.Join(root, "tools")
	if err := os.Mkdir(tools, 0755); err != nil {
		t.Fatal(err)
	}
	marker := []byte("local-edit")
	if err := os.WriteFile(filepath.Join(tools, "_common.ps1"), marker, 0644); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(tools, "_common.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(marker) {
		t.Fatalf("checkout script was overwritten")
	}
}

func TestEnsureKeepsUserMapsURL(t *testing.T) {
	root := t.TempDir()
	conf := filepath.Join(root, "conf")
	if err := os.Mkdir(conf, 0755); err != nil {
		t.Fatal(err)
	}
	custom := []byte("https://example.com/maps.zip\n")
	if err := os.WriteFile(filepath.Join(conf, "maps-url.txt"), custom, 0644); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(conf, "maps-url.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(custom) {
		t.Fatalf("maps-url.txt was overwritten: %q", got)
	}
}

func TestEnsureReplacesPlaceholderMapsURL(t *testing.T) {
	root := t.TempDir()
	conf := filepath.Join(root, "conf")
	if err := os.Mkdir(conf, 0755); err != nil {
		t.Fatal(err)
	}
	placeholder := []byte("# no url yet\n")
	if err := os.WriteFile(filepath.Join(conf, "maps-url.txt"), placeholder, 0644); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(conf, "maps-url.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == string(placeholder) {
		t.Fatal("placeholder maps-url.txt should be replaced from the bundle")
	}
}

func TestEnsureRefreshesToolsOnNewBundle(t *testing.T) {
	root := t.TempDir()
	if err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	common := filepath.Join(root, "tools", "_common.ps1")
	if err := os.WriteFile(common, []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}
	stamp := filepath.Join(root, "data", ".launcher-bundle")
	if err := os.WriteFile(stamp, []byte("old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(common)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) == "stale" {
		t.Fatal("expected tools script to refresh from the embedded bundle")
	}
}
