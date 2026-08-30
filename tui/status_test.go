package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGatherStatusRequiresBothServerComponents(t *testing.T) {
	root := t.TempDir()
	server := filepath.Join(root, "server")
	if err := os.MkdirAll(server, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"mangosd.exe", "mangosd.conf"} {
		if err := os.WriteFile(filepath.Join(server, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	st := gatherStatus(root)
	if st.HasServer {
		t.Fatal("server should be incomplete without realmd.exe")
	}
	if !st.HasMangosdBin || st.HasRealmdBin {
		t.Fatalf("unexpected server component state: mangosd=%v realmd=%v", st.HasMangosdBin, st.HasRealmdBin)
	}
	if st.HasConf {
		t.Fatal("configs should be incomplete without realmd.conf")
	}
	if !st.HasMangosdConf || st.HasRealmdConf {
		t.Fatalf("unexpected config state: mangosd=%v realmd=%v", st.HasMangosdConf, st.HasRealmdConf)
	}
}

func TestFindMariaDBExecutablePrefersMariadbd(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "mariadb", "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"mysqld.exe", "mariadbd.exe"} {
		if err := os.WriteFile(filepath.Join(bin, name), nil, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if got := findMariaDBExecutable(root); got != filepath.Join(bin, "mariadbd.exe") {
		t.Fatalf("selected %q, want mariadbd.exe", got)
	}
}
