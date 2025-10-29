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
			{Key: "readme", Label: "Create README? (y/N)", Default: "y"},
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
			if force == "y" || force == "yes" {
				args := []string{"push", "--force", remote, branch}
				return "git", args, "git push --force " + remote + " " + branch
			}
			args := []string{"push", remote, branch}
			return "git", args, "git push " + remote + " " + branch
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
		Help: "Undo last commit (soft/mixed/hard) — convenience wrapper for HEAD~1",
		Prompts: []Prompt{
			{Key: "mode", Label: "Mode (soft/mixed/hard)", Default: "mixed", Required: true},
		},
		BuildFunc: func(in ActionInput) (string, []string, string) {
			mode := in["mode"]
			args := []string{"reset"}
			switch strings.ToLower(mode) {
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
			return strings.ToLower(in["mode"]) == "hard"
		},
	})

	r.Register(&ActionDef{
		Name: "reset",
		Help: "Reset current branch (soft/mixed/hard) to a specified ref",
		Prompts: []Prompt{
			{Key: "mode", Label: "Mode (soft/mixed/hard)", Default: "mixed", Required: true},
			{Key: "ref", Label: "Reference (e.g. HEAD~1 or origin/main)", Default: "HEAD~1", Required: true},
		},
		BuildFunc: func(in ActionInput) (string, []string, string) {
			mode := in["mode"]
			ref := in["ref"]
			args := []string{"reset"}
			switch strings.ToLower(mode) {
			case "soft":
				args = append(args, "--soft", ref)
			case "hard":
				args = append(args, "--hard", ref)
			default:
				args = append(args, "--mixed", ref)
			}
			return "git", args, "git " + strings.Join(args, " ")
		},
		IsDestructive: func(in ActionInput) bool {
			return strings.ToLower(in["mode"]) == "hard"
		},
	})

	r.Register(&ActionDef{
		Name: "clean",
		Help: "Remove untracked files (git clean). Default shows dry-run (-n).",
		Prompts: []Prompt{
			{Key: "args", Label: "clean args (e.g. -fd)", Default: "-n"},
		},
		BuildFunc: func(in ActionInput) (string, []string, string) {
			extra := strings.TrimSpace(in["args"])
			args := []string{"clean"}
			if extra != "" {
				parts := strings.Fields(extra)
				args = append(args, parts...)
			}
			return "git", args, "git " + strings.Join(args, " ")
		},
		IsDestructive: func(in ActionInput) bool {
			a := in["args"]
			return strings.Contains(a, "-f") || strings.Contains(a, "-fd") || strings.Contains(a, "-fx")
		},
	})

	r.Register(&ActionDef{
		Name: "merge",
		Help: "Merge a branch into current (preview with --no-commit by default)",
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
		Help: "Interactive rebase helper (reorder/squash/edit msgs). Presents commits for editing before running rebase -i.",
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
			preview += "\n(EzGit will present the commits and let you reorder/squash via a UI; this is a high-risk operation.)"
			return "git", args, preview
		},
		IsDestructive: func(in ActionInput) bool {
			return true
		},
	})

	r.Register(&ActionDef{
		Name: "raw",
		Help: "Run raw git command (expert mode). Type args exactly as you would in shell, without leading 'git'.",
		Prompts: []Prompt{
			{Key: "command", Label: "Full git command (without leading 'git')", Required: true},
		},
		BuildFunc: func(in ActionInput) (string, []string, string) {
			cmdline := in["command"]
			parts := strings.Fields(cmdline)
			preview := "git " + strings.Join(parts, " ")
			return "git", parts, preview
		},
	})
	registerPassthrough(r, "rebase", "Rebase (non-interactive)", []Prompt{{Key: "args", Label: "rebase args (e.g. origin/main)", Default: ""}}, []string{"rebase"})
	registerPassthrough(r, "diff", "Show changes", []Prompt{{Key: "args", Label: "diff args (e.g. HEAD~1..HEAD)", Default: ""}}, []string{"diff"})
	registerPassthrough(r, "log", "Show commit history", []Prompt{{Key: "args", Label: "log args (e.g. --oneline -n 20)", Default: "--oneline -n 20"}}, []string{"log"})
	registerPassthrough(r, "show", "Show object", []Prompt{{Key: "args", Label: "show args (e.g. HEAD:filename)", Default: ""}}, []string{"show"})
	registerPassthrough(r, "branch", "Branch operations", []Prompt{{Key: "args", Label: "branch args (e.g. -a / new-branch)", Default: "-a"}}, []string{"branch"})
	registerPassthrough(r, "checkout", "Switch or restore files (checkout)", []Prompt{{Key: "args", Label: "checkout args (branch or -- file)", Default: ""}}, []string{"checkout"})
	registerPassthrough(r, "switch", "Switch branches (preferred)", []Prompt{{Key: "args", Label: "switch args (branch)", Default: ""}}, []string{"switch"})
	registerPassthrough(r, "mv", "Move/rename files in index", []Prompt{{Key: "args", Label: "mv args (src dst)", Default: ""}}, []string{"mv"})
	registerPassthrough(r, "restore", "Restore working tree files (git restore)", []Prompt{{Key: "args", Label: "restore args (e.g. --staged file)", Default: ""}}, []string{"restore"})
	registerPassthrough(r, "rm", "Remove files from tree/index", []Prompt{{Key: "args", Label: "rm args (files)", Default: ""}}, []string{"rm"})
	registerPassthrough(r, "tag", "Create/list/delete tags", []Prompt{{Key: "args", Label: "tag args", Default: "-l"}}, []string{"tag"})
	registerPassthrough(r, "fetch", "Fetch from remotes", []Prompt{{Key: "args", Label: "fetch args", Default: ""}}, []string{"fetch"})
	registerPassthrough(r, "pull", "Fetch + merge/rebase", []Prompt{{Key: "args", Label: "pull args", Default: ""}}, []string{"pull"})
	registerPassthrough(r, "remote", "Manage remotes", []Prompt{{Key: "args", Label: "remote args (add/origin url)", Default: "-v"}}, []string{"remote"})
	registerPassthrough(r, "ls-remote", "List refs in a remote repo", []Prompt{{Key: "args", Label: "ls-remote args (remote URL)", Default: ""}}, []string{"ls-remote"})
	registerPassthrough(r, "credential", "Credential helper interface", []Prompt{{Key: "args", Label: "credential args (fill/get/store)", Default: ""}}, []string{"credential"})
	registerPassthrough(r, "daemon", "Run git daemon (advanced)", []Prompt{{Key: "args", Label: "daemon args", Default: ""}}, []string{"daemon"})
	registerPassthrough(r, "stash", "Stash operations (list/save/apply/pop/drop)", []Prompt{{Key: "args", Label: "stash args (list | save <msg> | pop stash@{0})", Default: "list"}}, []string{"stash"})
	registerPassthrough(r, "bisect", "Bisect to find the commit that introduced a bug", []Prompt{{Key: "args", Label: "bisect args (start/next/good/bad/visualize)", Default: ""}}, []string{"bisect"})
	registerPassthrough(r, "reflog", "Show reference log", []Prompt{{Key: "args", Label: "reflog args (e.g. --decorate -n 50)", Default: "--decorate -n 50"}}, []string{"reflog"})
	registerPassthrough(r, "help", "Show git help for a command", []Prompt{{Key: "args", Label: "help args (e.g. commit)", Default: ""}}, []string{"help"})
	registerPassthrough(r, "rerere", "Reuse recorded resolution (git rerere)", []Prompt{{Key: "args", Label: "rerere args", Default: ""}}, []string{"rerere"})
	registerPassthrough(r, "stash-pop", "Pop a stash entry", []Prompt{{Key: "args", Label: "pop args (e.g. stash@{0})", Default: "stash@{0}"}}, []string{"stash", "pop"})
	registerPassthrough(r, "stash-drop", "Drop a stash entry", []Prompt{{Key: "args", Label: "drop args (e.g. stash@{0})", Default: "stash@{0}"}}, []string{"stash", "drop"})
	registerPassthrough(r, "stash-list", "List stash entries", nil, []string{"stash", "list"})
	registerPassthrough(r, "apply", "Apply a patch or stash (git apply)", []Prompt{{Key: "args", Label: "apply args (e.g. path/to/patch)", Default: ""}}, []string{"apply"})
	registerPassthrough(r, "am", "Apply patches from mailbox (git am)", []Prompt{{Key: "args", Label: "am args (patch-file)", Default: ""}}, []string{"am"})
	registerPassthrough(r, "format-patch", "Create patch files (git format-patch)", []Prompt{{Key: "args", Label: "format-patch args (range)", Default: ""}}, []string{"format-patch"})
	registerPassthrough(r, "cherry", "Find commits not merged", []Prompt{{Key: "args", Label: "cherry args", Default: ""}}, []string{"cherry"})
	registerPassthrough(r, "cherry-pick", "Apply the changes introduced by existing commits", []Prompt{{Key: "args", Label: "cherry-pick args (commit...)", Default: ""}}, []string{"cherry-pick"})
	registerPassthrough(r, "revert", "Create a new commit that reverts earlier commits", []Prompt{{Key: "args", Label: "revert args (commit...)", Default: ""}}, []string{"revert"})
	registerPassthrough(r, "filter-branch", "Rewrite branches (dangerous)", []Prompt{{Key: "args", Label: "filter-branch args (e.g. --env-filter ...)", Default: ""}}, []string{"filter-branch"})
	registerPassthrough(r, "describe", "Describe a commit", []Prompt{{Key: "args", Label: "describe args (e.g. --tags --long)", Default: ""}}, []string{"describe"})
	registerPassthrough(r, "fsck", "Check repository integrity", []Prompt{{Key: "args", Label: "fsck args", Default: ""}}, []string{"fsck"})
	registerPassthrough(r, "verify-pack", "Verify pack files", []Prompt{{Key: "args", Label: "verify-pack args", Default: ""}}, []string{"verify-pack"})
	registerPassthrough(r, "count-objects", "Count objects in repo", []Prompt{{Key: "args", Label: "count-objects args", Default: ""}}, []string{"count-objects"})
	registerPassthrough(r, "prune", "Prune unreachable objects", []Prompt{{Key: "args", Label: "prune args", Default: ""}}, []string{"prune"})
	registerPassthrough(r, "gc", "Garbage collect repository", []Prompt{{Key: "args", Label: "gc args", Default: ""}}, []string{"gc"})
	registerPassthrough(r, "shortlog", "Summary of commits by author", []Prompt{{Key: "args", Label: "shortlog args (e.g. -s -n)", Default: "--summary"}}, []string{"shortlog"})
	registerPassthrough(r, "whatchanged", "Older log-like command", []Prompt{{Key: "args", Label: "whatchanged args", Default: ""}}, []string{"whatchanged"})
	registerPassthrough(r, "archive", "Create an archive of a tree", []Prompt{{Key: "args", Label: "archive args (format path)", Default: ""}}, []string{"archive"})
	registerPassthrough(r, "bundle", "Create or apply a bundle", []Prompt{{Key: "args", Label: "bundle args", Default: ""}}, []string{"bundle"})
	registerPassthrough(r, "submodule", "Manage submodules", []Prompt{{Key: "args", Label: "submodule args (add/update/status)", Default: ""}}, []string{"submodule"})
	registerPassthrough(r, "worktree", "Manage worktrees", []Prompt{{Key: "args", Label: "worktree args (add/list/remove)", Default: ""}}, []string{"worktree"})
	registerPassthrough(r, "config", "Get/set git config", []Prompt{{Key: "args", Label: "config args (e.g. --global user.name 'Me')", Default: ""}}, []string{"config"})
	registerPassthrough(r, "grep", "Search in tracked files", []Prompt{{Key: "args", Label: "grep args", Default: "-n -- \"TODO\""}}, []string{"grep"})
	registerPassthrough(r, "blame", "Annotate a file", []Prompt{{Key: "args", Label: "blame args (file)", Default: ""}}, []string{"blame"})
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
