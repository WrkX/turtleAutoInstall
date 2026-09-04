package portable

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (j *Job) importSQLFile(client, bin, myIni, rootUser, rootPass, database, file string, port int, force bool) error {
	j.log("  " + filepath.Base(file))
	_, err := j.invokeMysql(client, bin, myIni, rootUser, rootPass, port, database, "", file, force)
	if err != nil {
		return fmt.Errorf("import failed: %s: %w", file, err)
	}
	return nil
}

func fileSHA1Hex(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum(b)
	return strings.ToUpper(hex.EncodeToString(sum[:])), nil
}

func (j *Job) markMigration(client, bin, myIni, rootUser, rootPass, database, file string, port int) {
	name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	hash, err := fileSHA1Hex(file)
	if err != nil {
		return
	}
	escName := strings.ReplaceAll(name, `'`, `''`)
	escHash := strings.ReplaceAll(hash, `'`, `''`)
	sql := fmt.Sprintf("INSERT INTO migrations (Name,Hash,AppliedAt) VALUES ('%s','%s',NOW()) ON DUPLICATE KEY UPDATE Hash='%s', AppliedAt=NOW();", escName, escHash, escHash)
	_, _ = j.invokeMysql(client, bin, myIni, rootUser, rootPass, port, database, sql, "", false)
}

func listSQL(dir string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.sql"))
	sort.Strings(matches)
	return matches
}

func (j *Job) ImportDatabases(fl flags) error {
	port, err := j.getInt("MYSQL_PORT", "3307")
	if err != nil {
		return err
	}
	rootUser := j.get("MYSQL_ROOT_USER", "root")
	rootPass := j.get("MYSQL_ROOT_PASSWORD", "")
	user := j.get("MYSQL_USER", "mangos")
	pass := j.get("MYSQL_PASSWORD", "mangos")
	bin := findMariaDBBin(j.Root)
	if bin == "" {
		return fmt.Errorf("MariaDB not found. Run Full setup first")
	}
	client, err := mysqlClientPath(bin)
	if err != nil {
		return err
	}
	myIni := filepath.Join(j.Root, "conf", "my.ini")
	sqlRoot := filepath.Join(j.Root, "sql")
	if err := j.waitMysqlReady(client, bin, myIni, rootUser, rootPass, port, 60); err != nil {
		return err
	}

	userSQL, err := sqlLiteral(user)
	if err != nil {
		return err
	}
	passSQL, err := sqlLiteral(pass)
	if err != nil {
		return err
	}
	j.logf("Creating user %s (no grants yet) ...", user)
	createUser := fmt.Sprintf("CREATE USER IF NOT EXISTS %s@'localhost' IDENTIFIED BY %s;\nCREATE USER IF NOT EXISTS %s@'127.0.0.1' IDENTIFIED BY %s;\nFLUSH PRIVILEGES;", userSQL, passSQL, userSQL, passSQL)
	if _, err := j.invokeMysql(client, bin, myIni, rootUser, rootPass, port, "", createUser, "", false); err != nil {
		return err
	}
	grant := fmt.Sprintf(`
GRANT ALL PRIVILEGES ON tw_char.* TO %s@'localhost';
GRANT ALL PRIVILEGES ON tw_logon.* TO %s@'localhost';
GRANT ALL PRIVILEGES ON tw_world.* TO %s@'localhost';
GRANT ALL PRIVILEGES ON tw_logs.* TO %s@'localhost';
GRANT ALL PRIVILEGES ON tw_char.* TO %s@'127.0.0.1';
GRANT ALL PRIVILEGES ON tw_logon.* TO %s@'127.0.0.1';
GRANT ALL PRIVILEGES ON tw_world.* TO %s@'127.0.0.1';
GRANT ALL PRIVILEGES ON tw_logs.* TO %s@'127.0.0.1';
FLUSH PRIVILEGES;
`, userSQL, userSQL, userSQL, userSQL, userSQL, userSQL, userSQL, userSQL)

	createSQL := filepath.Join(sqlRoot, "create_databases.sql")
	if !fileExists(createSQL) {
		return fmt.Errorf("missing %s - run Sync SQL from source first", createSQL)
	}
	j.log("Importing create_databases.sql ...")
	if err := j.importSQLFile(client, bin, myIni, rootUser, rootPass, "", createSQL, port, false); err != nil {
		return err
	}
	j.log("Granting on tw_* ...")
	if _, err := j.invokeMysql(client, bin, myIni, rootUser, rootPass, port, "", grant, "", false); err != nil {
		return err
	}

	if !fl.SkipBase {
		baseDir := filepath.Join(sqlRoot, "base")
		if !fileExists(baseDir) {
			return fmt.Errorf("missing %s", baseDir)
		}
		files := listSQL(baseDir)
		j.logf("Importing %d base world files into tw_world ...", len(files))
		for _, f := range files {
			if err := j.importSQLFile(client, bin, myIni, rootUser, rootPass, "tw_world", f, port, false); err != nil {
				return err
			}
		}
	}

	if !fl.SkipUpdates {
		updatesRoot := filepath.Join(sqlRoot, "database_updates")
		if fileExists(updatesRoot) {
			var files []string
			files = append(files, listSQL(updatesRoot)...)
			for _, sub := range []string{"world", "character", "auth"} {
				files = append(files, listSQL(filepath.Join(updatesRoot, sub))...)
			}
			sort.Strings(files)
			uniq := files[:0]
			seen := map[string]bool{}
			for _, f := range files {
				if !seen[f] {
					seen[f] = true
					uniq = append(uniq, f)
				}
			}
			files = uniq
			j.logf("Migrations (%d), --force, then mark applied with SHA1", len(files))
			for _, f := range files {
				db := "tw_world"
				dirName := filepath.Base(filepath.Dir(f))
				base := filepath.Base(f)
				switch {
				case dirName == "character":
					db = "tw_char"
				case dirName == "auth":
					db = "tw_logon"
				case strings.Contains(base, "_character"):
					db = "tw_char"
				case strings.Contains(base, "_auth"):
					db = "tw_logon"
				}
				if err := j.importSQLFile(client, bin, myIni, rootUser, rootPass, db, f, port, true); err != nil {
					return err
				}
				j.markMigration(client, bin, myIni, rootUser, rootPass, db, f, port)
			}
		} else {
			j.warn("No database_updates - skipped")
		}
	}

	if !fl.SkipPlayerbots {
		pb := filepath.Join(sqlRoot, "playerbots")
		if fileExists(pb) {
			j.log("Playerbots world SQL")
			for _, f := range listSQL(filepath.Join(pb, "world")) {
				if err := j.importSQLFile(client, bin, myIni, rootUser, rootPass, "tw_world", f, port, false); err != nil {
					return err
				}
			}
			classic := filepath.Join(pb, "world", "classic")
			if fileExists(classic) {
				for _, f := range listSQL(classic) {
					if err := j.importSQLFile(client, bin, myIni, rootUser, rootPass, "tw_world", f, port, false); err != nil {
						return err
					}
				}
			}
			j.log("Playerbots characters SQL")
			for _, f := range listSQL(filepath.Join(pb, "characters")) {
				if err := j.importSQLFile(client, bin, myIni, rootUser, rootPass, "tw_char", f, port, false); err != nil {
					return err
				}
			}
		} else {
			j.warn("No sql\\playerbots - mangosd will assert if bots are on")
		}
	}

	if err := j.SyncRealmlist(); err != nil {
		return err
	}
	j.log("Import done.")
	return nil
}

func (j *Job) SyncRealmlist() error {
	port, err := j.getInt("MYSQL_PORT", "3307")
	if err != nil {
		return err
	}
	rootUser := j.get("MYSQL_ROOT_USER", "root")
	rootPass := j.get("MYSQL_ROOT_PASSWORD", "")
	realmName := j.get("REALM_NAME", "TurtleWoW")
	realmAddress := j.get("REALM_ADDRESS", "127.0.0.1")
	worldPort, err := j.getInt("WORLD_PORT", "8090")
	if err != nil {
		return err
	}
	bin := findMariaDBBin(j.Root)
	if bin == "" {
		return fmt.Errorf("MariaDB not found. Run Full setup first")
	}
	client, err := mysqlClientPath(bin)
	if err != nil {
		return err
	}
	myIni := filepath.Join(j.Root, "conf", "my.ini")
	if !fileExists(myIni) {
		return fmt.Errorf("no %s - run Full setup first", myIni)
	}
	if err := j.waitMysqlReady(client, bin, myIni, rootUser, rootPass, port, 60); err != nil {
		return err
	}
	nameSQL, err := sqlLiteral(realmName)
	if err != nil {
		return err
	}
	addrSQL, err := sqlLiteral(realmAddress)
	if err != nil {
		return err
	}
	sql := fmt.Sprintf(`INSERT INTO tw_logon.realmlist (id, name, address, port, icon, realmflags, timezone, allowedSecurityLevel, population, realmbuilds)
VALUES (1, %s, %s, %d, 0, 0, 1, 0, 0, '7272')
ON DUPLICATE KEY UPDATE name=VALUES(name), address=VALUES(address), port=VALUES(port);`, nameSQL, addrSQL, worldPort)
	j.logf("syncing realmlist (%s -> %s:%d)", realmName, realmAddress, worldPort)
	_, err = j.invokeMysql(client, bin, myIni, rootUser, rootPass, port, "", sql, "", false)
	return err
}
