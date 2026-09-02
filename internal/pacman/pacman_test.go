package pacman

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mikevalstar/myplace/internal/run"
)

func TestParseList(t *testing.T) {
	// checkupdates --nocolor output (pacman-contrib 1.13): one `name old -> new`
	// per line. yay -Qua prints the same shape, sometimes indented/coloured,
	// and appends " [ignored]" for IgnorePkg entries.
	out := "linux 7.1.9.arch1-2 -> 7.1.10.arch1-1\nmesa 1:26.1.4-1 -> 1:26.1.5-1\n"
	pkgs := ParseList([]byte(out), "")
	if len(pkgs) != 2 {
		t.Fatalf("want 2 packages, got %d: %v", len(pkgs), pkgs)
	}
	if pkgs[0] != (Package{Name: "linux", Current: "7.1.9.arch1-2", Latest: "7.1.10.arch1-1"}) {
		t.Errorf("linux: got %+v", pkgs[0])
	}
	if pkgs[1].Name != "mesa" || pkgs[1].Current != "1:26.1.4-1" || pkgs[1].Latest != "1:26.1.5-1" {
		t.Errorf("mesa (epoch version): got %+v", pkgs[1])
	}

	aur := " \x1b[1myay\x1b[0m 12.4.0-1 -> 12.4.1-1\nfnm 1.38.1-1 -> 1.38.2-1 [ignored]\n:: some warning line\n"
	pkgs = ParseList([]byte(aur), "aur:")
	if len(pkgs) != 2 {
		t.Fatalf("want 2 AUR packages (warning line skipped), got %d: %v", len(pkgs), pkgs)
	}
	if pkgs[0] != (Package{Name: "aur:yay", Current: "12.4.0-1", Latest: "12.4.1-1"}) {
		t.Errorf("coloured/indented aur line: got %+v", pkgs[0])
	}
	if pkgs[1].Name != "aur:fnm" || pkgs[1].Latest != "1.38.2-1" {
		t.Errorf("[ignored] entry should be kept: got %+v", pkgs[1])
	}
}

func TestParseListEmpty(t *testing.T) {
	for _, in := range []string{"", "\n", "   \n"} {
		if got := ParseList([]byte(in), ""); len(got) != 0 {
			t.Errorf("%q: want none, got %v", in, got)
		}
	}
}

// fakeRunner scripts responses per command line ("checkupdates --nocolor").
type fakeRunner struct {
	out map[string]string
	err map[string]error
}

func (f fakeRunner) Run(_ context.Context, _ string, name string, args ...string) ([]byte, error) {
	key := strings.TrimSpace(name + " " + strings.Join(args, " "))
	return []byte(f.out[key]), f.err[key]
}

func TestOutdatedNothingToDo(t *testing.T) {
	// The documented status exits — checkupdates 2, yay 1 — with silent output
	// are "nothing outdated", not failures.
	r := fakeRunner{
		out: map[string]string{},
		err: map[string]error{
			"checkupdates --nocolor": &run.Error{Msg: "checkupdates --nocolor: exit status 2", Code: 2},
			"yay -Qua":               &run.Error{Msg: "yay -Qua: exit status 1", Code: 1},
		},
	}
	pkgs, err := New(r).Outdated(context.Background())
	if err != nil {
		t.Fatalf("status exits must not error: %v", err)
	}
	if len(pkgs) != 0 {
		t.Fatalf("want none, got %v", pkgs)
	}
}

func TestOutdatedMergesChannels(t *testing.T) {
	r := fakeRunner{
		out: map[string]string{
			"checkupdates --nocolor": "linux 1-1 -> 1-2\n",
			"yay -Qua":               "fnm 1.0-1 -> 1.1-1\n",
		},
		err: map[string]error{},
	}
	pkgs, err := New(r).Outdated(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 2 || pkgs[0].Name != "linux" || pkgs[1].Name != "aur:fnm" {
		t.Fatalf("want repo row then aur: row, got %v", pkgs)
	}
}

func TestOutdatedRealFailure(t *testing.T) {
	// checkupdates exit 1 (or any exit with stderr) is a real failure.
	r := fakeRunner{
		out: map[string]string{},
		err: map[string]error{
			"checkupdates --nocolor": &run.Error{Msg: "checkupdates --nocolor: ERROR: Cannot fetch updates", Stderr: "ERROR: Cannot fetch updates", Code: 1},
		},
	}
	if _, err := New(r).Outdated(context.Background()); err == nil {
		t.Fatal("expected an error for a real checkupdates failure")
	}
	// Exit 2 *with* stderr is a failure too — the status exit is the silent one.
	r.err["checkupdates --nocolor"] = &run.Error{Msg: "x", Stderr: "ERROR: something", Code: 2}
	if _, err := New(r).Outdated(context.Background()); err == nil {
		t.Fatal("expected an error when exit 2 carries stderr")
	}
	// A non-run.Error (fake runner, wrapped error) is never mistaken for a status.
	r.err["checkupdates --nocolor"] = errors.New("boom")
	if _, err := New(r).Outdated(context.Background()); err == nil {
		t.Fatal("expected a plain error to propagate")
	}
}

func TestOutdatedAURFailureIsBestEffort(t *testing.T) {
	// A broken/offline yay never hides the repo result.
	r := fakeRunner{
		out: map[string]string{"checkupdates --nocolor": "linux 1-1 -> 1-2\n"},
		err: map[string]error{"yay -Qua": &run.Error{Msg: "yay: rpc failed", Stderr: "rpc failed", Code: 1}},
	}
	pkgs, err := New(r).Outdated(context.Background())
	if err != nil || len(pkgs) != 1 || pkgs[0].Name != "linux" {
		t.Fatalf("got %v, %v", pkgs, err)
	}
}
