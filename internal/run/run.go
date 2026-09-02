// Package run abstracts external command execution so the chezmoi/mise
// wrappers stay testable without the real binaries on PATH.
package run

import (
	"bytes"
	"context"
	"errors"
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
		rerr := &Error{Msg: fmt.Sprintf("%s %s: %s", name, strings.Join(args, " "), msg), Stderr: strings.TrimSpace(stderr.String()), Code: -1}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			rerr.Code = exitErr.ExitCode()
		}
		return stdout.Bytes(), rerr
	}
	return stdout.Bytes(), nil
}

// Error is the failure Exec returns. Its message is the same "<cmd> <args>:
// <stderr-or-cause>" line callers have always seen; Code and Stderr let a
// wrapper tell a meaningful non-zero exit apart from a real failure — pacman's
// `checkupdates` exits 2 for "nothing to update", `yay -Qua` exits 1 — without
// string-matching the message. Code is -1 when the process never ran (not on
// PATH, context cancelled).
type Error struct {
	Msg    string
	Stderr string
	Code   int
}

func (e *Error) Error() string { return e.Msg }

// ExitCode extracts the process exit code from an error returned by a Runner.
// ok is false when err isn't a run.Error (a fake runner, a wrapped error) or
// the process never started. Stderr is returned alongside so callers can
// distinguish a silent status exit from a real failure.
func ExitCode(err error) (code int, stderr string, ok bool) {
	var rerr *Error
	if errors.As(err, &rerr) && rerr.Code >= 0 {
		return rerr.Code, rerr.Stderr, true
	}
	return 0, "", false
}

func (e Exec) tail(s string) string {
	if e.tailFn != nil {
		return e.tailFn(s)
	}
	return strings.TrimSpace(s)
}
