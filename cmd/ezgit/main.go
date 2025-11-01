package main

import (
	"context"
	"ezgit/internal/combos"
	"fmt"
	"os"
	"strconv"
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
	mode             string
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
	comboInputs      map[string]*textinput.Model
	comboOrder       []string
	comboFocusIndex  int
	advancedVisible  bool
	includedFlags    map[string]bool
	validationErrors map[string]string
	termWidth        int
	previewParams    []string
	previewSelected  int
	editingParamKey  string
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
	0: {
		"Create repo", "Clone repo", "Export snapshot", "Apply bundle", "Show history", "Show commit",
		"Search", "Blame", "Commits by author", "Describe",
	},
	1: {
		"Stage files", "Unstage/Restore", "Rename/Move file", "Commit", "Status", "Diff", "Clean workspace",
	},
	2: {
		"Create branch", "Switch branch", "Merge branch", "Rebase", "Tag release", "Manage worktrees",
	},
	3: {
		"Revert commit", "Reset (soft/mixed/hard)", "Reflog", "Bisect", "Cherry-pick", "Format patch", "Rewrite history",
	},
	4: {
		"Manage remote", "Fetch", "Pull", "Push", "Show remote refs", "Credentials", "Submodules",
	},
	5: {
		"Stash", "Apply stash", "Pop stash", "List stashes", "FSCK", "GC", "Prune", "Verify packs",
	},
}

func initialModel() *model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 512
	ti.Width = 60

	m := model{
		items: []string{
			"init", "clone", "add", "commit", "status", "push", "branch", "merge", "rebase",
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

		comboInputs:      make(map[string]*textinput.Model),
		includedFlags:    make(map[string]bool),
		validationErrors: make(map[string]string),

		termWidth: 80,
	}

	return &m
}

func (m model) Init() tea.Cmd { return nil }
func (m *model) validateFlagForKey(spec combos.CommandSpec, paramKey string) {
	if spec.Flags == nil {
		return
	}
	for _, f := range spec.Flags {
		if f.ParamKey != paramKey {
			continue
		}
		if m.validationErrors == nil {
			m.validationErrors = make(map[string]string)
		}
		delete(m.validationErrors, paramKey)

		if f.ManualOnly {
			if ti, ok := m.comboInputs[paramKey]; ok {
				v := strings.TrimSpace((*ti).Value())
				if f.Required && v == "" {
					m.validationErrors[paramKey] = "required"
					return
				}
				if f.Type == "int" && v != "" {
					if _, err := strconv.Atoi(v); err != nil {
						m.validationErrors[paramKey] = "must be integer"
						return
					}
					if f.Validate != nil {
						if minv, ok := f.Validate["min"]; ok {
							if minf, ok := minv.(float64); ok {
								iv, _ := strconv.Atoi(v)
								if iv < int(minf) {
									m.validationErrors[paramKey] = fmt.Sprintf("must be >= %v", int(minf))
									return
								}
							}
						}
					}
				}
			}
		}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		return m, nil
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
					if spec, ok := combos.Get(a.Name); ok {
						if m.comboInputs == nil {
							m.comboInputs = make(map[string]*textinput.Model)
						}
						if m.includedFlags == nil {
							m.includedFlags = make(map[string]bool)
						}
						for _, f := range spec.Flags {
							if f.ManualOnly {
								if _, exists := m.comboInputs[f.ParamKey]; !exists {
									ti := textinput.New()
									ti.Placeholder = f.Example
									ti.CharLimit = 512
									ti.Width = 36
									ti.Prompt = ""
									if f.Default != nil {
										ti.SetValue(fmt.Sprintf("%v", f.Default))
									}
									ti.Blur()
									m.comboInputs[f.ParamKey] = &ti
								}
							} else {
								if _, ok := m.includedFlags[f.ParamKey]; !ok {
									m.includedFlags[f.ParamKey] = false
								}
							}
						}
						m.comboOrder = nil
						for _, f := range spec.Flags {
							if f.ManualOnly {
								m.comboOrder = append(m.comboOrder, f.ParamKey)
							}
						}
						if len(m.comboOrder) > 0 {
							m.comboFocusIndex = 0
							if ti := m.comboInputs[m.comboOrder[0]]; ti != nil {
								(*ti).Focus()
							}
						} else {
							m.comboFocusIndex = -1
						}
						m.previewParams = nil
						for _, f := range spec.Flags {
							if f.Advanced && !m.advancedVisible {
								continue
							}
							m.previewParams = append(m.previewParams, f.ParamKey)
						}
						m.previewSelected = 0
						m.editingParamKey = ""
						m.mode = "preview"
						m.input.Blur()
					} else if len(a.Prompts) == 0 {
						m.mode = "preview"
						m.input.Blur()
					} else {
						m.mode = "wizard"
					}
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
					m.input.Blur()
					return m, cmd
				}
				p := m.currentAction.Prompts[m.promptIndex]
				m.wizardInputs[p.Key] = strings.TrimSpace(m.input.Value())
				m.input.SetValue("")
				m.promptIndex++
				if m.promptIndex >= len(m.currentAction.Prompts) {
					m.mode = "preview"
					m.input.Blur()
				}
			}
			if k == "esc" {
				m.mode = "verbs"
				return m, cmd
			}
			return m, cmd
		}

		if m.mode == "preview" {
			if spec, ok := combos.Get(m.currentAction.Name); ok {
				visible := make([]combos.FlagDef, 0, len(spec.Flags))
				for _, f := range spec.Flags {
					if f.Advanced && !m.advancedVisible {
						continue
					}
					visible = append(visible, f)
				}
				if len(visible) == 0 {
					return m, nil
				}
				if m.previewSelected < 0 {
					m.previewSelected = 0
				}
				if m.previewSelected >= len(visible) {
					m.previewSelected = len(visible) - 1
				}
				if m.editingParamKey == "" {
					kLower := strings.ToLower(k)
					switch kLower {
					case "up", "k":
						if m.previewSelected > 0 {
							m.previewSelected--
						}
						return m, nil
					case "down", "j":
						if m.previewSelected < len(visible)-1 {
							m.previewSelected++
						}
						return m, nil
					case " ", "space":

						visible := make([]combos.FlagDef, 0, len(spec.Flags))
						for _, f := range spec.Flags {
							if f.Advanced && !m.advancedVisible {
								continue
							}
							visible = append(visible, f)
						}
						if len(visible) == 0 {
							return m, nil
						}
						if m.previewSelected < 0 || m.previewSelected >= len(visible) {
							return m, nil
						}

						f := visible[m.previewSelected]
						if f.ManualOnly {

							if m.comboInputs == nil {
								m.comboInputs = make(map[string]*textinput.Model)
							}
							if _, exists := m.comboInputs[f.ParamKey]; !exists {
								t := textinput.New()
								t.Placeholder = f.Example
								t.CharLimit = 512
								t.Width = 36
								t.Prompt = ""
								if f.Default != nil {
									t.SetValue(fmt.Sprintf("%v", f.Default))
								}
								t.Blur()
								m.comboInputs[f.ParamKey] = &t
							}

							m.input.Blur()
							if ti := m.comboInputs[f.ParamKey]; ti != nil {
								(*ti).Focus()
							}

							m.editingParamKey = f.ParamKey
							return m, nil
						}

						if m.includedFlags == nil {
							m.includedFlags = map[string]bool{}
						}
						m.includedFlags[f.ParamKey] = !m.includedFlags[f.ParamKey]
						return m, nil
					case "e":
						if m.input.Focused() || m.editingParamKey != "" {
							return m, nil
						}
						idx := m.previewSelected
						if idx >= 0 && idx < len(visible) {
							f := visible[idx]
							if f.ManualOnly {
								if m.comboInputs == nil {
									m.comboInputs = make(map[string]*textinput.Model)
								}
								if _, exists := m.comboInputs[f.ParamKey]; !exists {
									t := textinput.New()
									t.Placeholder = f.Example
									t.CharLimit = 512
									t.Width = 36
									t.Prompt = ""
									if f.Default != nil {
										t.SetValue(fmt.Sprintf("%v", f.Default))
									}
									t.Blur()
									m.comboInputs[f.ParamKey] = &t
								}

								if ti := m.comboInputs[f.ParamKey]; ti != nil {
									(*ti).SetValue((*ti).Value())
									(*ti).Focus()
								}
								m.input.Blur()

								m.editingParamKey = f.ParamKey
								return m, nil
							}
						}
						return m, nil

					case "enter":

						return m.previewEnterHandler(spec)
					case "a":
						m.advancedVisible = !m.advancedVisible
						if m.previewSelected >= len(visible) {
							m.previewSelected = max(0, len(visible)-1)
						}
						return m, nil
					case "esc":
						m.mode = "verbs"
						return m, nil
					}
					return m, nil
				}

				var cmd tea.Cmd
				if m.editingParamKey != "" {
					if tiPtr, ok := m.comboInputs[m.editingParamKey]; ok && tiPtr != nil {

						t := *tiPtr
						t, cmd = t.Update(msg)
						*tiPtr = t
					} else {

						m.input, cmd = m.input.Update(msg)
					}

					kLower := strings.ToLower(k)
					if kLower == "enter" {

						if ti := m.comboInputs[m.editingParamKey]; ti != nil {
							v := strings.TrimSpace((*ti).Value())
							(*ti).SetValue(v)
							(*ti).Blur()
						}
						m.editingParamKey = ""

						return m, cmd
					}
					if kLower == "esc" {

						if ti := m.comboInputs[m.editingParamKey]; ti != nil {
							(*ti).Blur()
						}
						m.editingParamKey = ""
						return m, cmd
					}
					return m, cmd
				}

			}
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

		if m.mode == "preview-edit" {
			var cmd tea.Cmd

			if m.editingParamKey != "" {
				if tiPtr, ok := m.comboInputs[m.editingParamKey]; ok && tiPtr != nil {

					t := *tiPtr
					t, cmd = t.Update(msg)
					*tiPtr = t
				} else {

					m.input, cmd = m.input.Update(msg)
				}
			} else {

				m.input, cmd = m.input.Update(msg)
			}

			kLower := strings.ToLower(k)
			if kLower == "enter" {
				if m.editingParamKey != "" {

					if _, ok := m.comboInputs[m.editingParamKey]; !ok {
						t := textinput.New()
						t.Placeholder = ""
						m.comboInputs[m.editingParamKey] = &t
					}

					if ti := m.comboInputs[m.editingParamKey]; ti != nil {
						v := strings.TrimSpace((*ti).Value())
						(*ti).SetValue(v)
					}

					if ti := m.comboInputs[m.editingParamKey]; ti != nil {
						(*ti).Blur()
					}
				}
				m.editingParamKey = ""
				m.mode = "preview"
				return m, cmd
			}
			if kLower == "esc" {

				if m.editingParamKey != "" {
					if ti := m.comboInputs[m.editingParamKey]; ti != nil {
						(*ti).Blur()
					}
				}
				m.editingParamKey = ""
				m.mode = "preview"
				return m, cmd
			}
			return m, cmd
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
				m.input.Blur()
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
		m.input.Blur()
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

func (m *model) previewEnterHandler(spec combos.CommandSpec) (tea.Model, tea.Cmd) {
	m.validationErrors = map[string]string{}
	for _, f := range spec.Flags {
		if f.Advanced && !m.advancedVisible {
			continue
		}
		if f.ManualOnly {
			if ti, ok := m.comboInputs[f.ParamKey]; ok {
				val := strings.TrimSpace((*ti).Value())
				if f.Required && val == "" {
					m.validationErrors[f.ParamKey] = "required"
				} else if f.Type == "int" && val != "" {
					if _, err := strconv.Atoi(val); err != nil {
						m.validationErrors[f.ParamKey] = "must be integer"
					}
				}
			} else if f.Required {
				m.validationErrors[f.ParamKey] = "required"
			}
		}
	}
	if len(m.validationErrors) > 0 {
		return m, nil
	}

	m.wizardInputs = make(action.ActionInput)
	for _, f := range spec.Flags {
		if f.ManualOnly {
			if ti, ok := m.comboInputs[f.ParamKey]; ok {
				m.wizardInputs[f.ParamKey] = strings.TrimSpace((*ti).Value())
			}
			continue
		}

		if !m.includedFlags[f.ParamKey] {
			continue
		}

		isBool := false
		if strings.ToLower(f.Type) == "bool" {
			isBool = true
		}

		if f.Default != nil {
			switch dv := f.Default.(type) {
			case bool:
				isBool = true
			case float64:

				if dv == 0.0 || dv == 1.0 {
					isBool = true
				}
			}
		}

		if isBool {

			m.wizardInputs[f.ParamKey] = "true"
		} else {

			if f.Default != nil {
				m.wizardInputs[f.ParamKey] = fmt.Sprintf("%v", f.Default)
			} else {

				m.wizardInputs[f.ParamKey] = "true"
			}
		}
	}
	needTyped := false
	if m.currentAction.IsDestructive != nil && m.currentAction.IsDestructive(m.wizardInputs) {
		needTyped = true
	} else {
		for _, f := range spec.Flags {
			if f.Confirmation == "typed" || f.Confirmation == "always" {
				if f.ManualOnly {
					if v, ok := m.wizardInputs[f.ParamKey]; ok && strings.TrimSpace(fmt.Sprintf("%v", v)) != "" {
						needTyped = true
						break
					}
				} else if m.includedFlags[f.ParamKey] {
					needTyped = true
					break
				}
			}
		}
	}

	if needTyped {
		m.mode = "confirm"
		m.input.SetValue("")
		m.input.Placeholder = "type yes-I-mean-it to proceed"
		m.input.Focus()
		return m, nil
	}
	cmdName, args, _ := m.currentAction.Build(m.wizardInputs)
	cmd, cancel := runActionCmdWithCancel(cmdName, args)
	m.runCancel = cancel
	m.streamLines = nil
	m.mode = "running"
	m.currentRunCmd = cmd
	m.running = true
	return m, cmd
}

func (m *model) View() string {
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
		display := it
		if spec, ok := combos.Get(it); ok {
			if spec.Description != "" {
				display = spec.Description
			} else if spec.Name != "" {
				display = spec.Name
			}
		}

		if i == m.cursor {
			lines = append(lines, m.activeStyle.Render(fmt.Sprintf("> %s", display)))
		} else {
			lines = append(lines, m.itemStyle.Render(fmt.Sprintf("  %s", display)))
		}
	}
	input := ""
	if m.input.Focused() || m.input.Value() != "" {
		input = "\n\n" + lipgloss.NewStyle().Bold(true).Render("Advanced:") + "\n" + m.input.View()
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m model) renderPreview() string {
	term := m.termWidth
	if term <= 0 {
		term = 80
	}
	panelWidth := term * 40 / 100
	if panelWidth < 48 {
		panelWidth = 48
	}
	if panelWidth > 120 {
		panelWidth = 120
	}
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	labelStyle := lipgloss.NewStyle().Bold(true)
	valueStyle := lipgloss.NewStyle()
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	checkOn := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render("[x]")
	checkOff := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("[ ]")
	requiredBadge := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("⚠")
	advBadge := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("[adv]")

	lines := []string{headerStyle.Render("Preview"), ""}

	if m.currentAction == nil {
		lines = append(lines, "(no action selected)")
		return lipgloss.NewStyle().Width(panelWidth).Render(strings.Join(lines, "\n"))
	}

	spec, hasCombos := combos.Get(m.currentAction.Name)
	if !hasCombos {
		previews := m.currentAction.Preview(m.wizardInputs)
		for _, v := range previews {
			lines = append(lines, v)
		}
		if m.currentAction.IsDestructive != nil && m.currentAction.IsDestructive(m.wizardInputs) {
			lines = append(lines, "", "[This operation is DESTRUCTIVE. Press Enter → typed confirmation required]")
		} else {
			lines = append(lines, "", "[Press Enter to Run, Esc to go back]")
		}
		return lipgloss.NewStyle().Width(panelWidth).Render(strings.Join(lines, "\n"))
	}
	lines = append(lines, "Available flags & parameters:", "")
	visible := make([]combos.FlagDef, 0, len(spec.Flags))
	for _, f := range spec.Flags {
		if f.Advanced && !m.advancedVisible {
			continue
		}
		visible = append(visible, f)
	}
	maxLeft := 0
	for _, f := range visible {
		l := len(f.Key)
		if f.Label != "" && len(f.Label) > l {
			l = len(f.Label)
		}
		if l > maxLeft {
			maxLeft = l
		}
	}
	if maxLeft < 8 {
		maxLeft = 8
	}
	if maxLeft > 28 {
		maxLeft = 28
	}
	leftWidth := maxLeft + 2
	if len(m.previewParams) == 0 {
	}
	if m.previewSelected < 0 {
		m.previewSelected = 0
	}
	if m.previewSelected >= len(visible) && len(visible) > 0 {
		m.previewSelected = len(visible) - 1
	}
	for idx, f := range visible {
		selMark := "  "
		if idx == m.previewSelected {
			selMark = "➜ "
		}
		left := f.Key
		leftCol := fmt.Sprintf("%-*s", leftWidth, left)
		displayVal := ""
		if f.ManualOnly {

			if ti, ok := m.comboInputs[f.ParamKey]; ok {
				displayVal = strings.TrimSpace((*ti).Value())

				if displayVal == "" && f.Default != nil {

					switch f.Default.(type) {
					case bool:

						if f.Label != "" {
							displayVal = f.Label
						} else {
							displayVal = "<unset>"
						}
					default:
						displayVal = fmt.Sprintf("%v", f.Default)
					}
				}
			} else if f.Default != nil {

				switch f.Default.(type) {
				case bool:
					if f.Label != "" {
						displayVal = f.Label
					} else {
						displayVal = "<unset>"
					}
				default:
					displayVal = fmt.Sprintf("%v", f.Default)
				}
			} else {
				displayVal = "<unset>"
			}
		} else {
			if f.Default != nil {
				displayVal = fmt.Sprintf("%v", f.Default)
			} else {
				displayVal = f.Label
			}
		}
		rightParts := []string{}
		if f.ManualOnly {
			rightParts = append(rightParts, valueStyle.Render(displayVal), "✎")
		} else {

			if m.includedFlags[f.ParamKey] {
				rightParts = append(rightParts, checkOn)
			} else {
				rightParts = append(rightParts, checkOff)
			}

			displayLabel := ""
			isBool := strings.ToLower(f.Type) == "bool"
			if !isBool && f.Default != nil {
				displayLabel = fmt.Sprintf("%v", f.Default)
			} else if f.Label != "" {
				displayLabel = f.Label
			}
			if displayLabel != "" {
				rightParts = append(rightParts, valueStyle.Render(displayLabel))
			}
		}
		if f.Advanced {
			rightParts = append(rightParts, advBadge)
		}
		if f.Required {
			rightParts = append(rightParts, requiredBadge)
		}
		right := strings.Join(rightParts, " ")

		line := fmt.Sprintf("%s %s %s", selMark, labelStyle.Render(leftCol), right)
		if idx == m.previewSelected {
			lines = append(lines, m.activeStyle.Render(line))
		} else {
			lines = append(lines, m.itemStyle.Render(line))
		}

		if m.mode == "preview-edit" && m.editingParamKey == f.ParamKey && m.input.Prompt != "" {

			if ti := m.comboInputs[f.ParamKey]; ti != nil {
				lines = append(lines, "    "+(*ti).View())
			} else {
				if ti := m.comboInputs[f.ParamKey]; ti != nil {
					lines = append(lines, "    "+(*ti).View())
				} else {
					lines = append(lines, "    "+m.input.View())
				}
			}

			if errMsg, ok := m.validationErrors[m.editingParamKey]; ok && errMsg != "" {
				lines = append(lines, "    "+errStyle.Render("Error: "+errMsg))
			}
		} else if m.mode == "preview-edit" && m.editingParamKey == f.ParamKey {

			if ti := m.comboInputs[f.ParamKey]; ti != nil {
				lines = append(lines, "    "+(*ti).View())
			} else {
				if ti := m.comboInputs[f.ParamKey]; ti != nil {
					lines = append(lines, "    "+(*ti).View())
				} else {
					lines = append(lines, "    "+m.input.View())
				}
			}
			if errMsg, ok := m.validationErrors[m.editingParamKey]; ok && errMsg != "" {
				lines = append(lines, "    "+errStyle.Render("Error: "+errMsg))
			}
		}

	}
	hasAdvanced := false
	for _, f := range spec.Flags {
		if f.Advanced {
			hasAdvanced = true
			break
		}
	}
	if hasAdvanced {
		if m.advancedVisible {
			lines = append(lines, "", lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("[a] Hide advanced options"))
		} else {
			lines = append(lines, "", lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("[a] Show advanced options"))
		}
	}
	help := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("[↑/↓] select • [space] toggle (read-only) • [e/enter] edit • [a] adv • [esc] back")
	lines = append(lines, "", help)
	if m.editingParamKey != "" {
		if ti := m.comboInputs[m.editingParamKey]; ti != nil {
			lines = append(lines, "", lipgloss.NewStyle().Bold(true).Render("Edit value:"), (*ti).View())
		} else {
			lines = append(lines, "", lipgloss.NewStyle().Bold(true).Render("Edit value:"), m.input.View())
		}
		if errMsg, ok := m.validationErrors[m.editingParamKey]; ok && errMsg != "" {
			lines = append(lines, "    "+errStyle.Render("Error: "+errMsg))
		}
	}

	previewParts := []string{"git", m.currentAction.Name}
	for _, f := range visible {
		if f.ManualOnly {
			if ti, ok := m.comboInputs[f.ParamKey]; ok {
				v := strings.TrimSpace((*ti).Value())
				if v != "" {
					if strings.HasPrefix(f.Key, "-") {
						previewParts = append(previewParts, f.Key, v)
					} else {
						previewParts = append(previewParts, v)
					}
				} else if f.Default != nil {
					if strings.HasPrefix(f.Key, "-") {
						previewParts = append(previewParts, f.Key, fmt.Sprintf("%v", f.Default))
					} else {
						previewParts = append(previewParts, fmt.Sprintf("%v", f.Default))
					}
				}
			}
		} else {
			if !m.includedFlags[f.ParamKey] {

				continue
			}

			isBool := false
			if strings.ToLower(f.Type) == "bool" {
				isBool = true
			} else if f.Default != nil {
				switch f.Default.(type) {
				case bool:
					isBool = true
				}
			}

			if isBool {

				previewParts = append(previewParts, f.Key)
				continue
			}

			if f.Default != nil {
				if strings.HasPrefix(f.Key, "-") {
					previewParts = append(previewParts, f.Key, fmt.Sprintf("%v", f.Default))
				} else {
					previewParts = append(previewParts, fmt.Sprintf("%v", f.Default))
				}
			} else if f.ParamKey != "" {

				if strings.HasPrefix(f.Key, "-") {
					previewParts = append(previewParts, f.Key)
				} else {
					previewParts = append(previewParts, f.Key)
				}
			} else {

				previewParts = append(previewParts, f.Key)
			}
		}
	}
	maxLine := panelWidth - 6
	cur := ""
	previewLines := []string{}
	for i, p := range previewParts {
		add := p
		if i > 0 {
			add = " " + p
		}
		if len(cur)+len(add) > maxLine {
			if cur != "" {
				previewLines = append(previewLines, strings.TrimSpace(cur))
			}
			cur = p
		} else {
			cur += add
		}
	}
	if strings.TrimSpace(cur) != "" {
		previewLines = append(previewLines, strings.TrimSpace(cur))
	}

	lines = append(lines, "")
	lines = append(lines, "Preview:")
	for _, pl := range previewLines {
		lines = append(lines, "  "+pl)
	}

	return lipgloss.NewStyle().Width(panelWidth).Render(strings.Join(lines, "\n"))
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
	if dir, err := os.Getwd(); err == nil {
		fmt.Println("Working dir:", dir)
	} else {
		fmt.Println("Warning: failed to get working dir:", err)
	}

	const combosPath = "combos_updated.json"
	if doc, err := combos.LoadFromFile(combosPath); err == nil {
		combos.Register(doc)
		fmt.Println("Loaded combos_updated.json: enhanced Layer-3 preview enabled")
		fmt.Println("---- combos: verifying action_key -> registered action map ----")
		for _, c := range doc.Commands {
			fmt.Printf("combo available for action_key=%q\n", c.ActionKey)
		}
		fmt.Println("---- end combos verification ----")
	} else {
		fmt.Printf("combos: failed to load %s: %v\n", combosPath, err)
		if doc2, err2 := combos.LoadFromFile("combos.json"); err2 == nil {
			combos.Register(doc2)
			fmt.Println("Loaded combos.json: enhanced Layer-3 preview enabled")
		} else {
			fmt.Printf("combos: failed to load fallback combos.json: %v\n", err2)
			fmt.Println("combos: continuing without combos metadata (no UI change).")
		}
	}
	action.RegisterBuiltins(action.DefaultRegistry)

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
