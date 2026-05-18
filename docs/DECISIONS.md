<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Architecture Decisions — Living Index

This is the **single source of truth for "what we have actually decided"**
in Keleustes. PROPOSAL.md is the vision layer; deep-dive plans in
`docs/plans/` are working material; ADRs in `docs/adr/` are the durable
record. When the three disagree, the ADR wins.

Two kinds of entries appear below:

- **ADRs** — immutable accepted decisions. Each one supersedes
  specific PROPOSAL or plan text; that text should carry a "See ADR
  00XX" marker pointing back here.
- **Active interim contracts** — deep-dive plans that have stabilized
  enough to be consumed by code, but have not yet been promoted into
  an ADR. Treat them as authoritative until they promote or are
  rewritten.

Last updated: 2026-05-18.

## Accepted ADRs

| # | Title                                              | Status     | Supersedes                                                                                                          |
|---|----------------------------------------------------|------------|---------------------------------------------------------------------------------------------------------------------|
| [0001](./adr/0001-plugin-extension-model.md) | Plugin extension model (webhook-first, declarative CRDs)         | Accepted   | `2026-05-extensibility-plugin-surfaces.md` §10 (open questions). Refines PROPOSAL §9.2 (cross-cutting concerns) and §15 (policy model). |
| [0002](./adr/0002-default-observability-stack.md) | Default observability stack (Prom-Operator + OTel, dual-export) | Accepted   | `2026-05-observability-stack.md` §12 (open questions). Refines PROPOSAL §9.2 (observability bundle) and §21 (OpenTelemetry bullet).    |
| [0003](./adr/0003-git-source-of-truth-invariant.md) | Git source-of-truth invariant                                 | Accepted   | `2026-05-rbac-audit-and-git-invariant.md` §3, §6, §9, §11. Refines PROPOSAL §11 (Sync rules), §14 (Git mutation), §22 (design principles). |
| [0004](./adr/0004-crd-based-rbac.md) | CRD-based RBAC model                                                       | Accepted   | `2026-05-rbac-audit-and-git-invariant.md` §4–§5, §11 questions 1–5. Refines PROPOSAL §15 (policy gates) and §18 (API auth).            |
| [0005](./adr/0005-distributed-runtime.md) | Distributed runtime (hub + agents, NATS JetStream, no RDBMS)          | Accepted   | `2026-05-distributed-runtime-architecture.md` §13 (open questions). **Supersedes PROPOSAL §10 (deployment), §19 (data model — no Postgres), §21 (storage bullets — no Postgres/Redis).** |
| [0006](./adr/0006-engine-boundaries.md) | Engine boundaries and `gitops-engine` reuse                              | Accepted — amended twice on 2026-05-17 (SKA-327 spike findings; soft-fork strategy abandoned). Afternoon amendment's *Decision* paragraph **partially superseded by ADR 0007** (k8s.io ceiling no longer steady-state). | `2026-05-engine-boundaries-and-technology-integration.md` §7 questions 1–8 and 14–15. **Refines PROPOSAL §9 (architecture — 7 engines + Render, not 3), §11 (sync engine — `gitops-engine` reuse), §13 (diff model), §12 (health model).** |
| [0007](./adr/0007-hard-fork-gitops-engine.md) | Hard-fork `gitops-engine` into `skaphos/gitops-engine`                  | Accepted — amended 2026-05-18 (friendly-fork posture clarification — §3 "fork does not ship patches back" superseded; outbound now tracked in [`UPSTREAMING.md`](https://github.com/skaphos/gitops-engine/blob/main/UPSTREAMING.md) on the fork repo) | ADR 0006 "Soft-fork strategy abandoned" amendment's *Decision* paragraph (frozen v0.34 ceiling). ADR 0006 §4's "vanilla upstream" import-path implication. Refines `docs/plans/2026-05-gitops-engine-spike.md` (extraction strategy + backport workflow now in-tree). |

## Active interim contracts

These plans are stable enough to be consumed by code and Linear
tickets. They have not yet been promoted into ADRs because at least
one section is still in the open-questions pile. Until promotion,
treat them as authoritative — they win against PROPOSAL or earlier
plans on the same topic.

| Plan                                                                                    | Linear  | Stabilizes              | Promotes when                                                                  |
|-----------------------------------------------------------------------------------------|---------|-------------------------|---------------------------------------------------------------------------------|
| [`2026-05-render-contract-and-inventory-model.md`](./plans/2026-05-render-contract-and-inventory-model.md) | SKA-320 | The Render boundary types (`RenderRequest`, `RenderResult`, `Inventory`), pruning rules, content-addressing for the render cache, gitops-engine handoff. **Refines PROPOSAL §11 (Sync engine — inventory + prune)**. | §10 open questions (cluster-cache warm-up, APICapabilities snapshotting, GC sweeper, inventory cutover, renderer determinism conformance) resolve and at least one Renderer is implemented. Likely **ADR 0007** co-located with ADR 0006. |
| [`2026-05-audit-event-schema.md`](./plans/2026-05-audit-event-schema.md)               | SKA-322 | The audit envelope (`schemaVersion`, `eventId`, actor normalization, `payload.@type` discriminator), versioning policy, redaction rules, persistence demands on JetStream. **Refines PROPOSAL §19 (data model — audit lives in JetStream, not Postgres)**. | §15 open questions (partition value, requestId propagation through Git mutation, delegatedFrom depth, CBOR-at-rest deadline, redaction-of-redaction) resolve and SKA-332/SKA-347 implement the first emitter and consumer. Likely **ADR 0008**. |
| [`2026-05-rbac-crd-shapes.md`](./plans/2026-05-rbac-crd-shapes.md)                     | SKA-323 | Concrete CRD schemas for the five RBAC types from ADR 0004 (`IdentityProvider`, `Role`, `RoleBinding`, `Project`, `ApprovalPolicy`): Go shapes with kubebuilder validation markers, shared `Scope`/`Subject`/`VerbRef` primitives, validation webhook outline, status condition taxonomy, sample CRs. **Refines PROPOSAL §15 (Policy model) and §18 (API auth) — both already point at ADR 0004 in the top banner**; this plan is the schema layer beneath ADR 0004. Refines [`2026-05-rbac-audit-and-git-invariant.md`](./plans/2026-05-rbac-audit-and-git-invariant.md) §5.2 (CRD sketches). | §14 open questions (cross-project-grant enforcement timing, User principal name shape, ResolvedActor projection, label selectors inside Role, conditional permissions) resolve and SKA-330 / SKA-345 land the first reconcilers. Likely **ADR 0009**. |
| [`2026-05-jetstream-subject-and-stream-layout.md`](./plans/2026-05-jetstream-subject-and-stream-layout.md) | SKA-324 | The canonical JetStream subject grammar (`keleustes.<class>.<scope>.<kind>.<key>`), seven streams with per-stream retention/replication/discard policies, the `xxhash64`-based partition function and grow strategy, NATS KV bucket layout (`audit-index`, `agent-presence`, `controller-locks`, `deployment-snapshots`, `webhook-dedup`), object-storage archive layout, durable-consumer conventions, cross-shard dependency event delivery (engine plan §2.6 / ADR 0006 §8), multi-region supercluster rules. **Resolves [distributed runtime plan](./plans/2026-05-distributed-runtime-architecture.md) §13 Q15 (retention vs. archive cadence) and Q18 (subject / stream layout).** Satisfies SKA-322 §10 demands; resolves SKA-322 §15 Q1 (partition value semantics — derive from `subject.ulid` for events with subjects, literal `"cluster"` otherwise). | §13 open questions (`partitionCount` mutation cadence, xxhash vs SHA-256, cross-region segmenter ownership, per-Application audit subjects, WorkQueue vs Limits for `keleustes-dependency`) resolve and the MVP 1 benchmark validates the per-class partition counts. Likely **ADR 0010**. |
| [`2026-05-sharded-controller-pattern.md`](./plans/2026-05-sharded-controller-pattern.md) | SKA-328 | Custom controller-runtime predicate-filter sharder (no argo-cd code vendored — ADR 0006 §4 containment) backed by SKA-324's `controller-locks` NATS KV bucket for per-shard leader claim. Sharded controllers: `Application`, `Source`, `SyncPlan`, render worker pool — Promotion/Approval/HealthCheck stay single-leader. Co-shard policy: `SyncPlan` / `SyncRun` / `Deployment` shard on the parent `Application`'s name to preserve per-resource locality. Partition-count growth via two-fleet transition window matching SKA-324 §4.3. Reference Go implementation in `internal/sharder/` (~150 LOC). **Resolves [distributed runtime plan](./plans/2026-05-distributed-runtime-architecture.md) §13 Q14** (sharded vs. leader-elected; which library; when to commit). | §12 open questions (`maxShardsPerPod` edge case; cross-controller pod affinity; predicate cost at MVP 2 benchmark; static-vs-dynamic shard-count config; HPA cap enforcement) resolve and MVP 2's two-fleet cutover from `partitionCount=1` to `=16` has been exercised. Likely **ADR 0011**. |
| [`2026-05-scale-benchmark-harness.md`](./plans/2026-05-scale-benchmark-harness.md) | SKA-326 | Two complementary harnesses under `tools/benchmark/`: a Go binary (CRD-aware workload generation, mock-agent publication, metrics scrape, report generation) plus K6 scripts (provider-HMAC webhook bursts, synthetic UI query patterns). Hybrid cluster fixture (kind for `--profile ci`; real cluster for `--profile mvpN`). Pre-release-only CI cadence via `workflow_dispatch` (trade-off documented in §12.2). Mock agent for CI; real `internal/agent/` binaries for full benchmark. Absolute thresholds per MVP in `tools/benchmark/thresholds/mvpN.yaml` plus relative 110%-of-baseline regression detection. **Resolves [distributed runtime plan](./plans/2026-05-distributed-runtime-architecture.md) §13 Q13** (benchmark harness funding). Turns ADR 0005 §11.5 scale targets into machine-checkable pass criteria. | §14 open questions (threshold-relaxation review process; cloud-cluster IaC automation; long-term trend storage; chart fixture refresh; K6 Prometheus output stability) resolve and at least one MVP-exit gate has been run from a real release-manager workflow. Likely **ADR 0012**. |
| [`2026-05-agent-transport-interface.md`](./plans/2026-05-agent-transport-interface.md) | SKA-321 | Rich typed `internal/agent/transport.Transport` Go interface (`Connect`, `Disconnect`, `ClaimWork`, `ReleaseClaim`, `Heartbeat`, `PublishEvent`, `StreamLargePayload`, `FetchLargePayload`, `Subscribe`, `Status`) with NATS leaf as the only implementation through MVP 2. Outbound-only agent connection; NKey + JWT auth where JWT's `keleustes.deploymentTargets` claim is the authorization input. `Agent` CR spec'd (cluster-scoped, kubebuilder validation, status conditions, printer columns). Deterministic 1:N agent:target ownership via the same `controller-locks` NATS KV bucket SKA-324 §6 / SKA-328 §5 already specified. NATS Object Store for large transient payloads; existing object storage stays for content-addressed durable content. Pre-registered agents via `keleustesctl agent register` (no auto-approve). Identity propagation via SKA-322 §6.5 `actor.delegatedFrom`. Sketches of gRPC (SKA-378) and HTTP/2 long-poll prove interface generality. **Resolves [distributed runtime plan](./plans/2026-05-distributed-runtime-architecture.md) §13 Q8** (transport pluggability timing — interface from day one) **and §13 Q9** (Agent CR shape). | §16 open questions (JWT signing key rotation; multi-region claim affinity; hot-standby multi-agent pattern; bulk-claim API; cross-region emergency work-stealing) resolve and SKA-363's MVP 2 NATS leaf implementation runs through the benchmark harness's `agentsim/` real-agent profile. Likely **ADR 0013**. |

## Spikes and historical reports

These plans were time-boxed investigations. They are retained as
historical context but are not authoritative on their own — their
durable conclusions live in the ADRs they fed into.

| Plan                                                                          | Linear  | Verdict                                                                  |
|-------------------------------------------------------------------------------|---------|---------------------------------------------------------------------------|
| [`2026-05-gitops-engine-spike.md`](./plans/2026-05-gitops-engine-spike.md)   | SKA-327 | Adopt `gitops-engine`. The spike landed on "vanilla upstream + accept k8s.io ≤ v0.34 ceiling" (ADR 0006 amendments). **Superseded by [ADR 0007](./adr/0007-hard-fork-gitops-engine.md)** the same evening — Skaphos hard-forks the engine into `github.com/skaphos/gitops-engine` so the ceiling lift becomes a Skaphos-internal task (SKA-430 extraction, SKA-421 rescoped). |

## Plans that have not yet stabilized

These deep-dive plans are working material — assumptions inside them
may still move. Do not cite them as authoritative until they reach
the "active interim contract" tier.

- [`2026-05-distributed-runtime-architecture.md`](./plans/2026-05-distributed-runtime-architecture.md) — *most of this is promoted into ADR 0005; what remains is JetStream subject/stream layout (SKA-324), agent transport interface (SKA-321), and the scale benchmark harness design (SKA-326).*
- [`2026-05-engine-boundaries-and-technology-integration.md`](./plans/2026-05-engine-boundaries-and-technology-integration.md) — *§7 questions 1–8 and 14–15 promoted into ADR 0006; questions 9–13 promoted into ADR 0005. The package-layout sketch and the seven-engine taxonomy are now authoritative via ADR 0006.*
- [`2026-05-extensibility-plugin-surfaces.md`](./plans/2026-05-extensibility-plugin-surfaces.md) — *§10 open questions promoted into ADR 0001. Per-surface envelope and dispatcher specifics are still working material.*
- [`2026-05-observability-stack.md`](./plans/2026-05-observability-stack.md) — *§12 open questions promoted into ADR 0002. Per-engine dashboard set and alert taxonomy still working.*
- [`2026-05-rbac-audit-and-git-invariant.md`](./plans/2026-05-rbac-audit-and-git-invariant.md) — *Git invariant promoted into ADR 0003; RBAC into ADR 0004; audit envelope formalized in the active interim contract above (SKA-322).*
- [`2026-05-operator-crd-integration.md`](./plans/2026-05-operator-crd-integration.md) — *SKA-431. `HealthAssessor` + `DiffNormalizer` CRD surfaces for customer-extensible health/diff rules, Skaphos-curated registry, CRD-owner-shipped, precedence + audit. Draft as of 2026-05-18; promotes to an active interim contract once §12 open questions resolve and the first MVP 1 reconciler scaffolds land.*
- [`2026-05-value-change-promotion.md`](./plans/2026-05-value-change-promotion.md) — *SKA-432. Extends `Promotion.spec.changes[]` to carry structured value diffs; `Application.spec.values.schema[]` is the path-allowlist + Git-resolution contract; Git Mutation Engine produces one PR per Promotion. Mixed-mode (release + changes) composes both intent types. Draft as of 2026-05-18; promotes once MVP 2 ships the first reconciler + Git Mutation Engine handoff (SKA-352, SKA-353). **§5.2 `location` shape amended by [`2026-05-repo-layout-and-branch-promotion.md`](./plans/2026-05-repo-layout-and-branch-promotion.md) §8** (adds `branch` + `${envPath}`/`${addonPath}` tokens per layout).*
- [`2026-05-repo-layout-and-branch-promotion.md`](./plans/2026-05-repo-layout-and-branch-promotion.md) — *SKA-434. Three golden repo-layout paths — branch-per-env merge (default for `Application`), flat-with-env-dirs + waves (Application opt-in), library+integration two-repo (default for the new `Addon` CRD). `custom` escape hatch with documented primitive contract but no built-in handler. `Addon` is a first-class CRD distinct from Application with default-on consumer-aware upgrade gates, per-K8s-version compatibility, and scalable consumer enumeration via Application-side annotations. Per-layout `MutatingGit` semantics and audit-verb routing locked in. Draft as of 2026-05-18; promotes once Golden Paths 1 + 3 have shipped end-to-end with at least one real customer each.*
- [`2026-05-git-mutation-attribution.md`](./plans/2026-05-git-mutation-attribution.md) — *SKA-433. Three Git-mutation attribution modes — `user-to-server` (default; per-user OAuth against the Keleustes GitHub App), `bot-with-trailer` (installation-token + `Co-authored-by`), `service-account` (per-Project dedicated identity). Selectable via `IdentityProvider.spec.gitMutationAttribution`; per-Project / per-Application overrides supported. Token storage in NATS KV (default) with `secret-per-user` MVP-2 starter. Per-provider matrix for GitHub, GHES, GitLab, Azure DevOps, Bitbucket. Audit envelope's `actor.delegatedFrom` carries the Keleustes-side trigger when wire identity differs. Draft as of 2026-05-18; promotes once MVP 2 ships both modes for GitHub.com and a real customer exercises the per-Project override path.*

## Process: keeping this index honest

When an ADR is accepted (or an interim contract stabilizes) that
moves a material assumption from PROPOSAL or an earlier plan, the
author opens a small companion PR that:

1. Adds the entry to the table above.
2. Touches each superseded passage in PROPOSAL or the earlier plan
   with a `> **See [ADR 00XX](./adr/00XX-…)**` marker. The original
   text stays put for archaeological reasons; the marker prevents
   silent drift between layers.
3. Cross-references the marker from the ADR's `Supersedes:`
   front-matter line.

The full guidance lives in [`adr/README.md`](./adr/README.md).
