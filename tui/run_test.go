package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestChildEnvReplacesSecret(t *testing.T) {
	t.Setenv("TORTOISE_WOW_ACCOUNT_SHA_PASS_HASH", "old")
	env := childEnv(map[string]string{"TORTOISE_WOW_ACCOUNT_SHA_PASS_HASH": "new"})
	count := 0
	for _, item := range env {
		if strings.HasPrefix(item, "TORTOISE_WOW_ACCOUNT_SHA_PASS_HASH=") {
			count++
			if item != "TORTOISE_WOW_ACCOUNT_SHA_PASS_HASH=new" {
				t.Fatalf("unexpected secret entry %q", item)
			}
		}
	}
	if count != 1 {
		t.Fatalf("secret entry count = %d, want 1", count)
	}
}

func TestCleanupPartialArtifacts(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "tools", ".cache")
	credentialDir := filepath.Join(root, "data", "mysql-tmp")
	if err := os.MkdirAll(filepath.Join(cache, "maps-extract"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(credentialDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(cache, "asset.zip.partial"),
		filepath.Join(cache, "maps.zip.download"),
		filepath.Join(cache, "maps-url.txt.1234.tmp"),
		filepath.Join(cache, "unrelated.tmp"),
		filepath.Join(cache, "keep.zip"),
		filepath.Join(credentialDir, ".mysql-client-test.cnf"),
		filepath.Join(credentialDir, "keep.cnf"),
	} {
		if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	cleanupPartialArtifacts(root)

	for _, removed := range []string{
		filepath.Join(cache, "asset.zip.partial"),
		filepath.Join(cache, "maps.zip.download"),
		filepath.Join(cache, "maps-url.txt.1234.tmp"),
		filepath.Join(cache, "maps-extract"),
		filepath.Join(credentialDir, ".mysql-client-test.cnf"),
	} {
		if _, err := os.Stat(removed); !os.IsNotExist(err) {
			t.Fatalf("artifact still exists: %s", removed)
		}
	}
	for _, kept := range []string{
		filepath.Join(cache, "unrelated.tmp"),
		filepath.Join(cache, "keep.zip"),
		filepath.Join(credentialDir, "keep.cnf"),
	} {
		if _, err := os.Stat(kept); err != nil {
			t.Fatalf("expected file to remain: %s: %v", kept, err)
		}
	}
}

func TestPlanStepsStartsMySQLBeforeImport(t *testing.T) {
	a := menu[menuIndex("import")]
	m := model{status: Status{HasMariaDB: true, HasSQL: true, Mysqld: false}}
	steps := m.planSteps(a)
	if len(steps) != 2 {
		t.Fatalf("steps = %#v, want start-mysql and import", steps)
	}
	if steps[0].Script != "start-mysql.ps1" || steps[1].Script != "import-databases.ps1" {
		t.Fatalf("steps = %#v, want MySQL before import", steps)
	}
}

func TestLiveTUIGithubUpdate(t *testing.T) {
	if os.Getenv("LIVE_GITHUB_UPDATE") != "1" {
		t.Skip("set LIVE_GITHUB_UPDATE=1 to run update.ps1 through the TUI stream")
	}
	root, err := bootstrapRoot()
	if err != nil {
		t.Fatal(err)
	}

	msg := startStreamEnv(root, "update.ps1", nil, nil)()
	started, ok := msg.(streamStartedMsg)
	if !ok {
		if d, yes := msg.(doneMsg); yes {
			t.Fatalf("update.ps1 did not start: %v", d.err)
		}
		t.Fatalf("unexpected start message %T %#v", msg, msg)
	}

	var tm tea.Model = newModel(root)
	for line := range started.lines {
		t.Log(line)
		tm, _ = tm.Update(lineMsg(line))
	}
	waitErr := <-started.done
	tm, _ = tm.Update(doneMsg{err: waitErr})
	got := tm.(*model)
	if waitErr != nil {
		t.Fatalf("update.ps1 failed: %v\nlog:\n%s", waitErr, got.log.String())
	}
}

func TestRunLogSurvivesStreamAndDone(t *testing.T) {
	var tm tea.Model = newModel(t.TempDir())
	tm, _ = tm.Update(lineMsg("Resolving server zip from WrkX/tortoise-wow"))
	tm, _ = tm.Update(lineMsg("Downloading https://github.com/example/server.zip"))
	tm, _ = tm.Update(doneMsg{err: errors.New("exit status 1")})
	got := tm.(*model)
	log := got.log.String()
	if !strings.Contains(log, "error: exit status 1") {
		t.Fatalf("expected error line in log, got %q", log)
	}
}

func TestPlanStepsDoesNotAddStopForUpdate(t *testing.T) {
	a := menu[menuIndex("update")]
	m := model{status: Status{Mysqld: true, Realmd: true, Mangosd: true}}
	steps := m.planSteps(a)
	if len(steps) != 1 || steps[0].Script != "update.ps1" {
		t.Fatalf("steps = %#v, update should stop itself", steps)
	}
}

func TestMenuIDsAndShortcutsAreUnique(t *testing.T) {
	ids := map[string]bool{}
	keys := map[string]bool{}
	for _, a := range menu {
		if ids[a.ID] {
			t.Fatalf("duplicate menu ID %q", a.ID)
		}
		ids[a.ID] = true
		if a.Key != "" {
			if keys[a.Key] {
				t.Fatalf("duplicate menu shortcut %q", a.Key)
			}
			keys[a.Key] = true
		}
	}
}

func TestNumberKeyActivatesMenuItem(t *testing.T) {
	m := &model{status: Status{}}
	updated, _ := m.onHomeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	got := updated.(*model)
	if got.flash == "" {
		t.Fatal("number key selected the disabled item without activating it")
	}
}

func TestSelectedActionHelpUsesDisableReason(t *testing.T) {
	m := model{cursor: menuIndex("start"), status: Status{}}
	got := m.selectedActionHelp(40)
	if !strings.Contains(got, "no server binaries") {
		t.Fatalf("expected disable reason on the left, got %q", got)
	}
	m.status = Status{HasServer: true, HasConf: true, HasMapsAll: true}
	got = m.selectedActionHelp(40)
	if !strings.Contains(got, "Bring up MySQL, realmd, and mangosd") {
		t.Fatalf("expected action description on the left, got %q", got)
	}
	menuBody := m.renderMenu(40, 20)
	if strings.Contains(menuBody, "Bring up MySQL, realmd, and mangosd") {
		t.Fatalf("description should not sit under the menu item:\n%s", menuBody)
	}
}

func TestStartRequiresIdleDaemonsAndMaps(t *testing.T) {
	start := menu[menuIndex("start")]
	ready := Status{HasServer: true, HasConf: true, HasMapsAll: true}
	if reason := start.disabled(ready); reason != "" {
		t.Fatalf("ready realm disabled: %s", reason)
	}
	realmdRunning := ready
	realmdRunning.Realmd = true
	if reason := start.disabled(realmdRunning); reason == "" {
		t.Fatal("start should be disabled while realmd is running")
	}
	missingMaps := ready
	missingMaps.HasMapsAll = false
	if reason := start.disabled(missingMaps); reason == "" {
		t.Fatal("start should be disabled while maps are incomplete")
	}
}
