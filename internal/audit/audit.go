package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Audit struct {
	fpath string
	f     *os.File
	mu    sync.Mutex
}

type AuditEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Command     string    `json:"command"`
	Args        []string  `json:"args"`
	Stdout      string    `json:"stdout"`
	Stderr      string    `json:"stderr"`
	ExitCode    int       `json:"exit_code"`
	Explanation string    `json:"explanation"`
}

type Entry struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Command   string    `json:"command"`
	Args      []string  `json:"args"`
	ExitCode  int       `json:"exit_code"`
	Stdout    string    `json:"stdout,omitempty"`
	Stderr    string    `json:"stderr,omitempty"`
}

func AppendAudit(enabled bool, e Entry) error {
	if !enabled {
		return nil
	}
	home := os.Getenv("HOME")
	if home == "" {
		home = os.TempDir()
	}
	dir := filepath.Join(home, ".config", "ezgit")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	fpath := filepath.Join(dir, "audit.log")
	f, err := os.OpenFile(fpath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(e)
}

func NewAudit(path string) (*Audit, error) {
	if path == "" {
		path = "ezgit_actions.log"
	}
	if err := os.MkdirAll(filepathDir(path), 0o755); err != nil {
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &Audit{fpath: path, f: f}, nil
}

func filepathDir(path string) string {
	dir := ""
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			dir = path[:i]
			break
		}
	}
	if dir == "" {
		return "."
	}
	return dir
}

func (a *Audit) Log(e AuditEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, _ := json.Marshal(e)
	_, err := a.f.Write(append(b, '\n'))
	return err
}

func (a *Audit) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.f == nil {
		return nil
	}
	err := a.f.Sync()
	_ = a.f.Close()
	a.f = nil
	return err
}

func (a *Audit) Recent(n int) ([]AuditEntry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	fb, err := os.ReadFile(a.fpath)
	if err != nil {
		return nil, err
	}
	lines := bytesToLines(string(fb))
	res := []AuditEntry{}
	for i := len(lines) - 1; i >= 0 && len(res) < n; i-- {
		var e AuditEntry
		if err := json.Unmarshal([]byte(lines[i]), &e); err == nil {
			res = append(res, e)
		}
	}
	return res, nil
}

func bytesToLines(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
		} else {
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
