// Package toolchain reports whether the core tools the whole setup stands on are
// current: mise, chezmoi, rustup + the Rust stable toolchain, Go, and fnm.
//
// Why this exists (ADR-0027): every other outdated source reports what some
// package manager manages, and none of these tools is managed by one. `mise
// outdated` lists the tools *mise* manages, not mise; the cargo source covers
// `cargo install`ed binaries, not rustup or Rust itself. So the layer myplace
// depends on was watched by nobody — a two-month-stale mise silently broke a
// tool install (a since-fixed upstream attestation bug) while `doctor` read
// green, because doctor checks version *floors*, not currency.
//
// The split is deliberate: doctor answers "is this new enough to drive?" (a
// gate), this answers "is this current?" (inventory, never a gate).
//
// Strictly READ-ONLY: nothing here runs `mise self-update`, `rustup update`, or
// `chezmoi upgrade`. It reports; the human upgrades (ADR-0010's stance on brew
// and cargo, applied to the toolchain).
//
// Like internal/brew and internal/cargo this is present-if-installed per member:
// a member whose binary isn't on PATH is omitted rather than reported, so a
// server with no Rust simply yields no rustup/rust rows.
//
// The adapter that turns this into an outdated.Source lives in internal/outdated,
// so this package stays a thin wrapper with no cross-imports between clients.
package toolchain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mikevalstar/myplace/internal/release"
	"github.com/mikevalstar/myplace/internal/run"
)

type Client struct {
	r run.Runner
}

func New(r run.Runner) *Client { return &Client{r: r} }

// Package is one core tool with a newer version available. Field names match
// outdated.Package so the adapter is a trivial conversion.
type Package struct {
	Name    string
	Current string
	Latest  string
}

// lookupTimeout caps every probe this package makes — the HTTP lookups *and*
// the child commands. Without it a black-holed network (a dead proxy, a captive
// portal) hangs the dashboard pane instead of degrading to "couldn't check";
// measured at 75s before the commands were capped. It matches the 5s budget
// drift uses for its own release lookup.
//
// The commands need it as much as the HTTP does, which is easy to miss: `rustup
// check` queries the Rust release channels, and `mise version` runs mise's own
// upstream version check. Neither is the local, instant operation it looks like.
const lookupTimeout = 5 * time.Second

// GoVersionURL is go.dev's plaintext version endpoint: two lines, the first
// being e.g. "go1.27.0". Go has no GitHub release to read (the golang/go repo
// tags releases but publishes no release objects), and this endpoint is the
// documented way to ask "what's current?".
const GoVersionURL = "https://go.dev/VERSION?m=text"

// ghMember is a core tool whose upstream truth is a GitHub release.
type ghMember struct {
	name string
	repo string
	bin  string
	args []string
}

var ghMembers = []ghMember{
	{name: "mise", repo: "jdx/mise", bin: "mise", args: []string{"version"}},
	{name: "chezmoi", repo: "twpayne/chezmoi", bin: "chezmoi", args: []string{"--version"}},
	// fnm is skipped when Homebrew owns it — see brewOwned.
	{name: "fnm", repo: "Schniz/fnm", bin: "fnm", args: []string{"--version"}},
}

// dotted matches the first X.Y or X.Y.Z in a version banner, which is all these
// tools' `--version` output has in common:
//
//	mise:    "2026.8.15 macos-arm64 (2026-08-30)"
//	chezmoi: "chezmoi version v2.70.5, commit b81bd8d…, built at …"
//	go:      "go version go1.26.4 darwin/arm64"
//	fnm:     "fnm 1.39.0"
var dotted = regexp.MustCompile(`\d+\.\d+(?:\.\d+)?`)

// parseVersion pulls the version out of a banner, tolerating each tool's
// decorations. Empty when there's nothing version-shaped, which callers treat as
// "couldn't determine" rather than as an error.
func parseVersion(out []byte) string { return dotted.FindString(string(out)) }

// Installed reports whether this looks like a myplace-managed machine at all.
// mise and chezmoi are always present on one, so resolving either is enough; a
// box with neither has no core toolchain to report on. Cheap and offline — no
// upstream lookup happens here.
func (c *Client) Installed(ctx context.Context) bool {
	if _, err := c.probe(ctx, "mise", "version"); err == nil {
		return true
	}
	_, err := c.probe(ctx, "chezmoi", "--version")
	return err == nil
}

// probe runs one child command under the lookup budget. Every command this
// package issues goes through here, so none of them can hang the caller.
func (c *Client) probe(ctx context.Context, bin string, args ...string) ([]byte, error) {
	pctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	return c.r.Run(pctx, "", bin, args...)
}

// memberResult is one member's contribution, kept separate so a single failed
// upstream lookup doesn't blank the whole source. compared records that both
// sides of the comparison were known — an absent member sets neither field, so
// "no rows" and "couldn't tell" stay distinguishable.
type memberResult struct {
	pkgs     []Package
	err      error
	compared bool // the member's local and upstream versions were both known
}

// Outdated returns every core tool with a newer version upstream.
//
// Members are probed concurrently — three GitHub lookups plus go.dev, each
// sharing the caller's context — because they're independent network round
// trips and running them in series would make the dashboard pane the slowest
// thing on screen. rustup costs no HTTP at all: `rustup check` reports both
// sides itself.
//
// Partial failure returns BOTH the comparisons that succeeded and an error
// naming the members that didn't. Neither half is droppable: losing the rows
// would hide real staleness, and losing the error would make a partial answer
// look complete — which is exactly how a two-month-stale mise stayed invisible
// long enough to break a tool install (ADR-0027). outdated.Collect records both
// fields, and ExitCode still counts the rows.
func (c *Client) Outdated(ctx context.Context) ([]Package, error) {
	var (
		mu      sync.Mutex
		results []memberResult
		wg      sync.WaitGroup
	)
	add := func(res memberResult) {
		mu.Lock()
		results = append(results, res)
		mu.Unlock()
	}

	for _, m := range ghMembers {
		wg.Add(1)
		go func(m ghMember) {
			defer wg.Done()
			add(c.checkGitHub(ctx, m))
		}(m)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		add(c.checkGo(ctx))
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		add(c.checkRustup(ctx))
	}()
	wg.Wait()

	var (
		pkgs []Package
		errs []error
	)
	for _, res := range results {
		pkgs = append(pkgs, res.pkgs...)
		if res.err != nil {
			errs = append(errs, res.err)
		}
	}
	sortByMemberOrder(pkgs)
	if len(errs) > 0 {
		// Whether or not anything compared, say what couldn't be checked. When
		// nothing compared at all this is the only signal there is, and an
		// offline box gets one clear error instead of a silent empty list.
		return pkgs, errors.Join(errs...)
	}
	return pkgs, nil
}

// checkGitHub compares a member's local version against its newest GitHub
// release. A missing binary is not an error — the member is simply absent from
// this machine.
func (c *Client) checkGitHub(ctx context.Context, m ghMember) memberResult {
	out, err := c.probe(ctx, m.bin, m.args...)
	if err != nil {
		return memberResult{} // not installed here
	}
	current := parseVersion(out)
	if current == "" {
		return memberResult{err: fmt.Errorf("%s: unrecognized version banner", m.name)}
	}
	// Leave anything Homebrew owns to the brew source rather than reporting it
	// twice on one dashboard. This bites fnm in practice: the provision script
	// installs it to ~/.local/bin only when it isn't already on PATH, so on a
	// Mac it's usually brew's. It applies to any member for the same reason.
	if brewOwned(m.bin) {
		return memberResult{}
	}
	lctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	tag, err := release.LatestTagIn(lctx, m.repo)
	if err != nil {
		return memberResult{err: fmt.Errorf("%s: %w", m.name, err)}
	}
	return compare(m.name, current, release.NormalizeTag(tag))
}

// checkGo compares the `go` on PATH against go.dev's current release. Go is
// covered here however it was installed: a machine-wide inventory shouldn't care
// whether it came from mise, brew, or a tarball.
func (c *Client) checkGo(ctx context.Context) memberResult {
	out, err := c.probe(ctx, "go", "version")
	if err != nil {
		return memberResult{}
	}
	current := parseVersion(out)
	if current == "" {
		return memberResult{err: errors.New("go: unrecognized version banner")}
	}
	lctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	latest, err := latestGo(lctx)
	if err != nil {
		return memberResult{err: fmt.Errorf("go: %w", err)}
	}
	return compare("go", current, latest)
}

// latestGo reads go.dev's plaintext endpoint, whose first line is "go1.27.0"
// (the second is a timestamp).
func latestGo(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GoVersionURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("go.dev VERSION: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if err != nil {
		return "", err
	}
	v := parseVersion(body)
	if v == "" {
		return "", errors.New("go.dev VERSION: no version in response")
	}
	return v, nil
}

// checkRustup reports rustup and the Rust toolchains from a single `rustup
// check`, which needs no HTTP of ours — it queries the release channels itself
// and prints both the installed and available versions.
func (c *Client) checkRustup(ctx context.Context) memberResult {
	// `rustup check` reaches out to the release channels, so this is a networked
	// probe despite looking like a local one — the budget matters here most.
	out, err := c.probe(ctx, "rustup", "check")
	if err != nil && len(strings.TrimSpace(string(out))) == 0 {
		if ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
			return memberResult{err: errors.New("rustup: check timed out")}
		}
		return memberResult{} // no rustup on this machine
	}
	pkgs := ParseRustupCheck(out)
	// `rustup check` reports both sides for every line it prints, so reaching
	// here at all means the comparison happened — an empty result means
	// "everything current", not "couldn't tell".
	return memberResult{pkgs: pkgs, compared: true}
}

// ParseRustupCheck extracts the outdated set from `rustup check`:
//
//	stable-aarch64-apple-darwin - update available: 1.98.0 (88d9e12ae 2026-08-18) -> 1.99.0 (abc123def 2026-09-29)
//	rustup - up to date : 1.29.0
//
// "up to date" lines are skipped. Crucially, so are "update available" lines
// whose two sides are the *same* version — rustup really does emit those (seen
// on the primary Mac: `1.98.0 (88d9e12ae 2026-08-18) -> 1.98.0 (88d9e12ae
// 2026-08-18)`), and passing one through would put a permanent phantom "rust is
// outdated" row on the dashboard.
//
// Unparseable lines are skipped rather than erroring: a cosmetic upstream change
// should degrade to "nothing outdated", not to a failed source.
func ParseRustupCheck(out []byte) []Package {
	var pkgs []Package
	for _, raw := range strings.Split(string(out), "\n") {
		line := strings.TrimSpace(strings.Trim(raw, "\r"))
		name, rest, ok := strings.Cut(line, " - ")
		if !ok {
			continue
		}
		if !strings.Contains(rest, "update available") {
			continue // "up to date", or anything else rustup decides to say
		}
		_, versions, ok := strings.Cut(rest, ":")
		if !ok {
			continue
		}
		curRaw, latestRaw, ok := strings.Cut(versions, "->")
		if !ok {
			continue
		}
		cur, latest := parseVersion([]byte(curRaw)), parseVersion([]byte(latestRaw))
		if cur == "" || latest == "" || !lessVersion(cur, latest) {
			continue
		}
		pkgs = append(pkgs, Package{Name: rustName(strings.TrimSpace(name)), Current: cur, Latest: latest})
	}
	return pkgs
}

// rustName turns a rustup toolchain id into an inventory-friendly name:
// "stable-aarch64-apple-darwin" is the Rust everyone means, so it reads "rust";
// other channels keep their channel so the row isn't ambiguous. "rustup" (the
// installer itself) passes through.
func rustName(id string) string {
	switch {
	case id == "rustup":
		return "rustup"
	case strings.HasPrefix(id, "stable"):
		return "rust"
	case strings.HasPrefix(id, "beta"):
		return "rust (beta)"
	case strings.HasPrefix(id, "nightly"):
		return "rust (nightly)"
	default:
		return id
	}
}

// compare emits a row only when upstream is genuinely ahead. Equality is not
// enough to decide this: a locally built or pre-release binary can be *newer*
// than the newest published release, and reporting that as an available update
// would be wrong.
func compare(name, current, latest string) memberResult {
	if latest == "" {
		return memberResult{err: fmt.Errorf("%s: no upstream version", name)}
	}
	res := memberResult{compared: true}
	if lessVersion(current, latest) {
		res.pkgs = []Package{{Name: name, Current: current, Latest: latest}}
	}
	return res
}

// lessVersion reports whether a is an earlier version than b, comparing
// dot-separated numeric segments left to right. It covers every scheme in play
// here — semver (chezmoi, rustup, Rust, fnm), Go's x.y.z, and mise's calver
// (2026.8.15) — because all of them are numeric-dotted and compare correctly
// segment-wise. A non-numeric segment sorts as 0, so a weird build string
// degrades to "not newer" rather than to a false update row.
func lessVersion(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		av, bv := segment(as, i), segment(bs, i)
		if av != bv {
			return av < bv
		}
	}
	return false
}

func segment(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(parts[i]))
	if err != nil {
		return 0
	}
	return n
}

// brewOwned reports whether a binary on PATH is Homebrew's, so the toolchain
// source can leave it to the brew source instead of double-reporting it. The
// symlink is resolved first: brew puts a link in <prefix>/bin pointing into
// <prefix>/Cellar, and on an Intel Mac that link lives in the very generic
// /usr/local/bin — the Cellar path is what actually identifies it.
func brewOwned(name string) bool {
	p, err := exec.LookPath(name)
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	return strings.Contains(p, "/Cellar/") || strings.Contains(p, "/homebrew/")
}

// memberOrder is the display order: the two tools myplace itself drives first
// (they're the ones whose staleness breaks myplace), then the language
// toolchains. Keeps the pane stable across runs despite the concurrent probes.
var memberOrder = []string{"mise", "chezmoi", "rustup", "rust", "go", "fnm"}

func sortByMemberOrder(pkgs []Package) {
	rank := func(name string) int {
		for i, n := range memberOrder {
			if name == n || strings.HasPrefix(name, n+" ") {
				return i
			}
		}
		return len(memberOrder)
	}
	// Insertion sort: this list is at most a handful of rows.
	for i := 1; i < len(pkgs); i++ {
		for j := i; j > 0 && rank(pkgs[j].Name) < rank(pkgs[j-1].Name); j-- {
			pkgs[j], pkgs[j-1] = pkgs[j-1], pkgs[j]
		}
	}
}
