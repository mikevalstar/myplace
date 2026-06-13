package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mikevalstar/myplace/internal/outdated"
)

func newOutdatedCmd(sources ...outdated.Source) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "outdated",
		Short: "List outdated packages across package managers (read-only, informational)",
		Long: "Reports packages with a newer version available, grouped by source (mise,\n" +
			"and brew when present). Informational and read-only: it never upgrades\n" +
			"anything, and it does NOT affect the drift verdict or `status` exit codes.\n" +
			"brew is skipped when it isn't on PATH.\n" +
			"Exit codes: 0 all current, 1 updates available, 3 error.",
		Annotations: map[string]string{
			annHeadless:     "myplace outdated --json",
			annExitCodes:    exitCodesOutdated,
			annOutputSchema: "docs/features/outdated-packages.md",
			annInteractive:  "false",
			annNote:         "informational inventory; never mutates and never upgrades. brew is brew-if-present (skipped when not on PATH). Distinct from the drift verdict: 1 means 'updates available', not 'out of sync'.",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			inv := outdated.Collect(cmd.Context(), sources...)
			code := outdated.ExitCode(inv)
			logger.Info("outdated", "sources", len(inv.Sources), "exit", code)
			if jsonOut {
				emitJSON(inv)
			} else {
				fmt.Print(renderOutdatedText(inv))
			}
			os.Exit(code)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit a single JSON document on stdout")
	return cmd
}

func renderOutdatedText(inv outdated.Inventory) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — outdated packages\n", inv.Machine)
	total := 0
	for _, s := range inv.Sources {
		switch {
		case !s.Available:
			fmt.Fprintf(&b, "  %s: not available\n", s.Name)
		case s.Error != "":
			fmt.Fprintf(&b, "  %s: ! %s\n", s.Name, s.Error)
		case len(s.Packages) == 0:
			fmt.Fprintf(&b, "  %s: up to date\n", s.Name)
		default:
			fmt.Fprintf(&b, "  %s: %d outdated\n", s.Name, len(s.Packages))
			for _, p := range s.Packages {
				fmt.Fprintf(&b, "    %s %s → %s\n", p.Name, p.Current, p.Latest)
			}
			total += len(s.Packages)
		}
	}
	if total == 0 {
		fmt.Fprintln(&b, "  all packages current")
	}
	return b.String()
}
