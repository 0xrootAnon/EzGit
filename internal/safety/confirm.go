package safety

import (
	"fmt"
	"strings"
)

type Config struct {
	RequireTypedConfirmation bool
}

type Safety struct {
	cfg Config
}

func New(cfg Config) *Safety {
	return &Safety{cfg: cfg}
}

func (s *Safety) RequiresConfirmation(cmd string, args []string) (bool, string) {
	if !s.cfg.RequireTypedConfirmation {
		return false, ""
	}
	if s.IsDestructive(cmd, args) {
		return true, "Destructive operation — requires typed confirmation."
	}
	if cmd == "git" && len(args) > 0 && args[0] == "push" {
		for _, a := range args {
			if a == "--force" || a == "-f" {
				return true, "Force push detected — destructive."
			}
		}
	}
	return false, ""
}

func (s *Safety) IsDestructive(cmd string, args []string) bool {
	if cmd != "git" {
		return false
	}
	if len(args) == 0 {
		return false
	}
	verb := args[0]
	switch verb {
	case "reset", "clean", "rebase", "filter-branch", "push":
		if verb == "push" {
			for _, a := range args {
				if strings.Contains(a, "force") || a == "-f" {
					return true
				}
			}
			return false
		}
		return true
	default:
		return false
	}
}

func TypedPrompt(cmd string, args []string) string {
	return fmt.Sprintf("Type 'yes-I-mean-it' to confirm %s %v", cmd, args)
}
