// Package run abstracts external command execution so the chezmoi/mise
// wrappers stay testable without the real binaries on PATH.
package run

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runner executes an external command and returns its stdout.
// dir is the working directory; "" inherits the current one.
type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error)
}

// Exec is the real implementation.
type Exec struct{}

func (Exec) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	// Closed stdin (not the inherited terminal): a child that tries to prompt
	// gets EOF and fails fast instead of blocking forever — critical when the
	// caller is the TUI, which owns the real TTY.
	cmd.Stdin = nil
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.Bytes(), fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}
