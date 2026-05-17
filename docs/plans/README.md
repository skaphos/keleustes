<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Planning Documents

This directory contains mutable, reviewable planning artifacts for Keleustes.

## Purpose

`docs/plans/` holds working plans, roadmaps, and architectural exploration documents. These are **not** decisions. They exist to:

- Explore complex, long-term topics before architecture is locked in.
- Provide concrete, reviewable material for the team.
- Surface assumptions, options, and trade-offs.
- Feed into Architecture Decision Records (ADRs) when sections stabilize.

Plans are allowed (and expected) to evolve or be superseded. When a planning document reaches sufficient clarity on a topic, the relevant portions are promoted into an ADR in `docs/adr/`. The original plan is retained for historical context and can be marked as superseded.

## Naming Convention

- `YYYY-MM-descriptive-slug.md` (e.g., `2026-05-distributed-runtime-architecture.md`)
- Use lowercase with hyphens.
- Date prefix reflects when the plan was first drafted (not last updated).

## Lifecycle

1. **Draft** — Initial version, open for discussion.
2. **In Review** — Actively being discussed with stakeholders.
3. **Superseded** — A newer plan or an ADR has replaced the content. Add a note at the top linking to the successor.
4. **Partially Promoted** — Key sections have become ADRs; the plan remains as context.

## Relationship to Other Documents

- **PROPOSAL.md** — The canonical high-level vision and MVP roadmap. Plans should align with or propose updates to it.
- **FACTS.md** — Stable facts about the project.
- **docs/adr/** — Immutable records of accepted architectural decisions. Plans are the input to this process.
- Individual CRD types and controller stubs in the codebase are the current implementation baseline.

## Current Plans

> **Status tags** below indicate whether a plan is still authoritative
> as-written, or whether an ADR (or active interim contract) has moved
> on top of it. The **[Architecture Decisions Living Index](../DECISIONS.md)**
> is the consolidated view.

| Status | Meaning |
|---|---|
| 🟢 **Active interim contract** | Stabilized enough to be cited by code/tickets; not yet promoted to an ADR. Authoritative until then. |
| 🟡 **Partially promoted** | Most decisions already live in an ADR; remaining sections are working material. |
| 🟠 **Spike report** | Time-boxed investigation. Conclusions live in the ADR(s) they fed. Retained for historical context. |
| ⚪ **Working material** | Still draft; do not cite as authoritative. |

### Architecture / runtime

- 🟡 [2026-05-distributed-runtime-architecture.md](./2026-05-distributed-runtime-architecture.md) — Hub + regional agent/runner model; NATS leaf transport; no-RDBMS storage. **§13 open questions promoted into [ADR 0005](../adr/0005-distributed-runtime.md).** What remains: JetStream subject/stream layout (SKA-324), agent transport interface (SKA-321), scale benchmark harness (SKA-326).
- 🟡 [2026-05-engine-boundaries-and-technology-integration.md](./2026-05-engine-boundaries-and-technology-integration.md) — Internal Go package structure, engine ownership boundaries, `gitops-engine` reuse, render technology stack. **§7 questions 1–8 and 14–15 promoted into [ADR 0006](../adr/0006-engine-boundaries.md); questions 9–13 into [ADR 0005](../adr/0005-distributed-runtime.md).** Section §8 action 6 ("Render Contract & Inventory Model deep-dive") is now the SKA-320 interim contract below.

### Active interim contracts (cite-as-authoritative)

- 🟢 [2026-05-render-contract-and-inventory-model.md](./2026-05-render-contract-and-inventory-model.md) — **SKA-320.** `RenderRequest` / `RenderResult` Go shapes, `Inventory` model with stable `ResourceKey` ownership, pruning rules (set-difference, hand-off, CRD-and-instance ordering), content-addressed render cache, `gitops-engine` handoff at `internal/sync/`. Promotes to ADR 0007 when §10 open questions resolve.
- 🟢 [2026-05-audit-event-schema.md](./2026-05-audit-event-schema.md) — **SKA-322.** Audit envelope (`schemaVersion`, `eventId`, actor normalization, `payload.@type` discriminated union), versioning policy, redaction rules, event-type registry across 9 categories. Promotes to ADR 0008 (likely) when §15 open questions resolve and SKA-332/SKA-347 implement the first emitter and consumer.
- 🟢 [2026-05-rbac-crd-shapes.md](./2026-05-rbac-crd-shapes.md) — **SKA-323.** Concrete CRD schemas for the five RBAC types from ADR 0004 (`IdentityProvider`, `Role`, `RoleBinding`, `Project`, `ApprovalPolicy`): Go shapes with kubebuilder markers, shared `Scope` / `Subject` / `VerbRef` primitives, validation webhook outline (per-CRD admission rules), status condition taxonomy, sample CRs. Promotes to ADR 0009 (likely) when §14 open questions resolve and SKA-330 / SKA-345 land the first reconcilers.

### Spike reports (historical)

- 🟠 [2026-05-gitops-engine-spike.md](./2026-05-gitops-engine-spike.md) — **SKA-327.** Empirical adoption cost. Original verdict ("soft fork + upstream PR + 90-day check") was reversed within hours on 2026-05-17 when `pkg/utils/kube/scheme` was discovered to hold the k8s.io ≤ v0.34 ceiling independently of `pkg/health`. Conclusions promoted into [ADR 0006](../adr/0006-engine-boundaries.md)'s 2026-05-17 amendments.

### Cross-cutting

- 🟡 [2026-05-rbac-audit-and-git-invariant.md](./2026-05-rbac-audit-and-git-invariant.md) — Git invariant promoted into [ADR 0003](../adr/0003-git-source-of-truth-invariant.md); RBAC promoted into [ADR 0004](../adr/0004-crd-based-rbac.md); audit envelope formalized in the SKA-322 active interim contract above.
- 🟡 [2026-05-extensibility-plugin-surfaces.md](./2026-05-extensibility-plugin-surfaces.md) — **§10 open questions promoted into [ADR 0001](../adr/0001-plugin-extension-model.md).** Per-surface envelope and dispatcher specifics remain working material.
- 🟡 [2026-05-observability-stack.md](./2026-05-observability-stack.md) — **§12 open questions promoted into [ADR 0002](../adr/0002-default-observability-stack.md).** Per-engine dashboard set and alert taxonomy remain working material.

## How to Use These Plans

- Read them as input for design discussions.
- Link to specific sections from issues, PRs, or meeting notes.
- When a plan section is ready to become durable guidance, open a discussion and promote it via the ADR process (see `docs/adr/README.md` and the `adr-write` skill).

Do not treat content in `docs/plans/` as binding until it has been captured in an ADR or explicitly accepted in another durable artifact.
