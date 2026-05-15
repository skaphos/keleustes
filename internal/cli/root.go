/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

// Package cli builds the keleustesctl cobra command tree.
package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCommand returns the root keleustesctl command with all subcommands
// registered. The subcommand surface mirrors PROPOSAL §17.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "keleustesctl",
		Short: "Keleustes operator CLI",
		Long: "keleustesctl is the operational CLI for the Keleustes GitOps " +
			"delivery control plane. It supports app inspection, matrix views, " +
			"promotion, diff, blockers, rollback, and administration.",
		SilenceUsage: true,
	}

	root.AddCommand(newAppCommand())
	root.AddCommand(newMatrixCommand())
	root.AddCommand(newReleaseCommand())
	root.AddCommand(newPromoteCommand())
	root.AddCommand(newDiffCommand())
	root.AddCommand(newBlockersCommand())
	root.AddCommand(newRollbackCommand())
	root.AddCommand(newVersionCommand())

	return root
}
