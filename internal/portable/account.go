package portable

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

func (j *Job) CreateAccount(fl flags) error {
	user := strings.ToUpper(strings.TrimSpace(fl.Username))
	if !regexp.MustCompile(`^[A-Za-z0-9]{2,16}$`).MatchString(user) {
		return fmt.Errorf("username must be 2-16 letters or digits")
	}
	if fl.GMLevel < 0 || fl.GMLevel > 3 {
		return fmt.Errorf("GM level must be 0-3")
	}
	hash := strings.ToLower(strings.TrimSpace(j.ExtraEnv["TORTOISE_WOW_ACCOUNT_SHA_PASS_HASH"]))
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(hash) {
		return fmt.Errorf("account hash missing; create the account from the TUI")
	}

	port, err := j.getInt("MYSQL_PORT", "3307")
	if err != nil {
		return err
	}
	rootUser := j.get("MYSQL_ROOT_USER", "root")
	rootPass := j.get("MYSQL_ROOT_PASSWORD", "")
	bin := findMariaDBBin(j.Root)
	if bin == "" {
		return fmt.Errorf("MariaDB not found. Run Full setup first")
	}
	client, err := mysqlClientPath(bin)
	if err != nil {
		return err
	}
	myIni := filepath.Join(j.Root, "conf", "my.ini")
	if err := j.waitMysqlReady(client, bin, myIni, rootUser, rootPass, port, 30); err != nil {
		return err
	}

	quiet := func(q string) (string, error) {
		out, err := j.invokeMysql(client, bin, myIni, rootUser, rootPass, port, "", q, "", false)
		return strings.TrimSpace(out), err
	}
	userSQL, err := sqlLiteral(user)
	if err != nil {
		return err
	}
	hashSQL, err := sqlLiteral(hash)
	if err != nil {
		return err
	}
	colsRaw, err := quiet("SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='tw_logon' AND TABLE_NAME='account'")
	if err != nil || colsRaw == "" {
		return fmt.Errorf("tw_logon.account not found - run Full setup first")
	}
	colSet := map[string]bool{}
	for _, line := range strings.Split(colsRaw, "\n") {
		n := strings.ToLower(strings.TrimSpace(line))
		if n != "" {
			colSet[n] = true
		}
	}
	if !colSet["username"] || !colSet["sha_pass_hash"] {
		return fmt.Errorf("tw_logon.account is missing username/sha_pass_hash")
	}
	existing, err := quiet("SELECT id FROM tw_logon.account WHERE username=" + userSQL + " LIMIT 1")
	if err != nil {
		return err
	}
	action := "created"
	id := existing
	if regexp.MustCompile(`^\d+$`).MatchString(existing) {
		action = "updated"
		set := "sha_pass_hash=" + hashSQL
		if colSet["v"] {
			set += ", v=''"
		}
		if colSet["s"] {
			set += ", s=''"
		}
		if colSet["sessionkey"] {
			set += ", sessionkey=''"
		}
		if colSet["gmlevel"] {
			set += fmt.Sprintf(", gmlevel=%d", fl.GMLevel)
		}
		if _, err := quiet("UPDATE tw_logon.account SET " + set + " WHERE id=" + existing); err != nil {
			return err
		}
	} else {
		cols := []string{"username", "sha_pass_hash"}
		vals := []string{userSQL, hashSQL}
		if colSet["gmlevel"] {
			cols = append(cols, "gmlevel")
			vals = append(vals, fmt.Sprintf("%d", fl.GMLevel))
		}
		if colSet["expansion"] {
			cols = append(cols, "expansion")
			vals = append(vals, "0")
		}
		if _, err := quiet(fmt.Sprintf("INSERT INTO tw_logon.account (%s) VALUES (%s)", strings.Join(cols, ", "), strings.Join(vals, ", "))); err != nil {
			return err
		}
		id, err = quiet("SELECT id FROM tw_logon.account WHERE username=" + userSQL + " LIMIT 1")
		if err != nil {
			return err
		}
	}
	hasAccess, err := quiet("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='tw_logon' AND TABLE_NAME='account_access'")
	if err == nil && hasAccess == "1" && regexp.MustCompile(`^\d+$`).MatchString(id) {
		_, _ = quiet("DELETE FROM tw_logon.account_access WHERE id=" + id)
		if fl.GMLevel > 0 {
			_, _ = quiet(fmt.Sprintf("INSERT INTO tw_logon.account_access (id, gmlevel, RealmID) VALUES (%s, %d, -1)", id, fl.GMLevel))
		}
	}
	j.logf("OK: account %s %s (id %s, gm %d)", user, action, id, fl.GMLevel)
	j.log("set realmlist " + j.get("REALM_ADDRESS", "127.0.0.1"))
	return nil
}
