<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Copilot review instructions

This repo has strong conventions a generic reviewer would miss. Flag any
violation of the rules below; suggest the convention-aligned fix.

This file is advisory — Copilot reviews do not gate merges. The
authoritative source for every rule below is the cited ADR, plan, or
configuration file; cite them in review comments when possible so authors
can read the rationale.

## Hard invariants (block-on-violation suggestions)

These are non-negotiable architectural commitments. A violation is always
worth a comment.

1. **Git is the only path to desired state.** No code may create or mutate
   cluster-side desired state outside Git. Argo-CD-style "edit live
   resource," "parameter override," or "sync with overridden parameters"
   patterns are forbidden. Break-glass is the one sanctioned exception
   and is itself audited.
   - Source: [ADR 0003](../docs/adr/0003-git-source-of-truth-invariant.md);
     [`docs/plans/2026-05-rbac-audit-and-git-invariant.md`](../docs/plans/2026-05-rbac-audit-and-git-invariant.md) §3.

2. **No RDBMS on the critical path.** Active state lives in CRDs (etcd),
   event/audit history in NATS JetStream, hot indexes in NATS KV, large
   content-addressed artifacts in object storage, derived analytics in
   DuckDB-on-parquet. A Postgres/MySQL/SQL Server dependency in the
   operator's runtime path is disqualified. A reference SQL consumer is
   allowed under `contrib/`; it is **not** a runtime dependency.
   - Source: [ADR 0005](../docs/adr/0005-distributed-runtime.md) §3.

3. **Reconcilers are idempotent and bounded.** No unbounded work in a
   `Reconcile` loop; reconciling the same input twice must produce the
   same output. Brief double-coverage during sharded failover and replay
   after JetStream reconnect rely on this guarantee.
   - Source: `CLAUDE.md` § Hard invariants.

4. **No `Ingress` API resources.** Flag any manifest under `config/` that
   ships `kind: Ingress` or imports `networking.k8s.io/v1.Ingress` /
   `extensions/v1beta1.Ingress` types. Use Gateway API v1
   (`gateway.networking.k8s.io/v1`) instead. Documentation references
   that *explain* the ban (e.g. `config/gateway/README.md`) are fine —
   don't flag them.
   - Source: [`docs/plans/2026-05-distributed-runtime-architecture.md`](../docs/plans/2026-05-distributed-runtime-architecture.md) §7.5;
     `config/gateway/README.md`.

5. **`gitops-engine` containment.** Imports of `gitops-engine` may
   appear **only** in files under `internal/sync/`, `internal/diff/`,
   `internal/health/`, or `internal/kube/`. This applies to both module
   paths — `github.com/argoproj/argo-cd/gitops-engine/...` (current,
   pending the SKA-430 go.mod swap) and `github.com/skaphos/gitops-engine/...`
   (post-swap). Any other package importing the engine via either path
   is a containment violation.
   - Source: [ADR 0006](../docs/adr/0006-engine-boundaries.md) §4;
     [ADR 0007](../docs/adr/0007-hard-fork-gitops-engine.md) §4.

6. **Audit envelope shape is frozen.** The `Envelope` struct in
   `internal/audit/envelope.go` must match `docs/plans/2026-05-audit-event-schema.md`
   §3 verbatim — top-level keys are closed, required fields are forever,
   new event types go in `payload`/`evidence`/`context`. Changes need a
   `schemaVersion` bump per §5.3 (rare; deprecation lane).
   - Source: [`docs/plans/2026-05-audit-event-schema.md`](../docs/plans/2026-05-audit-event-schema.md).

## Repo-specific quality bar

- **DCO required.** Every commit must carry a `Signed-off-by:` trailer.
  Authors use `git commit --signoff`.
<!-- REUSE-IgnoreStart -->
- **REUSE 3.3 compliant.** Hand-written Go / YAML / Markdown source
  files carry inline SPDX headers (the standard two-line
  `SPDX-FileCopyrightText: 2026 Skaphos` + `SPDX-License-Identifier: MIT`
  pair). Dotfiles, JSON configs, and other formats that can't carry
  comments (e.g. `.gitignore`, `.tool-versions`, `staticcheck.conf`,
  `.repokeeper-repo.yaml`) are covered by the blanket `**` annotation
  in `REUSE.toml` and need no inline header. Generated files inherit
  headers via tooling. **Let `pipx run reuse lint` be the arbiter** —
  only flag a missing header when `reuse lint` would also flag it.
<!-- REUSE-IgnoreEnd -->
- **Conventional Commits.** Commit subjects use `feat:`, `fix:`, `docs:`,
  `chore:`, `ci:`, `refactor:`, `test:`, `perf:`. `release-please` infers
  next version from these on `main`.
- **Generated files are read-only by hand.** `zz_generated.deepcopy.go`,
  `config/crd/bases/*.yaml`, `config/rbac/role.yaml`,
  `THIRD_PARTY_LICENSES.md` are produced by tooling. Flag any PR that
  edits them directly without re-running the corresponding `task` target.
- **Taskfile is the source of truth for tool versions.** Tools invoked via
  `go run <module>@<version>` in `Taskfile.yml` must stay pinned to
  explicit versions, never `@latest`. The same applies to
  `tools/go.mod`'s tool directives.
- **`@latest` is banned in CI workflows and in `Taskfile.yml`.**
  `.github/workflows/*.yml` and `Taskfile.yml` may not run
  `go run X@latest` — all tool fetches are pinned. Exception:
  `actions/*` first-party Actions may pin to a floating major tag
  (`@v6`); third-party Actions must be SHA-pinned with a version comment.

## Engine ownership boundaries (Go imports)

Per [ADR 0006](../docs/adr/0006-engine-boundaries.md) §1–§3 — flag any
diff that crosses these boundaries:

- `internal/controller/` may depend on engine packages but **not** the
  other way around. Reconcilers stay thin (~150 LOC max) and delegate to
  engines.
- Engines (`internal/source/`, `internal/sync/`, `internal/promotion/`,
  `internal/mutation/`, `internal/policy/`, `internal/health/`,
  `internal/diff/`) may depend on shared packages (`render/`,
  `inventory/`, `events/`, `store/`, `kube/`, `git/`, `plugins/`) but
  **not** on each other directly — communicate via the event bus or
  narrow interfaces.
- `internal/render/` is the shared rendering package every engine
  depends on. It does not depend on engines.
- Additional blessed packages from
  [ADR 0006 §2](../docs/adr/0006-engine-boundaries.md): `internal/api/`
  (future REST/gRPC server + authz middleware), `internal/cli/`
  (keleustesctl tree — thin), `internal/agent/` (agent protocol +
  transport), `internal/webhooks/` (provider webhook receivers),
  `internal/util/` (small, genuinely shared utilities). New top-level
  packages under `internal/` outside this set need an ADR or plan
  justification.

## Audit, observability, RBAC conventions

- **State-mutating reconciler writes go through `internal/audit/`.** New
  reconcilers that don't call `audit.Emit` for create/update/delete on
  their owned resources are incomplete. The MVP 0 emitter is `LogEmitter`
  (one canonical-JSON log line per event); JetStream wiring lands in
  MVP 1 (SKA-347).
- **Structured logs use the closed label vocabulary.** Engine code uses
  the label constants in `internal/observability/labels.go`. Don't
  invent new label keys — extending the vocabulary requires updating
  the plan and the constants file.
- **Application + target labels are unbounded — gauge/counter only,
  never histogram.** The `application` and `target` label values are
  unbounded by the customer's application count. They are permitted on
  counters and gauges where the application/target dimension is
  essential, but never on histograms (would explode bucket cardinality).
  Bounded labels (`engine`, `environment`, `region`, `result`, `phase`)
  may appear anywhere. Flag histogram metrics that carry `application`
  or `target` labels.
  - Source: [`docs/plans/2026-05-observability-stack.md`](../docs/plans/2026-05-observability-stack.md) §3.1;
    `internal/observability/labels.go`.
- **Audit redaction is centralized.** Code that snapshots a Kubernetes
  resource into audit must call `audit/redaction.Apply` before
  serializing — don't hand-redact, don't skip.
- **CRD RBAC is via `+kubebuilder:rbac` markers** on the reconciler.
  Cluster-wide RBAC beyond what the operator needs is a red flag.

## Gateway API + config conventions

- **Gateway API v1 only.** Flag any `kind: Ingress` resource. Flag any
  `v1beta1` Gateway API resources except in vendored upstream charts
  (none exist here yet).
- **Customer-provided `GatewayClass`.** Sample Gateways use
  `gatewayClassName: REPLACE_ME` — flag any PR that hardcodes a
  controller-specific class without a corresponding overlay-patch
  example.
- **Tiered Gateway separation.** Internal/API, public webhooks,
  agent-transport, and UI tiers each get their own `Gateway`. Don't
  consolidate them into one — different identity models, different
  scaling profiles. See [`config/gateway/README.md`](../config/gateway/README.md) §"What this overlay
  deliberately does not do".

## CRD types (`api/v1alpha1/`)

- **Kubebuilder markers required.** Validation
  (`+kubebuilder:validation:*`), printer columns, status subresources,
  scope (`Namespaced` unless cluster-scoped is justified) — all via
  markers, not hand-written CRD YAML.
- **CEL `XValidation` rules** are preferred over admission webhook
  business logic for cross-field invariants. `Notifier` is the
  reference example (`endpoint.builtin` XOR `endpoint.webhook`).
- **Status carries `ObservedGeneration` + a typed `Conditions` array.**
  Reconcilers set `Conditions[type=Accepted]` minimum; richer condition
  taxonomy comes per-engine.
- **Adding a new CRD requires regenerating `zz_generated.deepcopy.go`,
  `config/crd/bases/`, and `config/rbac/role.yaml`** via `task lint`.
  Flag PRs that add an API type without the matching regenerated
  artifacts.

## `skaphos/gitops-engine` posture (for code referring to the fork)

- **Friendly fork.** The fork at `github.com/skaphos/gitops-engine`
  actively maintains the intent to upstream Skaphos-originated work
  back to `argoproj/argo-cd`. Commits there should be shaped for
  upstream PR submission (atomic, upstream-style subject, no
  Skaphos-only jargon).
  - Source: [ADR 0007](../docs/adr/0007-hard-fork-gitops-engine.md)
    2026-05-18 amendment.

- **Don't reach back to `github.com/argoproj/argo-cd/gitops-engine`.**
  All imports of the engine should use `github.com/skaphos/gitops-engine`
  once the Keleustes `go.mod` swap lands. Until then both paths may
  appear; flag any *new* import of the argoproj path in code committed
  after the swap.

## ADR + DECISIONS conventions

- **ADRs are immutable once accepted.** Corrections that change a
  decision land as a new ADR that supersedes the old one. Corrections
  that clarify intent without changing the decision land as a dated
  amendment section appended to the ADR (precedent: ADR 0006 and
  ADR 0007 amendments).
- **Supersession marker process.** When an ADR/interim contract moves a
  material assumption, the same PR must (a) drop a
  `> **Superseded by [ADR 00XX](...).**` blockquote on the superseded
  passage, (b) update the `Supersedes:` front-matter line in the new
  ADR, and (c) update `docs/DECISIONS.md`. Flag PRs that update one
  layer but not the others.
- **Generated graph artifacts** (`graphify-out/`) are refreshed via
  `graphify update .` and committed as a separate `chore(graphify):`
  commit, not bundled with the feature commit. Flag bundled graph
  refreshes when the feature commit is otherwise reviewable on its own.

## What to ignore (no comments needed)

- The `replace` block in `go.mod` pinning `k8s.io/*` to `v0.34.0`.
  Pre-existing constraint per ADR 0006 amendment; SKA-421 against the
  fork will lift it. Don't suggest dependency bumps that conflict
  with the pin.
- govulncheck advisory findings rooted in `k8s.io/api v0.34.0`. Same
  reason. CI marks the vuln job `continue-on-error: true`.
- The `mfacenet/argo-cd` remote in any operator-local git config
  documentation. Operator's personal mirror; unrelated to the fork.

## Tone

Brief, specific, cite a source file or ADR by section. Don't restate
what `golangci-lint`, `staticcheck`, or `go vet` would say — assume
those ran and passed.
