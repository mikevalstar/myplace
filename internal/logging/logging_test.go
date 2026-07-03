package logging

import (
	"fmt"
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

func TestTailLines(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MYPLACE_STATE_DIR", dir)
	var b strings.Builder
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	got := TailLines(5, 1<<20)
	if len(got) != 5 || got[0] != "line 96" || got[4] != "line 100" {
		t.Errorf("TailLines(5) should return the last 5 lines, got %v", got)
	}
	// A window smaller than the file must drop the partial first line and
	// still end at the newest line.
	small := TailLines(1000, 64)
	if len(small) == 0 || small[len(small)-1] != "line 100" {
		t.Errorf("small window should still end at the newest line, got %v", small)
	}
	for _, ln := range small {
		if !strings.HasPrefix(ln, "line ") {
			t.Errorf("partial first line should be dropped, got %q", ln)
		}
	}
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
