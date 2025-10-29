package tui

import (
	"bufio"
	"fmt"
	"strings"

	"ezgit/internal/action"
)

func BasicPrompter(a *action.ActionDef, r *bufio.Reader) (action.ActionInput, error) {
	inputs := action.ActionInput{}
	for _, p := range a.Prompts {
		def := p.Default
		label := p.Label
		if label == "" {
			label = p.Key
		}
		fmt.Printf("%s [%s]: ", label, def)
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			line = def
		}
		inputs[p.Key] = line
	}
	return inputs, nil
}

func TypedConfirmation(r *bufio.Reader, cmd string, args []string) bool {
	prompt := "Type 'yes-I-mean-it' to confirm: "
	fmt.Printf("%s\n%s", prompt, "")
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	return line == "yes-I-mean-it"
}
