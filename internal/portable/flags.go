package portable

import (
	"fmt"
	"strconv"
	"strings"
)

type flags struct {
	Force          bool
	ForceReimport  bool
	SkipDownload   bool
	SkipSqlSync    bool
	SkipBase       bool
	SkipUpdates    bool
	SkipPlayerbots bool
	Username       string
	Source         string
	GMLevel        int
}

func parsePSArgs(args []string) (flags, error) {
	var f flags
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, value, hasValue := splitArg(a)
		switch strings.ToLower(name) {
		case "-force":
			f.Force = boolArg(value, hasValue, true)
		case "-forcereimport":
			f.ForceReimport = boolArg(value, hasValue, true)
		case "-skipdownload":
			f.SkipDownload = boolArg(value, hasValue, true)
		case "-skipsqlsync":
			f.SkipSqlSync = boolArg(value, hasValue, true)
		case "-skipbase":
			f.SkipBase = boolArg(value, hasValue, true)
		case "-skipupdates":
			f.SkipUpdates = boolArg(value, hasValue, true)
		case "-skipplayerbots":
			f.SkipPlayerbots = boolArg(value, hasValue, true)
		case "-username":
			v, err := takeValue(args, &i, value, hasValue, name)
			if err != nil {
				return f, err
			}
			f.Username = v
		case "-source":
			v, err := takeValue(args, &i, value, hasValue, name)
			if err != nil {
				return f, err
			}
			f.Source = v
		case "-gmlevel":
			v, err := takeValue(args, &i, value, hasValue, name)
			if err != nil {
				return f, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return f, fmt.Errorf("invalid %s: %s", name, v)
			}
			f.GMLevel = n
		default:
			return f, fmt.Errorf("unknown argument %q", a)
		}
	}
	return f, nil
}

func splitArg(a string) (name, value string, hasValue bool) {
	name = a
	if strings.HasPrefix(a, "-") {
		if i := strings.IndexAny(a, ":="); i > 0 {
			return a[:i], a[i+1:], true
		}
	}
	return name, "", false
}

func boolArg(value string, hasValue, def bool) bool {
	if !hasValue {
		return def
	}
	switch strings.ToLower(value) {
	case "0", "false", "$false":
		return false
	default:
		return true
	}
}

func takeValue(args []string, i *int, value string, hasValue bool, name string) (string, error) {
	if hasValue {
		return value, nil
	}
	if *i+1 >= len(args) {
		return "", fmt.Errorf("%s needs a value", name)
	}
	*i++
	return args[*i], nil
}
