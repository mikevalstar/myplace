package drift

import "testing"

func intp(n int) *int { return &n }

func TestDecide(t *testing.T) {
	clean := Dotfiles{BehindOrigin: intp(0), UncommittedFiles: intp(0), UnpushedCommits: intp(0)}
	noTools := Tools{}

	cases := []struct {
		name     string
		d        Dotfiles
		t        Tools
		unknown  bool
		fatal    bool
		want     string
		wantExit int
	}{
		{"all clean", clean, noTools, false, false, VerdictInSync, 0},
		{"behind origin", Dotfiles{BehindOrigin: intp(2), UncommittedFiles: intp(0), UnpushedCommits: intp(0)}, noTools, false, false, VerdictDrifted, 1},
		{"local modification", Dotfiles{LocalModified: []string{".zshrc"}, BehindOrigin: intp(0)}, noTools, false, false, VerdictDrifted, 1},
		{"unpushed commits", Dotfiles{BehindOrigin: intp(0), UnpushedCommits: intp(1)}, noTools, false, false, VerdictDrifted, 1},
		{"missing tool", clean, Tools{Missing: []string{"node"}}, false, false, VerdictDrifted, 1},
		{"outdated tool", clean, Tools{Outdated: []ToolIssue{{Name: "node"}}}, false, false, VerdictDrifted, 1},
		{"offline but otherwise clean", Dotfiles{}, noTools, true, false, VerdictUnknown, 2},
		{"drift wins over unknown", Dotfiles{ToApply: []string{".zshrc"}}, noTools, true, false, VerdictDrifted, 1},
		{"not bootstrapped", Dotfiles{}, noTools, false, true, VerdictError, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Decide(c.d, c.t, c.unknown, c.fatal)
			if got != c.want {
				t.Errorf("verdict: want %s, got %s", c.want, got)
			}
			if code := ExitCode(got); code != c.wantExit {
				t.Errorf("exit code: want %d, got %d", c.wantExit, code)
			}
		})
	}
}
