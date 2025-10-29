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
	paths := []string{
		`C:\Program Files\Git\bin\git.exe`,
		`C:\Program Files (x86)\Git\bin\git.exe`,
		`C:\Program Files\Git\cmd\git.exe`,
		`C:\Program Files (x86)\Git\cmd\git.exe`,
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func OpenDownloadURL() string {
	return "https://git-scm.com/download/win"
}

func OpenBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		return cmd.Start()
	case "darwin":
		cmd := exec.Command("open", url)
		return cmd.Start()
	default:
		cmd := exec.Command("xdg-open", url)
		return cmd.Start()
	}
}
