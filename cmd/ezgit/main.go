package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"ezgit/internal/action"
	"ezgit/internal/audit"
	execpkg "ezgit/internal/exec"
	"ezgit/internal/windows"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	focusList = iota
	focusInput
	focusRun
)

type model struct {
	items            []string
	cursor           int
	selectedCategory int
	currentCategory  int
	mode             string // "home","verbs","wizard","preview","confirm","running"
	quitting         bool
	currentAction    *action.ActionDef
	wizardInputs     action.ActionInput
	promptIndex      int
	statusLines      []string
	streamLines      []string
	scroll           int
	running          bool
	runCancel        context.CancelFunc
	input            textinput.Model
	headStyle        lipgloss.Style
	itemStyle        lipgloss.Style
	activeStyle      lipgloss.Style
	footerStyle      lipgloss.Style
	panelStyle       lipgloss.Style
	currentRunCmd    tea.Cmd
}

type streamLineMsg struct {
	Line  string
	IsErr bool
}
type actionDoneMsg struct {
	Exit   int
	Out    string
	ErrOut string
	Err    error
}

type Category struct {
	ID    int
	Title string
	Help  string
}

var registry *action.Registry

func actionRegistryGet(name string) (*action.ActionDef, bool) {
	return action.DefaultRegistry.Get(name)
}

var categories = []Category{
	{ID: 0, Title: "Repository"},
	{ID: 1, Title: "Work on changes"},
	{ID: 2, Title: "Branching & merging"},
	{ID: 3, Title: "History & fixes"},
	{ID: 4, Title: "Remotes & Collaboration"},
	{ID: 5, Title: "Maintenance"},
}

var itemsByCategory = map[int][]string{
	0: { // Repository
		"Create repo", "Clone repo", "Export snapshot", "Apply bundle", "Show history", "Show commit",
		"Search", "Blame", "Commits by author", "Describe",
	},
	1: { // Work on changes
		"Stage files", "Unstage/Restore", "Rename/Move file", "Commit", "Status", "Diff", "Clean workspace",
	},
	2: { // Branching & merging
		"Create branch", "Switch branch", "Merge branch", "Rebase", "Tag release", "Manage worktrees",
	},
	3: { // History & fixes
		"Revert commit", "Reset (soft/mixed/hard)", "Reflog", "Bisect", "Cherry-pick", "Format patch", "Rewrite history",
	},
	4: { // Remotes & Collaboration
		"Manage remote", "Fetch", "Pull", "Push", "Show remote refs", "Credentials", "Submodules",
	},
	5: { // Maintenance
		"Stash", "Apply stash", "Pop stash", "List stashes", "FSCK", "GC", "Prune", "Verify packs",
	},
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 512
	ti.Width = 60

	return model{
		items: []string{
			"init",
			"clone",
			"add",
			"commit",
			"status",
			"push",
			"branch",
			"merge",
			"rebase",
			"raw",
		},
		cursor:           0,
		selectedCategory: 0,
		currentCategory:  0,
		mode:             "home",
		input:            ti,
		wizardInputs:     make(action.ActionInput),

		headStyle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")),
		itemStyle:   lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("250")),
		activeStyle: lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("39")).Background(lipgloss.Color("236")).Bold(true),
		footerStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		panelStyle:  lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1),
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		k := msg.String()

		if k == "q" || k == "ctrl+c" {
			if m.runCancel != nil && m.mode == "running" {
				m.runCancel()
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit
		}

		if m.mode == "running" {
			switch k {
			case "c", "ctrl+c":
				if m.runCancel != nil {
					m.runCancel()
					m.statusLines = append(m.statusLines, "[cancelling running command]")
				}
				return m, nil
			case "pgup":
				if m.scroll > 0 {
					m.scroll -= 10
					if m.scroll < 0 {
						m.scroll = 0
					}
				}
				return m, nil
			case "pgdown":
				m.scroll += 10
				return m, nil
			}
		}

		if m.mode == "home" {
			switch k {
			case "up", "k":
				if m.selectedCategory > 0 {
					m.selectedCategory--
				}
				return m, nil
			case "down", "j":
				if m.selectedCategory < len(categories)-1 {
					m.selectedCategory++
				}
				return m, nil
			case "enter":
				m.mode = "verbs"
				m.cursor = 0
				actions := action.DefaultRegistry.List()
				var list []string
				for _, a := range actions {
					if a.Category == m.selectedCategory {
						list = append(list, a.Name)
					}
				}
				if len(list) == 0 {
					for _, a := range actions {
						list = append(list, a.Name)
					}
				}
				m.items = list
				return m, nil
			case "esc":
				return m, nil
			}
		}

		if m.mode == "verbs" {
			switch k {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
				return m, nil
			case "down", "j":
				if m.cursor < len(m.items)-1 {
					m.cursor++
				}
				return m, nil
			case "enter":
				name := m.items[m.cursor]
				if a, ok := actionRegistryGet(name); ok {
					m.currentAction = a
					m.wizardInputs = make(action.ActionInput)
					m.promptIndex = 0
					m.input.SetValue("")
					m.mode = "wizard"
				} else {
					a := &action.ActionDef{
						Name: name,
						Prompts: []action.Prompt{
							{
								Key:      "extra",
								Label:    "Extra args (optional)",
								Default:  "",
								Required: false,
							},
						},
						BuildFunc: func(inputs action.ActionInput) (string, []string, string) {
							args := []string{name}
							if v, ok := inputs["extra"]; ok {
								v = strings.TrimSpace(v)
								if v != "" {
									fields := strings.Fields(v)
									args = append(args, fields...)
								}
							}
							preview := "git " + strings.Join(args, " ")
							return "git", args, preview
						},
						IsDestructive: func(inputs action.ActionInput) bool {
							return false
						},
					}
					m.currentAction = a
					m.wizardInputs = make(action.ActionInput)
					m.promptIndex = 0
					m.input.SetValue("")
					m.input.Placeholder = "optional args (e.g. -a --force)"
					m.mode = "wizard"
				}
				return m, nil
			case "esc":
				m.mode = "home"
				m.items = nil
				m.cursor = 0
				return m, nil
			default:
				if isPrintableKey(k) {
					m.input.Focus()
					v := m.input.Value()
					m.input.SetValue(v + msg.String())
					return m, nil
				}
				return m, nil
			}
		}

		if m.mode == "wizard" {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			if k == "enter" {
				if m.currentAction == nil || m.promptIndex >= len(m.currentAction.Prompts) {
					m.mode = "preview"
					return m, cmd
				}
				p := m.currentAction.Prompts[m.promptIndex]
				m.wizardInputs[p.Key] = strings.TrimSpace(m.input.Value())
				m.input.SetValue("")
				m.promptIndex++
				if m.promptIndex >= len(m.currentAction.Prompts) {
					m.mode = "preview"
				}
			}
			if k == "esc" {
				m.mode = "verbs"
				return m, cmd
			}
			return m, cmd
		}

		if m.mode == "preview" {
			switch k {
			case "enter":
				if m.currentAction != nil && m.currentAction.IsDestructive != nil && m.currentAction.IsDestructive(m.wizardInputs) {
					m.mode = "confirm"
					m.input.SetValue("")
					m.input.Placeholder = "type yes-I-mean-it to proceed"
					m.input.Focus()
					return m, nil
				}
				if m.currentAction != nil {
					cmdName, args, _ := m.currentAction.Build(m.wizardInputs)
					cmd, cancel := runActionCmdWithCancel(cmdName, args)
					m.runCancel = cancel
					m.streamLines = nil
					m.mode = "running"
					m.runCancel = cancel
					m.currentRunCmd = cmd
					m.running = true
					return m, cmd
				}
				return m, nil
			case "esc":
				m.mode = "wizard"
				m.promptIndex = intMax(0, len(m.currentAction.Prompts)-1)
				return m, nil
			}
		}

		if m.mode == "confirm" {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			if k == "enter" {
				if strings.TrimSpace(m.input.Value()) == "yes-I-mean-it" {
					backup := "preop/" + time.Now().Format("20060102-150405")
					_, _, _, _ = (&execpkg.Runner{}).Run(context.Background(), "git", []string{"branch", backup}, nil, 0)

					cmdName, args, _ := m.currentAction.Build(m.wizardInputs)
					cmdRun, cancel := runActionCmdWithCancel(cmdName, args)
					m.runCancel = cancel
					m.streamLines = nil
					m.mode = "running"
					m.currentRunCmd = cmdRun
					m.running = true
					return m, cmdRun
				}
				m.statusLines = append(m.statusLines, "[typed confirmation failed; aborting]")
				m.mode = "preview"
				return m, nil
			}
			return m, cmd
		}

		if m.mode == "verbs" && m.input.Focused() {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			if k == "enter" {
				raw := strings.TrimSpace(m.input.Value())
				if strings.TrimSpace(m.input.Value()) != "" {
					fields := strings.Fields(raw)
					if fields[0] == "git" {
						fields = fields[1:]
					}
					cmdRun, cancel := runActionCmdWithCancel("git", fields)
					m.runCancel = cancel
					m.streamLines = nil
					m.mode = "running"
					m.currentRunCmd = cmdRun
					m.running = true
					return m, cmdRun
				}
			}
			return m, cmd
		}

	case streamLineMsg:
		m.streamLines = append(m.streamLines, msg.Line)
		if m.currentRunCmd != nil {
			return m, m.currentRunCmd
		}
		return m, nil

	case actionDoneMsg:
		m.running = false
		m.mode = "preview"
		m.currentRunCmd = nil
		m.runCancel = nil
		if strings.TrimSpace(msg.Out) != "" {
			m.statusLines = append(m.statusLines, strings.Split(msg.Out, "\n")...)
		}
		if strings.TrimSpace(msg.ErrOut) != "" {
			m.statusLines = append(m.statusLines, strings.Split(msg.ErrOut, "\n")...)
		}
		if msg.Err != nil {
			m.statusLines = append(m.statusLines, fmt.Sprintf("[process finished with error: %v]", msg.Err))
		} else {
			m.statusLines = append(m.statusLines, "[process finished successfully]")
		}
		_ = audit.AppendAudit(true, audit.Entry{
			Timestamp: time.Now(),
			Action: func() string {
				if m.currentAction != nil {
					return m.currentAction.Name
				}
				return ""
			}(),
			Command: "", Args: nil,
			ExitCode: msg.Exit, Stdout: msg.Out, Stderr: msg.ErrOut,
		})
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	head := m.headStyle.Render("EzGit by 0xrootAnon")
	help := m.footerStyle.Render("Arrows: move • Enter: select • Type: start typing • Esc: back • q: quit • PgUp/PgDn: scroll output")

	var left string
	switch m.mode {
	case "home":
		left = m.renderCategoriesBox()
	case "verbs":
		left = m.renderVerbsPane()
	case "wizard":
		left = m.renderWizard()
	case "preview":
		left = m.renderPreview()
	case "confirm":
		left = m.renderConfirm()
	default:
		left = m.renderCategoriesBox()
	}

	outputBox := m.renderOutputWithStream()
	main := lipgloss.JoinHorizontal(lipgloss.Top, m.panelStyle.Render(left), lipgloss.NewStyle().PaddingLeft(1).Render(outputBox))

	return lipgloss.JoinVertical(lipgloss.Left, head, main, "", help)
}

func (m model) renderVerbsPane() string {
	lines := []string{}
	for i, it := range m.items {
		if i == m.cursor {
			lines = append(lines, m.activeStyle.Render(fmt.Sprintf("> %s", it)))
		} else {
			lines = append(lines, m.itemStyle.Render(fmt.Sprintf("  %s", it)))
		}
	}
	input := ""
	if m.input.Focused() || m.input.Value() != "" {
		input = "\n\n" + lipgloss.NewStyle().Bold(true).Render("Advanced / Raw:") + "\n" + m.input.View()
	}
	body := lipgloss.JoinVertical(lipgloss.Left, strings.Join(lines, "\n"), input)
	return lipgloss.NewStyle().Width(40).Render(body)
}

func (m model) renderWizard() string {
	if m.currentAction == nil {
		return lipgloss.NewStyle().Render("(no action selected)")
	}
	if len(m.currentAction.Prompts) == 0 {
		return lipgloss.NewStyle().Render("(no prompts for this action)")
	}
	p := m.currentAction.Prompts[m.promptIndex]
	hdr := lipgloss.NewStyle().Bold(true).Render("Prompt")
	label := p.Label
	def := ""
	if p.Default != "" {
		def = fmt.Sprintf(" (default: %s)", p.Default)
	}
	tempInputs := make(action.ActionInput)
	for k, v := range m.wizardInputs {
		tempInputs[k] = v
	}
	if strings.TrimSpace(m.input.Value()) != "" {
		tempInputs[p.Key] = strings.TrimSpace(m.input.Value())
	}

	previewLines := []string{}
	if m.currentAction != nil {
		previewLines = m.currentAction.Preview(tempInputs)
	}

	previewHdr := lipgloss.NewStyle().Bold(true).Render("Preview")
	previewText := "(no preview)"
	if len(previewLines) > 0 {
		previewText = strings.Join(previewLines, "\n")
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		hdr,
		label+def,
		m.input.View(),
		"",
		previewHdr,
		previewText,
	)
}

func (m model) renderPreview() string {
	hdr := lipgloss.NewStyle().Bold(true).Render("Preview")
	lines := []string{hdr}
	if m.currentAction != nil {
		previews := m.currentAction.Preview(m.wizardInputs)
		for _, v := range previews {
			lines = append(lines, v)
		}
		if m.currentAction.IsDestructive != nil && m.currentAction.IsDestructive(m.wizardInputs) {
			lines = append(lines, "", "[This operation is DESTRUCTIVE. Press Enter → typed confirmation required]")
		} else {
			lines = append(lines, "", "[Press Enter to Run, Esc to go back]")
		}
	}
	return lipgloss.NewStyle().Width(40).Render(strings.Join(lines, "\n"))
}

func (m model) renderConfirm() string {
	hdr := lipgloss.NewStyle().Bold(true).Render("Confirm (type yes-I-mean-it)")
	return lipgloss.JoinVertical(lipgloss.Left, hdr, m.input.View())
}

func (m model) renderOutputWithStream() string {
	head := lipgloss.NewStyle().Bold(true).Render("Output")
	var content string
	if len(m.streamLines) > 0 {
		content = strings.Join(m.streamLines, "\n")
	} else {
		content = strings.Join(m.statusLines, "\n")
	}
	if strings.TrimSpace(content) == "" {
		content = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("(no output yet)")
	}
	return lipgloss.NewStyle().Width(80).Render(lipgloss.JoinVertical(lipgloss.Left, head, content))
}

func (m model) renderCategoriesBox() string {
	lines := []string{}
	for i, c := range categories {
		if i == m.selectedCategory {
			lines = append(lines, m.activeStyle.Render(fmt.Sprintf("> %s ", c.Title)))
		} else {
			lines = append(lines, m.itemStyle.Render(fmt.Sprintf("  %s ", c.Title)))
		}
	}
	return lipgloss.NewStyle().Width(40).Render(strings.Join(lines, "\n"))
}

func isPrintableKey(k string) bool {
	if len(k) == 1 {
		r := k[0]
		if r >= 32 && r <= 126 {
			return true
		}
	}
	return false
}

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func runActionCmdWithCancel(cmdName string, args []string) (tea.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	lineCh := make(chan streamLineMsg, 512)
	doneCh := make(chan actionDoneMsg, 1)

	go func() {
		runner := &execpkg.Runner{}
		exit, out, errOut, err := runner.Run(ctx, cmdName, args, func(line string, isErr bool) {
			select {
			case lineCh <- streamLineMsg{Line: line, IsErr: isErr}:
			default:
			}
		}, 0)

		doneCh <- actionDoneMsg{Exit: exit, Out: out, ErrOut: errOut, Err: err}
		close(lineCh)
	}()

	cmd := func() tea.Msg {
		select {
		case l, ok := <-lineCh:
			if !ok {
				return <-doneCh
			}
			return l
		case d := <-doneCh:
			return d
		}
	}

	return cmd, cancel
}

func (m *model) focusHandleInputStart() {
	m.input.Focus()
}

func main() {
	path := windows.DetectGit()
	if path == "" {
		fmt.Print("Git not found on this machine. Open download page? (y/N): ")
		var resp string
		fmt.Scanln(&resp)
		if strings.ToLower(strings.TrimSpace(resp)) == "y" {
			_ = windows.OpenBrowser(windows.OpenDownloadURL())
		}
	}
	action.RegisterBuiltins(action.DefaultRegistry)

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
