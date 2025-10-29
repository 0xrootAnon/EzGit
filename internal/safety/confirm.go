package safety

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
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

func (s *Safety) RequireTypedConfirmation(cmd string, args []string) bool {
	return s.RequireTypedConfirmationReader(os.Stdin, cmd, args)
}

func (s *Safety) RequireTypedConfirmationReader(r io.Reader, cmd string, args []string) bool {
	confirmStr := "yes-I-mean-it"
	fmt.Printf("\n*** Destructive operation preview ***\n")
	preview := fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
	fmt.Printf("Preview: %s\n", preview)
	fmt.Printf("\nThis operation is potentially destructive.\nType '%s' to proceed, or press Enter to abort: ", confirmStr)

	reader := bufio.NewReader(r)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == confirmStr {
		return true
	}

	fmt.Printf("Confirmation failed. Aborting.\n")
	time.Sleep(150 * time.Millisecond)
	return false
}
