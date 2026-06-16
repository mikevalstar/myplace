package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/mikevalstar/myplace/internal/chezmoi"
	"github.com/mikevalstar/myplace/internal/drift"
	"github.com/mikevalstar/myplace/internal/mise"
	"github.com/mikevalstar/myplace/internal/version"
)

// interactive reports whether a human is plausibly at the keyboard.
func interactive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// emitJSON writes exactly one document to stdout — the whole --json contract.
func emitJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(os.Stderr, "error encoding JSON:", err)
		os.Exit(3)
	}
}

func newStatusCmd(ch *chezmoi.Client, ms *mise.Client) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report drift in both directions (read-only)",
		Long: "Checks dotfiles (behind origin, unapplied, locally modified, unpushed) and\n" +
			"tools (missing, outdated) without changing anything.\n" +
			"Exit codes: 0 in sync, 1 drifted, 2 unknown, 3 error.",
		Annotations: map[string]string{
			annHeadless:     "myplace status --json",
			annExitCodes:    exitCodesDrift,
			annOutputSchema: "docs/features/headless-cli-and-json-output.md",
			annInteractive:  "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			rep := drift.Compute(cmd.Context(), ch, ms, version.Version)
			logger.Info("status", "verdict", rep.Verdict,
				"to_apply", len(rep.Dotfiles.ToApply), "local_modified", len(rep.Dotfiles.LocalModified),
				"tools_missing", len(rep.Tools.Missing), "tools_outdated", len(rep.Tools.Outdated))
			if jsonOut {
				emitJSON(rep)
			} else {
				fmt.Print(renderStatusText(rep))
			}
			os.Exit(drift.ExitCode(rep.Verdict))
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit a single JSON document on stdout")
	return cmd
}

func renderStatusText(r drift.Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s) — %s\n", r.Machine, r.Profile, strings.ToUpper(strings.ReplaceAll(r.Verdict, "_", " ")))
	n := func(p *int) string {
		if p == nil {
			return "?"
		}
		return fmt.Sprintf("%d", *p)
	}
	fmt.Fprintf(&b, "  dotfiles: %s behind origin, %d to apply, %d modified locally, %s uncommitted, %s unpushed\n",
		n(r.Dotfiles.BehindOrigin), len(r.Dotfiles.ToApply), len(r.Dotfiles.LocalModified),
		n(r.Dotfiles.UncommittedFiles), n(r.Dotfiles.UnpushedCommits))
	if r.Dotfiles.PushAllowed != nil && !*r.Dotfiles.PushAllowed {
		fmt.Fprintf(&b, "    push policy: disabled for this profile; unpushed commits are parked locally\n")
	}
	for _, f := range r.Dotfiles.ToApply {
		fmt.Fprintf(&b, "    ↓ %s\n", f)
	}
	for _, f := range r.Dotfiles.LocalModified {
		fmt.Fprintf(&b, "    ↑ %s\n", f)
	}
	fmt.Fprintf(&b, "  tools:    %d missing, %d outdated\n", len(r.Tools.Missing), len(r.Tools.Outdated))
	for _, t := range r.Tools.Missing {
		fmt.Fprintf(&b, "    + %s\n", t)
	}
	for _, o := range r.Tools.Outdated {
		fmt.Fprintf(&b, "    %s %s → %s\n", o.Name, o.Current, o.Wanted)
	}
	if r.Myplace.Latest != nil && *r.Myplace.Latest != r.Myplace.Current {
		fmt.Fprintf(&b, "  myplace:  %s → %s available (myplace self-update)\n", r.Myplace.Current, *r.Myplace.Latest)
	}
	for _, e := range r.Errors {
		fmt.Fprintf(&b, "  ! %s\n", e)
	}
	return b.String()
}
