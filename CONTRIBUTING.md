<!--
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
-->

# Contributing to Keleustes

Thanks for contributing.

## Development Setup

- Go version: see `go.mod`.
- Tool versions: see `.tool-versions` (Go, golangci-lint, operator-sdk).
- Bootstrap tooling once:

  ```bash
  cd tools && go mod tidy
  ```

- Run task targets without installing Task globally:

  ```bash
  go -C tools tool task --list
  go -C tools tool task ci
  ```

Keleustes is scaffolded with operator-sdk (`go/v4` plugin) for OLM bundle
support. Kubernetes tooling (`controller-gen`, `kustomize`, `setup-envtest`)
is pinned and invoked via `go run`. `operator-sdk`, `opm`, `kind`, `kubectl`,
and `docker` must be on `PATH` when running the corresponding tasks.

## Branching and Commits

- Create focused branches from `main`. Suggested prefixes:
  - `feature/<short-description>` — new functionality
  - `fix/<short-description>` — bug fixes
  - `chore/<short-description>` — maintenance, deps, tooling
  - `docs/<short-description>` — documentation only
  - `refactor/<short-description>` — internal restructuring
  - `test/<short-description>` — test-only changes
- Keep commits small and scoped.
- Use DCO sign-offs on every commit:
  - `git commit --signoff`
  - Required trailer format: `Signed-off-by: Your Name <you@example.com>`
- Use Conventional Commits on commits that land on `main`:
  - `feat:` -> minor
  - `fix:` / `perf:` -> patch
  - `docs:`, `test:`, `ci:`, `chore:`, `refactor:` -> no bump by default
- If you use squash merges, the final squash commit message must also follow
  Conventional Commit format.

## Coding Standards

- Follow Go conventions and keep code readable.
- Keep REUSE metadata valid:
  <!-- REUSE-IgnoreStart -->
  - Source files should include SPDX headers (`SPDX-License-Identifier: MIT`).
  <!-- REUSE-IgnoreEnd -->
  - Use `reuse lint` to validate licensing metadata.
- Format and lint:
  - `go -C tools tool task fmt`
  - `go -C tools tool task lint`

## Testing

Run before opening a PR:

- `go -C tools tool task test`
- `go -C tools tool task staticcheck`
- `go -C tools tool task vuln`

Or run full local CI:

- `go -C tools tool task ci`

End-to-end tests (`task test-e2e`) require a local `kind` cluster and Docker.

## Pull Requests

PRs should include:

- Summary of what changed
- Why the change is needed
- Testing performed (commands and results)
- Docs updates when behavior changes (`README.md`, ADRs under `docs/adr/`)

## Safety Expectations

- Reconcilers must be idempotent and bounded.
- Application deploys mutate Git, not cluster state, unless explicitly running
  a break-glass workflow.
- Do not introduce cluster-wide RBAC beyond what the operator strictly needs.
- Keep `keleustesctl` capable enough that the UI is not a single point of
  operational failure.
