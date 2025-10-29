package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInitCommitStatus(t *testing.T) {
	tmp := t.TempDir()
	cmd := exec.Command("git", "init", tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, string(out))
	}
	file := filepath.Join(tmp, "hello.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	cmd = exec.Command("git", "-C", tmp, "add", "hello.txt")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v: %s", err, string(out))
	}
	cmd = exec.Command("git", "-C", tmp, "commit", "-m", "test")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v: %s", err, string(out))
	}
}
