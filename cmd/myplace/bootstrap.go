package main

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/mikevalstar/myplace/internal/chezmoi"
	"github.com/mikevalstar/myplace/internal/drift"
	"github.com/mikevalstar/myplace/internal/logging"
	"github.com/mikevalstar/myplace/internal/mise"
	"github.com/mikevalstar/myplace/internal/run"
	"github.com/mikevalstar/myplace/internal/version"
)

// This repo IS the dotfiles + mise config repo (ADR-0003).
const defaultRepo = "https://github.com/mikevalstar/myplace.git"

var profiles = []string{"personal-mac", "work-mac", "server"}

type bootstrapOpts struct {
	repo    string
	profile string
	yes     bool
}

func newBootstrapCmd(ch *chezmoi.Client, ms *mise.Client) *cobra.Command {
	var opts bootstrapOpts
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Set up a fresh machine: install chezmoi + mise, apply dotfiles, install tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBootstrap(cmd, ch, ms, opts)
		},
	}
	cmd.Flags().StringVar(&opts.repo, "repo", defaultRepo, "dotfiles repo (this repo, or a fork)")
	cmd.Flags().StringVar(&opts.profile, "profile", "", "machine profile: personal-mac, work-mac, or server")
	cmd.Flags().BoolVar(&opts.yes, "yes", false, "run without prompts (requires --profile)")
	return cmd
}

func runBootstrap(cmd *cobra.Command, ch *chezmoi.Client, ms *mise.Client, opts bootstrapOpts) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	if ch.Initialized(ctx) {
		fmt.Fprintln(os.Stderr, "this machine is already set up — use `myplace update` (or `myplace` for the dashboard)")
		os.Exit(3)
	}

	if opts.repo == "" {
		opts.repo = defaultRepo
	}
	if opts.yes {
		if opts.profile == "" {
			fmt.Fprintln(os.Stderr, "--yes requires --profile (personal-mac, work-mac, or server)")
			os.Exit(3)
		}
	} else {
		if !interactive() {
			fmt.Fprintln(os.Stderr, "no TTY: run headless with --repo <url> --profile <name> --yes")
			os.Exit(3)
		}
		var confirm bool
		form := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Dotfiles repo").Value(&opts.repo),
			huh.NewSelect[string]().Title("Machine profile").
				Options(huh.NewOptions(profiles...)...).Value(&opts.profile),
			huh.NewConfirm().Title("Install chezmoi + mise (if missing), apply dotfiles, install tools?").
				Value(&confirm),
		))
		if err := form.Run(); err != nil {
			return err
		}
		if !confirm {
			fmt.Fprintln(os.Stderr, "aborted")
			os.Exit(3)
		}
	}

	r := run.WithLogger(logger, logging.Tail)
	progress := func(format string, a ...any) {
		logger.Info("bootstrap", "step", fmt.Sprintf(format, a...))
		fmt.Fprintf(os.Stderr, format+"\n", a...)
	}

	if !ch.Installed(ctx) {
		progress("installing chezmoi → ~/.local/bin ...")
		if _, err := r.Run(ctx, "", "sh", "-c", `sh -c "$(curl -fsLS get.chezmoi.io)" -- -b "$HOME/.local/bin"`); err != nil {
			return fmt.Errorf("installing chezmoi: %w", err)
		}
	}
	if !ms.Installed(ctx) {
		progress("installing mise → ~/.local/bin ...")
		if _, err := r.Run(ctx, "", "sh", "-c", `curl -fsSL https://mise.run | MISE_INSTALL_PATH="$HOME/.local/bin/mise" sh`); err != nil {
			return fmt.Errorf("installing mise: %w", err)
		}
	}

	progress("applying dotfiles from %s (profile: %s) ...", opts.repo, opts.profile)
	if err := ch.InitApply(ctx, opts.repo, opts.profile); err != nil {
		return fmt.Errorf("chezmoi init: %w", err)
	}

	progress("installing tools ...")
	ms.Trust(ctx)
	if err := ms.Install(ctx); err != nil {
		// Per the workflow: tool failures don't abort bootstrap; they stay
		// visible as drift in the closing status.
		progress("warning: mise install: %v", err)
	}

	progress("verifying ...")
	rep := drift.Compute(ctx, ch, ms, version.Version)
	fmt.Print(renderStatusText(rep))
	progress("\nbootstrap complete — open a new shell, then run `myplace` for the dashboard")
	fmt.Fprintf(os.Stderr, "logs: %s\n", logging.Path())
	os.Exit(drift.ExitCode(rep.Verdict))
	return nil
}
