// Package drift computes the bidirectional sync report defined in
// docs/workflows/check-machine-status.md and the JSON envelope defined in
// docs/features/headless-cli-and-json-output.md. It is read-only: nothing
// here mutates the machine (the one network op is a git fetch).
package drift

import (
	"context"
	"os"
	"time"

	"github.com/mikevalstar/myplace/internal/chezmoi"
	"github.com/mikevalstar/myplace/internal/mise"
)

const (
	VerdictInSync  = "in_sync"
	VerdictDrifted = "drifted"
	VerdictUnknown = "unknown"
	VerdictError   = "error"
)

// Schema is bumped only on breaking changes to the JSON shape.
const Schema = 1

type Report struct {
	Schema    int       `json:"schema"`
	Machine   string    `json:"machine"`
	Profile   string    `json:"profile,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
	Verdict   string    `json:"verdict"`
	Dotfiles  Dotfiles  `json:"dotfiles"`
	Tools     Tools     `json:"tools"`
	Myplace   Myplace   `json:"myplace"`
	Errors    []string  `json:"errors,omitempty"`
}

// Int pointers are null in JSON when the check couldn't run (offline, no upstream).
type Dotfiles struct {
	BehindOrigin     *int     `json:"behind_origin"`
	ToApply          []string `json:"to_apply"`
	LocalModified    []string `json:"local_modified"`
	UncommittedFiles *int     `json:"uncommitted_files"`
	UnpushedCommits  *int     `json:"unpushed_commits"`
}

type ToolIssue struct {
	Name    string `json:"name"`
	Current string `json:"current"`
	Wanted  string `json:"wanted"`
}

type Tools struct {
	Missing  []string    `json:"missing"`
	Outdated []ToolIssue `json:"outdated"`
}

type Myplace struct {
	Current string  `json:"current"`
	Latest  *string `json:"latest"`
}

// ExitCode maps a verdict to the CLI contract: 0 in sync, 1 drifted,
// 2 unknown, 3 error.
func ExitCode(verdict string) int {
	switch verdict {
	case VerdictInSync:
		return 0
	case VerdictDrifted:
		return 1
	case VerdictUnknown:
		return 2
	default:
		return 3
	}
}

// Decide picks the overall verdict. Known drift wins over unknown checks:
// if we can already see drift, partial blindness doesn't change the answer.
func Decide(d Dotfiles, t Tools, hadUnknown, hadFatal bool) string {
	if hadFatal {
		return VerdictError
	}
	drifted := len(d.ToApply) > 0 || len(d.LocalModified) > 0 ||
		(d.BehindOrigin != nil && *d.BehindOrigin > 0) ||
		(d.UncommittedFiles != nil && *d.UncommittedFiles > 0) ||
		(d.UnpushedCommits != nil && *d.UnpushedCommits > 0) ||
		len(t.Missing) > 0 || len(t.Outdated) > 0
	if drifted {
		return VerdictDrifted
	}
	if hadUnknown {
		return VerdictUnknown
	}
	return VerdictInSync
}

// Compute gathers the full report. Individual check failures degrade the
// verdict (unknown) rather than aborting; only missing/uninitialized core
// tools are fatal (error).
func Compute(ctx context.Context, ch *chezmoi.Client, ms *mise.Client, myplaceVersion string) Report {
	r := Report{
		Schema:    Schema,
		CheckedAt: time.Now().UTC(),
		Dotfiles:  Dotfiles{ToApply: []string{}, LocalModified: []string{}},
		Tools:     Tools{Missing: []string{}, Outdated: []ToolIssue{}},
		Myplace:   Myplace{Current: myplaceVersion},
	}
	if h, err := os.Hostname(); err == nil {
		r.Machine = h
	}

	hadUnknown, hadFatal := false, false
	fail := func(msg string) {
		r.Errors = append(r.Errors, msg)
		hadUnknown = true
	}

	switch {
	case !ch.Installed(ctx):
		r.Errors = append(r.Errors, "chezmoi is not installed")
		hadFatal = true
	case !ch.Initialized(ctx):
		r.Errors = append(r.Errors, "chezmoi is not initialized on this machine — run `myplace bootstrap`")
		hadFatal = true
	default:
		if p, err := ch.Profile(ctx); err == nil {
			r.Profile = p
		}
		if files, err := ch.Status(ctx); err != nil {
			fail("chezmoi status: " + err.Error())
		} else {
			for _, f := range files {
				if f.ApplyChanges {
					r.Dotfiles.ToApply = append(r.Dotfiles.ToApply, f.Path)
				}
				if f.LocalChanged {
					r.Dotfiles.LocalModified = append(r.Dotfiles.LocalModified, f.Path)
				}
			}
		}
		if err := ch.Fetch(ctx); err != nil {
			fail("git fetch failed (offline?): " + err.Error())
		}
		if n, err := ch.BehindUpstream(ctx); err == nil {
			r.Dotfiles.BehindOrigin = &n
		} else {
			hadUnknown = true
		}
		if n, err := ch.AheadUpstream(ctx); err == nil {
			r.Dotfiles.UnpushedCommits = &n
		} else {
			hadUnknown = true
		}
		if n, err := ch.Uncommitted(ctx); err == nil {
			r.Dotfiles.UncommittedFiles = &n
		} else {
			hadUnknown = true
		}
	}

	if !ms.Installed(ctx) {
		r.Errors = append(r.Errors, "mise is not installed")
		hadFatal = true
	} else {
		if tools, err := ms.Ls(ctx); err != nil {
			fail("mise ls: " + err.Error())
		} else {
			for _, t := range tools {
				if !t.Installed {
					r.Tools.Missing = append(r.Tools.Missing, t.Name)
				}
			}
		}
		if outdated, err := ms.Outdated(ctx); err != nil {
			fail("mise outdated: " + err.Error())
		} else {
			for _, o := range outdated {
				r.Tools.Outdated = append(r.Tools.Outdated, ToolIssue(o))
			}
		}
	}

	// Latest-release lookup is phase-2-adjacent; null = not checked.
	r.Verdict = Decide(r.Dotfiles, r.Tools, hadUnknown, hadFatal)
	return r
}
