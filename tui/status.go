package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type Status struct {
	Root          string
	SetupComplete bool
	HasMariaDB    bool
	HasServer     bool
	HasSQL        bool
	HasConf       bool
	HasMapsAll    bool
	DBC           bool
	Maps          bool
	VMaps         bool
	MMaps         bool
	Mysqld        bool
	Realmd        bool
	Mangosd       bool
	RealmPort     string
	WorldPort     string
	BotRange      string
	RealmName     string
	ReleasePin    string
	MariaVer      string
	ServerRelease string
	SQLRelease    string
	Latest        string
	LatestErr     string
	UpdateAvail   bool
	CheckedAt     time.Time
}

func dirHasEntries(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}

func processRunning(names ...string) bool {
	for _, name := range names {
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("tasklist", "/NH", "/FI", "IMAGENAME eq "+name+".exe")
		} else {
			cmd = exec.Command("pgrep", "-x", name)
		}
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		s := strings.TrimSpace(string(out))
		if s == "" {
			continue
		}
		if runtime.GOOS == "windows" && strings.Contains(strings.ToLower(s), "no tasks") {
			continue
		}
		return true
	}
	return false
}

func findMariaDB(root string) bool {
	candidates := []string{
		filepath.Join(root, "mariadb", "bin"),
	}
	entries, _ := os.ReadDir(filepath.Join(root, "mariadb"))
	for _, e := range entries {
		if e.IsDir() {
			candidates = append(candidates, filepath.Join(root, "mariadb", e.Name(), "bin"))
		}
	}
	for _, bin := range candidates {
		for _, exe := range []string{"mysqld.exe", "mariadbd.exe", "mysqld", "mariadbd"} {
			if _, err := os.Stat(filepath.Join(bin, exe)); err == nil {
				return true
			}
		}
	}
	return false
}

func gatherStatus(root string) Status {
	st := Status{Root: root, CheckedAt: time.Now()}
	env := loadEnv(root)

	st.RealmPort = envOr(env, "REALM_PORT", "3724")
	st.WorldPort = envOr(env, "WORLD_PORT", "8090")
	st.ReleasePin = envOr(env, "TORTOISE_WOW_RELEASE", "latest")
	st.MariaVer = envOr(env, "MARIADB_VERSION", "10.11")
	st.RealmName = envOr(env, "REALM_NAME", "TurtleWoW")
	minBots := envOr(env, "MIN_RANDOM_BOTS", "20")
	maxBots := envOr(env, "MAX_RANDOM_BOTS", "20")
	st.BotRange = minBots + "–" + maxBots

	st.SetupComplete = fileExists(filepath.Join(root, "data", ".setup-complete"))
	st.HasMariaDB = findMariaDB(root)
	st.ServerRelease = readTag(filepath.Join(root, "data", ".server-release"))
	st.SQLRelease = readTag(filepath.Join(root, "data", ".sql-release"))

	server := filepath.Join(root, "server")
	for _, exe := range []string{"mangosd.exe", "mangosd"} {
		if fileExists(filepath.Join(server, exe)) {
			st.HasServer = true
			break
		}
	}
	for _, conf := range []string{"mangosd.conf", "realmd.conf"} {
		if fileExists(filepath.Join(server, conf)) {
			st.HasConf = true
			break
		}
	}
	st.HasSQL = fileExists(filepath.Join(root, "sql", "create_databases.sql"))

	maps := filepath.Join(root, "maps")
	st.DBC = dirHasEntries(filepath.Join(maps, "dbc"))
	st.Maps = dirHasEntries(filepath.Join(maps, "maps"))
	st.VMaps = dirHasEntries(filepath.Join(maps, "vmaps"))
	st.MMaps = dirHasEntries(filepath.Join(maps, "mmaps"))
	st.HasMapsAll = st.DBC && st.Maps && st.VMaps && st.MMaps

	st.Mysqld = processRunning("mysqld", "mariadbd")
	st.Realmd = processRunning("realmd")
	st.Mangosd = processRunning("mangosd")
	return st
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (s *Status) applyLatest(tag, errStr string) {
	s.Latest = tag
	s.LatestErr = errStr
	s.recomputeUpdate()
}

func (s *Status) recomputeUpdate() {
	have := s.ServerRelease
	if have == "" {
		have = s.SQLRelease
	}
	pin := s.ReleasePin
	if pin == "" {
		pin = "latest"
	}
	if s.Latest == "" || have == "" {
		s.UpdateAvail = false
		return
	}
	if pin != "latest" {
		s.UpdateAvail = pin != have
		return
	}
	s.UpdateAvail = s.Latest != have
}

func mark(ok bool) string {
	if ok {
		return okStyle.Render("●")
	}
	return badStyle.Render("○")
}

func procRow(on bool, name, extra string) string {
	state := dimStyle.Render("stopped")
	if on {
		state = okStyle.Render("running")
	}
	row := mark(on) + " " + nameStyle.Render(name) + " " + state
	if extra != "" {
		row += "  " + dimStyle.Render(extra)
	}
	return row
}

func fileRow(ok bool, name, val string) string {
	return mark(ok) + " " + nameStyle.Render(name) + " " + val
}

func (s Status) mapsValue() string {
	type pair struct {
		ok   bool
		name string
	}
	parts := []pair{
		{s.DBC, "dbc"},
		{s.Maps, "maps"},
		{s.VMaps, "vmaps"},
		{s.MMaps, "mmaps"},
	}
	var have, miss []string
	for _, p := range parts {
		if p.ok {
			have = append(have, p.name)
		} else {
			miss = append(miss, p.name)
		}
	}
	if len(miss) == 0 {
		return okStyle.Render("dbc maps vmaps mmaps")
	}
	if len(have) == 0 {
		return badStyle.Render("empty — Turtle 1.18.1 into maps\\")
	}
	return dimStyle.Render(strings.Join(have, " ")) + "  " + badStyle.Render("need "+strings.Join(miss, " "))
}

func (s Status) versionLabel(local string, present bool) string {
	if local != "" {
		if s.Latest != "" && local != s.Latest {
			return warnStyle.Render(local) + dimStyle.Render("  latest "+s.Latest)
		}
		return okStyle.Render(local)
	}
	if present {
		return dimStyle.Render("present")
	}
	return badStyle.Render("missing")
}

func (s Status) processesCard(width int) string {
	body := strings.Join([]string{
		procRow(s.Mysqld, "MySQL", ""),
		procRow(s.Realmd, "realmd", ":"+s.RealmPort),
		procRow(s.Mangosd, "mangosd", ":"+s.WorldPort),
	}, "\n")
	fg := colBorder
	if s.Mysqld && s.Realmd && s.Mangosd {
		fg = colOk
	}
	return card("realm", body, width, fg)
}

func (s Status) filesCard(width int) string {
	setupVal := badStyle.Render("not yet")
	if s.SetupComplete {
		setupVal = okStyle.Render("complete")
	}
	mariadbVal := badStyle.Render("missing")
	if s.HasMariaDB {
		mariadbVal = okStyle.Render(s.MariaVer)
	}
	confVal := badStyle.Render("missing")
	if s.HasConf {
		confVal = dimStyle.Render("mangosd + realmd")
	}
	body := strings.Join([]string{
		fileRow(s.HasMariaDB, "MariaDB", mariadbVal),
		fileRow(s.HasServer, "Server", s.versionLabel(s.ServerRelease, s.HasServer)),
		fileRow(s.HasSQL, "SQL", s.versionLabel(s.SQLRelease, s.HasSQL)),
		fileRow(s.HasConf, "Configs", confVal),
		fileRow(s.HasMapsAll, "Maps", s.mapsValue()),
		fileRow(s.SetupComplete, "Setup", setupVal),
	}, "\n")
	return card("install", body, width, colBorder)
}

func (s Status) Hint() string {
	switch {
	case !s.HasMapsAll:
		return "Put Turtle WoW 1.18.1 (build 7272) into maps\\ — dbc, maps, vmaps, mmaps. The installer does not fetch client data."
	case !s.HasMariaDB || !s.HasServer || !s.HasSQL:
		return "Run Full setup to download MariaDB, server binaries, and SQL, then import the databases."
	case !s.SetupComplete:
		return "Downloads look present. Run Full setup to import databases and write confs."
	case s.UpdateAvail:
		have := s.ServerRelease
		if have == "" {
			have = s.SQLRelease
		}
		return fmt.Sprintf("GitHub has %s (you have %s). Update pulls new binaries and SQL; characters stay unless you reimport.", s.Latest, have)
	case !s.Mangosd:
		return fmt.Sprintf("Start the realm, then in the mangosd window: account create <name> <pass>. realmlist → %s  ·  bots %s", "127.0.0.1", s.BotRange)
	default:
		return fmt.Sprintf("%s is live. Auth %s · world %s · realmlist 127.0.0.1", s.RealmName, s.RealmPort, s.WorldPort)
	}
}

func (s Status) hintCard(width int) string {
	fg := colBorder
	title := "next"
	if s.UpdateAvail {
		fg = colWarn
		title = "update"
	} else if s.Mysqld && s.Realmd && s.Mangosd {
		fg = colOk
		title = "live"
	}
	body := lipgloss.NewStyle().Foreground(colText).Width(max(width-6, 12)).Render(s.Hint())
	return card(title, body, width, fg)
}

func (s Status) column(width int) string {
	return lipgloss.JoinVertical(lipgloss.Left,
		s.processesCard(width),
		s.filesCard(width),
		s.hintCard(width),
	)
}

func (s Status) versionBadge() string {
	if s.UpdateAvail && s.Latest != "" {
		have := s.ServerRelease
		if have == "" {
			have = "local"
		}
		return warnStyle.Render(have + " → " + s.Latest)
	}
	if s.ServerRelease != "" {
		return okStyle.Render(s.ServerRelease)
	}
	if s.Latest != "" {
		return dimStyle.Render("latest " + s.Latest)
	}
	if s.LatestErr != "" {
		return dimStyle.Render("github offline")
	}
	return dimStyle.Render(s.ReleasePin)
}

func (s Status) liveStrip() string {
	return strings.Join([]string{
		procRow(s.Mysqld, "MySQL", ""),
		procRow(s.Realmd, "realmd", ""),
		procRow(s.Mangosd, "mangosd", ""),
	}, "   ")
}

func renderHeader(s Status, width int) string {
	brand := brandStyle.Render("TORTOISE WOW") + tagStyle.Render("  portable")
	badge := s.versionBadge()
	topW := lipgloss.Width(brand) + lipgloss.Width(badge)
	var top string
	if width > topW+2 {
		gap := width - topW
		if gap < 1 {
			gap = 1
		}
		top = brand + strings.Repeat(" ", gap) + badge
		if lipgloss.Width(top) > width {
			top = brand
		}
	} else {
		top = brand
	}
	sub := subStyle.Render("installer  ·  updater  ·  realm control")
	if s.RealmName != "" && s.RealmName != "TurtleWoW" {
		sub = subStyle.Render(s.RealmName + "  ·  installer  ·  updater")
	}
	rule := headerRuleStyle.Render(strings.Repeat("─", max(width, 8)))
	return top + "\n" + sub + "\n" + rule
}
