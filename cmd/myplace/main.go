package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	charmlog "github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/mikevalstar/myplace/internal/brew"
	"github.com/mikevalstar/myplace/internal/chezmoi"
	"github.com/mikevalstar/myplace/internal/drift"
	"github.com/mikevalstar/myplace/internal/logging"
	"github.com/mikevalstar/myplace/internal/mise"
	"github.com/mikevalstar/myplace/internal/outdated"
	"github.com/mikevalstar/myplace/internal/run"
	"github.com/mikevalstar/myplace/internal/tui"
	"github.com/mikevalstar/myplace/internal/version"
)

// logger is set once in main and shared by all subcommands (same package) so
// they can log outcomes without threading it through every constructor.
var logger *charmlog.Logger

func main() {
	// Bootstrap installs into ~/.local/bin; make sure we can see binaries
	// there even before the user's next shell does.
	if home, err := os.UserHomeDir(); err == nil {
		os.Setenv("PATH", filepath.Join(home, ".local", "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	}

	// Persistent debug trace to <state-dir>/myplace.log (ADR-0005). Tag the
	// session with the subcommand (best-effort from argv) so interleaved runs
	// stay separable; lifecycle and every subprocess go to the file.
	var closeLog func()
	logger, closeLog = logging.New(subcommand(os.Args))
	defer closeLog()
	logger.Info("start", "version", version.Version, "argv", strings.Join(os.Args[1:], " "))

	r := run.WithLogger(logger, logging.Tail)
	ch := chezmoi.New(r)
	ms := mise.New(r)

	root := newRootCmd(r, ch, ms)

	if err := root.Execute(); err != nil {
		logger.Error("exit with error", "err", err.Error())
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(3)
	}
	logger.Info("exit ok")
}

// newRootCmd builds the fully-wired command tree: the subcommands, the custom
// help command (which also powers `help --json` and `help --llm`), and the
// per-command annotations the manifest reads back (see help.go). Shared by
// main and the help tests so the tree under test is exactly the real one.
func newRootCmd(r run.Runner, ch *chezmoi.Client, ms *mise.Client) *cobra.Command {
	// Package-manager sources for `outdated` and the dashboard's Updates pane.
	// Slice order is display order. The brew source self-reports Available()
	// == false when brew isn't on PATH, so listing it is safe everywhere
	// (brew-if-present, ADR-0008/0009/0010).
	sources := []outdated.Source{
		outdated.MiseSource(ms),
		outdated.BrewSource(brew.New(r)),
	}
	root := &cobra.Command{
		Use:   "myplace",
		Short: "Bootstrap, update, and check machines managed by chezmoi + mise",
		Long: "AI agents: run `myplace help --llm` for a copy-paste brief of every command,\n" +
			"or `myplace help --json` for a machine-readable manifest.\n\n" +
			"myplace orchestrates chezmoi (dotfiles) and mise (tools) to bootstrap new\n" +
			"machines, update existing ones, and report drift. Run with no arguments\n" +
			"for the TUI dashboard; every subcommand also works headlessly (--json).\n\n" +
			"Debug log: " + logging.Path(),
		SilenceUsage:  true,
		SilenceErrors: true,
		Annotations: map[string]string{
			annHeadless:    "myplace status --json",
			annExitCodes:   exitCodesDrift,
			annInteractive: "true",
			annNote:        "bare `myplace` with a TTY launches the dashboard; off a TTY (agent or pipe) it prints the status summary and exits with the drift code. Prefer `myplace status --json`.",
		},
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
			// The dashboard needs a TTY. Off one (an agent or pipe ran bare
			// `myplace`), don't error on a missing terminal — fall back to the
			// read-only status summary, which is the useful no-arg answer.
			// (ADR-0006: every command path is agent-runnable.)
			if !interactive() {
				rep := drift.Compute(ctx, ch, ms, version.Version)
				logger.Info("status (bare, non-interactive)", "verdict", rep.Verdict)
				fmt.Print(renderStatusText(rep))
				os.Exit(drift.ExitCode(rep.Verdict))
			}
			return tui.Run(ch, ms, sources, version.Version)
		},
	}

	// Keep the command surface focused on myplace's own verbs: drop cobra's
	// auto-generated `completion` command so it doesn't show up in help or the
	// agent manifest.
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetHelpCommand(newHelpCmd())
	root.AddCommand(
		newStatusCmd(ch, ms),
		newOutdatedCmd(sources...),
		newUpdateCmd(ch, ms),
		newBootstrapCmd(ch, ms),
		newVersionCmd(),
		newSelfUpdateCmd(),
	)
	return root
}

// subcommand returns the first non-flag argument (the cobra subcommand) for
// log tagging, or "tui" when invoked bare.
func subcommand(argv []string) string {
	for _, a := range argv[1:] {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return "tui"
}
