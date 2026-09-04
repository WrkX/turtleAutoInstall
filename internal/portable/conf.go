package portable

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func sqlLiteral(value string) (string, error) {
	if strings.IndexByte(value, 0) >= 0 {
		return "", fmt.Errorf("SQL values may not contain NUL bytes")
	}
	return "'" + strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `'`, `''`) + "'", nil
}

func iniPath(p string) string {
	return strings.ReplaceAll(p, `\`, `/`)
}

func writeFromTemplate(templatePath, outPath string, replacements map[string]string) error {
	body, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}
	text := string(body)
	for k, v := range replacements {
		text = strings.ReplaceAll(text, k, v)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(outPath, []byte(text), 0644)
}

func patchConfDatabaseLines(path, host string, port int, user, password string) error {
	for _, part := range []string{user, password} {
		if strings.ContainsAny(part, ";\"\r\n") {
			return fmt.Errorf("MYSQL_USER/MYSQL_PASSWORD cannot contain semicolons, double quotes, or line breaks")
		}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("missing conf: %s", path)
	}
	info := fmt.Sprintf(`"%s;%d;%s;%s`, host, port, user, password)
	replacements := map[string]string{
		`LoginDatabase\.Info\s*=\s*".*"`:     "LoginDatabase.Info = " + info + `;tw_logon"`,
		`LoginDatabaseInfo\s*=\s*".*"`:       "LoginDatabaseInfo = " + info + `;tw_logon"`,
		`WorldDatabase\.Info\s*=\s*".*"`:     "WorldDatabase.Info = " + info + `;tw_world"`,
		`CharacterDatabase\.Info\s*=\s*".*"`: "CharacterDatabase.Info = " + info + `;tw_char"`,
		`LogsDatabase\.Info\s*=\s*".*"`:      "LogsDatabase.Info = " + info + `;tw_logs"`,
	}
	text := string(content)
	for pattern, repl := range replacements {
		re := regexp.MustCompile(pattern)
		text = re.ReplaceAllString(text, repl)
	}
	return os.WriteFile(path, []byte(text), 0644)
}

func setConfValue(path, key, value string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(content)
	pattern := regexp.MustCompile(`(?m)^(\s*)` + regexp.QuoteMeta(key) + `\s*=\s*.*$`)
	if pattern.MatchString(text) {
		text = pattern.ReplaceAllString(text, `${1}`+key+` = `+value)
	} else {
		text = strings.TrimRight(text, "\r\n") + "\r\n" + key + " = " + value + "\r\n"
	}
	return os.WriteFile(path, []byte(text), 0644)
}

func quoteCnf(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`)
}
