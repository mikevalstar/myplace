package chezmoi

import (
	"context"
	"strings"
	"testing"
)

type fakeRunner struct {
	args []string
	out  []byte
}

func (f *fakeRunner) Run(_ context.Context, _ string, name string, args ...string) ([]byte, error) {
	f.args = append([]string{name}, args...)
	return f.out, nil
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

func TestPullAndApplyArgs(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		run  func(*Client) error
		want string
	}{
		{
			name: "pull",
			run:  func(c *Client) error { return c.Pull(ctx) },
			want: "chezmoi --no-tty git -- pull --rebase",
		},
		{
			name: "apply all",
			run:  func(c *Client) error { return c.Apply(ctx) },
			want: "chezmoi --no-tty apply",
		},
		{
			name: "apply target",
			run:  func(c *Client) error { return c.ApplyTarget(ctx, "/Users/me/.zshrc") },
			want: "chezmoi --no-tty apply /Users/me/.zshrc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeRunner{}
			c := New(f)
			if err := tt.run(c); err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(f.args, " "); got != tt.want {
				t.Errorf("args:\n got: %s\nwant: %s", got, tt.want)
			}
		})
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

func TestPushPolicy(t *testing.T) {
	tests := []struct {
		name   string
		data   string
		want   bool
		source string
	}{
		{name: "explicit true", data: `{"profile":"server","push":true}`, want: true, source: "data.push"},
		{name: "explicit false", data: `{"profile":"work-mac","push":false}`, want: false, source: "data.push"},
		{name: "server default false", data: `{"profile":"server"}`, want: false, source: "profile:server"},
		{name: "work mac default true", data: `{"profile":"work-mac"}`, want: true, source: "default"},
		{name: "personal mac default true", data: `{"profile":"personal-mac"}`, want: true, source: "default"},
		{name: "personal linux default true", data: `{"profile":"personal-linux"}`, want: true, source: "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(&fakeRunner{out: []byte(tt.data)})
			got, err := c.PushPolicy(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if got.Allowed != tt.want || got.Source != tt.source {
				t.Errorf("policy: want allowed=%v source=%q, got %+v", tt.want, tt.source, got)
			}
		})
	}
}
