// Package pacman wraps the read-only update checkers of an Arch-family system
// (Omarchy, plain Arch — ADR-0026) for `myplace outdated`. Like internal/brew and
// internal/shelly it is present-if-installed: callers check Installed first and
// skip it when `checkupdates` (pacman-contrib) is not on PATH, so it is safe to
// wire up on every OS. It never installs, upgrades, or syncs the real package
// database: `checkupdates` refreshes a *temporary* copy of the sync DB under
// $TMPDIR and diffs it against what is installed, and `yay -Qua` asks the AUR
// RPC for newer versions of foreign packages. `pacman -Syu` is never run from
// here — on Omarchy that is `omarchy update`'s job (migrations, keyring, mise
// up, firmware). The adapter that turns this into an outdated.Source lives in
// internal/outdated (ADR-0010).
package pacman

import (
	"bufio"
	"bytes"
	"context"
	"regexp"
	"strings"

	"github.com/mikevalstar/myplace/internal/run"
)

type Client struct {
	r run.Runner
}

func New(r run.Runner) *Client {
	return &Client{r: r}
}

// Installed reports whether `checkupdates` is on PATH. It is the one required
// tool; the AUR helper is optional on top (see Outdated).
func (c *Client) Installed(ctx context.Context) bool {
	_, err := c.r.Run(ctx, "", "checkupdates", "--version")
	return err == nil
}

// Package is one outdated package. Field names match outdated.Package so the
// adapter in internal/outdated is a trivial conversion.
type Package struct {
	Name    string
	Current string
	Latest  string
}

// ansi strips CSI sequences: checkupdates takes --nocolor, but yay colours its
// output whenever it feels like it.
var ansi = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// ParseList parses the `name old -> new` lines both `checkupdates` and
// `yay -Qua` print (one package per line; yay may indent or colour them, and
// may append " [ignored]" for IgnorePkg entries, which are kept — an update
// is still waiting). prefix is prepended to every name (`aur:` for the AUR
// channel) so the two channels stay distinguishable in one source row. Lines
// that don't fit the shape are skipped, never an error: these tools print
// nothing else on stdout in normal operation, but a warning line must not
// turn a whole check into a parse failure.
func ParseList(out []byte, prefix string) []Package {
	var pkgs []Package
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(ansi.ReplaceAllString(sc.Text(), ""))
		if line == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 4 || f[2] != "->" {
			continue
		}
		pkgs = append(pkgs, Package{Name: prefix + f[0], Current: f[1], Latest: f[3]})
	}
	return pkgs
}

// Outdated returns the packages with a newer version in the sync repos
// (`checkupdates --nocolor`) plus, when an AUR helper is present, foreign
// packages behind the AUR (`yay -Qua`, name-prefixed `aur:`). Both tools use
// their exit code as a status — checkupdates exits 2 and yay exits 1 when there
// is nothing to update — so a "no updates" exit with empty output is a clean
// empty result, and only a real failure (a different code, or anything on
// stderr) is returned as an error. The AUR check is best-effort: yay missing
// or failing (it needs the AUR RPC) never hides the repo result.
func (c *Client) Outdated(ctx context.Context) ([]Package, error) {
	out, err := c.r.Run(ctx, "", "checkupdates", "--nocolor")
	if err != nil && !noUpdates(err, out, 2) {
		return nil, err
	}
	pkgs := ParseList(out, "")

	if _, err := c.r.Run(ctx, "", "yay", "--version"); err == nil {
		out, err := c.r.Run(ctx, "", "yay", "-Qua")
		if err == nil || noUpdates(err, out, 1) {
			pkgs = append(pkgs, ParseList(out, "aur:")...)
		}
	}
	return pkgs, nil
}

// noUpdates reports whether err is the tool's documented "nothing to do" exit:
// the given code, no stdout, and nothing on stderr. Anything else is a failure.
func noUpdates(err error, out []byte, code int) bool {
	c, stderr, ok := run.ExitCode(err)
	return ok && c == code && len(bytes.TrimSpace(out)) == 0 && stderr == ""
}
