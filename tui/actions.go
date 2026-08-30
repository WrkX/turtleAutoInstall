package main

type action struct {
	ID         string
	Group      string
	Title      string
	Desc       string
	Key        string
	Danger     bool
	Confirm    string
	Script     string
	Args       []string
	Builtin    string
	NeedsStop  bool
	NeedsMySQL bool
	Disable    func(Status) string
}

func (a action) disabled(s Status) string {
	if a.Disable == nil {
		return ""
	}
	return a.Disable(s)
}

var menu = []action{
	{
		ID:     "start",
		Group:  "Realm",
		Title:  "Start realm",
		Desc:   "Bring up MySQL, realmd, and mangosd",
		Script: "start.ps1",
		Disable: func(s Status) string {
			if s.Realmd || s.Mangosd {
				return "realm daemon is already running — use Restart realm"
			}
			if !s.HasServer {
				return "no server binaries — run Full setup"
			}
			if !s.HasConf {
				return "no confs — run Full setup"
			}
			if !s.HasMapsAll {
				return "maps are incomplete — run Fetch maps"
			}
			return ""
		},
	},
	{
		ID:     "stop",
		Group:  "Realm",
		Title:  "Stop realm",
		Desc:   "Kill mangosd, realmd, and MySQL",
		Script: "stop.ps1",
		Disable: func(s Status) string {
			if !s.Mysqld && !s.Realmd && !s.Mangosd {
				return "nothing is running"
			}
			return ""
		},
	},
	{
		ID:        "restart",
		Group:     "Realm",
		Title:     "Restart realm",
		Desc:      "Stop everything, then start MySQL, realmd, and mangosd",
		Script:    "start.ps1",
		NeedsStop: true,
		Confirm:   "Stop the realm, then start it again?",
		Disable: func(s Status) string {
			if !s.Mysqld && !s.Realmd && !s.Mangosd {
				return "nothing is running — use Start realm"
			}
			if !s.HasServer {
				return "no server binaries — run Full setup"
			}
			if !s.HasConf {
				return "no confs — run Full setup"
			}
			if !s.HasMapsAll {
				return "maps are incomplete — run Fetch maps"
			}
			return ""
		},
	},
	{
		ID:      "account",
		Group:   "Realm",
		Title:   "Create account",
		Desc:    "Add or reset a login in tw_logon (same hash as account create)",
		Key:     "a",
		Builtin: "account",
		Disable: func(s Status) string {
			if !s.HasMariaDB {
				return "MariaDB is missing — run Full setup"
			}
			if !s.SetupComplete {
				return "databases not imported yet — run Full setup"
			}
			return ""
		},
	},
	{
		ID:     "start-mysql",
		Group:  "Realm",
		Title:  "Start MySQL",
		Desc:   "Portable MariaDB only — no realm daemons",
		Script: "start-mysql.ps1",
		Disable: func(s Status) string {
			if s.Mysqld {
				return "MySQL is already up"
			}
			if !s.HasMariaDB {
				return "MariaDB is missing — run Full setup"
			}
			return ""
		},
	},
	{
		ID:     "stop-mysql",
		Group:  "Realm",
		Title:  "Stop MySQL",
		Desc:   "SHUTDOWN the portable mysqld",
		Script: "stop-mysql.ps1",
		Disable: func(s Status) string {
			if !s.Mysqld {
				return "MySQL is not running"
			}
			return ""
		},
	},
	{
		ID:        "setup",
		Group:     "Install",
		Title:     "Full setup",
		Desc:      "Download MariaDB, server, SQL, maps — import DB — write confs",
		Script:    "setup.ps1",
		NeedsStop: true,
		Confirm:   "Stops the realm if it is running, then downloads anything still missing (server, SQL, MariaDB, and maps if a zip URL is set), imports the databases if needed, and writes server confs.",
	},
	{
		ID:      "update",
		Group:   "Install",
		Title:   "Update from GitHub",
		Desc:    "Latest server + SQL release; keeps characters; rewrites confs",
		Key:     "u",
		Script:  "update.ps1",
		Confirm: "Stops the realm, downloads the latest tortoise-wow Windows server and SQL zips, and rewrites confs from the new dist files.\n\nDatabases are kept. Use Reimport if you want a clean SQL reload.",
	},
	{
		ID:     "fetch-maps",
		Group:  "Install",
		Title:  "Fetch maps",
		Desc:   "Download dbc/maps/vmaps/mmaps from conf/maps-url.txt (refreshed from GitHub)",
		Script: "fetch-maps.ps1",
		Disable: func(s Status) string {
			if !s.MapsUrlSet {
				return "no maps URL in conf/maps-url.txt"
			}
			return ""
		},
	},
	{
		ID:     "fetch-mariadb",
		Group:  "Install",
		Title:  "Fetch MariaDB",
		Desc:   "Download portable MariaDB 10.11 into this folder",
		Script: "fetch-mariadb.ps1",
		Disable: func(s Status) string {
			if s.HasMariaDB {
				return "MariaDB is already in place"
			}
			return ""
		},
	},
	{
		ID:        "reimport",
		Group:     "Database",
		Title:     "Reimport databases",
		Desc:      "DROP tw_* then reload SQL — destroys characters and accounts",
		Danger:    true,
		Script:    "setup.ps1",
		Args:      []string{"-ForceReimport", "-SkipDownload"},
		NeedsStop: true,
		Confirm:   "This DROPS tw_world, tw_char, tw_logon, and tw_logs, then reloads SQL.\n\nCharacters, accounts, and realm data are gone. The realm is stopped first.",
		Disable: func(s Status) string {
			if !s.HasMariaDB {
				return "MariaDB is missing — run Full setup"
			}
			if !s.HasSQL {
				return "no SQL files — run Full setup or Update"
			}
			return ""
		},
	},
	{
		ID:         "import",
		Group:      "Database",
		Title:      "Import SQL",
		Desc:       "Run import-databases.ps1 (MySQL must be up or will be started)",
		Script:     "import-databases.ps1",
		NeedsMySQL: true,
		Disable: func(s Status) string {
			if !s.HasMariaDB {
				return "MariaDB is missing — run Full setup"
			}
			if !s.HasSQL {
				return "no SQL files — run Full setup or Update"
			}
			return ""
		},
	},
	{
		ID:     "sync-sql",
		Group:  "Database",
		Title:  "Sync SQL from source",
		Desc:   "Copy sql\\ from TORTOISE_WOW_SRC (or a sibling checkout)",
		Script: "sync-sql.ps1",
	},
	{
		ID:      "settings",
		Group:   "Config",
		Title:   "Settings",
		Desc:    "Ports, bots, realm name, release pin — writes portable.local.env",
		Key:     "s",
		Builtin: "settings",
	},
	{
		ID:     "apply-config",
		Group:  "Config",
		Title:  "Apply config",
		Desc:   "Rewrite server confs from env — no download, no reimport",
		Script: "setup.ps1",
		Args:   []string{"-SkipDownload"},
		Disable: func(s Status) string {
			if !s.HasServer {
				return "no server binaries — run Full setup"
			}
			return ""
		},
	},
	{
		ID:      "quit",
		Group:   "Config",
		Title:   "Quit",
		Desc:    "Leave the installer (does not stop the realm)",
		Key:     "q",
		Builtin: "quit",
	},
}

func menuIndex(id string) int {
	for i, a := range menu {
		if a.ID == id {
			return i
		}
	}
	return -1
}

func menuIndexByKey(k string) int {
	for i, a := range menu {
		if a.Key == k {
			return i
		}
	}
	return -1
}
