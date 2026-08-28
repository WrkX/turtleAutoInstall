package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type settingField struct {
	Key         string
	Label       string
	Placeholder string
}

var settingSpec = []settingField{
	{"REALM_NAME", "Realm name", "TurtleWoW"},
	{"REALM_ADDRESS", "Realm address", "127.0.0.1"},
	{"REALM_PORT", "Auth port", "3724"},
	{"WORLD_PORT", "World port", "8090"},
	{"MYSQL_PORT", "MySQL port", "3306"},
	{"MYSQL_USER", "MySQL user", "mangos"},
	{"MYSQL_PASSWORD", "MySQL password", "mangos"},
	{"MIN_RANDOM_BOTS", "Min random bots", "20"},
	{"MAX_RANDOM_BOTS", "Max random bots", "20"},
	{"TORTOISE_WOW_RELEASE", "Release tag", "latest"},
	{"TORTOISE_WOW_REPO", "GitHub repo", "WrkX/tortoise-wow"},
	{"TORTOISE_WOW_SRC", "Local source tree", `C:\dev\tortoise-wow`},
}

type settingsForm struct {
	inputs []textinput.Model
	focus  int
}

func newSettings(env map[string]string) settingsForm {
	inputs := make([]textinput.Model, len(settingSpec))
	for i, spec := range settingSpec {
		ti := textinput.New()
		ti.Placeholder = spec.Placeholder
		ti.CharLimit = 160
		ti.Width = 36
		ti.Prompt = "  "
		ti.PromptStyle = dimStyle
		ti.TextStyle = textStyle
		ti.PlaceholderStyle = faintStyle
		ti.Cursor.Style = lipgloss.NewStyle().Foreground(colGoldHi)
		if v := env[spec.Key]; v != "" {
			ti.SetValue(v)
		} else {
			ti.SetValue(spec.Placeholder)
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
		case "up", "k", "shift+tab":
			f.focus = (f.focus - 1 + len(f.inputs)) % len(f.inputs)
			f.syncFocus()
			return f.inputs[f.focus].Focus()
		case "down", "j", "tab":
			f.focus = (f.focus + 1) % len(f.inputs)
			f.syncFocus()
			return f.inputs[f.focus].Focus()
		}
	}
	var cmd tea.Cmd
	f.inputs[f.focus], cmd = f.inputs[f.focus].Update(msg)
	return cmd
}

func (f settingsForm) Values() map[string]string {
	out := make(map[string]string, len(settingSpec))
	for i, spec := range settingSpec {
		out[spec.Key] = strings.TrimSpace(f.inputs[i].Value())
	}
	return out
}

func (f settingsForm) View(width, height int) string {
	var b strings.Builder
	b.WriteString(cardTitleStyle.Render("settings"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Writes portable.local.env. Run Apply config to patch live server confs."))
	b.WriteString("\n\n")

	inner := max(width-4, 20)
	for i, spec := range settingSpec {
		label := dimStyle.Render(spec.Label)
		if i == f.focus {
			label = selTitleStyle.Render(spec.Label)
		}
		f.inputs[i].Width = max(inner-4, 12)
		b.WriteString(label)
		b.WriteByte('\n')
		b.WriteString(f.inputs[i].View())
		b.WriteByte('\n')
	}

	all := strings.TrimRight(b.String(), "\n")
	lines := strings.Split(all, "\n")
	focusLine := 3 + f.focus*2
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
