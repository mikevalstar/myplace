// Package doctor runs read-only preflight diagnostics: it inspects whether this
// machine can run myplace's other commands (chezmoi/mise present and recent,
// PATH sane, dotfiles repo + GitHub reachable, state dir writable) and, for
// anything wrong, names the remedy. It is TUI-free and never mutates managed
// state (ADR-0002, ADR-0006); see docs/features/doctor-preflight-diagnostics.md.
package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mikevalstar/myplace/internal/chezmoi"
	"github.com/mikevalstar/myplace/internal/logging"
	"github.com/mikevalstar/myplace/internal/mise"
	"github.com/mikevalstar/myplace/internal/release"
)

// Schema is bumped only on breaking changes to the JSON shape.
const Schema = 1

// Per-check status values.
const (
	StatusPass = "pass"
	StatusWarn = "warn"
	StatusFail = "fail"
)

// Overall verdict values; ExitCode maps these to the CLI contract.
const (
	VerdictPass       = "pass"
	VerdictIncomplete = "incomplete"
	VerdictFail       = "fail"
)

// Minimum tool versions myplace relies on. Kept deliberately low — these are
// floors below which myplace's own invocations are known to break (chezmoi 2.x
// status columns/--no-tty/data --format json; mise calver ls/outdated --json),
// not a "stay current" nudge. status, not doctor, reports outdated tools.
const (
	chezmoiFloor = "2.0.0"
	miseFloor    = "2024.1.0"
)

// Check is one diagnostic: a stable id, a pass/warn/fail status, a one-line
// detail, and (when not passing) a concrete remedy naming the value involved.
type Check struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	Remedy string `json:"remedy,omitempty"`
	Label  string `json:"-"` // human-facing label for the text render
}

// Report is the `myplace doctor --json` document.
type Report struct {
	Schema    int       `json:"schema"`
	Machine   string    `json:"machine"`
	Profile   string    `json:"profile,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
	Verdict   string    `json:"verdict"`
	Checks    []Check   `json:"checks"`
}

// Options carries environment facts the cmd layer resolves and hands in, so the
// core stays pure and testable: the user's real PATH (captured before myplace
// prepended its own bin/shim dirs) and whether stdout is a TTY.
type Options struct {
	PATH      string
	StdoutTTY bool
}

// ExitCode maps a verdict to the CLI contract: 0 ready, 1 a check failed,
// 2 checks incomplete (e.g. offline), 3 error.
func ExitCode(verdict string) int {
	switch verdict {
	case VerdictPass:
		return 0
	case VerdictIncomplete:
		return 2
	case VerdictFail:
		return 1
	default:
		return 3
	}
}

// Run executes every check and returns a populated report. One failing check
// never aborts the rest, so a single invocation surfaces every problem.
func Run(ctx context.Context, ch *chezmoi.Client, ms *mise.Client, opts Options) Report {
	r := Report{Schema: Schema, CheckedAt: time.Now().UTC()}
	if h, err := os.Hostname(); err == nil {
		r.Machine = h
	}

	czInstalled := ch.Installed(ctx)
	miseInstalled := ms.Installed(ctx)

	r.add(checkInstalled("chezmoi_installed", "chezmoi installed", czInstalled,
		func() (string, error) { return ch.Version(ctx) },
		"install chezmoi (myplace bootstrap installs it) — https://www.chezmoi.io/install/"))
	r.add(checkVersion("chezmoi_version", "chezmoi version", czInstalled,
		func() (string, error) { return ch.Version(ctx) }, chezmoiFloor, "chezmoi upgrade"))

	r.add(checkInstalled("mise_installed", "mise installed", miseInstalled,
		func() (string, error) { return ms.Version(ctx) },
		"install mise (myplace bootstrap installs it) — https://mise.jdx.dev"))
	r.add(checkVersion("mise_version", "mise version", miseInstalled,
		func() (string, error) { return ms.Version(ctx) }, miseFloor, "mise self-update"))

	r.add(checkPATH(opts.PATH))

	initialized := czInstalled && ch.Initialized(ctx)
	if initialized {
		if p, err := ch.Profile(ctx); err == nil {
			r.Profile = p
		}
	}
	r.add(checkInitialized(czInstalled, initialized, r.Profile))
	r.add(checkDotfilesRemote(ctx, ch, initialized))
	r.add(checkGitHub(ctx))
	r.add(checkStateDir())
	r.add(checkTTY(opts.StdoutTTY))

	r.Verdict = decide(r.Checks)
	return r
}

func (r *Report) add(c Check) { r.Checks = append(r.Checks, c) }

// decide picks the verdict: any fail loses (machine not ready); otherwise any
// warn means a check couldn't complete (e.g. offline) and we can't fully
// assert readiness; all pass is ready. Mirrors the tool-wide 0/1/2 convention.
func decide(checks []Check) string {
	hasFail, hasWarn := false, false
	for _, c := range checks {
		switch c.Status {
		case StatusFail:
			hasFail = true
		case StatusWarn:
			hasWarn = true
		}
	}
	switch {
	case hasFail:
		return VerdictFail
	case hasWarn:
		return VerdictIncomplete
	default:
		return VerdictPass
	}
}

func checkInstalled(id, label string, installed bool, version func() (string, error), remedy string) Check {
	c := Check{ID: id, Label: label}
	if !installed {
		c.Status = StatusFail
		c.Detail = "not found on PATH"
		c.Remedy = remedy
		return c
	}
	c.Status = StatusPass
	if v, err := version(); err == nil && v != "" {
		c.Detail = v
	}
	return c
}

func checkVersion(id, label string, installed bool, version func() (string, error), floor, remedy string) Check {
	c := Check{ID: id, Label: label}
	if !installed {
		c.Status = StatusFail
		c.Detail = "not installed"
		c.Remedy = remedy
		return c
	}
	v, err := version()
	if err != nil || v == "" {
		// Installed but the banner didn't parse — don't claim a failure we
		// can't substantiate; degrade to a (non-blocking) warning.
		c.Status = StatusWarn
		c.Detail = "could not determine version"
		return c
	}
	if compareDotted(v, floor) < 0 {
		c.Status = StatusFail
		c.Detail = fmt.Sprintf("%s is older than the required %s", v, floor)
		c.Remedy = remedy
		return c
	}
	c.Status = StatusPass
	c.Detail = v
	return c
}

func checkPATH(path string) Check {
	c := Check{ID: "path", Label: "PATH"}
	home, _ := os.UserHomeDir()
	bin := filepath.Join(home, ".local", "bin")
	if pathHas(path, bin) {
		c.Status = StatusPass
		c.Detail = "~/.local/bin on PATH"
		return c
	}
	c.Status = StatusFail
	c.Detail = "~/.local/bin not on PATH"
	c.Remedy = `add 'export PATH="$HOME/.local/bin:$PATH"' to your shell profile`
	return c
}

func pathHas(path, dir string) bool {
	for _, p := range filepath.SplitList(path) {
		if p == dir {
			return true
		}
	}
	return false
}

func checkInitialized(czInstalled, initialized bool, profile string) Check {
	c := Check{ID: "chezmoi_initialized", Label: "chezmoi initialized"}
	switch {
	case !czInstalled:
		c.Status = StatusFail
		c.Detail = "chezmoi not installed"
		c.Remedy = "install chezmoi, then run myplace bootstrap"
	case !initialized:
		c.Status = StatusFail
		c.Detail = "no source state on this machine"
		c.Remedy = "myplace bootstrap"
	default:
		c.Status = StatusPass
		if profile != "" {
			c.Detail = "profile: " + profile
		}
	}
	return c
}

func checkDotfilesRemote(ctx context.Context, ch *chezmoi.Client, initialized bool) Check {
	c := Check{ID: "dotfiles_remote", Label: "dotfiles remote"}
	if !initialized {
		// Nothing to reach yet — being offline isn't broken, and neither is not
		// having a source repo. Warn, don't fail.
		c.Status = StatusWarn
		c.Detail = "skipped (chezmoi not initialized)"
		return c
	}
	url, _ := ch.RemoteURL(ctx)
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := ch.LsRemote(rctx); err != nil {
		c.Status = StatusWarn
		if url != "" {
			c.Detail = "could not reach " + url + " (offline?)"
		} else {
			c.Detail = "could not reach the dotfiles remote (offline?)"
		}
		return c
	}
	c.Status = StatusPass
	c.Detail = url
	return c
}

func checkGitHub(ctx context.Context) Check {
	c := Check{ID: "github_api", Label: "GitHub API reachable"}
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := release.LatestTag(rctx); err != nil {
		c.Status = StatusWarn
		c.Detail = "unreachable (offline or rate-limited) — gates self-update"
		return c
	}
	c.Status = StatusPass
	return c
}

func checkStateDir() Check {
	c := Check{ID: "state_dir", Label: "state dir writable", Detail: logging.Dir()}
	dir := logging.Dir()
	// Probe writability the only reliable cross-platform way: create and remove
	// a temp file. Net-zero — nothing the user manages is touched.
	f, err := os.CreateTemp(dir, ".doctor-*")
	if err != nil {
		c.Status = StatusFail
		c.Detail = dir + " is not writable — the debug log is silently dropped"
		c.Remedy = "create it or fix permissions: mkdir -p " + dir
		return c
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	c.Status = StatusPass
	return c
}

func checkTTY(isTTY bool) Check {
	// Informational only: never a failure, just surfaced so an agent can see
	// which mode it's in.
	c := Check{ID: "tty", Label: "stdout", Status: StatusPass}
	if isTTY {
		c.Detail = "interactive (TTY)"
	} else {
		c.Detail = "non-interactive (no TTY)"
	}
	return c
}

// compareDotted compares two dotted-numeric versions ("2.70.5", "2026.6.10").
// Missing or non-numeric components compare as 0. Returns -1, 0, or 1.
func compareDotted(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		ai, bi := 0, 0
		if i < len(as) {
			ai, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bi, _ = strconv.Atoi(bs[i])
		}
		if ai != bi {
			if ai < bi {
				return -1
			}
			return 1
		}
	}
	return 0
}
