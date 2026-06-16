package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/mikevalstar/myplace/internal/chezmoi"
	"github.com/mikevalstar/myplace/internal/drift"
	"github.com/mikevalstar/myplace/internal/mise"
)

type stepResult struct {
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type updateReport struct {
	Schema  int          `json:"schema"`
	Steps   []stepResult `json:"steps"`
	Verdict string       `json:"verdict"` // "ok" or "partial"
}

func newUpdateCmd(ch *chezmoi.Client, ms *mise.Client) *cobra.Command {
	var yes, jsonOut, dotfilesOnly, toolsOnly bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Pull and apply dotfiles, install and upgrade tools",
		Long: "Converges this machine on the shared config: pull + apply dotfiles,\n" +
			"then mise install + upgrade. Interactively, it first walks any local edits to\n" +
			"managed files so you can keep (re-add + push) or discard them, then shows\n" +
			"per-file diffs for incoming dotfile changes before applying them. Headless\n" +
			"(--yes) never captures local edits — it reports them and skips the dotfiles\n" +
			"apply, leaving the rest of the update to proceed.",
		Annotations: map[string]string{
			annHeadless:     "myplace update --yes --json",
			annRequired:     "yes",
			annExitCodes:    exitCodesConverge,
			annOutputSchema: "docs/workflows/update-machine.md",
			annInteractive:  "true",
			annNote:         "headless (--yes) is converge-only: it applies incoming changes and upgrades tools but never captures or pushes local edits. Files with local edits are reported and skipped (exit 1). Run interactive `myplace update` to keep/discard/push them.",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !ch.Installed(ctx) || !ch.Initialized(ctx) {
				fmt.Fprintln(os.Stderr, "this machine is not set up; run: myplace bootstrap")
				os.Exit(3)
			}
			if !yes {
				if !interactive() {
					fmt.Fprintln(os.Stderr, "refusing to update without confirmation in a non-interactive session; pass --yes")
					os.Exit(3)
				}
				var ok bool
				err := huh.NewConfirm().
					Title("Update this machine?").
					Description("capture local edits, pull + review/apply dotfiles, then mise install + upgrade").
					Value(&ok).Run()
				if err != nil || !ok {
					fmt.Fprintln(os.Stderr, "aborted")
					os.Exit(3)
				}
				// Outgoing capture runs BEFORE pull+apply so local edits are
				// committed instead of clobbered (see update workflow doc).
				// Interactive only: cron must never auto-push unreviewed edits.
				if !toolsOnly {
					if err := captureOutgoing(ctx, ch); err != nil {
						fmt.Fprintln(os.Stderr, "capture aborted:", err)
						os.Exit(3)
					}
				}
			}

			doDotfiles := !toolsOnly
			doTools := !dotfilesOnly

			var steps []stepResult
			step := func(name string, fn func() error) bool {
				err := fn()
				res := stepResult{Name: name, OK: err == nil}
				if err != nil {
					res.Error = err.Error()
					logger.Error("update step failed", "step", name, "err", err.Error())
				} else {
					logger.Info("update step ok", "step", name)
				}
				if !jsonOut {
					mark := "ok"
					if err != nil {
						mark = "FAILED: " + res.Error
					}
					fmt.Fprintf(os.Stderr, "%-16s %s\n", name, mark)
				}
				steps = append(steps, res)
				return err == nil
			}

			if doDotfiles {
				// Applying aborts on a managed file that has uncaptured local
				// edits — apply would overwrite it, so chezmoi prompts, and
				// with no TTY that prompt fails. Detect that up front and
				// report it plainly instead of letting it surface as a cryptic
				// "EOF". Capturing (keep/discard) is the resolution; in
				// headless runs it must be done interactively.
				if mods := localModified(ctx, ch); len(mods) > 0 {
					var hint string
					if yes {
						hint = "run `myplace update` (interactive) to keep or discard them"
					} else {
						hint = "re-run and choose keep or discard instead of skip"
					}
					msg := fmt.Sprintf("not applied — local edits to %s; %s", strings.Join(mods, ", "), hint)
					logger.Error("update dotfiles skipped (local edits)", "files", strings.Join(mods, ","))
					if !jsonOut {
						fmt.Fprintf(os.Stderr, "%-16s SKIPPED: %s\n", "chezmoi apply", msg)
					}
					steps = append(steps, stepResult{Name: "chezmoi apply", OK: false, Error: msg})
				} else {
					if step("chezmoi pull", func() error { return ch.Pull(ctx) }) {
						step("chezmoi apply", func() error {
							if mods := localModified(ctx, ch); len(mods) > 0 {
								return fmt.Errorf("not applied — local edits to %s; run `myplace update` (interactive) to keep or discard them", strings.Join(mods, ", "))
							}
							if yes {
								return ch.Apply(ctx)
							}
							return reviewAndApplyIncoming(ctx, ch)
						})
					}
				}
			}
			if doTools {
				ms.Trust(ctx)
				step("mise install", func() error { return ms.Install(ctx) })
				step("mise upgrade", func() error { return ms.Upgrade(ctx) })
			}

			rep := updateReport{Schema: drift.Schema, Steps: steps, Verdict: "ok"}
			exit := 0
			for _, s := range steps {
				if !s.OK {
					rep.Verdict = "partial"
					exit = 1
				}
			}
			if jsonOut {
				emitJSON(rep)
			}
			os.Exit(exit)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt (required headless)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit a result document on stdout")
	cmd.Flags().BoolVar(&dotfilesOnly, "dotfiles", false, "only update dotfiles")
	cmd.Flags().BoolVar(&toolsOnly, "tools", false, "only update tools")
	return cmd
}

// localModified returns the managed files that differ from what chezmoi last
// wrote (outgoing drift). These block a plain apply, so the converge step
// checks for them first. A status error returns nil — don't block the update
// on an inability to read status.
func localModified(ctx context.Context, ch *chezmoi.Client) []string {
	files, err := ch.Status(ctx)
	if err != nil {
		return nil
	}
	var mods []string
	for _, f := range files {
		if f.LocalChanged {
			mods = append(mods, f.Path)
		}
	}
	return mods
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// reviewAndApplyIncoming shows the diff for every file that `chezmoi apply`
// would change after the source repo has been pulled, then applies only the
// files the user approves. Headless runs intentionally bypass this and apply
// everything, because there is no decision channel.
func reviewAndApplyIncoming(ctx context.Context, ch *chezmoi.Client) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	files, err := ch.Status(ctx)
	if err != nil {
		return fmt.Errorf("chezmoi status: %w", err)
	}

	var incoming []chezmoi.FileStatus
	for _, f := range files {
		if f.ApplyChanges {
			incoming = append(incoming, f)
		}
	}
	if len(incoming) == 0 {
		fmt.Fprintln(os.Stderr, "chezmoi apply    no incoming dotfile changes")
		return nil
	}

	applied, skipped := 0, 0
	for _, f := range incoming {
		target := filepath.Join(home, f.Path)
		// Show the exact target-file patch before asking whether to apply it.
		if diff, err := ch.Diff(ctx, target); err == nil && strings.TrimSpace(diff) != "" {
			fmt.Fprintf(os.Stderr, "\n── %s (incoming change to apply) ──\n%s\n", f.Path, diff)
		} else {
			fmt.Fprintf(os.Stderr, "\n── %s (incoming change) ──\n", f.Path)
		}

		var choice string
		err := huh.NewSelect[string]().
			Title(f.Path).
			Options(
				huh.NewOption("apply — repo wins for this file", "apply"),
				huh.NewOption("skip — leave this file for later", "skip"),
				huh.NewOption("abort dotfiles apply", "abort"),
			).Value(&choice).Run()
		if err != nil {
			return err
		}

		switch choice {
		case "apply":
			if err := ch.ApplyTarget(ctx, target); err != nil {
				return fmt.Errorf("apply %s: %w", f.Path, err)
			}
			applied++
		case "skip":
			skipped++
		case "abort":
			return fmt.Errorf("incoming dotfiles apply aborted at %s", f.Path)
		}
	}

	if skipped > 0 {
		return fmt.Errorf("applied %d incoming file(s), skipped %d; re-run update to finish dotfiles", applied, skipped)
	}
	return nil
}

// captureOutgoing walks locally-modified managed files and lets the user
// keep & share (re-add), discard (apply --force), or skip each one, then
// offers to commit and push whatever the source repo has pending.
func captureOutgoing(ctx context.Context, ch *chezmoi.Client) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	files, err := ch.Status(ctx)
	if err != nil {
		return fmt.Errorf("chezmoi status: %w", err)
	}

	for _, f := range files {
		if !f.LocalChanged {
			continue
		}
		target := filepath.Join(home, f.Path)
		if diff, err := ch.Diff(ctx, target); err == nil && strings.TrimSpace(diff) != "" {
			fmt.Fprintf(os.Stderr, "\n── %s (applying would make this change, i.e. undo your local edit) ──\n%s\n", f.Path, diff)
		} else {
			fmt.Fprintf(os.Stderr, "\n── %s (modified locally) ──\n", f.Path)
		}
		var choice string
		err := huh.NewSelect[string]().
			Title(f.Path).
			Options(
				huh.NewOption("keep & share — machine wins, update the repo", "share"),
				huh.NewOption("discard — repo wins, overwrite my local edit", "discard"),
				huh.NewOption("skip — decide later", "skip"),
			).Value(&choice).Run()
		if err != nil {
			return err
		}
		switch choice {
		case "share":
			if err := ch.ReAdd(ctx, target); err != nil {
				return fmt.Errorf("re-add %s: %w", f.Path, err)
			}
			// chezmoi re-add silently no-ops on a templated source file: the
			// edit isn't reverse-rendered into the template, so the file stays
			// modified. Detect that and point the user at the real fix instead
			// of pretending the change was captured.
			if contains(localModified(ctx, ch), f.Path) {
				fmt.Fprintf(os.Stderr,
					"  ! %s is templated — re-add can't capture it. Edit the source directly: `chezmoi edit %s`, then commit in the source repo.\n",
					f.Path, target)
				logger.Error("re-add did not capture templated file", "file", f.Path)
			}
		case "discard":
			if err := ch.ApplyForce(ctx, target); err != nil {
				return fmt.Errorf("apply %s: %w", f.Path, err)
			}
		}
	}

	n, err := ch.Uncommitted(ctx)
	if err != nil || n == 0 {
		return nil
	}
	var share bool
	if err := huh.NewConfirm().
		Title(fmt.Sprintf("Source repo has %d uncommitted change(s) — commit and push?", n)).
		Value(&share).Run(); err != nil {
		return err
	}
	if !share {
		return nil
	}
	host, _ := os.Hostname()
	msg := "captured changes from " + host
	if err := huh.NewInput().Title("Commit message").Value(&msg).Run(); err != nil {
		return err
	}
	if err := ch.CommitAll(ctx, msg); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	if err := ch.Push(ctx); err != nil {
		// Not fatal: the commit is safe locally; status shows it as unpushed.
		fmt.Fprintln(os.Stderr, "push failed (commit kept locally):", err)
	}
	return nil
}
