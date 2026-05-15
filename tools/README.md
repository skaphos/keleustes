<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# tools

This module pins developer tooling so contributors do not need Task or other
binaries installed globally. Run any task target with:

    go -C tools tool task --list
    go -C tools tool task <name>

The first invocation triggers `go mod download` against `tools/go.mod`.

The full transitive dependency list lives in `tools/go.sum`; bootstrap it with:

    cd tools && go mod tidy
