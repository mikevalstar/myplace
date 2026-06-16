package drift

import "testing"

func intp(n int) *int    { return &n }
func boolp(b bool) *bool { return &b }

func TestDecide(t *testing.T) {
	clean := Dotfiles{BehindOrigin: intp(0), UncommittedFiles: intp(0), UnpushedCommits: intp(0)}
	noTools := Tools{}

	strp := func(s string) *string { return &s }
	cases := []struct {
		name     string
		d        Dotfiles
		t        Tools
		m        Myplace
		unknown  bool
		fatal    bool
		want     string
		wantExit int
	}{
		{"myplace outdated", clean, Tools{}, Myplace{Current: "0.1.0", Latest: strp("0.2.0")}, false, false, VerdictDrifted, 1},
		{"myplace current", clean, Tools{}, Myplace{Current: "0.2.0", Latest: strp("0.2.0")}, false, false, VerdictInSync, 0},
		{"all clean", clean, noTools, Myplace{}, false, false, VerdictInSync, 0},
		{"behind origin", Dotfiles{BehindOrigin: intp(2), UncommittedFiles: intp(0), UnpushedCommits: intp(0)}, noTools, Myplace{}, false, false, VerdictDrifted, 1},
		{"local modification", Dotfiles{LocalModified: []string{".zshrc"}, BehindOrigin: intp(0)}, noTools, Myplace{}, false, false, VerdictDrifted, 1},
		{"unpushed commits", Dotfiles{BehindOrigin: intp(0), UnpushedCommits: intp(1)}, noTools, Myplace{}, false, false, VerdictDrifted, 1},
		{"unpushed commits parked by policy", Dotfiles{BehindOrigin: intp(0), UnpushedCommits: intp(1), PushAllowed: boolp(false)}, noTools, Myplace{}, false, false, VerdictInSync, 0},
		{"uncommitted files still drift when push disabled", Dotfiles{BehindOrigin: intp(0), UncommittedFiles: intp(1), UnpushedCommits: intp(1), PushAllowed: boolp(false)}, noTools, Myplace{}, false, false, VerdictDrifted, 1},
		{"missing tool", clean, Tools{Missing: []string{"node"}}, Myplace{}, false, false, VerdictDrifted, 1},
		{"outdated tool", clean, Tools{Outdated: []ToolIssue{{Name: "node"}}}, Myplace{}, false, false, VerdictDrifted, 1},
		{"offline but otherwise clean", Dotfiles{}, noTools, Myplace{}, true, false, VerdictUnknown, 2},
		{"drift wins over unknown", Dotfiles{ToApply: []string{".zshrc"}}, noTools, Myplace{}, true, false, VerdictDrifted, 1},
		{"not bootstrapped", Dotfiles{}, noTools, Myplace{}, false, true, VerdictError, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Decide(c.d, c.t, c.m, c.unknown, c.fatal)
			if got != c.want {
				t.Errorf("verdict: want %s, got %s", c.want, got)
			}
			if code := ExitCode(got); code != c.wantExit {
				t.Errorf("exit code: want %d, got %d", c.wantExit, code)
			}
		})
	}
}
