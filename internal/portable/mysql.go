package portable

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func testMysqldBinDir(dir string) bool {
	return fileExists(filepath.Join(dir, "mysqld.exe")) || fileExists(filepath.Join(dir, "mariadbd.exe"))
}

func findMariaDBBin(root string) string {
	mariadbRoot := filepath.Join(root, "mariadb")
	if !fileExists(mariadbRoot) {
		return ""
	}
	direct := filepath.Join(mariadbRoot, "bin")
	if testMysqldBinDir(direct) {
		return direct
	}
	entries, err := os.ReadDir(mariadbRoot)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		bin := filepath.Join(mariadbRoot, e.Name(), "bin")
		if testMysqldBinDir(bin) {
			return bin
		}
	}
	return ""
}

func firstExisting(dir string, names ...string) string {
	for _, name := range names {
		p := filepath.Join(dir, name)
		if fileExists(p) {
			return p
		}
	}
	return ""
}

func mysqldPath(binDir string) (string, error) {
	p := firstExisting(binDir, "mariadbd.exe", "mysqld.exe")
	if p == "" {
		return "", fmt.Errorf("no mariadbd.exe/mysqld.exe under %s", binDir)
	}
	return p, nil
}

func mysqlClientPath(binDir string) (string, error) {
	p := firstExisting(binDir, "mariadb.exe", "mysql.exe")
	if p == "" {
		return "", fmt.Errorf("no mariadb.exe/mysql.exe under %s", binDir)
	}
	return p, nil
}

func installDBPath(binDir string) string {
	return firstExisting(binDir, "mariadb-install-db.exe", "mysql_install_db.exe")
}

func mysqlPluginDir(binDir string) string {
	p := filepath.Join(filepath.Dir(binDir), "lib", "plugin")
	if fileExists(p) {
		return p
	}
	return ""
}

func mysqlDatadirInitialized(dataDir string) bool {
	return fileExists(filepath.Join(dataDir, "mysql"))
}

func (j *Job) newMysqlDefaultsFile(user, password string, port int) (string, error) {
	dir := filepath.Join(j.Root, "data", "mysql-tmp")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	var id [8]byte
	_, _ = rand.Read(id[:])
	path := filepath.Join(dir, ".mysql-client-"+hex.EncodeToString(id[:])+".cnf")
	text := fmt.Sprintf("[client]\r\nuser=\"%s\"\r\npassword=\"%s\"\r\nhost=\"127.0.0.1\"\r\nport=%d\r\n",
		quoteCnf(user), quoteCnf(password), port)
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		return "", err
	}
	return path, nil
}

func removeMysqlDefaultsFile(args []string) {
	const prefix = "--defaults-extra-file="
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			_ = os.Remove(strings.TrimPrefix(a, prefix))
		}
	}
}

func (j *Job) mysqlClientArgs(binDir, defaultsFile, user, password string, port int, extra []string) ([]string, error) {
	var args []string
	if password != "" {
		cred, err := j.newMysqlDefaultsFile(user, password, port)
		if err != nil {
			return nil, err
		}
		args = append(args, "--defaults-extra-file="+cred)
	} else if defaultsFile != "" && fileExists(defaultsFile) {
		args = append(args, "--defaults-file="+defaultsFile)
	}
	if binDir != "" {
		if plugin := mysqlPluginDir(binDir); plugin != "" {
			args = append(args, "--plugin-dir="+plugin)
		}
	}
	args = append(args, "-u"+user, "-h127.0.0.1", fmt.Sprintf("-P%d", port))
	args = append(args, extra...)
	return args, nil
}

func (j *Job) invokeMysql(client, binDir, defaultsFile, user, password string, port int, database, execute, inputFile string, force bool) (string, error) {
	args, err := j.mysqlClientArgs(binDir, defaultsFile, user, password, port, nil)
	if err != nil {
		return "", err
	}
	defer removeMysqlDefaultsFile(args)
	if force {
		args = append(args, "--force")
	}
	if database != "" {
		args = append(args, database)
	}
	cmd := hiddenCommand(client, args...)
	cmd.Dir = j.Root
	switch {
	case execute != "":
		cmd.Stdin = strings.NewReader(execute)
	case inputFile != "":
		f, err := os.Open(inputFile)
		if err != nil {
			return "", err
		}
		defer f.Close()
		cmd.Stdin = f
	default:
		return "", fmt.Errorf("mysql requires SQL or an input file")
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	out := stdout.String()
	if runErr != nil && !force {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return out, fmt.Errorf("mysql failed: %s", msg)
		}
		return out, fmt.Errorf("mysql failed: %w", runErr)
	}
	return out, nil
}

func (j *Job) mysqlReady(client, binDir, defaultsFile, user, password string, port int) bool {
	_, err := j.invokeMysql(client, binDir, defaultsFile, user, password, port, "", "SELECT 1;", "", false)
	return err == nil
}

func (j *Job) waitMysqlReady(client, binDir, defaultsFile, user, password string, port, timeoutSec int) error {
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	for time.Now().Before(deadline) {
		if err := j.errIfDone(); err != nil {
			return err
		}
		if j.mysqlReady(client, binDir, defaultsFile, user, password, port) {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("MySQL did not become ready on port %d within %ds", port, timeoutSec)
}

func (j *Job) assertMysqlPort(port int, mysqld string) error {
	if !tcpPortInUse(port) {
		return nil
	}
	if mysqld != "" {
		pid := listeningPID(port)
		if pid > 0 && testPortablePID(pid, mysqld) {
			return nil
		}
	}
	hint := fmt.Sprintf("Port %d is already used by another MySQL/MariaDB server.", port)
	if port == 3306 {
		hint += " A local MySQL 8 install often owns 3306 (and 33060)."
	}
	hint += " Stop that service or set MYSQL_PORT=3307 in portable.local.env, then run setup again."
	return fmt.Errorf("%s", hint)
}

func (j *Job) clearMysqlDatadir(dataDir string) error {
	if mysqlDatadirInitialized(dataDir) {
		return fmt.Errorf("refusing to wipe initialized datadir: %s", dataDir)
	}
	if fileExists(dataDir) {
		entries, err := os.ReadDir(dataDir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := os.RemoveAll(filepath.Join(dataDir, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	return os.MkdirAll(dataDir, 0755)
}

func runLogged(cmd *exec.Cmd) error {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

func (j *Job) initMysqlDatadir(dataDir, binDir, mysqld, defaultsFile, rootPass string, port int) error {
	if err := j.clearMysqlDatadir(dataDir); err != nil {
		return err
	}
	initialized := false
	if install := installDBPath(binDir); install != "" {
		cmd := hiddenCommand(install, "--datadir="+dataDir, fmt.Sprintf("--port=%d", port))
		if err := runLogged(cmd); err == nil {
			initialized = true
		} else {
			j.warn(fmt.Sprintf("mariadb-install-db failed (%v), falling back to --initialize-insecure", err))
			if err := j.clearMysqlDatadir(dataDir); err != nil {
				return err
			}
		}
	} else {
		j.log("no mariadb-install-db, falling back to --initialize-insecure")
	}
	if !initialized {
		cmd := hiddenCommand(mysqld, "--defaults-file="+defaultsFile, "--initialize-insecure")
		if err := runLogged(cmd); err != nil {
			return fmt.Errorf("mysqld --initialize-insecure failed: %w", err)
		}
	}
	if rootPass == "" {
		return nil
	}
	client, err := mysqlClientPath(binDir)
	if err != nil {
		return err
	}
	j.log("setting the initial root password")
	pid, err := j.startHidden(mysqld, j.Root, "--defaults-file="+defaultsFile)
	if err != nil {
		return err
	}
	defer func() {
		if testPortablePID(pid, mysqld) {
			_, _ = j.invokeMysql(client, binDir, defaultsFile, "root", rootPass, port, "", "SHUTDOWN;", "", true)
			if waitGone([]int{pid}, 30*time.Second) != nil {
				killPID(pid)
			}
		}
	}()
	if err := j.waitMysqlReady(client, binDir, defaultsFile, "root", "", port, 90); err != nil {
		if testPortablePID(pid, mysqld) {
			killPID(pid)
		}
		return err
	}
	passSQL, err := sqlLiteral(rootPass)
	if err != nil {
		return err
	}
	sql := "ALTER USER IF EXISTS 'root'@'localhost' IDENTIFIED BY " + passSQL + ";\n" +
		"ALTER USER IF EXISTS 'root'@'127.0.0.1' IDENTIFIED BY " + passSQL + ";\nFLUSH PRIVILEGES;"
	if _, err := j.invokeMysql(client, binDir, defaultsFile, "root", "", port, "", sql, "", false); err != nil {
		return err
	}
	if !j.mysqlReady(client, binDir, defaultsFile, "root", rootPass, port) {
		return fmt.Errorf("root password was set but could not be verified over the portable MySQL connection")
	}
	return nil
}

func (j *Job) ensureDatabaseUser(client, binDir, defaultsFile, rootUser, rootPass, user, pass string, port int) error {
	userSQL, err := sqlLiteral(user)
	if err != nil {
		return err
	}
	passSQL, err := sqlLiteral(pass)
	if err != nil {
		return err
	}
	sql := fmt.Sprintf(`
CREATE USER IF NOT EXISTS %s@'localhost' IDENTIFIED BY %s;
CREATE USER IF NOT EXISTS %s@'127.0.0.1' IDENTIFIED BY %s;
ALTER USER %s@'localhost' IDENTIFIED BY %s;
ALTER USER %s@'127.0.0.1' IDENTIFIED BY %s;
GRANT ALL PRIVILEGES ON tw_char.* TO %s@'localhost';
GRANT ALL PRIVILEGES ON tw_logon.* TO %s@'localhost';
GRANT ALL PRIVILEGES ON tw_world.* TO %s@'localhost';
GRANT ALL PRIVILEGES ON tw_logs.* TO %s@'localhost';
GRANT ALL PRIVILEGES ON tw_char.* TO %s@'127.0.0.1';
GRANT ALL PRIVILEGES ON tw_logon.* TO %s@'127.0.0.1';
GRANT ALL PRIVILEGES ON tw_world.* TO %s@'127.0.0.1';
GRANT ALL PRIVILEGES ON tw_logs.* TO %s@'127.0.0.1';
FLUSH PRIVILEGES;
`, userSQL, passSQL, userSQL, passSQL, userSQL, passSQL, userSQL, passSQL,
		userSQL, userSQL, userSQL, userSQL, userSQL, userSQL, userSQL, userSQL)
	_, err = j.invokeMysql(client, binDir, defaultsFile, rootUser, rootPass, port, "", sql, "", false)
	return err
}
