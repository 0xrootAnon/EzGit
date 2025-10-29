package action

import (
	"fmt"
	"strings"
)

func registerPassthrough(r *Registry, name, help string, prompts []Prompt, defaultArgs []string) {
	r.Register(&ActionDef{
		Name:    name,
		Help:    help,
		Prompts: prompts,
		BuildFunc: func(in ActionInput) (string, []string, string) {
			args := append([]string{}, defaultArgs...)
			if extra, ok := in["args"]; ok && strings.TrimSpace(extra) != "" {
				parts := strings.Fields(extra)
				args = append(args, parts...)
			}
			preview := "git " + strings.Join(args, " ")
			return "git", args, preview
		},
	})
}

func RegisterBuiltins(r *Registry) {
	r.Register(&ActionDef{
		Name: "init",
		Help: "Initialize a new git repository and optional README",
		Prompts: []Prompt{
			{Key: "path", Label: "Directory", Default: ".", Required: true},
			{Key: "readme", Label: "Create README?", Default: "y"},
		},
		BuildFunc: func(in ActionInput) (string, []string, string) {
			path := in["path"]
			readme := strings.ToLower(in["readme"])
			preview := fmt.Sprintf("git init %s", path)
			if readme == "y" || readme == "yes" {
				preview = preview + " && echo \"# Project\" > README.md && git add README.md && git commit -m \"initial commit\""
			}
			return "git", []string{"init", path}, preview
		},
	})

	registerPassthrough(r, "diff", "Show changes", []Prompt{{Key: "args", Label: "Additional args (e.g. HEAD~1..HEAD)", Default: ""}}, []string{"diff"})
	registerPassthrough(r, "log", "Show commit history", []Prompt{{Key: "args", Label: "Log args (e.g. --oneline -n 20)", Default: "--oneline -n 20"}}, []string{"log"})
	registerPassthrough(r, "show", "Show object", []Prompt{{Key: "args", Label: "Show args (e.g. HEAD:filename)", Default: ""}}, []string{"show"})
	registerPassthrough(r, "branch", "Branch operations", []Prompt{{Key: "args", Label: "branch args (e.g. -a / new-branch)", Default: "-a"}}, []string{"branch"})
	registerPassthrough(r, "checkout", "Switch or restore files (checkout)", []Prompt{{Key: "args", Label: "checkout args (branch or -- file)", Default: ""}}, []string{"checkout"})
	registerPassthrough(r, "switch", "Switch branches (preferred)", []Prompt{{Key: "args", Label: "switch args (branch)", Default: ""}}, []string{"switch"})
	registerPassthrough(r, "mv", "Move/rename files in index", []Prompt{{Key: "args", Label: "mv args (src dst)", Default: ""}}, []string{"mv"})
	registerPassthrough(r, "rm", "Remove files from tree/index", []Prompt{{Key: "args", Label: "rm args (files)", Default: ""}}, []string{"rm"})
	registerPassthrough(r, "tag", "Create/list/delete tags", []Prompt{{Key: "args", Label: "tag args", Default: "-l"}}, []string{"tag"})
	registerPassthrough(r, "fetch", "Fetch from remotes", []Prompt{{Key: "args", Label: "fetch args", Default: ""}}, []string{"fetch"})
	registerPassthrough(r, "pull", "Fetch + merge/rebase", []Prompt{{Key: "args", Label: "pull args", Default: ""}}, []string{"pull"})
	registerPassthrough(r, "remote", "Manage remotes", []Prompt{{Key: "args", Label: "remote args (add/origin url)", Default: "-v"}}, []string{"remote"})
	registerPassthrough(r, "stash", "Stash operations", []Prompt{{Key: "args", Label: "stash args (save/pop/list)", Default: "list"}}, []string{"stash"})
	registerPassthrough(r, "grep", "Search in tracked files", []Prompt{{Key: "args", Label: "grep args", Default: "-n -- \"TODO\""}}, []string{"grep"})
	registerPassthrough(r, "blame", "Annotate a file", []Prompt{{Key: "args", Label: "blame args (file)", Default: ""}}, []string{"blame"})
	registerPassthrough(r, "clean", "Remove untracked files", []Prompt{{Key: "args", Label: "clean args (e.g. -fd)", Default: "-n"}}, []string{"clean"})
	registerPassthrough(r, "archive", "Create an archive of a tree", []Prompt{{Key: "args", Label: "archive args (format path)", Default: ""}}, []string{"archive"})
	registerPassthrough(r, "reflog", "Show reference log", nil, []string{"reflog"})

	r.Register(&ActionDef{
		Name:    "status",
		Help:    "Show git status",
		Prompts: []Prompt{},
		BuildFunc: func(in ActionInput) (string, []string, string) {
			return "git", []string{"status"}, "git status"
		},
	})

	r.Register(&ActionDef{
		Name: "add",
		Help: "Stage files",
		Prompts: []Prompt{
			{Key: "paths", Label: "Files to add (comma or space separated)", Default: "-A", Required: true},
		},
		BuildFunc: func(in ActionInput) (string, []string, string) {
			paths := in["paths"]
			var args []string
			if paths == "" || paths == "-A" {
				args = []string{"add", "-A"}
			} else {
				parts := splitPaths(paths)
				args = append([]string{"add"}, parts...)
			}
			return "git", args, "git " + strings.Join(args, " ")
		},
	})

	r.Register(&ActionDef{
		Name: "commit",
		Help: "Create a commit",
		Prompts: []Prompt{
			{Key: "message", Label: "Commit message", Default: "", Required: true},
			{Key: "stage", Label: "Stage all changes first? (y/N)", Default: "y"},
		},
		BuildFunc: func(in ActionInput) (string, []string, string) {
			msg := in["message"]
			stage := strings.ToLower(in["stage"])
			if stage == "" {
				stage = "y"
			}
			preview := ""
			if stage == "y" || stage == "yes" {
				preview = fmt.Sprintf("git add -A && git commit -m %q", msg)
			} else {
				preview = fmt.Sprintf("git commit -m %q", msg)
			}
			return "git", []string{"commit", "-m", msg}, preview
		},
	})

	r.Register(&ActionDef{
		Name: "push",
		Help: "Push to remote",
		Prompts: []Prompt{
			{Key: "remote", Label: "Remote name", Default: "origin"},
			{Key: "branch", Label: "Branch", Default: "main"},
			{Key: "force", Label: "Force push? (y/N)", Default: "n"},
		},
		BuildFunc: func(in ActionInput) (string, []string, string) {
			remote := in["remote"]
			branch := in["branch"]
			force := strings.ToLower(in["force"])
			args := []string{"push", remote, branch}
			preview := fmt.Sprintf("git push %s %s", remote, branch)
			if force == "y" || force == "yes" {
				args = append([]string{"push"}, "--force", remote, branch)
				preview = "git push --force " + remote + " " + branch
			}
			return "git", args, preview
		},
		ValidateFunc: func(in ActionInput) error {
			if strings.ContainsAny(in["remote"], " \t\n\r") {
				return fmt.Errorf("invalid remote name")
			}
			if strings.ContainsAny(in["branch"], " \t\n\r") {
				return fmt.Errorf("invalid branch name")
			}
			return nil
		},
		IsDestructive: func(in ActionInput) bool {
			force := strings.ToLower(in["force"])
			return force == "y" || force == "yes"
		},
	})

	r.Register(&ActionDef{
		Name: "clone",
		Help: "Clone a repository",
		Prompts: []Prompt{
			{Key: "url", Label: "Repository URL", Required: true},
			{Key: "path", Label: "Directory (optional)", Default: ""},
		},
		BuildFunc: func(in ActionInput) (string, []string, string) {
			args := []string{"clone", in["url"]}
			if p := in["path"]; p != "" {
				args = append(args, p)
			}
			return "git", args, "git " + strings.Join(args, " ")
		},
	})

	r.Register(&ActionDef{
		Name: "undo",
		Help: "Undo last commit (soft/mixed/hard)",
		Prompts: []Prompt{
			{Key: "mode", Label: "Mode (soft/mixed/hard)", Default: "mixed", Required: true},
		},
		BuildFunc: func(in ActionInput) (string, []string, string) {
			mode := in["mode"]
			if mode == "" {
				mode = "mixed"
			}
			args := []string{"reset"}
			switch mode {
			case "soft":
				args = append(args, "--soft", "HEAD~1")
			case "hard":
				args = append(args, "--hard", "HEAD~1")
			default:
				args = append(args, "--mixed", "HEAD~1")
			}
			return "git", args, "git " + strings.Join(args, " ")
		},
		IsDestructive: func(in ActionInput) bool {
			m := strings.ToLower(in["mode"])
			return m == "hard"
		},
	})

	r.Register(&ActionDef{
		Name: "merge",
		Help: "Merge a branch into current (preview with no-commit)",
		Prompts: []Prompt{
			{Key: "branch", Label: "Branch to merge", Default: "", Required: true},
			{Key: "strategy", Label: "Strategy (default/ours/theirs)", Default: ""},
			{Key: "no-ff", Label: "Prefer no-ff (y/N)", Default: "y"},
		},
		BuildFunc: func(in ActionInput) (string, []string, string) {
			branch := in["branch"]
			args := []string{"merge"}
			if strings.ToLower(in["no-ff"]) == "y" || strings.ToLower(in["no-ff"]) == "yes" {
				args = append(args, "--no-commit", "--no-ff")
			}
			if s := in["strategy"]; s != "" {
				args = append(args, "-s", s)
			}
			args = append(args, branch)
			preview := "git " + strings.Join(args, " ")
			preview += "\n(Preview will run merge with --no-commit so you can inspect conflicts before finalizing.)"
			return "git", args, preview
		},
		IsDestructive: func(in ActionInput) bool {
			return false
		},
	})

	r.Register(&ActionDef{
		Name: "rebase-interactive",
		Help: "Interactive rebase helper (reorder/squash/edit msgs)",
		Prompts: []Prompt{
			{Key: "base", Label: "Base ref (e.g. HEAD~5)", Default: "HEAD~5", Required: true},
			{Key: "autosquash", Label: "Autosquash? (y/N)", Default: "n"},
		},
		BuildFunc: func(in ActionInput) (string, []string, string) {
			base := in["base"]
			args := []string{"rebase", "-i", base}
			if strings.ToLower(in["autosquash"]) == "y" {
				args = append(args, "--autosquash")
			}
			preview := "git " + strings.Join(args, " ")
			preview += "\n(Interactive: EzGit will present commits and let you reorder/squash. EzGit constructs a TODO and runs rebase with a temp GIT_SEQUENCE_EDITOR.)"
			return "git", args, preview
		},
		IsDestructive: func(in ActionInput) bool {
			return true
		},
	})

	r.Register(&ActionDef{
		Name: "raw",
		Help: "Run raw git command (expert mode, edit before executing)",
		Prompts: []Prompt{
			{Key: "command", Label: "Full git command (without leading 'git')", Required: true},
		},
		BuildFunc: func(in ActionInput) (string, []string, string) {
			cmdline := in["command"]
			parts := strings.Fields(cmdline)
			preview := "git " + strings.Join(parts, " ")
			args := parts
			return "git", args, preview
		},
	})
}

func splitPaths(s string) []string {
	s = strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(s)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
