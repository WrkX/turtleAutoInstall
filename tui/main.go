package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen int

const (
	screenHome screen = iota
	screenRun
	screenSettings
	screenAccount
	screenConfirm
)

type model struct {
	root   string
	status Status
	screen screen
	cursor int

	spin spinner.Model
	vp   viewport.Model

	running string
	queue   []scriptStep
	stepI   int
	stepN   int
	log     *strings.Builder
	lines   <-chan string
	done    <-chan error
	cancel  func()
	lastErr error
	aborted bool

	settings settingsForm
	account  accountForm

	confirmTitle string
	confirmBody  string
	pending      []scriptStep
	pendingLabel string

	flash    string
	width    int
	height   int
	ready    bool
	quitting bool
}

func newModel(root string) *model {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = lipgloss.NewStyle().Foreground(colGoldHi)

	st := gatherStatus(root)
	return &model{
		root:   root,
		status: st,
		spin:   sp,
		cursor: firstEnabled(st),
		log:    &strings.Builder{},
	}
}

func firstEnabled(s Status) int {
	for i, a := range menu {
		if a.disabled(s) == "" && a.Builtin != "quit" {
			return i
		}
	}
	return 0
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, refreshStatus(m.root), checkLatest(m.root), scheduleTick())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.layout()
		return m, nil

	case statusMsg:
		latest, latestErr := m.status.Latest, m.status.LatestErr
		m.status = Status(msg)
		m.status.applyLatest(latest, latestErr)
		if m.screen == screenHome && menu[m.cursor].disabled(m.status) != "" {
			m.cursor = m.nextSelectable(1)
		}
		return m, nil

	case latestMsg:
		errStr := ""
		if msg.err != nil {
			errStr = msg.err.Error()
		}
		m.status.applyLatest(msg.tag, errStr)
		return m, nil

	case tickMsg:
		cmds := []tea.Cmd{scheduleTick()}
		if m.screen == screenHome || m.screen == screenRun {
			cmds = append(cmds, refreshStatus(m.root))
		}
		return m, tea.Batch(cmds...)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case streamStartedMsg:
		m.lines = msg.lines
		m.done = msg.done
		m.cancel = msg.cancel
		return m, awaitLine(m.lines, m.done)

	case lineMsg:
		m.log.WriteString(colorizeLine(string(msg)))
		m.log.WriteByte('\n')
		m.vp.SetContent(m.log.String())
		m.vp.GotoBottom()
		if m.lines == nil {
			return m, nil
		}
		return m, awaitLine(m.lines, m.done)

	case doneMsg:
		return m.onDone(msg.err)

	case tea.KeyMsg:
		return m.onKey(msg)
	}

	if m.screen == screenRun {
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) onDone(err error) (tea.Model, tea.Cmd) {
	m.lastErr = err
	m.lines = nil
	m.done = nil
	m.cancel = nil
	if m.aborted {
		m.log.WriteString(errStyle.Render("aborted.") + "\n")
		m.aborted = false
		m.queue = nil
	} else if err != nil {
		m.log.WriteString(errStyle.Render("error: "+err.Error()) + "\n")
		m.queue = nil
	} else {
		m.log.WriteString(okStyle.Render("done.") + "\n")
		if len(m.queue) > 0 {
			m.vp.SetContent(m.log.String())
			m.vp.GotoBottom()
			return m, tea.Batch(refreshStatus(m.root), m.runNext())
		}
	}
	m.vp.SetContent(m.log.String())
	m.vp.GotoBottom()
	return m, refreshStatus(m.root)
}

func (m *model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		m.quitting = true
		if m.cancel != nil {
			m.cancel()
		}
		return m, tea.Quit
	}

	switch m.screen {
	case screenRun:
		return m.onRunKey(msg)
	case screenSettings:
		return m.onSettingsKey(msg)
	case screenAccount:
		return m.onAccountKey(msg)
	case screenConfirm:
		return m.onConfirmKey(msg)
	default:
		return m.onHomeKey(msg)
	}
}

func (m *model) onRunKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		if m.lines == nil {
			m.screen = screenHome
			m.log.Reset()
			m.lastErr = nil
			m.running = ""
			m.queue = nil
			m.stepI = 0
			m.stepN = 0
			return m, nil
		}
	case "x":
		if m.cancel != nil {
			m.aborted = true
			m.cancel()
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m *model) onSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenHome
		return m, nil
	case "ctrl+s":
		return m.saveSettings()
	case "enter":
		if m.settings.focus >= len(m.settings.inputs)-1 {
			return m.saveSettings()
		}
		m.settings.focus++
		m.settings.syncFocus()
		return m, m.settings.inputs[m.settings.focus].Focus()
	}
	cmd := m.settings.Update(msg)
	return m, cmd
}

func (m *model) saveSettings() (tea.Model, tea.Cmd) {
	if err := m.settings.Validate(); err != nil {
		m.settings.err = err.Error()
		return m, nil
	}
	if err := saveLocalEnv(m.root, m.settings.Values()); err != nil {
		m.flash = "could not save: " + err.Error()
		m.screen = screenHome
		return m, refreshStatus(m.root)
	}
	m.flash = "saved portable.local.env — run Apply config to patch server confs"
	m.screen = screenHome
	return m, tea.Batch(refreshStatus(m.root), checkLatest(m.root))
}

func (m *model) onConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		steps := m.pending
		label := m.pendingLabel
		m.pending = nil
		m.pendingLabel = ""
		m.confirmTitle = ""
		m.confirmBody = ""
		return m.beginRun(label, steps)
	case "n", "N", "esc", "q":
		m.screen = screenHome
		m.pending = nil
		m.pendingLabel = ""
		return m, nil
	}
	return m, nil
}

func (m *model) onAccountKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenHome
		return m, nil
	case "ctrl+s":
		return m.submitAccount()
	case "enter":
		if m.account.focus >= len(m.account.inputs)-1 {
			return m.submitAccount()
		}
		m.account.focus++
		m.account.syncFocus()
		return m, m.account.inputs[m.account.focus].Focus()
	}
	cmd := m.account.Update(msg)
	return m, cmd
}

func (m *model) submitAccount() (tea.Model, tea.Cmd) {
	user, hash, gm, err := m.account.validate()
	if err != nil {
		m.account.err = err.Error()
		return m, nil
	}
	// Keep only the derived hash for the short-lived child handoff; the plain
	// password is no longer needed after validation.
	m.account.inputs[1].SetValue("")
	var steps []scriptStep
	if !m.status.Mysqld {
		steps = append(steps, scriptStep{Label: "Start MySQL", Script: "start-mysql.ps1"})
	}
	steps = append(steps, scriptStep{
		Label:  "Create account",
		Script: "create-account.ps1",
		Args:   []string{"-Username", user, "-GMLevel", gm},
		Env:    map[string]string{"TORTOISE_WOW_ACCOUNT_SHA_PASS_HASH": hash},
	})
	return m.beginRun("Create account", steps)
}

func (m *model) copyRealmlist() (tea.Model, tea.Cmd) {
	line := "set realmlist " + m.status.realmAddress()
	if err := clipboard.WriteAll(line); err != nil {
		m.flash = line
		return m, nil
	}
	m.flash = "copied  " + line
	return m, nil
}

func (m *model) openLogs() (tea.Model, tea.Cmd) {
	dir := filepath.Join(m.root, "logs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		m.flash = err.Error()
		return m, nil
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("explorer", dir)
	} else {
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		m.flash = err.Error()
		return m, nil
	}
	m.flash = "opened logs"
	return m, nil
}

func (m *model) onHomeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "r":
		m.flash = ""
		return m, tea.Batch(refreshStatus(m.root), checkLatest(m.root))
	case "c":
		return m.copyRealmlist()
	case "l":
		return m.openLogs()
	case "g":
		return m, checkLatest(m.root)
	case "up", "k":
		m.cursor = m.nextSelectable(-1)
		m.flash = ""
		return m, nil
	case "down", "j":
		m.cursor = m.nextSelectable(1)
		m.flash = ""
		return m, nil
	case "enter", " ":
		return m.activate()
	default:
		if i := menuIndexByKey(msg.String()); i >= 0 {
			m.cursor = i
			m.flash = ""
			return m.activate()
		}
		if len(msg.String()) == 1 && msg.String()[0] >= '1' && msg.String()[0] <= '9' {
			n := int(msg.String()[0] - '1')
			if n >= 0 && n < len(menu) {
				m.cursor = n
				m.flash = ""
				return m.activate()
			}
		}
	}
	return m, nil
}

func (m model) nextSelectable(delta int) int {
	if len(menu) == 0 {
		return 0
	}
	if delta == 0 {
		if menu[m.cursor].disabled(m.status) == "" {
			return m.cursor
		}
		delta = 1
	}
	for n := 0; n < len(menu); n++ {
		idx := (m.cursor + delta*(n+1) + len(menu)*2) % len(menu)
		if menu[idx].disabled(m.status) == "" {
			return idx
		}
	}
	return m.cursor
}

func (m *model) activate() (tea.Model, tea.Cmd) {
	a := menu[m.cursor]
	if reason := a.disabled(m.status); reason != "" {
		m.flash = reason
		return m, nil
	}

	switch a.Builtin {
	case "quit":
		m.quitting = true
		return m, tea.Quit
	case "settings":
		m.settings = newSettings(loadLocalEnv(m.root))
		m.screen = screenSettings
		m.layout()
		return m, m.settings.inputs[0].Focus()
	case "account":
		m.account = newAccountForm()
		m.screen = screenAccount
		m.layout()
		return m, m.account.inputs[0].Focus()
	}

	steps := m.planSteps(a)
	label := a.Title

	if a.ID == "fetch-maps" && m.status.HasMapsAll {
		m.screen = screenConfirm
		m.confirmTitle = a.Title
		m.confirmBody = "Maps are already in maps\\. Re-download and replace dbc/maps/vmaps/mmaps?"
		m.pending = steps
		m.pendingLabel = label
		return m, nil
	}

	if a.Confirm != "" {
		if a.ID == "setup" && !m.status.SetupComplete && !m.status.anythingRunning() {
			return m.beginRun(label, steps)
		}
		m.screen = screenConfirm
		m.confirmTitle = a.Title
		m.confirmBody = a.Confirm
		m.pending = steps
		m.pendingLabel = label
		return m, nil
	}

	if a.NeedsStop && m.status.anythingRunning() {
		m.screen = screenConfirm
		m.confirmTitle = a.Title
		m.confirmBody = "The realm is running. Stop it, then continue?"
		m.pending = steps
		m.pendingLabel = label
		return m, nil
	}

	return m.beginRun(label, steps)
}

func (s Status) anythingRunning() bool {
	return s.Mysqld || s.Realmd || s.Mangosd
}

func (m model) planSteps(a action) []scriptStep {
	var steps []scriptStep
	if a.NeedsStop && m.status.anythingRunning() && a.ID != "update" {
		// update.ps1 stops on its own
		steps = append(steps, scriptStep{Label: "Stop realm", Script: "stop.ps1"})
	}
	if a.NeedsMySQL && !m.status.Mysqld {
		steps = append(steps, scriptStep{Label: "Start MySQL", Script: "start-mysql.ps1"})
	}
	args := append([]string{}, a.Args...)
	if a.ID == "fetch-maps" && m.status.HasMapsAll {
		args = []string{"-Force"}
	}
	steps = append(steps, scriptStep{Label: a.Title, Script: a.Script, Args: args})
	return steps
}

func (m *model) beginRun(label string, steps []scriptStep) (tea.Model, tea.Cmd) {
	if len(steps) == 0 {
		return m, nil
	}
	m.screen = screenRun
	m.running = label
	m.queue = steps
	m.stepI = 0
	m.stepN = len(steps)
	if m.log == nil {
		m.log = &strings.Builder{}
	}
	m.log.Reset()
	m.lastErr = nil
	m.aborted = false
	m.vp.SetContent("")
	m.layout()
	return m, tea.Batch(m.spin.Tick, m.runNext())
}

func (m *model) runNext() tea.Cmd {
	if len(m.queue) == 0 {
		return nil
	}
	step := m.queue[0]
	m.queue = m.queue[1:]
	m.stepI++
	m.running = step.Label
	if m.stepN > 1 {
		m.log.WriteString(dimStyle.Render(fmt.Sprintf("— %d/%d %s —", m.stepI, m.stepN, step.Label)) + "\n")
		m.vp.SetContent(m.log.String())
		m.vp.GotoBottom()
	}
	return startStreamEnv(m.root, step.Script, step.Args, step.Env)
}

func (m *model) layout() {
	w := max(m.width-4, 20)
	h := max(m.height-8, 6)
	if m.screen == screenRun {
		h = max(m.height-7, 6)
	}
	if m.vp.Width == 0 && m.vp.Height == 0 {
		m.vp = viewport.New(w, h)
	} else {
		m.vp.Width = w
		m.vp.Height = h
	}
	m.vp.SetContent(m.log.String())
}

func splitWidths(total int) (left, right int) {
	if total < 78 {
		return total, total
	}
	left = 44
	if total > 110 {
		left = 48
	}
	right = total - left - 1
	if right < 32 {
		right = total / 2
		left = total - right - 1
	}
	return left, right
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	if !m.ready {
		return dimStyle.Render(" · loading")
	}

	switch m.screen {
	case screenRun:
		return m.runView()
	case screenSettings:
		return m.settingsView()
	case screenAccount:
		return m.accountView()
	case screenConfirm:
		return m.confirmView()
	default:
		return m.homeView()
	}
}

func (m model) homeView() string {
	header := renderHeader(m.status, m.width)
	hint := m.status.hintBanner(m.width)
	leftW, rightW := splitWidths(m.width)
	left := lipgloss.JoinVertical(lipgloss.Left, m.status.column(leftW), m.selectedActionHelp(leftW))

	headerH := lipgloss.Height(header)
	hintH := lipgloss.Height(hint)
	helpH := 1
	remain := m.height - headerH - hintH - helpH - 2
	if remain < 10 {
		remain = 10
	}

	var body string
	if m.width >= 78 {
		menuH := remain
		right := m.menuCard(rightW, menuH)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	} else {
		leftH := lipgloss.Height(left)
		menuH := remain - leftH
		if menuH < 8 {
			menuH = 8
		}
		right := m.menuCard(rightW, menuH)
		body = lipgloss.JoinVertical(lipgloss.Left, left, right)
	}

	help := helpStyle.Render("↑↓ enter  a account  c copy  s settings  u update  l logs  r refresh  q")
	if m.flash != "" {
		help = warnStyle.Render(m.flash)
	}
	if runtime.GOOS != "windows" {
		help += "\n" + faintStyle.Render("scripts expect Windows + PowerShell; pwsh is used here")
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, hint, body, help)
}

func (m model) selectedActionHelp(width int) string {
	if m.cursor < 0 || m.cursor >= len(menu) {
		return ""
	}
	a := menu[m.cursor]
	text := a.Desc
	border := colMoss
	bodyStyle := selDescStyle
	if reason := a.disabled(m.status); reason != "" {
		text = reason
		border = colWarn
		bodyStyle = warnStyle
	}
	inner := max(width-4, 8)
	body := bodyStyle.Width(inner).Render(text)
	return card(a.Title, body, width, border)
}

func (m model) menuCard(width, height int) string {
	innerW := max(width-4, 16)
	innerH := max(height-3, 6)
	body := m.renderMenu(innerW, innerH)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colGold).
		Padding(0, 1).
		Width(max(width-2, 14)).
		Height(max(height-2, 6)).
		Render(cardTitleStyle.Render("actions") + "\n" + body)
}

func (m model) renderMenu(width, height int) string {
	var lines []string
	cursorLine := 0
	lastGroup := ""
	for i, a := range menu {
		if a.Group != lastGroup {
			if lastGroup != "" {
				lines = append(lines, "")
			}
			label := strings.ToUpper(a.Group)
			pad := width - len(label) - 1
			head := label
			if pad > 2 {
				head = label + " " + strings.Repeat("─", pad)
			}
			lines = append(lines, faintStyle.Render(head))
			lastGroup = a.Group
		}
		disabled := a.disabled(m.status) != ""
		selected := i == m.cursor
		if selected {
			cursorLine = len(lines)
		}

		key := a.Key
		if key == "q" {
			key = ""
		}

		if selected {
			style := selRowStyle
			if a.Danger {
				style = selRowStyle.Foreground(colDanger)
			}
			row := spread("▸ "+a.Title, key, max(width, 8))
			lines = append(lines, style.Width(max(width, 8)).Render(row))
		} else {
			st := itemStyle
			if disabled {
				st = offStyle
			} else if a.Danger {
				st = dangerStyle
			}
			left := "  " + st.Render(a.Title)
			right := keyStyle.Render(key)
			lines = append(lines, spread(left, right, max(width, 8)))
		}
	}

	if len(lines) > height && height > 0 {
		start := cursorLine - height/3
		start = clamp(start, 0, len(lines)-height)
		lines = lines[start : start+height]
	}
	return strings.Join(lines, "\n")
}

func (m model) runView() string {
	header := renderHeader(m.status, m.width)
	title := m.running
	if m.lines != nil {
		title = m.spin.View() + " " + m.running
		if m.stepN > 1 {
			title += dimStyle.Render(fmt.Sprintf("  %d/%d", m.stepI, m.stepN))
		}
	}
	head := runningTitleStyle.Render(title)
	help := helpStyle.Render("running…  x abort  ctrl+c quit")
	if m.lines == nil {
		if m.lastErr != nil {
			help = helpStyle.Render("esc back  ·  failed")
		} else {
			help = helpStyle.Render("esc back  ·  q quit")
		}
	}

	used := lipgloss.Height(header) + lipgloss.Height(head) + lipgloss.Height(help) + 2
	vpH := max(m.height-used, 6)
	m.vp.Width = max(m.width-4, 16)
	m.vp.Height = vpH

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colBorder).
		Width(max(m.width-2, 20)).
		Render(m.vp.View())

	return lipgloss.JoinVertical(lipgloss.Left, header, head, box, help)
}

func (m model) settingsView() string {
	header := renderHeader(m.status, m.width)
	used := lipgloss.Height(header) + 2
	body := m.settings.View(m.width, max(m.height-used, 8))
	help := helpStyle.Render("↑↓ field  enter next/save  ctrl+s save  esc cancel")
	if m.settings.err != "" {
		help = errStyle.Render("error: " + m.settings.err)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, help)
}

func (m model) accountView() string {
	header := renderHeader(m.status, m.width)
	used := lipgloss.Height(header) + 2
	body := m.account.View(m.width, max(m.height-used, 8))
	help := helpStyle.Render("↑↓ field  enter next/create  ctrl+s create  esc cancel")
	if m.account.err != "" {
		help = errStyle.Render("error: " + m.account.err)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, help)
}

func (m model) confirmView() string {
	header := renderHeader(m.status, m.width)
	title := m.confirmTitle
	if title == "" {
		title = "confirm"
	}
	body := strings.TrimSpace(m.confirmBody)
	inner := runningTitleStyle.Render(title) + "\n\n" + textStyle.Width(min(56, max(m.width-10, 20))).Render(body) +
		"\n\n" + okStyle.Render("y") + dimStyle.Render(" yes    ") + badStyle.Render("n") + dimStyle.Render(" no")
	dlg := confirmBoxStyle.Width(min(64, max(m.width-4, 24))).Render(inner)
	help := helpStyle.Render("y confirm  ·  n / esc cancel")
	hh := lipgloss.Height(header)
	helpH := lipgloss.Height(help)
	placed := lipgloss.Place(m.width, max(m.height-hh-helpH, 8),
		lipgloss.Center, lipgloss.Center, dlg)
	return lipgloss.JoinVertical(lipgloss.Left, header, placed, help)
}

func main() {
	root, err := bootstrapRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	p := tea.NewProgram(newModel(root), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
