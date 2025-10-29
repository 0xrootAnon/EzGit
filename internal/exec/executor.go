package exec

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Runner struct{}

func NewRunner() *Runner {
	return &Runner{}
}

func (r *Runner) Run(ctx context.Context, name string, args ...string) (exitCode int, stdout string, stderr string, execErr error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return -1, "", "", err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return -1, "", "", err
	}

	if err := cmd.Start(); err != nil {
		return -1, "", "", err
	}

	var outBuf, errBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(&outBuf, stdoutPipe)
	}()
	go func() {
		defer wg.Done()
		io.Copy(&errBuf, stderrPipe)
	}()
	wg.Wait()
	err = cmd.Wait()
	exit := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			if status, ok2 := ee.Sys().(syscall.WaitStatus); ok2 {
				exit = status.ExitStatus()
			} else {
				exit = -1
			}
		} else {
			execErr = err
			exit = -1
		}
	}

	stdoutStr := strings.TrimSpace(outBuf.String())
	stderrStr := strings.TrimSpace(errBuf.String())

	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			execErr = ctx.Err()
		}
	}

	return exit, stdoutStr, stderrStr, execErr
}
