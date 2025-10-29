package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

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
	items       []string
	cursor      int
	quitting    bool
	statusLines []string
	input       textinput.Model
	focus       int
	running     bool
	scroll      int

	headStyle   lipgloss.Style
	itemStyle   lipgloss.Style
	activeStyle lipgloss.Style
	footerStyle lipgloss.Style
	panelStyle  lipgloss.Style
}

type doneMsg struct {
	err    error
	output string
}

func initialModel() model {
	items := []string{
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
	}

	ti := textinput.New()
	ti.Placeholder = "type git command here (eg: status | commit -m \"msg\")"
	ti.CharLimit = 512
	ti.Width = 60

	return model{
		items:       items,
		input:       ti,
		focus:       focusList,
		headStyle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")),
		itemStyle:   lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("250")),
		activeStyle: lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("39")).Background(lipgloss.Color("236")).Bold(true),
		footerStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		panelStyle:  lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		k := msg.String()
		if k == "q" || k == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		if k == "esc" {
			m.focus = focusList
			m.input.Blur()
			return m, nil
		}
		if m.focus == focusList {
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
				sel := m.items[m.cursor]
				m.input.SetValue(sel)
				m.input.CursorEnd()
				m.focus = focusInput
				m.input.Focus()
				return m, nil
			default:
				if isPrintableKey(k) {
					m.focus = focusInput
					m.input.Focus()
					v := m.input.Value()
					m.input.SetValue(v + msg.String())
				}
				return m, nil
			}
		}

		if m.focus == focusInput {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			if k == "enter" && strings.TrimSpace(m.input.Value()) != "" && !m.running {
				m.running = true
				cmdStr := m.input.Value()
				return m, tea.Batch(runGitCmd(cmdStr), cmd)
			}
			return m, cmd
		}

		if m.focus == focusRun {
			switch k {
			case "pgup":
				if m.scroll > 0 {
					m.scroll -= 10
					if m.scroll < 0 {
						m.scroll = 0
					}
				}
			case "pgdown":
				m.scroll += 10
			}
		}

	case doneMsg:
		m.running = false
		if msg.output != "" {
			lines := strings.Split(msg.output, "\n")
			m.statusLines = append(m.statusLines, lines...)
		}
		if msg.err != nil {
			m.statusLines = append(m.statusLines, fmt.Sprintf("\n[process finished with error: %v]", msg.err))
		} else {
			m.statusLines = append(m.statusLines, "\n[process finished successfully]")
		}
		m.focus = focusRun
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	head := m.headStyle.Render("EzGit by 0xrootAnon")
	help := m.footerStyle.Render("Arrows: move • Enter: select/prefill • Type: start typing • Esc: back • q: quit • PgUp/PgDn: scroll output")

	listBox := m.renderList()
	inputBox := m.renderInput()
	outputBox := m.renderOutput()

	left := lipgloss.JoinVertical(lipgloss.Left, listBox, "", inputBox)
	right := outputBox

	main := lipgloss.JoinHorizontal(lipgloss.Top, m.panelStyle.Render(left), lipgloss.NewStyle().PaddingLeft(1).Render(right))

	return lipgloss.JoinVertical(lipgloss.Left, head, main, "", help)
}

func (m model) renderList() string {
	lines := []string{}
	for i, it := range m.items {
		if i == m.cursor {
			lines = append(lines, m.activeStyle.Render(fmt.Sprintf("> %s", it)))
		} else {
			lines = append(lines, m.itemStyle.Render(fmt.Sprintf("  %s", it)))
		}
	}
	return lipgloss.NewStyle().Width(30).Render(strings.Join(lines, "\n"))
}

func (m model) renderInput() string {
	hdr := lipgloss.NewStyle().Bold(true).Render("Command")

	val := m.input.View()
	if m.focus == focusInput {
		val = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(val)
	}
	return lipgloss.JoinVertical(lipgloss.Left, hdr, val)
}

func (m model) renderOutput() string {
	head := lipgloss.NewStyle().Bold(true).Render("Output")
	content := strings.Join(m.statusLines, "\n")
	lines := strings.Split(content, "\n")
	start := m.scroll
	end := start + 20
	if start > len(lines) {
		start = len(lines)
	}
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[start:end]
	body := strings.Join(visible, "\n")
	if body == "" {
		body = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("(no output yet)")
	}
	return lipgloss.NewStyle().Width(80).Render(lipgloss.JoinVertical(lipgloss.Left, head, body))
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

func runGitCmd(raw string) tea.Cmd {
	return func() tea.Msg {
		fields := strings.Fields(raw)
		if len(fields) == 0 {
			return doneMsg{err: fmt.Errorf("empty command")}
		}
		if fields[0] == "git" {
			fields = fields[1:]
		}
		cmd := exec.Command("git", fields...)
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf

		err := cmd.Run()
		combined := strings.TrimSpace(outBuf.String() + "\n" + errBuf.String())
		return doneMsg{err: err, output: combined}
	}
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
