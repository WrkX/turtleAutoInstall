package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type settingField struct {
	Key         string
	Label       string
	Placeholder string
	Group       string
	Secret      bool
}

var settingSpec = []settingField{
	{"REALM_NAME", "Realm name", "TurtleWoW", "Realm", false},
	{"REALM_ADDRESS", "Realm address", "127.0.0.1", "Realm", false},
	{"REALM_PORT", "Auth port", "3724", "Realm", false},
	{"WORLD_PORT", "World port", "8090", "Realm", false},
	{"MIN_RANDOM_BOTS", "Min random bots", "20", "Realm", false},
	{"MAX_RANDOM_BOTS", "Max random bots", "20", "Realm", false},
	{"MYSQL_PORT", "MySQL port", "3307", "Database", false},
	{"MYSQL_USER", "MySQL user", "mangos", "Database", false},
	{"MYSQL_PASSWORD", "MySQL password", "mangos", "Database", true},
	{"TORTOISE_WOW_RELEASE", "Release tag", "latest", "Downloads", false},
	{"TORTOISE_WOW_REPO", "GitHub repo", "WrkX/tortoise-wow", "Downloads", false},
	{"TORTOISE_WOW_SRC", "Local source tree", `C:\dev\tortoise-wow`, "Downloads", false},
	{"TORTOISE_WOW_MAPS_ZIP_URL", "Maps zip URL override", "leave empty to use conf/maps-url.txt", "Downloads", false},
}

type settingsForm struct {
	inputs []textinput.Model
	focus  int
	err    string
}

func newSettings(env map[string]string) settingsForm {
	inputs := make([]textinput.Model, len(settingSpec))
	for i, spec := range settingSpec {
		ti := textinput.New()
		ti.Placeholder = spec.Placeholder
		ti.CharLimit = 160
		if spec.Key == "TORTOISE_WOW_MAPS_ZIP_URL" {
			ti.CharLimit = 500
		}
		ti.Width = 36
		ti.Prompt = "  "
		ti.PromptStyle = dimStyle
		ti.TextStyle = textStyle
		ti.PlaceholderStyle = faintStyle
		ti.Cursor.Style = lipgloss.NewStyle().Foreground(colGoldHi)
		if v := strings.TrimSpace(env[spec.Key]); v != "" {
			ti.SetValue(v)
		}
		if spec.Secret {
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '•'
		}
		inputs[i] = ti
	}
	f := settingsForm{inputs: inputs, focus: 0}
	f.syncFocus()
	return f
}

func (f *settingsForm) syncFocus() {
	for i := range f.inputs {
		if i == f.focus {
			f.inputs[i].Focus()
			f.inputs[i].Prompt = "▸ "
			f.inputs[i].PromptStyle = selTitleStyle
			f.inputs[i].TextStyle = selTitleStyle
		} else {
			f.inputs[i].Blur()
			f.inputs[i].Prompt = "  "
			f.inputs[i].PromptStyle = dimStyle
			f.inputs[i].TextStyle = textStyle
		}
	}
}

func (f *settingsForm) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "shift+tab":
			f.focus = (f.focus - 1 + len(f.inputs)) % len(f.inputs)
			f.syncFocus()
			return f.inputs[f.focus].Focus()
		case "down", "tab":
			f.focus = (f.focus + 1) % len(f.inputs)
			f.syncFocus()
			return f.inputs[f.focus].Focus()
		}
	}
	var cmd tea.Cmd
	f.inputs[f.focus], cmd = f.inputs[f.focus].Update(msg)
	f.err = ""
	return cmd
}

func (f settingsForm) value(key string) string {
	for i, spec := range settingSpec {
		if spec.Key == key {
			return strings.TrimSpace(f.inputs[i].Value())
		}
	}
	return ""
}

func (f settingsForm) Validate() error {
	for _, spec := range settingSpec {
		value := f.value(spec.Key)
		if strings.ContainsAny(value, "\r\n") || strings.IndexByte(value, 0) >= 0 {
			return fmt.Errorf("%s cannot contain line breaks", spec.Label)
		}
	}
	portOwners := map[int]string{}
	for _, field := range []struct{ key, label, def string }{
		{"REALM_PORT", "Auth port", "3724"},
		{"WORLD_PORT", "World port", "8090"},
		{"MYSQL_PORT", "MySQL port", "3307"},
	} {
		value := f.value(field.key)
		if value == "" {
			value = field.def
		}
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("%s must be between 1 and 65535", field.label)
		}
		if owner, exists := portOwners[port]; exists {
			return fmt.Errorf("%s and %s cannot both use port %d", owner, field.label, port)
		}
		portOwners[port] = field.label
	}
	minBots, err := settingInt(f.value("MIN_RANDOM_BOTS"), 20)
	if err != nil || minBots < 0 {
		return fmt.Errorf("Min random bots must be a non-negative number")
	}
	maxBots, err := settingInt(f.value("MAX_RANDOM_BOTS"), 20)
	if err != nil || maxBots < 0 {
		return fmt.Errorf("Max random bots must be a non-negative number")
	}
	if minBots > maxBots {
		return fmt.Errorf("Min random bots cannot exceed Max random bots")
	}
	address := f.value("REALM_ADDRESS")
	if strings.ContainsAny(address, " \t") {
		return fmt.Errorf("Realm address cannot contain spaces")
	}
	user := f.value("MYSQL_USER")
	if strings.ContainsAny(user, " \t`;\"") {
		return fmt.Errorf("MySQL user cannot contain spaces, semicolons, or quotes")
	}
	if strings.ContainsAny(f.value("MYSQL_PASSWORD"), ";\"") {
		return fmt.Errorf("MySQL password cannot contain semicolons or double quotes")
	}
	repo := f.value("TORTOISE_WOW_REPO")
	if repo != "" && (strings.Count(repo, "/") != 1 || strings.ContainsAny(repo, " \t")) {
		return fmt.Errorf("GitHub repo must look like owner/repository")
	}
	mapsURL := f.value("TORTOISE_WOW_MAPS_ZIP_URL")
	if mapsURL != "" {
		u, err := url.Parse(mapsURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("Maps zip URL must be an http(s) URL")
		}
	}
	return nil
}

func settingInt(value string, def int) (int, error) {
	if value == "" {
		return def, nil
	}
	return strconv.Atoi(value)
}

func (f settingsForm) Values() map[string]string {
	out := make(map[string]string, len(settingSpec))
	for i, spec := range settingSpec {
		if value := strings.TrimSpace(f.inputs[i].Value()); value != "" {
			out[spec.Key] = value
		} else {
			// Include a cleared key so saveLocalEnv can remove an old override.
			out[spec.Key] = ""
		}
	}
	return out
}

func (f settingsForm) View(width, height int) string {
	var lines []string
	lines = append(lines, cardTitleStyle.Render("settings"))
	lines = append(lines, dimStyle.Render("Writes portable.local.env. Run Apply config to patch live server confs."))
	if f.err != "" {
		lines = append(lines, errStyle.Render("error: "+f.err))
	}
	lines = append(lines, "")

	inner := max(width-4, 20)
	lastGroup := ""
	focusLine := 0
	for i, spec := range settingSpec {
		if spec.Group != lastGroup {
			if lastGroup != "" {
				lines = append(lines, "")
			}
			lines = append(lines, faintStyle.Render(strings.ToUpper(spec.Group)))
			lastGroup = spec.Group
		}
		label := dimStyle.Render(spec.Label)
		if i == f.focus {
			label = selTitleStyle.Render(spec.Label)
			focusLine = len(lines)
		}
		f.inputs[i].Width = max(inner-4, 12)
		lines = append(lines, label)
		lines = append(lines, f.inputs[i].View())
	}
	if height > 4 && len(lines) > height {
		start := focusLine - height/3
		start = clamp(start, 0, len(lines)-height)
		lines = lines[start : start+height]
	}
	body := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colGold).
		Padding(0, 1).
		Width(max(width-2, 16)).
		Render(body)
}
