package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mikevalstar/myplace/internal/chezmoi"
	"github.com/mikevalstar/myplace/internal/mise"
	"github.com/mikevalstar/myplace/internal/run"
	"github.com/mikevalstar/myplace/internal/tui"
	"github.com/mikevalstar/myplace/internal/version"
)

func main() {
	// Bootstrap installs into ~/.local/bin; make sure we can see binaries
	// there even before the user's next shell does.
	if home, err := os.UserHomeDir(); err == nil {
		os.Setenv("PATH", filepath.Join(home, ".local", "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	}

	r := run.Exec{}
	ch := chezmoi.New(r)
	ms := mise.New(r)

	root := &cobra.Command{
		Use:   "myplace",
		Short: "Bootstrap, update, and check machines managed by chezmoi + mise",
		Long: "myplace orchestrates chezmoi (dotfiles) and mise (tools) to bootstrap new\n" +
			"machines, update existing ones, and report drift. Run with no arguments\n" +
			"for the TUI dashboard; every subcommand also works headlessly (--json).",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if !ch.Installed(ctx) || !ch.Initialized(ctx) {
				if !interactive() {
					fmt.Fprintln(os.Stderr, "this machine is not set up; run: myplace bootstrap --repo <url> --profile <name> --yes")
					os.Exit(3)
				}
				// Fresh machine + a human present: route to the wizard.
				return runBootstrap(cmd, ch, ms, bootstrapOpts{})
			}
			return tui.Run(ch, ms, version.Version)
		},
	}

	root.AddCommand(
		newStatusCmd(ch, ms),
		newUpdateCmd(ch, ms),
		newBootstrapCmd(ch, ms),
		newVersionCmd(),
		newSelfUpdateCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(3)
	}
}
