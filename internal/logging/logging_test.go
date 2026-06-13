package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirPrecedence(t *testing.T) {
	t.Setenv("MYPLACE_STATE_DIR", "/tmp/explicit")
	t.Setenv("XDG_STATE_HOME", "/tmp/xdg")
	if Dir() != "/tmp/explicit" {
		t.Errorf("MYPLACE_STATE_DIR should win, got %s", Dir())
	}

	t.Setenv("MYPLACE_STATE_DIR", "")
	if got, want := Dir(), filepath.Join("/tmp/xdg", "myplace"); got != want {
		t.Errorf("XDG_STATE_HOME path: want %s, got %s", want, got)
	}

	t.Setenv("XDG_STATE_HOME", "")
	home, _ := os.UserHomeDir()
	if got, want := Dir(), filepath.Join(home, ".local", "state", "myplace"); got != want {
		t.Errorf("default path: want %s, got %s", want, got)
	}
}

func TestNewWritesToStateDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MYPLACE_STATE_DIR", dir)
	l, closeLog := New("test")
	l.Info("hello", "k", "v")
	closeLog()

	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "hello") || !strings.Contains(string(data), "cmd=test") {
		t.Errorf("log file missing expected content:\n%s", data)
	}
}

func TestNewToleratesUnwritableDir(t *testing.T) {
	// A path whose parent is a file cannot be created; New must not panic and
	// must return a usable (no-op) logger.
	f := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MYPLACE_STATE_DIR", filepath.Join(f, "nope"))
	l, closeLog := New("test")
	defer closeLog()
	l.Info("should not panic") // writing to discard is fine
}

func TestTail(t *testing.T) {
	if Tail("  hi  ") != "hi" {
		t.Error("Tail should trim whitespace")
	}
	big := strings.Repeat("x", stderrTailKB+500)
	got := Tail(big)
	if len(got) > stderrTailKB+len("…") || !strings.HasPrefix(got, "…") {
		t.Errorf("Tail should cap and ellipsize, got len %d", len(got))
	}
}
