package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/mikevalstar/myplace/internal/chezmoi"
	"github.com/mikevalstar/myplace/internal/doctor"
	"github.com/mikevalstar/myplace/internal/mise"
)

func newDoctorCmd(ch *chezmoi.Client, ms *mise.Client) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose whether this machine can run myplace (read-only)",
		Long: "Runs preflight checks — chezmoi and mise installed and recent, ~/.local/bin\n" +
			"on PATH, dotfiles repo and GitHub reachable, state dir writable — and names a\n" +
			"remedy for anything wrong. It never prompts and never changes the machine.\n" +
			"Exit codes: 0 ready, 1 a check failed, 2 checks incomplete (offline), 3 error.",
		Annotations: map[string]string{
			annHeadless:     "myplace doctor --json",
			annExitCodes:    exitCodesDoctor,
			annOutputSchema: "docs/features/doctor-preflight-diagnostics.md",
			annInteractive:  "false",
			annNote:         "read-only preflight; run it first on a flaky machine. Reachability checks degrade to warn (exit 2), not fail, when offline — diagnoses readiness, not drift (that's status).",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			rep := doctor.Run(cmd.Context(), ch, ms, doctor.Options{
				PATH:      userPATH(),
				StdoutTTY: term.IsTerminal(int(os.Stdout.Fd())),
			})
			logger.Info("doctor", "verdict", rep.Verdict, "checks", len(rep.Checks))
			if jsonOut {
				emitJSON(rep)
			} else {
				fmt.Print(renderDoctorText(rep))
			}
			os.Exit(doctor.ExitCode(rep.Verdict))
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit a single JSON document on stdout")
	return cmd
}

// userPATH is the user's real PATH as it was before main() prepended myplace's
// own bin/shim dirs — the meaningful value for the doctor PATH check. main
// stashes it in MYPLACE_ORIG_PATH; fall back to the live PATH if unset.
func userPATH() string {
	if p := os.Getenv("MYPLACE_ORIG_PATH"); p != "" {
		return p
	}
	return os.Getenv("PATH")
}

func renderDoctorText(r doctor.Report) string {
	var b strings.Builder
	header := "myplace doctor — host: " + r.Machine
	if r.Profile != "" {
		header += ", profile: " + r.Profile
	}
	fmt.Fprintf(&b, "%s\n\n", header)
	for _, c := range r.Checks {
		line := fmt.Sprintf("  %s %-22s %s", doctorGlyph(c.Status), c.Label, c.Detail)
		fmt.Fprintln(&b, strings.TrimRight(line, " "))
		if c.Remedy != "" {
			fmt.Fprintf(&b, "      → %s\n", c.Remedy)
		}
	}
	fmt.Fprintf(&b, "\nverdict: %s\n", doctorVerdictLine(r))
	return b.String()
}

func doctorGlyph(status string) string {
	switch status {
	case doctor.StatusPass:
		return "✓"
	case doctor.StatusWarn:
		return "⚠"
	default:
		return "✗"
	}
}

func doctorVerdictLine(r doctor.Report) string {
	var fails, warns int
	for _, c := range r.Checks {
		switch c.Status {
		case doctor.StatusFail:
			fails++
		case doctor.StatusWarn:
			warns++
		}
	}
	switch r.Verdict {
	case doctor.VerdictPass:
		return "ready — all checks passed"
	case doctor.VerdictIncomplete:
		return fmt.Sprintf("incomplete — %d warning(s); some checks could not complete (offline?)", warns)
	default:
		return fmt.Sprintf("problems found — %d failed, %d warning(s)", fails, warns)
	}
}
