package chezmoi

import (
	"context"
	"strings"
	"testing"
)

type fakeRunner struct{ args []string }

func (f *fakeRunner) Run(_ context.Context, _ string, name string, args ...string) ([]byte, error) {
	f.args = append([]string{name}, args...)
	return nil, nil
}

// Locks the init contract that bit us: --no-tty is always present, and the
// --promptString keys match the data keys (chezmoi matches by prompt text), in
// sorted order, with the repo last.
func TestInitApplyArgs(t *testing.T) {
	f := &fakeRunner{}
	c := New(f)
	_ = c.InitApply(context.Background(), "repo-url", "server",
		map[string]string{"gitEmail": "a@b.com", "gitName": "A B", "blank": ""})
	got := strings.Join(f.args, " ")
	want := "chezmoi --no-tty init --apply --promptChoice profile=server " +
		"--promptString gitEmail=a@b.com --promptString gitName=A B repo-url"
	if got != want {
		t.Errorf("init args:\n got: %s\nwant: %s", got, want)
	}
}

func TestParseStatus(t *testing.T) {
	out := []byte(" M .zshrc\nMM .config/mise/config.toml\n A .config/new\n\n")
	files := ParseStatus(out)
	if len(files) != 3 {
		t.Fatalf("want 3 files, got %d: %v", len(files), files)
	}

	zshrc := files[0]
	if zshrc.Path != ".zshrc" || zshrc.LocalChanged || !zshrc.ApplyChanges {
		t.Errorf("zshrc: want incoming-only change, got %+v", zshrc)
	}

	both := files[1]
	if !both.LocalChanged || !both.ApplyChanges {
		t.Errorf("mise config: want changes in both directions, got %+v", both)
	}

	added := files[2]
	if added.Path != ".config/new" || added.LocalChanged || !added.ApplyChanges {
		t.Errorf("new file: want incoming add, got %+v", added)
	}
}

func TestParseStatusEmpty(t *testing.T) {
	if files := ParseStatus([]byte("")); len(files) != 0 {
		t.Errorf("want no files from empty output, got %v", files)
	}
}
