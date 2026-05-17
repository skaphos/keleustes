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

Last updated: 2026-05-17.

## Accepted ADRs

| # | Title                                              | Status     | Supersedes                                                                                                          |
|---|----------------------------------------------------|------------|---------------------------------------------------------------------------------------------------------------------|
| [0001](./adr/0001-plugin-extension-model.md) | Plugin extension model (webhook-first, declarative CRDs)         | Accepted   | `2026-05-extensibility-plugin-surfaces.md` §10 (open questions). Refines PROPOSAL §9.2 (cross-cutting concerns) and §15 (policy model). |
| [0002](./adr/0002-default-observability-stack.md) | Default observability stack (Prom-Operator + OTel, dual-export) | Accepted   | `2026-05-observability-stack.md` §12 (open questions). Refines PROPOSAL §9.2 (observability bundle) and §21 (OpenTelemetry bullet).    |
| [0003](./adr/0003-git-source-of-truth-invariant.md) | Git source-of-truth invariant                                 | Accepted   | `2026-05-rbac-audit-and-git-invariant.md` §3, §6, §9, §11. Refines PROPOSAL §11 (Sync rules), §14 (Git mutation), §22 (design principles). |
| [0004](./adr/0004-crd-based-rbac.md) | CRD-based RBAC model                                                       | Accepted   | `2026-05-rbac-audit-and-git-invariant.md` §4–§5, §11 questions 1–5. Refines PROPOSAL §15 (policy gates) and §18 (API auth).            |
| [0005](./adr/0005-distributed-runtime.md) | Distributed runtime (hub + agents, NATS JetStream, no RDBMS)          | Accepted   | `2026-05-distributed-runtime-architecture.md` §13 (open questions). **Supersedes PROPOSAL §10 (deployment), §19 (data model — no Postgres), §21 (storage bullets — no Postgres/Redis).** |
| [0006](./adr/0006-engine-boundaries.md) | Engine boundaries and `gitops-engine` reuse                              | Accepted — amended twice on 2026-05-17 (SKA-327 spike findings; soft-fork strategy abandoned) | `2026-05-engine-boundaries-and-technology-integration.md` §7 questions 1–8 and 14–15. **Refines PROPOSAL §9 (architecture — 7 engines + Render, not 3), §11 (sync engine — `gitops-engine` reuse), §13 (diff model), §12 (health model).** |

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

## Spikes and historical reports

These plans were time-boxed investigations. They are retained as
historical context but are not authoritative on their own — their
durable conclusions live in the ADRs they fed into.

| Plan                                                                          | Linear  | Verdict                                                                  |
|-------------------------------------------------------------------------------|---------|---------------------------------------------------------------------------|
| [`2026-05-gitops-engine-spike.md`](./plans/2026-05-gitops-engine-spike.md)   | SKA-327 | Adopt vanilla upstream `gitops-engine`; accept k8s.io ≤ v0.34 as steady-state ceiling. **Originally** recommended a soft fork; reversed within hours after the `pkg/utils/kube/scheme` blanket-install was discovered to hold the ceiling independently. Conclusions promoted into ADR 0006's amendments. |

## Plans that have not yet stabilized

These deep-dive plans are working material — assumptions inside them
may still move. Do not cite them as authoritative until they reach
the "active interim contract" tier.

- [`2026-05-distributed-runtime-architecture.md`](./plans/2026-05-distributed-runtime-architecture.md) — *most of this is promoted into ADR 0005; what remains is JetStream subject/stream layout (SKA-324), agent transport interface (SKA-321), and the scale benchmark harness design (SKA-326).*
- [`2026-05-engine-boundaries-and-technology-integration.md`](./plans/2026-05-engine-boundaries-and-technology-integration.md) — *§7 questions 1–8 and 14–15 promoted into ADR 0006; questions 9–13 promoted into ADR 0005. The package-layout sketch and the seven-engine taxonomy are now authoritative via ADR 0006.*
- [`2026-05-extensibility-plugin-surfaces.md`](./plans/2026-05-extensibility-plugin-surfaces.md) — *§10 open questions promoted into ADR 0001. Per-surface envelope and dispatcher specifics are still working material.*
- [`2026-05-observability-stack.md`](./plans/2026-05-observability-stack.md) — *§12 open questions promoted into ADR 0002. Per-engine dashboard set and alert taxonomy still working.*
- [`2026-05-rbac-audit-and-git-invariant.md`](./plans/2026-05-rbac-audit-and-git-invariant.md) — *Git invariant promoted into ADR 0003; RBAC into ADR 0004; audit envelope formalized in the active interim contract above (SKA-322).*

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
