package portable

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePSArgs(t *testing.T) {
	got, err := parsePSArgs([]string{"-SkipDownload", "-ForceReimport", "-Username", "bob", "-GMLevel", "3"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.SkipDownload || !got.ForceReimport || got.Username != "bob" || got.GMLevel != 3 {
		t.Fatalf("%+v", got)
	}
}

func TestSQLLiteral(t *testing.T) {
	got, err := sqlLiteral(`o'reilly\path`)
	if err != nil {
		t.Fatal(err)
	}
	if got != `'o''reilly\\path'` {
		t.Fatalf("got %s", got)
	}
}

func TestMapsPresent(t *testing.T) {
	dir := t.TempDir()
	if mapsPresent(dir) {
		t.Fatal("empty should be incomplete")
	}
	for _, name := range neededMaps {
		p := filepath.Join(dir, name)
		if err := os.Mkdir(p, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "x"), []byte("1"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if !mapsPresent(dir) {
		t.Fatal("expected complete maps")
	}
}

func TestPatchConfDatabaseLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mangosd.conf")
	src := "LoginDatabase.Info = \"old\"\nWorldDatabase.Info = \"old\"\n"
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	if err := patchConfDatabaseLines(path, "127.0.0.1", 3307, "mangos", "secret"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	text := string(got)
	if !strings.Contains(text, `127.0.0.1;3307;mangos;secret;tw_logon`) || !strings.Contains(text, `tw_world`) {
		t.Fatalf("patched conf: %s", got)
	}
}

func TestUnknownJob(t *testing.T) {
	err := Run(context.Background(), t.TempDir(), "nope.ps1", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGoogleDriveFileID(t *testing.T) {
	id := googleDriveFileID("https://drive.google.com/file/d/1-cG8r_Kypd3g7Hsf-GISLticA_y02B3E/view?usp=sharing")
	if id != "1-cG8r_Kypd3g7Hsf-GISLticA_y02B3E" {
		t.Fatalf("id=%s", id)
	}
}
