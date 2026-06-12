package main

import (
	"fmt"
	"os"

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
		Long: "Converges this machine on the shared config: chezmoi update (pull + apply),\n" +
			"then mise install + upgrade. Capturing OUTGOING drift (re-add, commit, push)\n" +
			"is deliberately not part of unattended updates — review local edits in the\n" +
			"source repo (chezmoi cd) until interactive capture lands.",
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
					Description("chezmoi update (pull + apply), then mise install + upgrade").
					Value(&ok).Run()
				if err != nil || !ok {
					fmt.Fprintln(os.Stderr, "aborted")
					os.Exit(3)
				}
			}

			doDotfiles := !toolsOnly
			doTools := !dotfilesOnly

			var steps []stepResult
			step := func(name string, fn func() error) {
				err := fn()
				res := stepResult{Name: name, OK: err == nil}
				if err != nil {
					res.Error = err.Error()
				}
				if !jsonOut {
					mark := "ok"
					if err != nil {
						mark = "FAILED: " + res.Error
					}
					fmt.Fprintf(os.Stderr, "%-16s %s\n", name, mark)
				}
				steps = append(steps, res)
			}

			if doDotfiles {
				step("chezmoi update", func() error { return ch.Update(ctx) })
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
