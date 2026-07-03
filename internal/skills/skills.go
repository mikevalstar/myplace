// Package skills wraps the "skills" CLI (skills.sh, the Vercel-Labs npm package
// commonly run as `npx skills`) for read-only inventory of AI agent skills that
// have a newer version available upstream. Like internal/brew and internal/shelly
// it is present-if-installed: callers check Installed first and skip skills
// entirely when the CLI can't be resolved, so this package is safe to wire up on
// every machine (it self-reports unavailable on a headless server with no Node,
// or a box that has never installed a skill). It never installs, updates, or
// otherwise mutates anything — it only reads `skills check` (ADR-0023).
//
// The distinction this feature draws (ADR-0023): a user's OWN authored skills are
// managed as ordinary dotfiles by chezmoi (a `home/dot_claude/skills/` tree); the
// THIRD-PARTY "stable" of skills installed from git repos via this CLI are the
// external, updatable dependencies reported here — the mise-vs-chezmoi split,
// applied to skills. (The CLI's own canonical store is ~/.agents/skills/, which it
// symlinks into each agent's dir like ~/.claude/skills/; that tree is the CLI's,
// so chezmoi never manages it — same stance as not managing mise's install dir.)
// This inventory is INFORMATIONAL: like the rest of internal/outdated it never
// feeds the drift verdict or status exit codes (ADR-0010). The adapter that turns
// this into an outdated.Source lives in internal/outdated, so this package stays a
// thin CLI wrapper with no cross-imports.
package skills

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/mikevalstar/myplace/internal/run"
)

// ansi matches CSI escape sequences (colors, and cursor/erase codes like the
// stray "\x1b[K" the skills CLI emits) so `check`'s human output can be reduced
// to plain text before parsing.
var ansi = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

type Client struct {
	r    run.Runner
	home string

	// Resolved skills invocation, probed once (see resolve). argv is nil when
	// the CLI can't be found; probed guards the one-time probe.
	argv   []string
	probed bool
}

func New(r run.Runner) *Client {
	home, _ := os.UserHomeDir()
	return &Client{r: r, home: home}
}

// resolve picks how to invoke the skills CLI, probing once and caching the
// result. It prefers a global `skills` on PATH; failing that it falls back to
// `npx --no-install skills`, which runs a locally/globally installed skills
// package WITHOUT ever downloading it — so the probe stays offline and never
// blocks (the dashboard shells this out on every open, and run.Runner closes
// stdin so a prompt can't hang us either). Returns the argv prefix (binary +
// leading args) and whether the CLI is usable.
//
// Verified against skills v1.5.14: `npx --no-install skills --version` prints the
// version and exits 0 when the package is cached/installed, and errors cleanly
// otherwise — a good present-if-installed gate.
func (c *Client) resolve(ctx context.Context) ([]string, bool) {
	if c.probed {
		return c.argv, c.argv != nil
	}
	c.probed = true
	if _, err := c.r.Run(ctx, c.home, "skills", "--version"); err == nil {
		c.argv = []string{"skills"}
	} else if _, err := c.r.Run(ctx, c.home, "npx", "--no-install", "skills", "--version"); err == nil {
		c.argv = []string{"npx", "--no-install", "skills"}
	}
	return c.argv, c.argv != nil
}

// Installed reports whether the skills CLI can be resolved. Cheap (a --version
// probe) and side-effect-free.
func (c *Client) Installed(ctx context.Context) bool {
	_, ok := c.resolve(ctx)
	return ok
}

// Package is one skill with a newer version available. Field names match
// outdated.Package so the adapter in internal/outdated is a trivial conversion.
// Skills are git-based, so Current/Latest may be commit refs or semver tags —
// they're opaque display strings here, and either may be empty when `check`
// doesn't print a version delta.
type Package struct {
	Name    string
	Current string
	Latest  string
}

// upToDate matches the CLI's all-clear line ("✓ All global skills are up to
// date"), the reliably-observed happy path.
var upToDate = regexp.MustCompile(`(?i)up[ -]to[ -]date|no updates|all skills are current`)

// arrow normalizes the version-delta separators a "needs update" line might use.
var arrow = regexp.MustCompile(`\s*(?:→|->|=>)\s*`)

// ParseCheck derives the outdated set from `skills check`'s output.
//
// IMPORTANT (verified against skills v1.5.14): `skills check` emits ANSI-colored
// HUMAN text and ignores `--json` — unlike mise/brew/shelly, there is no
// machine-readable mode for the update check (only `skills list --json` is
// structured, and it carries no version info). So this parses text, the same way
// internal/chezmoi line-parses `chezmoi status`.
//
// Reliable, observed cases: the all-clear line ("✓ All global skills are up to
// date") and the per-source "Checking skills from source: <owner/repo>" progress
// lines both yield zero outdated skills. The "updates available" line format was
// NOT observable here (this machine's skills were current), so that branch is
// best-effort and tolerant: any non-noise line carrying a version arrow
// (`a → b`) or an update keyword is treated as one outdated skill. If a real
// outdated case prints a different shape, the fix is confined to this function
// and its tests — confirm against a machine with an outdated skill (see
// docs/features/outdated-packages.md).
func ParseCheck(out []byte) ([]Package, error) {
	text := ansi.ReplaceAllString(string(out), "")
	var pkgs []Package
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(strings.Trim(raw, "\r"))
		if line == "" {
			continue
		}
		low := strings.ToLower(line)
		// Progress/summary noise, and the all-clear line: never an outdated skill.
		if strings.HasPrefix(low, "checking") || upToDate.MatchString(low) {
			continue
		}
		if loc := arrow.FindStringIndex(line); loc != nil {
			name, cur, latest := parseArrowLine(line, loc)
			if name != "" {
				pkgs = append(pkgs, Package{Name: name, Current: cur, Latest: latest})
				continue
			}
		}
		if strings.Contains(low, "update available") || strings.Contains(low, "outdated") || strings.Contains(low, "needs update") {
			if name := skillName(line); name != "" {
				pkgs = append(pkgs, Package{Name: name, Latest: "update available"})
			}
		}
	}
	return pkgs, nil
}

// parseArrowLine pulls (name, current, latest) out of a "name cur → latest" line,
// given the arrow's location. The name is everything before the version token on
// the left; latest is the first token on the right.
func parseArrowLine(line string, loc []int) (name, cur, latest string) {
	left := strings.Fields(stripGlyphs(line[:loc[0]]))
	right := strings.Fields(stripGlyphs(line[loc[1]:]))
	if len(right) > 0 {
		latest = right[0]
	}
	switch len(left) {
	case 0:
		return "", "", latest
	case 1:
		return left[0], "", latest
	default:
		// Last token is the current version; everything before it is the name.
		return strings.Join(left[:len(left)-1], " "), left[len(left)-1], latest
	}
}

// skillName extracts a plausible skill identifier from a keyword line: the last
// path-ish/identifier token on the line, minus leading status glyphs.
func skillName(line string) string {
	fields := strings.Fields(stripGlyphs(line))
	for _, f := range fields {
		f = strings.Trim(f, ":()[]\"'`")
		if f != "" && !isNoiseWord(f) {
			return f
		}
	}
	return ""
}

func isNoiseWord(s string) bool {
	switch strings.ToLower(s) {
	case "skill", "update", "available", "outdated", "needs", "has", "an", "a", "the", "is":
		return true
	}
	return false
}

// stripGlyphs drops the leading status glyphs the CLI decorates lines with
// (✓ ✗ ⨯ ↑ • - *) so tokenizing sees the content.
func stripGlyphs(s string) string {
	return strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(s), "✓✗⨯✔✖↑•-*·>»"))
}

// Outdated returns every installed skill with a newer version available. It
// reads only — it never runs `skills update`/`add`, so it never mutates the
// installed set. It runs `check -g` (global scope): myplace's concern is the
// machine's global skill set, and `-g` avoids the interactive scope prompt off a
// TTY (run.Runner closes stdin, so an un-forced prompt would fail fast anyway).
// The CLI exits 0 when up to date; a non-zero exit that still produced output is
// tolerated (same stance as brew/mise/shelly) so a "flag updates via exit code"
// build doesn't turn a real result into an error.
func (c *Client) Outdated(ctx context.Context) ([]Package, error) {
	argv, ok := c.resolve(ctx)
	if !ok {
		return nil, fmt.Errorf("skills: CLI not available")
	}
	name := argv[0]
	args := append(append([]string{}, argv[1:]...), "check", "-g")
	out, err := c.r.Run(ctx, c.home, name, args...)
	if err != nil && len(strings.TrimSpace(ansi.ReplaceAllString(string(out), ""))) == 0 {
		return nil, err
	}
	return ParseCheck(out)
}
