// Package logging gives myplace a persistent, machine-local debug trace.
// Everything is written to a single rolling file in the XDG state dir
// (ADR-0005); see docs/features/logging.md. Logging must never break a run —
// every failure path degrades to a no-op logger.
package logging

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

const (
	fileName     = "myplace.log"
	maxLogBytes  = 5 << 20 // rotate past ~5 MB, keeping one backup
	stderrTailKB = 2 << 10 // cap stderr captured into a log line
)

// Dir is the machine-local state directory (ADR-0005):
// $MYPLACE_STATE_DIR, else $XDG_STATE_HOME/myplace, else ~/.local/state/myplace.
func Dir() string {
	if d := os.Getenv("MYPLACE_STATE_DIR"); d != "" {
		return d
	}
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(os.TempDir(), "myplace")
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "myplace")
}

// Path is the full path to the log file.
func Path() string { return filepath.Join(Dir(), fileName) }

func level() log.Level {
	switch strings.ToLower(os.Getenv("MYPLACE_LOG_LEVEL")) {
	case "info":
		return log.InfoLevel
	case "warn", "warning":
		return log.WarnLevel
	case "error":
		return log.ErrorLevel
	default:
		return log.DebugLevel // capture everything by default
	}
}

func discard() *log.Logger { return log.New(io.Discard) }

// New opens the log file (append, rotating if oversized) and returns a logger
// tagged with the subcommand and pid, plus a close func. On any I/O failure it
// returns a no-op logger so a run never fails because logging couldn't start.
func New(command string) (*log.Logger, func()) {
	dir := Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return discard(), func() {}
	}
	path := filepath.Join(dir, fileName)
	if fi, err := os.Stat(path); err == nil && fi.Size() > maxLogBytes {
		_ = os.Rename(path, path+".1") // single backup; previous is overwritten
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return discard(), func() {}
	}
	l := log.NewWithOptions(f, log.Options{
		ReportTimestamp: true,
		TimeFormat:      time.RFC3339,
		Level:           level(),
	})
	tagged := l.With("cmd", command, "pid", os.Getpid())
	return tagged, func() { _ = f.Close() }
}

// Tail trims s to the last stderrTailKB bytes for logging, so a chatty
// subprocess can't bloat a single log line.
func Tail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > stderrTailKB {
		return "…" + s[len(s)-stderrTailKB:]
	}
	return s
}
