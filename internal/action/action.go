package action

import (
	"errors"
	"fmt"
	"strings"
)

type ActionInput map[string]string

type ActionDef struct {
	Name          string
	Help          string
	Prompts       []Prompt
	BuildFunc     func(ActionInput) (cmd string, args []string, preview string)
	ValidateFunc  func(ActionInput) error
	IsDestructive func(ActionInput) bool
}

type Prompt struct {
	Key         string
	Label       string
	Default     string
	Placeholder string
	Required    bool
}

func (p Prompt) Ask(inputs ActionInput) string {
	if v, ok := inputs[p.Key]; ok {
		return v
	}
	return p.Default
}

func (a *ActionDef) Validate(inputs ActionInput) error {
	if a.ValidateFunc != nil {
		return a.ValidateFunc(inputs)
	}
	for _, p := range a.Prompts {
		if p.Required {
			v := strings.TrimSpace(inputs[p.Key])
			if v == "" {
				return fmt.Errorf("missing required input: %s", p.Key)
			}
		}
	}
	return nil
}

func (a *ActionDef) Preview(inputs ActionInput) []string {
	var previews []string

	if a.BuildFunc != nil {
		cmd, args, preview := a.Build(inputs)
		if preview != "" {
			previews = append(previews, preview)
		} else if cmd != "" {
			previews = append(previews, fmt.Sprintf("%s %s", cmd, strings.Join(args, " ")))
		}
		return previews
	}

	if strings.TrimSpace(a.Help) != "" {
		previews = append(previews, a.Help)
	}
	return previews
}

func (a *ActionDef) Build(inputs ActionInput) (string, []string, string) {
	if a.BuildFunc != nil {
		return a.BuildFunc(inputs)
	}
	return "", nil, ""
}

type Registry struct {
	actions map[string]*ActionDef
}

func NewRegistry() *Registry {
	return &Registry{actions: make(map[string]*ActionDef)}
}

func (r *Registry) Register(a *ActionDef) error {
	if a == nil || a.Name == "" {
		return errors.New("invalid action")
	}
	r.actions[a.Name] = a
	return nil
}

func (r *Registry) Get(name string) (*ActionDef, bool) {
	a, ok := r.actions[name]
	return a, ok
}

func (r *Registry) List() []*ActionDef {
	res := make([]*ActionDef, 0, len(r.actions))
	for _, v := range r.actions {
		res = append(res, v)
	}
	return res
}

type AuditEntry struct {
	Timestamp   any
	Command     string
	Args        []string
	Stdout      string
	Stderr      string
	ExitCode    int
	Explanation string
}
