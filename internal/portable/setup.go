package portable

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (j *Job) Update(fl flags) error {
	j.log("update (" + j.Root + ")")
	j.log("stopping realm if running")
	if err := j.StopRealm(); err != nil {
		return err
	}
	if err := j.FetchServer(true); err != nil {
		return err
	}
	if err := j.FetchSQL(true); err != nil {
		return err
	}
	if err := j.FetchMaps(false); err != nil {
		return err
	}
	j.log("rewriting confs (database kept unless -ForceReimport)")
	fl.SkipDownload = true
	if err := j.Setup(fl); err != nil {
		return err
	}
	j.log("update done. Start realm when you want it back.")
	return nil
}

func resolveDist(serverDir, confDir, name string) string {
	for _, c := range []string{
		filepath.Join(serverDir, name),
		filepath.Join(serverDir, name+".in"),
		filepath.Join(confDir, name),
	} {
		if fileExists(c) {
			return c
		}
	}
	return ""
}

func (j *Job) Setup(fl flags) error {
	j.log("setup (" + j.Root + ")")
	serverDir := filepath.Join(j.Root, "server")
	mapsDir := filepath.Join(j.Root, "maps")
	logsDir := filepath.Join(j.Root, "logs")
	dataMysql := filepath.Join(j.Root, "data", "mysql")
	confDir := filepath.Join(j.Root, "conf")
	marker := filepath.Join(j.Root, "data", ".setup-complete")
	incomplete := filepath.Join(j.Root, "data", ".setup-incomplete")

	if !fl.SkipDownload {
		if err := j.FetchServer(false); err != nil {
			return err
		}
		if err := j.FetchSQL(false); err != nil {
			return err
		}
		if err := j.FetchMariaDB(); err != nil {
			return err
		}
		if err := j.FetchMaps(false); err != nil {
			return err
		}
	} else if !fileExists(filepath.Join(serverDir, "mangosd.exe")) {
		j.warn("no server\\mangosd.exe - run without -SkipDownload or drop a build in server")
	}

	if err := assertMapsPresent(mapsDir); err != nil {
		return err
	}

	port, err := j.getInt("MYSQL_PORT", "3307")
	if err != nil {
		return err
	}
	rootUser := j.get("MYSQL_ROOT_USER", "root")
	rootPass := j.get("MYSQL_ROOT_PASSWORD", "")
	user := j.get("MYSQL_USER", "mangos")
	pass := j.get("MYSQL_PASSWORD", "mangos")
	minBots := j.get("MIN_RANDOM_BOTS", "20")
	maxBots := j.get("MAX_RANDOM_BOTS", "20")
	worldPort := j.get("WORLD_PORT", "8090")
	realmPort := j.get("REALM_PORT", "3724")

	bin := findMariaDBBin(j.Root)
	if bin == "" {
		return fmt.Errorf("MariaDB bin directory not found after fetch")
	}
	mysqld, err := mysqldPath(bin)
	if err != nil {
		return err
	}
	client, err := mysqlClientPath(bin)
	if err != nil {
		return err
	}
	basedir := filepath.Dir(bin)
	myIni := filepath.Join(confDir, "my.ini")
	tmpDir := filepath.Join(j.Root, "data", "mysql-tmp")
	for _, d := range []string{dataMysql, tmpDir, logsDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	if err := writeFromTemplate(filepath.Join(confDir, "my.ini.template"), myIni, map[string]string{
		"@MYSQL_PORT@": fmt.Sprintf("%d", port),
		"@BASEDIR@":    iniPath(basedir),
		"@DATADIR@":    iniPath(dataMysql),
		"@TMPDIR@":     iniPath(tmpDir),
	}); err != nil {
		return err
	}

	if !mysqlDatadirInitialized(dataMysql) {
		j.log("init datadir")
		if err := j.initMysqlDatadir(dataMysql, bin, mysqld, myIni, rootPass, port); err != nil {
			return err
		}
	} else {
		j.log("datadir already there, skipping init")
	}
	if err := j.assertMysqlPort(port, mysqld); err != nil {
		return err
	}

	alreadyReady := j.mysqlReady(client, bin, myIni, rootUser, rootPass, port)
	listener := listeningPID(port)
	alreadyRunning := listener > 0 && testPortablePID(listener, mysqld)
	startedHere := !alreadyReady && !alreadyRunning
	if startedHere || !alreadyReady {
		if err := j.StartMySQL(); err != nil {
			return err
		}
	}
	defer func() {
		if startedHere {
			j.log("stopping mysqld")
			_ = j.StopMySQL()
		}
	}()

	createSQL := filepath.Join(j.Root, "sql", "create_databases.sql")
	if !fileExists(createSQL) && !fl.SkipSqlSync {
		j.log("sql\\ still empty, syncing from source tree")
		if err := j.SyncSQL(""); err != nil {
			return err
		}
	}

	if fileExists(marker) && !fl.ForceReimport {
		j.log("already imported (" + marker + ") - pass -ForceReimport to wipe and reload")
	} else {
		recovering := fileExists(incomplete)
		if err := writeMarker(incomplete, time.Now().Format(time.RFC3339Nano)); err != nil {
			return err
		}
		_ = os.Remove(marker)
		if fl.ForceReimport || recovering {
			j.log("dropping tw_* databases")
			drop := "DROP DATABASE IF EXISTS tw_world; DROP DATABASE IF EXISTS tw_char; DROP DATABASE IF EXISTS tw_logon; DROP DATABASE IF EXISTS tw_logs;"
			if _, err := j.invokeMysql(client, bin, myIni, rootUser, rootPass, port, "", drop, "", false); err != nil {
				return err
			}
		}
		if err := j.ImportDatabases(flags{}); err != nil {
			_ = os.Remove(marker)
			return err
		}
		if err := writeMarker(marker, time.Now().Format(time.RFC3339Nano)); err != nil {
			return err
		}
		_ = os.Remove(incomplete)
	}

	pairs := []struct{ dist, out string }{
		{"mangosd.conf.dist", filepath.Join(serverDir, "mangosd.conf")},
		{"realmd.conf.dist", filepath.Join(serverDir, "realmd.conf")},
		{"aiplayerbot.conf.dist", filepath.Join(serverDir, "aiplayerbot.conf")},
		{"ahbot.conf.dist", filepath.Join(serverDir, "ahbot.conf")},
	}
	for _, p := range pairs {
		dist := resolveDist(serverDir, confDir, p.dist)
		if dist == "" {
			j.warn("no " + p.dist + ", skipping " + filepath.Base(p.out))
			continue
		}
		if err := copyFile(dist, p.out); err != nil {
			return err
		}
	}

	mangosdConf := filepath.Join(serverDir, "mangosd.conf")
	realmdConf := filepath.Join(serverDir, "realmd.conf")
	botConf := filepath.Join(serverDir, "aiplayerbot.conf")
	if fileExists(mangosdConf) {
		if err := patchConfDatabaseLines(mangosdConf, "127.0.0.1", port, user, pass); err != nil {
			return err
		}
		_ = setConfValue(mangosdConf, "DataDir", `"`+iniPath(mapsDir)+`"`)
		_ = setConfValue(mangosdConf, "LogsDir", `"`+iniPath(logsDir)+`"`)
		_ = setConfValue(mangosdConf, "WorldServerPort", worldPort)
		_ = setConfValue(mangosdConf, "Database.AutoUpdate.Enabled", "1")
		updatesPath := iniPath(filepath.Join(j.Root, "sql", "database_updates"))
		_ = setConfValue(mangosdConf, "Database.AutoUpdate.Path", `"`+updatesPath+`/"`)
		_ = setConfValue(mangosdConf, "LogSQL", "0")
		_ = setConfValue(mangosdConf, "LFT.BotFill.Enable", "1")
		_ = setConfValue(mangosdConf, "SoloDungeonRepopAlive.Enable", "1")
		_ = setConfValue(mangosdConf, "Leech.Enable", "1")
	}
	if fileExists(realmdConf) {
		if err := patchConfDatabaseLines(realmdConf, "127.0.0.1", port, user, pass); err != nil {
			return err
		}
		_ = setConfValue(realmdConf, "LogsDir", `"`+iniPath(logsDir)+`/"`)
		_ = setConfValue(realmdConf, "RealmServerPort", realmPort)
	}
	if fileExists(botConf) {
		_ = setConfValue(botConf, "AiPlayerbot.Enabled", "1")
		_ = setConfValue(botConf, "AiPlayerbot.RandomBotAutoCreate", "1")
		_ = setConfValue(botConf, "AiPlayerbot.DeleteRandomBotAccounts", "0")
		_ = setConfValue(botConf, "AiPlayerbot.MinRandomBots", minBots)
		_ = setConfValue(botConf, "AiPlayerbot.MaxRandomBots", maxBots)
	}

	j.log("syncing database credentials for " + user)
	if err := j.ensureDatabaseUser(client, bin, myIni, rootUser, rootPass, user, pass, port); err != nil {
		return err
	}
	if err := j.SyncRealmlist(); err != nil {
		return err
	}
	j.log("done. Start realm when the server and maps are ready")
	j.log("create an account from the TUI (Create account)")
	return nil
}
