package summarizer

import (
	"fmt"
	"strings"
)

type Summary struct {
	Short  string
	Detail string
}

type Summarizer struct{}

func NewSummarizer() *Summarizer {
	return &Summarizer{}
}

func (s *Summarizer) Summarize(cmd string, args []string, exitCode int, stdout, stderr string, execErr error) Summary {
	base := fmt.Sprintf("Ran: %s %s", cmd, strings.Join(args, " "))
	if execErr != nil {
		return Summary{
			Short:  fmt.Sprintf("Command failed: %v", execErr),
			Detail: fmt.Sprintf("%s\nstdout:\n%s\nstderr:\n%s", base, stdout, stderr),
		}
	}
	if exitCode != 0 {
		return Summary{
			Short:  fmt.Sprintf("Command exited with status %d", exitCode),
			Detail: fmt.Sprintf("%s\nstdout:\n%s\nstderr:\n%s", base, stdout, stderr),
		}
	}

	argsJoined := strings.Join(args, " ")
	switch {
	case strings.HasPrefix(argsJoined, "commit"):
		sha := extractFirstHash(stdout)
		if sha == "" {
			if strings.Contains(stdout, "created") || strings.Contains(stdout, "Committed") {
				sha = "committed"
			}
		}
		return Summary{
			Short:  fmt.Sprintf("Committed changes (%s).", sha),
			Detail: fmt.Sprintf("%s\n%s", base, stdout),
		}
	case strings.HasPrefix(argsJoined, "push"):
		return Summary{
			Short:  "Pushed to remote.",
			Detail: fmt.Sprintf("%s\n%s", base, stdout),
		}
	case strings.HasPrefix(argsJoined, "status"):
		return Summary{
			Short:  "Status displayed.",
			Detail: stdout,
		}
	default:
		return Summary{
			Short:  "Command completed successfully.",
			Detail: fmt.Sprintf("%s\n%s", base, stdout),
		}
	}
}

func extractFirstHash(s string) string {
	fields := strings.Fields(s)
	for _, f := range fields {
		if len(f) >= 7 {
			ok := true
			for _, r := range f[:7] {
				if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
					ok = false
					break
				}
			}
			if ok {
				return f[:7]
			}
		}
	}
	return ""
}
