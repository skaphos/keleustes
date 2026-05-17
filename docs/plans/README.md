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

- [2026-05-distributed-runtime-architecture.md](./2026-05-distributed-runtime-architecture.md) — Hub + regional agent/runner model for multi-region DR, load distribution, and resilience. Covers cross-connect transport (NATS leaf default, pluggable), tiered exposure (Gateway API), horizontal scaling at 2K+ repo cardinality, and a no-RDBMS-on-the-critical-path storage model (JetStream + object storage + DuckDB).
- [2026-05-engine-boundaries-and-technology-integration.md](./2026-05-engine-boundaries-and-technology-integration.md) — Internal Go package structure, engine ownership boundaries, technology integration map (including `gitops-engine` reuse, Kustomize/Helm/Helmfile/raw rendering, Gateway API, NATS JetStream + object storage + DuckDB derived layer), and cross-Application dependency ordering for the addon/CRD bootstrap case.
- [2026-05-rbac-audit-and-git-invariant.md](./2026-05-rbac-audit-and-git-invariant.md) — CRD-based RBAC (`IdentityProvider`, `Role`, `RoleBinding`, `Project`, `ApprovalPolicy`) that scales past Argo CD's policy-file model; first-class user-action audit on the JetStream stream; and the hard rule that **no Keleustes feature may create or mutate desired state outside Git** — explicitly forbidding the Argo CD parameter-override and edit-live-resource patterns. Break-glass is the one sanctioned, audited, time-bounded exception.
- [2026-05-extensibility-plugin-surfaces.md](./2026-05-extensibility-plugin-surfaces.md) — Plugin surfaces for the five extension points (`Notifier`, `SignatureVerifier`, `Scanner`, `PolicyGate`, `AuditDestination`), the declarative-CRD-pointing-at-an-HTTPS-endpoint mechanism, shared envelope schema, authentication model, default failure semantics per surface, and what is and is not pluggable. Refines SKA-354 / 370 / 381 / 388 to be implementations of an interface rather than hardcoded vendors.
- [2026-05-observability-stack.md](./2026-05-observability-stack.md) — Default observability bundle: Prometheus Operator manifests (`ServiceMonitor`, `PodMonitor`, `PrometheusRule`), `kube-state-metrics` CustomResourceState for CRD status, Grafana dashboards as ConfigMaps, OpenTelemetry SDK with dual-export, label and cardinality conventions, alert taxonomy (every alert must have a runbook), default SLOs, and regional-agent federation. Gives SKA-336 concrete shape.

## How to Use These Plans

- Read them as input for design discussions.
- Link to specific sections from issues, PRs, or meeting notes.
- When a plan section is ready to become durable guidance, open a discussion and promote it via the ADR process (see `docs/adr/README.md` and the `adr-write` skill).

Do not treat content in `docs/plans/` as binding until it has been captured in an ADR or explicitly accepted in another durable artifact.
