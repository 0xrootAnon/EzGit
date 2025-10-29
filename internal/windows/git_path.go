package windows

import (
	"os"
	"os/exec"
	"runtime"
)

func DetectGit() string {
	if runtime.GOOS != "windows" {
		if _, err := exec.LookPath("git"); err == nil {
			return "git"
		}
		return ""
	}
	if p, err := exec.LookPath("git"); err == nil {
		return p
	}
	possible := []string{
		`C:\Program Files\Git\cmd\git.exe`,
		`C:\Program Files (x86)\Git\cmd\git.exe`,
		`C:\Program Files\Git\bin\git.exe`,
	}
	for _, p := range possible {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func OpenDownloadURL() string {
	return "https://git-scm.com/download/win"
}

func IsGitInstalled() bool {
	return DetectGit() != ""
}
