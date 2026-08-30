package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type Status struct {
	Root           string
	SetupComplete  bool
	HasMariaDB     bool
	HasServer      bool
	HasMangosdBin  bool
	HasRealmdBin   bool
	HasSQL         bool
	HasConf        bool
	HasMangosdConf bool
	HasRealmdConf  bool
	HasMapsAll     bool
	DBC            bool
	Maps           bool
	VMaps          bool
	MMaps          bool
	Mysqld         bool
	Realmd         bool
	Mangosd        bool
	RealmPort      string
	WorldPort      string
	RealmAddress   string
	BotRange       string
	RealmName      string
	ReleasePin     string
	MariaVer       string
	ServerRelease  string
	SQLRelease     string
	MapsUrlSet     bool
	Latest         string
	LatestErr      string
	UpdateAvail    bool
	CheckedAt      time.Time
}

func dirHasEntries(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}

func processRunning(name, executable string) bool {
	if executable == "" {
		return false
	}
	if runtime.GOOS == "windows" {
		// tasklist only knows the image name, which can belong to a different
		// portable install. Query the executable path so status pills are scoped
		// to this installation just like start.ps1/stop.ps1.
		script := "$expected=[IO.Path]::GetFullPath($env:TORTOISE_EXPECTED_EXE).TrimEnd('\\').ToLowerInvariant(); $name=$env:TORTOISE_PROCESS_NAME+'.exe'; $found=@(Get-CimInstance Win32_Process -Filter (\"Name='\"+$name+\"'\") -ErrorAction SilentlyContinue | Where-Object { $_.ExecutablePath -and [IO.Path]::GetFullPath([string]$_.ExecutablePath).TrimEnd('\\').ToLowerInvariant() -eq $expected }); if ($found.Count -gt 0) { exit 0 }; exit 1"
		cmd := exec.Command(powershellBin(), "-NoProfile", "-Command", script)
		cmd.Env = childEnv(map[string]string{
			"TORTOISE_EXPECTED_EXE": executable,
			"TORTOISE_PROCESS_NAME": name,
		})
		return cmd.Run() == nil
	}

	// On Linux, /proc gives us the same exact-path check without relying on a
	// process name that could be shared by another checkout.
	out, err := exec.Command("pgrep", "-x", name).Output()
	if err != nil {
		return false
	}
	expected, err := filepath.EvalSymlinks(executable)
	if err != nil {
		expected = filepath.Clean(executable)
	}
	for _, raw := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(raw)
		if err != nil || pid <= 0 {
			continue
		}
		actual := ""
		if runtime.GOOS == "linux" {
			actual, err = os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
		} else {
			var psOut []byte
			psOut, err = exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
			actual = strings.TrimSpace(string(psOut))
		}
		if err != nil {
			continue
		}
		if resolved, err := filepath.EvalSymlinks(actual); err == nil {
			actual = resolved
		}
		if filepath.Clean(actual) == filepath.Clean(expected) {
			return true
		}
	}
	return false
}

func findMariaDBExecutable(root string) string {
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
		// Match tools/Get-MysqldPath: MariaDB distributions can ship both names,
		// but start-mysql.ps1 prefers mariadbd.
		for _, exe := range []string{"mariadbd.exe", "mysqld.exe", "mariadbd", "mysqld"} {
			path := filepath.Join(bin, exe)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return ""
}

func findMariaDB(root string) bool {
	return findMariaDBExecutable(root) != ""
}

func gatherStatus(root string) Status {
	st := Status{Root: root, CheckedAt: time.Now()}
	env := loadEnv(root)

	st.RealmPort = envOr(env, "REALM_PORT", "3724")
	st.WorldPort = envOr(env, "WORLD_PORT", "8090")
	st.RealmAddress = envOr(env, "REALM_ADDRESS", "127.0.0.1")
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
	st.HasMangosdBin = fileExists(filepath.Join(server, "mangosd.exe")) || fileExists(filepath.Join(server, "mangosd"))
	st.HasRealmdBin = fileExists(filepath.Join(server, "realmd.exe")) || fileExists(filepath.Join(server, "realmd"))
	st.HasServer = st.HasMangosdBin && st.HasRealmdBin
	st.HasMangosdConf = fileExists(filepath.Join(server, "mangosd.conf"))
	st.HasRealmdConf = fileExists(filepath.Join(server, "realmd.conf"))
	st.HasConf = st.HasMangosdConf && st.HasRealmdConf
	st.HasSQL = fileExists(filepath.Join(root, "sql", "create_databases.sql"))

	maps := filepath.Join(root, "maps")
	st.DBC = dirHasEntries(filepath.Join(maps, "dbc"))
	st.Maps = dirHasEntries(filepath.Join(maps, "maps"))
	st.VMaps = dirHasEntries(filepath.Join(maps, "vmaps"))
	st.MMaps = dirHasEntries(filepath.Join(maps, "mmaps"))
	st.HasMapsAll = st.DBC && st.Maps && st.VMaps && st.MMaps
	st.MapsUrlSet = mapsZipURL(root, env) != ""

	mysqlExe := findMariaDBExecutable(root)
	mysqlName := strings.TrimSuffix(filepath.Base(mysqlExe), filepath.Ext(mysqlExe))
	st.Mysqld = processRunning(mysqlName, mysqlExe)
	st.Realmd = processRunning("realmd", firstExisting(filepath.Join(server, "realmd.exe"), filepath.Join(server, "realmd")))
	st.Mangosd = processRunning("mangosd", firstExisting(filepath.Join(server, "mangosd.exe"), filepath.Join(server, "mangosd")))
	return st
}

func firstExisting(paths ...string) string {
	for _, path := range paths {
		if fileExists(path) {
			return path
		}
	}
	return ""
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
	return faintStyle.Render("○")
}

func fileRow(ok bool, name, val string) string {
	return mark(ok) + " " + nameStyle.Render(name) + " " + val
}

func pill(on bool, name, extra string) string {
	dot, label := faintStyle.Render("○"), faintStyle.Render(name)
	if on {
		dot = okStyle.Render("●")
		label = textStyle.Render(name)
	}
	out := dot + " " + label
	if extra != "" {
		out += dimStyle.Render(extra)
	}
	return out
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
		return badStyle.Render("empty — Fetch maps")
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

func (s Status) filesCard(width int) string {
	setupVal := badStyle.Render("not yet")
	if s.SetupComplete {
		setupVal = okStyle.Render("complete")
	}
	mariadbVal := badStyle.Render("missing")
	if s.HasMariaDB {
		mariadbVal = okStyle.Render(s.MariaVer)
	}
	confVal := badStyle.Render("missing mangosd.conf + realmd.conf")
	if s.HasConf {
		confVal = dimStyle.Render("mangosd + realmd")
	} else if s.HasMangosdConf || s.HasRealmdConf {
		missing := make([]string, 0, 2)
		if !s.HasMangosdConf {
			missing = append(missing, "mangosd.conf")
		}
		if !s.HasRealmdConf {
			missing = append(missing, "realmd.conf")
		}
		confVal = badStyle.Render("missing " + strings.Join(missing, " + "))
	}
	serverVal := s.versionLabel(s.ServerRelease, s.HasServer)
	if !s.HasServer && (s.HasMangosdBin || s.HasRealmdBin) {
		missing := make([]string, 0, 2)
		if !s.HasMangosdBin {
			missing = append(missing, "mangosd.exe")
		}
		if !s.HasRealmdBin {
			missing = append(missing, "realmd.exe")
		}
		serverVal = badStyle.Render("missing " + strings.Join(missing, " + "))
	}
	body := strings.Join([]string{
		fileRow(s.HasMariaDB, "MariaDB", mariadbVal),
		fileRow(s.HasServer, "Server", serverVal),
		fileRow(s.HasSQL, "SQL", s.versionLabel(s.SQLRelease, s.HasSQL)),
		fileRow(s.HasConf, "Configs", confVal),
		fileRow(s.HasMapsAll, "Maps", s.mapsValue()),
		fileRow(s.SetupComplete, "Setup", setupVal),
	}, "\n")
	fg := colBorder
	if s.SetupComplete && s.HasMariaDB && s.HasServer && s.HasSQL && s.HasConf && s.HasMapsAll {
		fg = colOk
	}
	return card("install", body, width, fg)
}

func (s Status) Hint() string {
	addr := s.realmAddress()
	switch {
	case !s.HasMapsAll && s.MapsUrlSet:
		return "Maps URL is set. Run Full setup or Fetch maps."
	case !s.HasMapsAll:
		return "No maps URL yet. Put a public Google Drive share link in conf/maps-url.txt, then Fetch maps."
	case !s.HasMariaDB || !s.HasServer || !s.HasSQL:
		return "Run Full setup to download MariaDB, server binaries, and SQL, then import the databases."
	case !s.SetupComplete:
		return "Downloads look present. Run Full setup to import databases and write confs."
	case s.UpdateAvail:
		have := s.ServerRelease
		if have == "" {
			have = s.SQLRelease
		}
		return fmt.Sprintf("GitHub has %s (you have %s). Update keeps characters; Reimport wipes them.", s.Latest, have)
	case !s.Mangosd:
		return fmt.Sprintf("Start the realm, then Create account (a). realmlist %s  ·  bots %s", addr, s.BotRange)
	default:
		return fmt.Sprintf("%s is live · set realmlist %s · press c to copy", s.RealmName, addr)
	}
}

func (s Status) realmAddress() string {
	if s.RealmAddress != "" {
		return s.RealmAddress
	}
	return "127.0.0.1"
}

func (s Status) hintBanner(width int) string {
	mark := accentStyle.Render("›")
	if s.UpdateAvail {
		mark = warnStyle.Render("›")
	} else if s.Mysqld && s.Realmd && s.Mangosd {
		mark = okStyle.Render("›")
	}
	body := lipgloss.NewStyle().Foreground(colText).Width(max(width-3, 12)).Render(s.Hint())
	return mark + " " + body
}

func (s Status) column(width int) string {
	return s.filesCard(width)
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

func (s Status) livePills() string {
	return strings.Join([]string{
		pill(s.Mysqld, "mysql", ""),
		pill(s.Realmd, "realmd", " :"+s.RealmPort),
		pill(s.Mangosd, "mangosd", " :"+s.WorldPort),
	}, "   ")
}

func renderHeader(s Status, width int) string {
	brand := brandStyle.Render("TORTOISE") + " " + brandStyle.Foreground(colCream).Render("WOW") + tagStyle.Render("  portable")
	badge := s.versionBadge()
	top := spread(brand, badge, width)

	meta := dimStyle.Render("bots " + s.BotRange)
	if s.RealmName != "" && s.RealmName != "TurtleWoW" {
		meta = dimStyle.Render(s.RealmName + "  ·  bots " + s.BotRange)
	}
	pills := s.livePills()
	sub := pills
	if width > lipgloss.Width(pills)+lipgloss.Width(meta)+2 {
		sub = spread(pills, meta, width)
	}

	rule := headerRuleStyle.Render(strings.Repeat("─", max(width, 8)))
	return top + "\n" + sub + "\n" + rule
}
