// Package run abstracts external command execution so the chezmoi/mise
// wrappers stay testable without the real binaries on PATH.
package run

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runner executes an external command and returns its stdout.
// dir is the working directory; "" inherits the current one.
type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error)
}

// Logger is the minimal slice of charmbracelet/log the runner needs, kept as
// an interface so this package doesn't depend on the logging library.
type Logger interface {
	Debug(msg interface{}, keyvals ...interface{})
	Error(msg interface{}, keyvals ...interface{})
}

// Exec is the real implementation. Log is optional; when set, every command
// is recorded with its duration and outcome (the high-value debug trace).
type Exec struct {
	Log    Logger
	tailFn func(string) string
}

// WithLogger returns an Exec that records every invocation. tail trims stderr
// captured into the failure line (pass logging.Tail).
func WithLogger(l Logger, tail func(string) string) Exec {
	return Exec{Log: l, tailFn: tail}
}

func (e Exec) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	// Closed stdin (not the inherited terminal): a child that tries to prompt
	// gets EOF and fails fast instead of blocking forever — critical when the
	// caller is the TUI, which owns the real TTY.
	cmd.Stdin = nil
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	dur := time.Since(start).Round(time.Millisecond)

	if e.Log != nil {
		joined := strings.Join(args, " ")
		if err != nil {
			e.Log.Error("exec failed", "tool", name, "args", joined, "dir", dir,
				"dur", dur.String(), "stderr", e.tail(stderr.String()))
		} else {
			e.Log.Debug("exec", "tool", name, "args", joined, "dir", dir, "dur", dur.String())
		}
	}

	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.Bytes(), fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

func (e Exec) tail(s string) string {
	if e.tailFn != nil {
		return e.tailFn(s)
	}
	return strings.TrimSpace(s)
}
