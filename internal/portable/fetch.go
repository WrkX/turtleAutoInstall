package portable

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (j *Job) FetchMariaDB() error {
	version := j.get("MARIADB_VERSION", "10.11.11")
	url := j.get("MARIADB_ZIP_URL", fmt.Sprintf("https://archive.mariadb.org/mariadb-%s/winx64-packages/mariadb-%s-winx64.zip", version, version))
	dest := filepath.Join(j.Root, "mariadb")
	cache := j.cacheDir()
	zipPath := filepath.Join(cache, "mariadb-"+version+"-winx64.zip")
	if err := j.mkdirAll(cache); err != nil {
		return err
	}
	if existing := findMariaDBBin(j.Root); existing != "" {
		j.log("MariaDB already at " + existing)
		return nil
	}
	if !fileExists(zipPath) {
		j.logf("Downloading MariaDB %s from %s", version, url)
		if err := j.downloadFile(url, zipPath, userAgent); err != nil {
			return err
		}
	}
	j.log("Unpacking...")
	extract := filepath.Join(cache, "extract-"+version)
	_ = os.RemoveAll(extract)
	if err := os.MkdirAll(extract, 0755); err != nil {
		return err
	}
	defer os.RemoveAll(extract)
	if err := unzip(zipPath, extract); err != nil {
		_ = os.Remove(zipPath)
		return err
	}
	entries, err := os.ReadDir(extract)
	if err != nil {
		return err
	}
	var inner string
	for _, e := range entries {
		if e.IsDir() {
			inner = filepath.Join(extract, e.Name())
			break
		}
	}
	if inner == "" {
		_ = os.Remove(zipPath)
		return fmt.Errorf("ZIP had no top-level folder under %s", extract)
	}
	_ = os.RemoveAll(dest)
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	children, err := os.ReadDir(inner)
	if err != nil {
		return err
	}
	for _, c := range children {
		if err := os.Rename(filepath.Join(inner, c.Name()), filepath.Join(dest, c.Name())); err != nil {
			_ = os.Remove(zipPath)
			return err
		}
	}
	bin := findMariaDBBin(j.Root)
	if bin == "" {
		_ = os.Remove(zipPath)
		return fmt.Errorf("unpack finished but mysqld/mariadbd is missing")
	}
	j.log("OK: " + bin)
	return nil
}

func (j *Job) FetchServer(force bool) error {
	serverDir := filepath.Join(j.Root, "server")
	cache := j.cacheDir()
	if err := j.mkdirAll(cache); err != nil {
		return err
	}
	if err := j.mkdirAll(serverDir); err != nil {
		return err
	}
	if fileExists(filepath.Join(serverDir, "mangosd.exe")) && fileExists(filepath.Join(serverDir, "realmd.exe")) && !force {
		j.log("Server already at " + serverDir + " (pass -Force to replace from GitHub)")
		return nil
	}
	asset, err := j.resolveReleaseAsset("tortoise-wow-windows-server-*.zip", "TORTOISE_WOW_SERVER_ZIP_URL", "tortoise-wow-windows-server.zip", "server zip")
	if err != nil {
		return err
	}
	zipPath := filepath.Join(cache, asset.Name)
	if force || !fileExists(zipPath) {
		j.log("Downloading " + asset.URL)
		if err := j.downloadFile(asset.URL, zipPath, userAgent); err != nil {
			return err
		}
	}
	j.log("Unpacking into server\\...")
	extract := filepath.Join(cache, "server-extract")
	backup := filepath.Join(cache, "server-backup-"+strings.ReplaceAll(asset.Name, " ", "_"))
	_ = os.RemoveAll(extract)
	if err := os.MkdirAll(extract, 0755); err != nil {
		return err
	}
	var backedUp, installed []string
	keepBackup := false
	defer func() {
		_ = os.RemoveAll(extract)
		if !keepBackup {
			_ = os.RemoveAll(backup)
		}
	}()
	if err := unzip(zipPath, extract); err != nil {
		_ = os.Remove(zipPath)
		return err
	}
	entries, err := os.ReadDir(extract)
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	has := func(name string) bool {
		for _, f := range files {
			if strings.EqualFold(f, name) {
				return true
			}
		}
		return false
	}
	for _, required := range []string{"mangosd.exe", "realmd.exe"} {
		if !has(required) {
			_ = os.Remove(zipPath)
			return fmt.Errorf("unpack finished but the archive has no %s at its top level", required)
		}
	}
	if err := os.MkdirAll(backup, 0755); err != nil {
		return err
	}
	rollback := func(cause error) error {
		for _, name := range installed {
			_ = os.Remove(filepath.Join(serverDir, name))
		}
		for _, name := range backedUp {
			old := filepath.Join(backup, name)
			if fileExists(old) {
				if err := os.Rename(old, filepath.Join(serverDir, name)); err != nil {
					keepBackup = true
					j.warn("could not restore server\\" + name + ": " + err.Error())
				}
			}
		}
		if keepBackup {
			j.warn("rollback was incomplete; preserving backup at " + backup)
		}
		_ = os.Remove(zipPath)
		return cause
	}
	for _, name := range files {
		dest := filepath.Join(serverDir, name)
		if fileExists(dest) {
			if err := os.Rename(dest, filepath.Join(backup, name)); err != nil {
				return rollback(err)
			}
			backedUp = append(backedUp, name)
		}
	}
	for _, name := range files {
		if err := os.Rename(filepath.Join(extract, name), filepath.Join(serverDir, name)); err != nil {
			return rollback(err)
		}
		installed = append(installed, name)
	}
	for _, required := range []string{"mangosd.exe", "realmd.exe"} {
		if !fileExists(filepath.Join(serverDir, required)) {
			return rollback(fmt.Errorf("unpack finished but server\\%s is missing", required))
		}
	}
	tag := asset.TagName
	if tag == "" {
		tag = asset.Name
	}
	if err := writeMarker(filepath.Join(j.Root, "data", ".server-release"), tag); err != nil {
		return err
	}
	j.logf("OK: %s (%s)", serverDir, tag)
	return nil
}

func (j *Job) FetchSQL(force bool) error {
	sqlDir := filepath.Join(j.Root, "sql")
	cache := j.cacheDir()
	if err := j.mkdirAll(cache); err != nil {
		return err
	}
	if fileExists(filepath.Join(sqlDir, "create_databases.sql")) && !force {
		j.log("SQL already at " + sqlDir + " (pass -Force to replace from GitHub)")
		return nil
	}
	asset, err := j.resolveReleaseAsset("tortoise-wow-sql-*.zip", "TORTOISE_WOW_SQL_ZIP_URL", "tortoise-wow-sql.zip", "SQL zip")
	if err != nil {
		return err
	}
	zipPath := filepath.Join(cache, asset.Name)
	if force || !fileExists(zipPath) {
		j.log("Downloading " + asset.URL)
		if err := j.downloadFile(asset.URL, zipPath, userAgent); err != nil {
			return err
		}
	}
	j.log("Unpacking into sql\\...")
	extract := filepath.Join(cache, "sql-extract")
	backup := filepath.Join(cache, "sql-backup-"+asset.Name)
	_ = os.RemoveAll(extract)
	if err := os.MkdirAll(extract, 0755); err != nil {
		return err
	}
	hadOld, moved := false, false
	keepBackup := false
	defer func() {
		_ = os.RemoveAll(extract)
		if !keepBackup {
			_ = os.RemoveAll(backup)
		}
	}()
	if err := unzip(zipPath, extract); err != nil {
		_ = os.Remove(zipPath)
		return err
	}
	srcSQL := filepath.Join(extract, "sql")
	if !fileExists(filepath.Join(srcSQL, "create_databases.sql")) {
		_ = os.Remove(zipPath)
		return fmt.Errorf("unpack finished but sql/create_databases.sql is missing - bad zip layout?")
	}
	rollback := func(cause error) error {
		if moved && fileExists(sqlDir) {
			_ = os.RemoveAll(sqlDir)
		}
		if hadOld && fileExists(backup) {
			if err := os.Rename(backup, sqlDir); err != nil {
				keepBackup = true
				j.warn("could not restore previous SQL tree: " + err.Error())
			}
		}
		if keepBackup {
			j.warn("rollback was incomplete; preserving backup at " + backup)
		}
		_ = os.Remove(zipPath)
		return cause
	}
	if fileExists(sqlDir) {
		if err := os.Rename(sqlDir, backup); err != nil {
			return rollback(err)
		}
		hadOld = true
	}
	if err := os.Rename(srcSQL, sqlDir); err != nil {
		return rollback(err)
	}
	moved = true
	tag := asset.TagName
	if tag == "" {
		tag = asset.Name
	}
	if err := writeMarker(filepath.Join(j.Root, "data", ".sql-release"), tag); err != nil {
		return err
	}
	j.logf("OK: %s (%s)", sqlDir, tag)
	return nil
}

func (j *Job) SyncSQL(source string) error {
	if source == "" {
		source = j.get("TORTOISE_WOW_SRC", "")
	}
	if source == "" {
		sibling := filepath.Join(filepath.Dir(j.Root), "tortoise-wow")
		if fileExists(filepath.Join(sibling, "sql", "create_databases.sql")) {
			source = sibling
		}
	}
	if source == "" || !fileExists(source) {
		return fmt.Errorf("set TORTOISE_WOW_SRC in portable.local.env to a tortoise-wow checkout")
	}
	srcSQL := filepath.Join(source, "sql")
	dstSQL := filepath.Join(j.Root, "sql")
	if !fileExists(filepath.Join(srcSQL, "create_databases.sql")) {
		return fmt.Errorf("not a tortoise-wow tree (missing sql/create_databases.sql): %s", source)
	}
	j.log("Copying SQL from " + source)
	if err := os.MkdirAll(dstSQL, 0755); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(srcSQL, "create_databases.sql"), filepath.Join(dstSQL, "create_databases.sql")); err != nil {
		return err
	}
	for _, dir := range []string{"base", "database_updates"} {
		from := filepath.Join(srcSQL, dir)
		to := filepath.Join(dstSQL, dir)
		if !fileExists(from) {
			j.warn("missing " + from + " - skipped")
			continue
		}
		_ = os.RemoveAll(to)
		if err := copyDir(from, to); err != nil {
			return err
		}
	}
	pbSrc := filepath.Join(source, "src", "modules", "PlayerBots", "sql")
	pbDst := filepath.Join(dstSQL, "playerbots")
	if fileExists(pbSrc) {
		_ = os.RemoveAll(pbDst)
		worldDst := filepath.Join(pbDst, "world")
		if err := os.MkdirAll(worldDst, 0755); err != nil {
			return err
		}
		matches, _ := filepath.Glob(filepath.Join(pbSrc, "world", "*.sql"))
		for _, m := range matches {
			if err := copyFile(m, filepath.Join(worldDst, filepath.Base(m))); err != nil {
				return err
			}
		}
		classic := filepath.Join(pbSrc, "world", "classic")
		if fileExists(classic) {
			if err := copyDir(classic, filepath.Join(worldDst, "classic")); err != nil {
				return err
			}
		}
		charDst := filepath.Join(pbDst, "characters")
		if err := os.MkdirAll(charDst, 0755); err != nil {
			return err
		}
		matches, _ = filepath.Glob(filepath.Join(pbSrc, "characters", "*.sql"))
		for _, m := range matches {
			if err := copyFile(m, filepath.Join(charDst, filepath.Base(m))); err != nil {
				return err
			}
		}
	} else {
		j.warn("no PlayerBots SQL at " + pbSrc)
	}
	j.log("Done.")
	return nil
}
