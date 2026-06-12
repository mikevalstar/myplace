package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mikevalstar/myplace/internal/drift"
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
	return &cobra.Command{
		Use:   "self-update",
		Short: "Update the myplace binary (not implemented yet)",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(os.Stderr, "self-update is not implemented yet — re-run the installer one-liner from the README")
			os.Exit(3)
		},
	}
}
