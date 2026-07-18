/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

// Command keleustesctl is the Keleustes operator CLI. See PROPOSAL §17 for the
// surface area. Subcommands are scaffolded as stubs; behavior arrives with the
// MVPs they belong to.
package main

import (
	"fmt"
	"os"

	"github.com/skaphos/keleustes/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
