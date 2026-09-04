package main

import (
	"strings"
	"testing"

	"github.com/WrkX/tortoise-wow-portable/internal/portable"
	tea "github.com/charmbracelet/bubbletea"
)

func TestLivePillsIncludePIDAndSkipMysqlClicks(t *testing.T) {
	s := Status{
		Mysqld: true, MysqldPID: 11,
		Realmd: true, RealmdPID: 22, RealmPort: "3724",
		Mangosd: true, MangosdPID: 33, WorldPort: "8090",
	}
	text, hits := s.livePillsLayout()
	if !strings.Contains(text, "22") || !strings.Contains(text, "33") || !strings.Contains(text, "11") {
		t.Fatalf("expected pids in pills: %q", text)
	}
	var mysql, realmd, mangosd pillHit
	for _, h := range hits {
		switch h.Name {
		case "mysql":
			mysql = h
		case "realmd":
			realmd = h
		case "mangosd":
			mangosd = h
		}
	}
	if mysql.Open {
		t.Fatal("mysql pill should not open an overlay")
	}
	if !realmd.Open || !mangosd.Open {
		t.Fatal("realmd and mangosd pills should be clickable")
	}
	if got := s.pillAt(mysql.Left, 1); got != "" {
		t.Fatalf("mysql click returned %q", got)
	}
	if got := s.pillAt(realmd.Left, 1); got != "realmd" {
		t.Fatalf("realmd click returned %q", got)
	}
	if got := s.pillAt(mangosd.Left, 1); got != "mangosd" {
		t.Fatalf("mangosd click returned %q", got)
	}
	if got := s.pillAt(realmd.Left, 0); got != "" {
		t.Fatalf("brand row should not be a pill hit, got %q", got)
	}
}

func TestMouseClickOpensMangosdOverlay(t *testing.T) {
	m := newModel(t.TempDir())
	m.status = Status{Mangosd: true, MangosdPID: 9, WorldPort: "8090", RealmPort: "3724"}
	_, hits := m.status.livePillsLayout()
	x := -1
	for _, h := range hits {
		if h.Name == "mangosd" {
			x = h.Left
			break
		}
	}
	if x < 0 {
		t.Fatal("missing mangosd pill")
	}
	got, _ := m.onMouse(tea.MouseMsg{
		X: x, Y: 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	if got.(*model).overlay != "mangosd" {
		t.Fatalf("overlay=%q", got.(*model).overlay)
	}
}

func TestF1OpensRealmdOverlay(t *testing.T) {
	m := newModel(t.TempDir())
	got, _ := m.onHomeKey(tea.KeyMsg{Type: tea.KeyF1})
	um := got.(*model)
	if um.overlay != "realmd" {
		t.Fatalf("overlay=%q", um.overlay)
	}
}

func TestOverlayUnavailableWithoutCapture(t *testing.T) {
	m := newModel(t.TempDir())
	m.status.Mangosd = true
	m.capture = portable.NewDaemonCapture()
	got := m.overlayContent("mangosd")
	if !strings.Contains(got, "stdout isn't available") {
		t.Fatalf("got %q", got)
	}
}

func TestStampOverlayKeepsSurroundingText(t *testing.T) {
	base := strings.Repeat("A", 20) + "\n" + strings.Repeat("B", 20) + "\n" + strings.Repeat("C", 20) + "\n" + strings.Repeat("D", 20)
	over := "XX"
	got := stampOverlay(base, over, 20, 12)
	if !strings.Contains(got, "XX") {
		t.Fatalf("missing overlay: %q", got)
	}
	if !strings.Contains(got, "AAAA") {
		t.Fatalf("header should remain: %q", got)
	}
}

func TestQuitActionStopsRealm(t *testing.T) {
	a := menu[menuIndex("quit")]
	if !strings.Contains(strings.ToLower(a.Desc), "stop") {
		t.Fatalf("quit should stop the realm: %q", a.Desc)
	}
}
