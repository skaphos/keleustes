/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// notImplemented returns the standard scaffold error for unimplemented commands.
// Each subcommand keeps this single source of truth so the message stays
// consistent as features land.
func notImplemented(c *cobra.Command, _ []string) error {
	return fmt.Errorf("%s: not implemented in this scaffold", c.CommandPath())
}

func newAppCommand() *cobra.Command {
	app := &cobra.Command{
		Use:   "app",
		Short: "Inspect applications",
	}
	app.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List applications",
		RunE:  notImplemented,
	})
	app.AddCommand(&cobra.Command{
		Use:   "get NAME",
		Short: "Show detail for one application",
		Args:  cobra.ExactArgs(1),
		RunE:  notImplemented,
	})
	return app
}

func newMatrixCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "matrix",
		Short: "Show the application/environment matrix",
		RunE:  notImplemented,
	}
	cmd.Flags().String("env", "", "Restrict the matrix to an environment")
	return cmd
}

func newReleaseCommand() *cobra.Command {
	rel := &cobra.Command{
		Use:   "release",
		Short: "Inspect releases",
	}
	rel.AddCommand(&cobra.Command{
		Use:   "list APPLICATION",
		Short: "List releases for an application",
		Args:  cobra.ExactArgs(1),
		RunE:  notImplemented,
	})
	return rel
}

func newPromoteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "promote APPLICATION",
		Short: "Request a promotion for an application",
		Args:  cobra.ExactArgs(1),
		RunE:  notImplemented,
	}
	cmd.Flags().String("release", "", "Release name to promote")
	cmd.Flags().String("to", "", "Destination environment")
	cmd.Flags().String("cell", "", "Restrict the promotion to a cell")
	cmd.Flags().String("region", "", "Restrict the promotion to a region")
	cmd.Flags().String("change", "", "Change-management identifier (e.g., a CRQ ID)")
	cmd.Flags().Bool("pr", false, "Mutate Git via pull request (default: directCommit)")
	return cmd
}

func newDiffCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff APPLICATION",
		Short: "Diff an application across environments",
		Args:  cobra.ExactArgs(1),
		RunE:  notImplemented,
	}
	cmd.Flags().String("from", "", "Source environment")
	cmd.Flags().String("to", "", "Destination environment")
	return cmd
}

func newBlockersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blockers APPLICATION",
		Short: "Show promotion blockers for an application",
		Args:  cobra.ExactArgs(1),
		RunE:  notImplemented,
	}
	cmd.Flags().String("to", "", "Destination environment")
	return cmd
}

func newRollbackCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollback APPLICATION",
		Short: "Roll an application back to an earlier release",
		Args:  cobra.ExactArgs(1),
		RunE:  notImplemented,
	}
	cmd.Flags().String("to-release", "", "Release to roll back to")
	cmd.Flags().String("env", "", "Environment to roll back")
	cmd.Flags().String("cell", "", "Cell to roll back")
	return cmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print keleustesctl version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "keleustesctl 0.0.0 (scaffold)")
			return err
		},
	}
}
