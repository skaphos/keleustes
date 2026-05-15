<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Repository Guidelines

This file is the authoritative briefing for AI coding agents and human
contributors working on Keleustes. It is also exposed as `CLAUDE.md`, a
symlink to `AGENTS.md`.

## What Keleustes Is

Keleustes is a Kubernetes-native GitOps delivery control plane. It owns the
following CRDs under the `keleustes.skaphos.dev` API group:

- `Application`, `Source`, `Release`, `Deployment`
- `Environment`, `Cell`, `DeploymentTarget`
- `Promotion`, `PromotionPolicy`, `Approval`, `FreezeWindow`
- `SyncPlan`, `SyncRun`, `HealthCheck`

The repository is scaffolded with `kubebuilder` (`go.kubebuilder.io/v4`) and
packaged via `operator-sdk` for OLM bundle distribution. See `PROJECT` for the
canonical resource list and the project metadata.

Keleustes is currently at the **scaffold** stage. Reconciler stubs set
`ObservedGeneration` and an `Accepted` condition, but the engines that drive
real behavior (Source, Sync, Promotion, Git Mutation, Policy, Health, Diff)
arrive across the MVP 0 → MVP 4 roadmap described in `README.md` and the
proposal at `docs/PROPOSAL.md`.

## Project Structure & Module Organization

- `api/v1alpha1/` — CRD type definitions. `zz_generated.deepcopy.go` is
  produced by `controller-gen`; never hand-edit it.
- `cmd/manager/main.go` — controller-runtime entrypoint that wires reconcilers
  into the manager and starts it.
- `cmd/keleustesctl/main.go` — thin entrypoint that constructs the cobra root
  command via `internal/cli` and runs it.
- `internal/cli/` — `keleustesctl` cobra command tree (PROPOSAL §17).
- `internal/controller/` — reconciler implementations.
- `config/` — kustomize overlays, RBAC, CRDs, sample CRs.
- `tools/` — pinned tooling launched via `go -C tools tool task ...` (Task,
  controller-gen, kustomize, setup-envtest, golangci-lint, staticcheck,
  govulncheck, goimports).
- `hack/boilerplate.go.txt` — SPDX/license header inserted by `controller-gen`
  into generated Go files.
- `docs/` — architecture notes and ADRs (`docs/adr/`).
- `ui/` — placeholder for the React/TypeScript UI described in PROPOSAL §16.

## Build, Test, and Development Commands

All workflows are wrapped in tasks; never invoke `controller-gen` /
`kustomize` / `setup-envtest` directly except via tasks so versions stay
pinned.

- `go -C tools tool task --list` — list available tasks.
- `go -C tools tool task fmt` — `goimports -w .` + `go fmt ./...`.
- `go -C tools tool task lint` — regenerates manifests + runs `golangci-lint`.
- `go -C tools tool task vet` — `go vet ./...`.
- `go -C tools tool task test` — unit tests with envtest, writes `coverage.out`.
- `go -C tools tool task test-e2e` — Kind-backed e2e (requires `kind` + Docker).
- `go -C tools tool task staticcheck` — `staticcheck ./...`.
- `go -C tools tool task vuln` — `govulncheck ./...`.
- `go -C tools tool task ci` — full local CI.
- `go -C tools tool task build` — `go build -o bin/manager ./cmd/manager`.
- `go -C tools tool task build-ctl` — `go build -o bin/keleustesctl ./cmd/keleustesctl`.
- `go -C tools tool task run` — run the manager against the current kubeconfig.
- `go -C tools tool task install` / `uninstall` — apply / remove CRDs.
- `go -C tools tool task deploy` / `undeploy` — apply / remove the operator.

## Coding Standards

- Go version: `go.mod` is the source of truth.
- Formatting: `gofmt` and `goimports` enforced via `golangci-lint`.
- Naming: standard Go (`PascalCase` exported, `camelCase` unexported). CRD
  types follow kubebuilder conventions; reconcilers are `<Kind>Reconciler`.
- File headers: every Go source file (and most non-generated text files)
  carries the SPDX header at `hack/boilerplate.go.txt`. `reuse lint` is
  enforced in CI.
- Generated files (`zz_generated*.go`, manifests under `config/crd/bases/`)
  are produced by tooling — re-run the appropriate task instead of editing
  them.

## Engineering Guardrails

- Keep cognitive load low: small functions, clear names, early returns, simple
  control flow over clever abstractions.
- Comment intent (invariants, edge cases, non-obvious tradeoffs), not mechanics.
- Reconcilers must be **idempotent and bounded**. No unbounded work in a
  `Reconcile` loop.
- **Application deploys mutate Git, not cluster state**, unless explicitly
  running a break-glass workflow. Every sync decision must be explainable from
  Git commit, render output, apply result, inventory, and health state.
- Do not introduce cluster-wide RBAC beyond what the operator needs. New
  permissions must show up under `config/rbac/` via `+kubebuilder:rbac`
  markers.
- Keep `keleustesctl` capable enough that the UI is not a single point of
  operational failure (PROPOSAL §17).
- Match existing patterns (kubebuilder layout, ginkgo specs, task wiring)
  instead of inventing new ones.

## Testing Guidelines

- Frameworks: Ginkgo v2 + Gomega for envtest and e2e suites; `testing`
  (stdlib) for plain unit tests.
- Unit tests live next to source as `*_test.go`. Suite bootstraps follow
  `suite_test.go`.
- New behavior must ship with direct test coverage. Bug fixes should add a
  regression test that fails before the fix.
- envtest binaries are managed by `setup-envtest`; the `test` task
  bootstraps them in `bin/k8s/`.

## Commit & Pull Request Guidelines

- All changes land via pull request. Never push directly to `main`.
- **DCO is mandatory** — every commit must carry a `Signed-off-by:` trailer
  (`git commit --signoff`).
- Cryptographic signing is encouraged (`git commit -S -s …`).
- Use Conventional Commits on commits that land on `main` so `release-please`
  can infer the next version:
  - `feat:` → minor bump
  - `fix:` / `perf:` → patch bump
  - `docs:`, `test:`, `ci:`, `chore:`, `refactor:` → no bump by default
  - `!` in the type or a `BREAKING CHANGE:` footer → major bump
- PRs should include: summary, motivation, the exact tests/checks that were
  run with outcomes, and doc updates when behavior changes.

## When Unsure

- Choose the safer behavior.
- Avoid expanding scope beyond the requested change.
- Cite PROPOSAL.md when designing a feature — the proposal is the canonical
  source of intent until ADRs supersede individual sections.
