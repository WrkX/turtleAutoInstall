package main

import (
	"crypto/sha1"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var accountNameRe = regexp.MustCompile(`^[A-Za-z0-9]{2,16}$`)

type accountField struct {
	Key         string
	Label       string
	Placeholder string
	Secret      bool
}

var accountSpec = []accountField{
	{"username", "Username", "2–16 letters or digits", false},
	{"password", "Password", "same rules as the WoW client", true},
	{"gmlevel", "GM level", "0 player, 3 GM", false},
}

type accountForm struct {
	inputs []textinput.Model
	focus  int
	err    string
}

func newAccountForm() accountForm {
	inputs := make([]textinput.Model, len(accountSpec))
	for i, spec := range accountSpec {
		ti := textinput.New()
		ti.Placeholder = spec.Placeholder
		ti.CharLimit = 16
		ti.Width = 36
		ti.Prompt = "  "
		ti.PromptStyle = dimStyle
		ti.TextStyle = textStyle
		ti.PlaceholderStyle = faintStyle
		ti.Cursor.Style = lipgloss.NewStyle().Foreground(colGoldHi)
		if spec.Secret {
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '•'
		}
		if spec.Key == "gmlevel" {
			ti.SetValue("0")
			ti.CharLimit = 1
		}
		inputs[i] = ti
	}
	f := accountForm{inputs: inputs, focus: 0}
	f.syncFocus()
	return f
}

func (f *accountForm) syncFocus() {
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

func (f *accountForm) Update(msg tea.Msg) tea.Cmd {
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

func (f accountForm) username() string {
	return strings.TrimSpace(f.inputs[0].Value())
}

func (f accountForm) password() string {
	return f.inputs[1].Value()
}

func (f accountForm) gmLevel() string {
	return strings.TrimSpace(f.inputs[2].Value())
}

func validateAccountName(name string) error {
	if !accountNameRe.MatchString(name) {
		return fmt.Errorf("username must be 2–16 letters or digits")
	}
	return nil
}

func wowPassHash(username, password string) string {
	src := strings.ToUpper(username) + ":" + strings.ToUpper(password)
	sum := sha1.Sum([]byte(src))
	return fmt.Sprintf("%x", sum)
}

func (f accountForm) validate() (user, hash, gm string, err error) {
	user = f.username()
	if err = validateAccountName(user); err != nil {
		return "", "", "", err
	}
	pass := f.password()
	if len(pass) < 1 || len(pass) > 16 {
		return "", "", "", fmt.Errorf("password must be 1–16 characters")
	}
	gm = f.gmLevel()
	if gm == "" {
		gm = "0"
	}
	n, convErr := strconv.Atoi(gm)
	if convErr != nil || n < 0 || n > 3 {
		return "", "", "", fmt.Errorf("GM level must be 0–3")
	}
	return user, wowPassHash(user, pass), gm, nil
}

func (f accountForm) View(width, height int) string {
	var b strings.Builder
	b.WriteString(cardTitleStyle.Render("create account"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Writes tw_logon.account. Existing names get a password reset. MySQL is started if needed."))
	b.WriteString("\n\n")

	inner := max(width-4, 20)
	for i, spec := range accountSpec {
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
	if f.err != "" {
		b.WriteByte('\n')
		b.WriteString(errStyle.Render(f.err))
	}

	all := strings.TrimRight(b.String(), "\n")
	lines := strings.Split(all, "\n")
	if height > 4 && len(lines) > height {
		start := clamp(0, 0, len(lines)-height)
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
