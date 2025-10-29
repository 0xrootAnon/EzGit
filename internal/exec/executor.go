package exec

import (
	"bufio"
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

type StreamCallback func(line string, isErr bool)

func (r *Runner) Run(ctx context.Context, name string, args []string, streamCb StreamCallback, timeout time.Duration) (int, string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return -1, "", "", err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return -1, "", "", err
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	if err := cmd.Start(); err != nil {
		return -1, "", "", err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	readPipe := func(rdr io.Reader, dest *bytes.Buffer, isErr bool) {
		defer wg.Done()
		scanner := bufio.NewScanner(rdr)
		for scanner.Scan() {
			line := scanner.Text()
			dest.WriteString(line + "\n")
			if streamCb != nil {
				streamCb(line, isErr)
			}
		}
	}

	go readPipe(stdoutPipe, &stdoutBuf, false)
	go readPipe(stderrPipe, &stderrBuf, true)

	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Wait()
	}()
	var waitErr error
	if timeout > 0 {
		select {
		case waitErr = <-errCh:
		case <-time.After(timeout):
			_ = cmd.Process.Kill()
			waitErr = context.DeadlineExceeded
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			waitErr = ctx.Err()
		}
	} else {
		select {
		case waitErr = <-errCh:
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			waitErr = ctx.Err()
		}
	}
	wg.Wait()

	stdoutStr := strings.TrimSuffix(stdoutBuf.String(), "\n")
	stderrStr := strings.TrimSuffix(stderrBuf.String(), "\n")

	exitCode := 0
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			} else {
				exitCode = 1
			}
		} else {
			if waitErr == context.DeadlineExceeded || waitErr == context.Canceled {
				return -1, stdoutStr, stderrStr, waitErr
			}
			return -1, stdoutStr, stderrStr, waitErr
		}
	}

	return exitCode, stdoutStr, stderrStr, nil
}
