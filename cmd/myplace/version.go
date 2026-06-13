package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mikevalstar/myplace/internal/drift"
	"github.com/mikevalstar/myplace/internal/release"
	"github.com/mikevalstar/myplace/internal/version"
)

func newVersionCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the myplace version",
		Run: func(cmd *cobra.Command, args []string) {
			if jsonOut {
				emitJSON(map[string]any{"schema": drift.Schema, "version": version.Version})
				return
			}
			fmt.Println("myplace", version.Version)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON on stdout")
	return cmd
}

func newSelfUpdateCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "self-update",
		Short: "Replace this binary with the latest GitHub release",
		Run: func(cmd *cobra.Command, args []string) {
			tag, err := release.SelfUpdate(cmd.Context(), version.Version)
			switch {
			case errors.Is(err, release.ErrUpToDate):
				if jsonOut {
					emitJSON(map[string]any{"schema": drift.Schema, "version": version.Version, "updated": false})
				} else {
					fmt.Fprintf(os.Stderr, "already up to date (%s)\n", version.Version)
				}
			case err != nil:
				fmt.Fprintln(os.Stderr, "self-update:", err)
				os.Exit(3)
			default:
				if jsonOut {
					emitJSON(map[string]any{"schema": drift.Schema, "version": release.NormalizeTag(tag), "updated": true})
				} else {
					fmt.Fprintf(os.Stderr, "updated %s → %s\n", version.Version, release.NormalizeTag(tag))
				}
			}
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON on stdout")
	return cmd
}
