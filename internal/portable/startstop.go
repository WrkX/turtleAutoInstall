package portable

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (j *Job) mysqlSettings() (port int, rootUser, rootPass, bin, mysqld, client, myIni string, err error) {
	port, err = j.getInt("MYSQL_PORT", "3307")
	if err != nil {
		return
	}
	rootUser = j.get("MYSQL_ROOT_USER", "root")
	rootPass = j.get("MYSQL_ROOT_PASSWORD", "")
	bin = findMariaDBBin(j.Root)
	if bin == "" {
		err = fmt.Errorf("MariaDB missing - run Full setup first")
		return
	}
	mysqld, err = mysqldPath(bin)
	if err != nil {
		return
	}
	client, err = mysqlClientPath(bin)
	if err != nil {
		return
	}
	myIni = filepath.Join(j.Root, "conf", "my.ini")
	return
}

func (j *Job) StartMySQL() error {
	port, rootUser, rootPass, bin, mysqld, client, myIni, err := j.mysqlSettings()
	if err != nil {
		return err
	}
	if !fileExists(myIni) {
		return fmt.Errorf("no %s - run Full setup first", myIni)
	}
	if err := j.assertMysqlPort(port, mysqld); err != nil {
		return err
	}
	if j.mysqlReady(client, bin, myIni, rootUser, rootPass, port) {
		j.logf("mysqld already up on %d", port)
		return nil
	}
	pid := listeningPID(port)
	if pid > 0 && testPortablePID(pid, mysqld) {
		return fmt.Errorf("portable mysqld is already listening on %d but rejected the configured credentials; check MYSQL_ROOT_USER/MYSQL_ROOT_PASSWORD", port)
	}
	j.log("starting mysqld")
	id, err := j.startHidden(mysqld, j.Root, "--defaults-file="+myIni)
	if err != nil {
		return err
	}
	if err := writePID(j.Root, "mysqld", id); err != nil {
		return err
	}
	if err := j.waitMysqlReady(client, bin, myIni, rootUser, rootPass, port, 90); err != nil {
		if testPortablePID(id, mysqld) {
			j.warn(fmt.Sprintf("mysqld did not become ready; stopping pid %d", id))
			killPID(id)
		}
		removePID(j.Root, "mysqld")
		return err
	}
	j.logf("mysqld pid %d", id)
	return nil
}

func (j *Job) StopMySQL() error {
	port, err := j.getInt("MYSQL_PORT", "3307")
	if err != nil {
		return err
	}
	rootUser := j.get("MYSQL_ROOT_USER", "root")
	rootPass := j.get("MYSQL_ROOT_PASSWORD", "")
	bin := findMariaDBBin(j.Root)
	if bin == "" {
		j.log("no mariadb folder")
		return nil
	}
	client, err := mysqlClientPath(bin)
	if err != nil {
		return err
	}
	mysqld, err := mysqldPath(bin)
	if err != nil {
		return err
	}
	myIni := filepath.Join(j.Root, "conf", "my.ini")
	owner := listeningPID(port)
	ownerIsOurs := owner > 0 && testPortablePID(owner, mysqld)
	recorded := readPID(j.Root, "mysqld")
	if !ownerIsOurs {
		if recorded > 0 && testPortablePID(recorded, mysqld) {
			j.logf("stopping portable mysqld pid %d", recorded)
			killPID(recorded)
			removePID(j.Root, "mysqld")
			return nil
		}
		removePID(j.Root, "mysqld")
		j.warn(fmt.Sprintf("could not verify ownership of the process listening on %d; nothing was stopped", port))
		return nil
	}
	if !j.mysqlReady(client, bin, myIni, rootUser, rootPass, port) {
		j.warn(fmt.Sprintf("portable mysqld owns port %d but rejected the configured credentials; stopping the verified process", port))
		if testPortablePID(owner, mysqld) {
			killPID(owner)
		}
		removePID(j.Root, "mysqld")
		return nil
	}
	j.logf("SHUTDOWN on %d", port)
	if _, err := j.invokeMysql(client, bin, myIni, rootUser, rootPass, port, "", "SHUTDOWN;", "", false); err != nil {
		j.warn("SHUTDOWN failed - killing the verified portable process")
		if testPortablePID(owner, mysqld) {
			killPID(owner)
		}
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if !j.mysqlReady(client, bin, myIni, rootUser, rootPass, port) {
			j.log("stopped")
			removePID(j.Root, "mysqld")
			return nil
		}
		time.Sleep(time.Second)
	}
	j.warn("still up after 30s")
	return nil
}

func assertMapsPresent(mapsDir string) error {
	for _, name := range []string{"dbc", "maps", "vmaps", "mmaps"} {
		p := filepath.Join(mapsDir, name)
		st, err := os.Stat(p)
		if err != nil || !st.IsDir() {
			return fmt.Errorf("missing maps\\%s. Run setup or Fetch maps, and provide the four client-data directories", name)
		}
		if !dirHasFiles(p) {
			return fmt.Errorf("maps\\%s is empty. Setup requires dbc, maps, vmaps, and mmaps to contain data", name)
		}
	}
	return nil
}

func (j *Job) StopRealm() error {
	serverDir := filepath.Join(j.Root, "server")
	for _, spec := range []struct{ name, exe string }{
		{"mangosd", filepath.Join(serverDir, "mangosd.exe")},
		{"realmd", filepath.Join(serverDir, "realmd.exe")},
	} {
		ids := processesByPath(spec.exe)
		recorded := readPID(j.Root, spec.name)
		if recorded > 0 && testPortablePID(recorded, spec.exe) {
			ids = append(ids, recorded)
		}
		ids = uniqueInts(ids)
		for _, id := range ids {
			j.logf("kill %s %d", spec.name, id)
			killPID(id)
		}
		if len(ids) > 0 {
			if leftover := waitGone(ids, 10*time.Second); len(leftover) > 0 {
				j.warn(spec.name + " did not exit after 10 seconds")
			} else {
				removePID(j.Root, spec.name)
			}
		} else {
			removePID(j.Root, spec.name)
		}
	}
	return j.StopMySQL()
}

func (j *Job) StartRealm() error {
	serverDir := filepath.Join(j.Root, "server")
	for _, name := range []string{"mangosd.exe", "realmd.exe", "realmd.conf", "mangosd.conf"} {
		if !fileExists(filepath.Join(serverDir, name)) {
			return fmt.Errorf("no server\\%s - run Full setup", name)
		}
	}
	if err := assertMapsPresent(filepath.Join(j.Root, "maps")); err != nil {
		return err
	}
	for _, spec := range []struct{ name, exe string }{
		{"realmd", filepath.Join(serverDir, "realmd.exe")},
		{"mangosd", filepath.Join(serverDir, "mangosd.exe")},
	} {
		running := processesByPath(spec.exe)
		if len(running) > 0 {
			return fmt.Errorf("%s is already running for this install (pid %d)", spec.name, running[0])
		}
		recorded := readPID(j.Root, spec.name)
		if recorded > 0 && testPortablePID(recorded, spec.exe) {
			return fmt.Errorf("%s is already running for this install (pid %d)", spec.name, recorded)
		}
	}
	_, _, _, bin, mysqld, _, _, err := j.mysqlSettings()
	if err != nil {
		return err
	}
	_ = bin
	mysqlBefore := readPID(j.Root, "mysqld")
	mysqlWasRecorded := mysqlBefore > 0 && testPortablePID(mysqlBefore, mysqld)
	if err := j.StartMySQL(); err != nil {
		return err
	}
	mysqlAfter := readPID(j.Root, "mysqld")
	startedMysqlHere := !mysqlWasRecorded && mysqlAfter > 0 && testPortablePID(mysqlAfter, mysqld)

	type started struct {
		name, path string
		id         int
	}
	var launched []started
	fail := func(cause error) error {
		for i := len(launched) - 1; i >= 0; i-- {
			item := launched[i]
			if testPortablePID(item.id, item.path) {
				j.warn("stopping " + item.name + " after startup failure")
				killPID(item.id)
			}
			removePID(j.Root, item.name)
		}
		if startedMysqlHere && testPortablePID(mysqlAfter, mysqld) {
			j.warn("stopping MySQL started by this failed realm launch")
			killPID(mysqlAfter)
			removePID(j.Root, "mysqld")
		}
		return cause
	}

	j.log("realmd (hidden - F1 or click the header pill)")
	realmd := filepath.Join(serverDir, "realmd.exe")
	rid, err := j.startRealmDaemon("realmd", realmd, serverDir)
	if err != nil {
		return fail(err)
	}
	_ = writePID(j.Root, "realmd", rid)
	launched = append(launched, started{"realmd", realmd, rid})
	time.Sleep(2 * time.Second)
	if !processAlive(rid) {
		return fail(fmt.Errorf("realmd exited during startup"))
	}

	j.log("mangosd (hidden - F2 or click the header pill)")
	mangosd := filepath.Join(serverDir, "mangosd.exe")
	mid, err := j.startRealmDaemon("mangosd", mangosd, serverDir)
	if err != nil {
		return fail(err)
	}
	_ = writePID(j.Root, "mangosd", mid)
	launched = append(launched, started{"mangosd", mangosd, mid})
	time.Sleep(2 * time.Second)
	if !processAlive(mid) {
		return fail(fmt.Errorf("mangosd exited during startup"))
	}

	addr := j.get("REALM_ADDRESS", "127.0.0.1")
	j.log("create an account from the TUI (Create account)")
	j.logf("then set realmlist.wtf -> %s", addr)
	return nil
}
