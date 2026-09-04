package portable

import (
	"context"
	"fmt"
	"strings"
)

// Run executes a portable installer job that used to live in tools/*.ps1.
func Run(ctx context.Context, root, script string, args []string, extraEnv map[string]string, log func(string)) error {
	j := newJob(ctx, root, extraEnv, log)
	fl, err := parsePSArgs(args)
	if err != nil {
		return err
	}
	name := strings.ToLower(strings.TrimSuffix(script, ".ps1"))
	switch name {
	case "start":
		return j.StartRealm()
	case "stop":
		return j.StopRealm()
	case "start-mysql":
		return j.StartMySQL()
	case "stop-mysql":
		return j.StopMySQL()
	case "setup":
		return j.Setup(fl)
	case "update":
		return j.Update(fl)
	case "fetch-maps":
		return j.FetchMaps(fl.Force)
	case "fetch-mariadb":
		return j.FetchMariaDB()
	case "fetch-server":
		return j.FetchServer(fl.Force)
	case "fetch-sql":
		return j.FetchSQL(fl.Force)
	case "import-databases":
		return j.ImportDatabases(fl)
	case "sync-sql":
		return j.SyncSQL(fl.Source)
	case "sync-realmlist":
		return j.SyncRealmlist()
	case "create-account":
		return j.CreateAccount(fl)
	default:
		return fmt.Errorf("unknown job %q", script)
	}
}
