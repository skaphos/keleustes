<!--
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
-->

# Keleustes Facts

## Identity

- Tool name: `Keleustes`
- Former working name: `Pilot` (retired due to Kubernetes-ecosystem collision)
- Naming status: `settled, collision-reviewed 2026-05-14`
- Repository: `skaphos/keleustes`
- System: `Skaphos`
- Status in Skaphos context: `proposed, not started`
- Primary module: `github.com/skaphos/keleustes`
- Primary language: `Go`
- Frontend language: `TypeScript`

## Purpose

Keleustes is a proposed Kubernetes-native GitOps delivery control plane for platform teams. It combines application inventory, topology-aware deployment state, GitOps reconciliation, release detection, promotion workflows, policy gates, health, diff, audit history, and developer/operator visibility.

Keleustes is intended to replace the combined operational role of Argo CD and Kargo for Skaphos-managed delivery, without replacing Git, Kubernetes, Helm, Kustomize, OCI registries, CI systems, admission controllers, or observability systems.

## Name rationale

`Keleustes` (Greek κελευστής) was the officer on a trireme who set the stroke and cadence for the rowers — coordinating movement and direction through constrained passage. The metaphor maps to GitOps delivery: the system directs release cadence across environments, it does not power the change itself. The name also reinforces the Skaphos Greek-nautical register.

Pronunciation: `kuh-LOO-stees` (anglicized); Greek `keh-loo-STEES`. Latin transliteration `Celeustes` is held as a fallback.

## Collision review (2026-05-14)

- `github.com/keleustes` is a dormant GitHub org (last activity ~2019–2023) holding Kubernetes-adjacent forks (kustomize fork, armada-operator, cluster-api-provider-airship). No active project; no published packages on any active registry. Accepted as search-noise only.
- `github.com/retr0h/keleustes` — abandoned 2017 xhyve tool, unrelated.
- CNCF landscape, Artifact Hub, npm, crates.io, PyPI, active pkg.go.dev modules: clean.
- No company, product, or trademark registered.
- Domains `.io`, `.dev`, `.sh`, `.org` available.

Skaphos disambiguates by publishing under the `skaphos/keleustes` repository path.

## Conceptual components

- Keleustes API: REST API for UI, CLI, integrations, and external automation.
- Keleustes UI: first-class developer/operator visibility into applications, releases, promotions, health, drift, diffs, and audit history.
- Keleustes Controller: Kubernetes controller-runtime based reconciliation core.
- Source Engine: watches Git, OCI, Helm, image, and provenance sources.
- Sync Engine: constrained GitOps reconciler based on server-side apply, inventory tracking, pruning, health checks, and drift detection.
- Promotion Engine: evaluates and executes release movement across environments, cells, regions, and targets.
- Git Mutation Engine: safely mutates Git by PR or direct commit for supported manifest update patterns.
- Policy Engine: evaluates native promotion gates and integrates with external policy/compliance systems over time.
- Health and Diff Engines: provide Argo CD class visibility for live state, desired state, drift, release comparisons, and rendered manifest changes.
- `keleustesctl` CLI: operational CLI for app inspection, matrix views, promotion, diff, blockers, rollback, and administration.

## Core API concepts

- `Application`
- `Source`
- `Release`
- `Environment`
- `Cell`
- `DeploymentTarget`
- `Deployment`
- `Promotion`
- `PromotionPolicy`
- `SyncPlan`
- `SyncRun`
- `HealthCheck`
- `FreezeWindow`
- `Approval`

## Deployment models

- Management cluster mode: one central Keleustes instance manages many clusters.
- Cluster-local mode: Keleustes runs in a single cluster and reconciles local state only.
- Future hub/agent mode: central Keleustes Hub with lightweight Keleustes Agents per managed cluster.

## Initial scope

MVP 0 should be a read-only replacement UI for the "look in Argo CD" habit:

- Application registry
- Environment, cell, and target registry
- Cluster connection
- Git desired state read
- Kubernetes live state read
- Application matrix
- Application health
- Deployed version view
- Resource tree

MVP 1 should add Keleustes-managed sync for constrained Kustomize, Helm, and raw manifest deployments.

MVP 2 should add releases and promotions.

## Boundaries

Keleustes should not initially build:

- container registry
- CI system
- secrets manager
- Backstage replacement
- metrics, logging, or tracing platform
- service mesh
- cloud provisioning system
- generic workflow engine
- change-management system of record

## External references

- Argo CD: declarative GitOps continuous delivery for Kubernetes, including application sync, drift detection, health, UI, and CLI.
- Kargo: continuous promotion platform with Warehouse, Freight, Stage, and Promotion concepts.
- Flux: GitOps Toolkit components, including source, kustomize, helm, notification, and image automation controllers.
