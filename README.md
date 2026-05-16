<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Skaphos Keleustes

Skaphos Keleustes is an open-source GitOps delivery control plane for
Kubernetes platforms.

Keleustes combines application inventory, topology-aware deployment, GitOps
reconciliation, promotion workflows, policy gates, and audit history into a
single Kubernetes-native system.

Keleustes is designed for platform teams running many applications across many
clusters, regions, environments, and failure domains.

> **Status.** Keleustes is in the **scaffold** stage. The repository pins the
> module layout, CRD surface, controller entrypoints, and tooling described in
> the design proposal. Reconciliation engines (Source, Sync, Promotion, Git
> Mutation, Policy, Health, Diff) arrive across MVP 0 → MVP 4 as described
> below.

## What Keleustes replaces

- Argo CD application sync and visibility
- Kargo-style promotion orchestration
- Custom CI scripts for environment promotion
- Spreadsheet-driven deployment tracking
- Ad hoc Git mutation bots

## What Keleustes does not replace

- Git
- Kubernetes
- Helm
- Kustomize
- OCI registries
- CI systems
- Observability stacks
- Admission controllers

## Design principles

1. Git is the durable desired-state boundary.
2. Promotion is a first-class object.
3. Topology is part of deployment state.
4. Reconciliation must be explainable.
5. The UI must show intent, not just objects.
6. Every mutation must be auditable.
7. The system must support read-only degradation.
8. Application count should not be a pricing weapon.
9. Kubernetes-native does not mean Kubernetes-only UX.
10. Operators must be able to recover with Git and CLI.

## Naming

`Keleustes` (Greek κελευστής) was the officer on a trireme who set the stroke
and cadence for the rowers — coordinating movement, pacing, and direction
through constrained passage. The metaphor maps to GitOps delivery: the system
directs the cadence of release movement, it does not power the change itself.
The name also reinforces the Skaphos Greek-nautical register.

Pronunciation: `kuh-LOO-stees` (anglicized); Greek `keh-loo-STEES`.

The Latin transliteration `Celeustes` is held as a fallback. See
`docs/NAMING.md` for the full collision review.

## Repository layout

```
api/v1alpha1/          # CRD type definitions (Application, Source, Release, …)
cmd/manager/           # Keleustes controller-manager binary
cmd/keleustesctl/      # keleustesctl operator CLI
internal/cli/          # cobra command tree for keleustesctl
internal/controller/   # Reconciler implementations
config/                # Kustomize manifests: CRDs, RBAC, manager, samples
docs/                  # Architecture/decision docs (ADRs live under docs/adr/)
hack/                  # Generated-code boilerplate header
tools/                 # Pinned developer tooling (Task, controller-gen, …)
ui/                    # Placeholder for the React/TypeScript UI (PROPOSAL §16)
```

## Core CRDs (group `keleustes.skaphos.io`)

| Kind                | Purpose                                                                  |
| ------------------- | ------------------------------------------------------------------------ |
| `Application`       | Central delivery abstraction: ownership, manifests, topology, status.    |
| `Source`            | Stream of deployable inputs (container images, Git, Helm, OCI).          |
| `Release`           | Pinned collection of artifacts for an Application.                       |
| `Environment`       | Ordered lifecycle boundary (dev, test, qa, prod).                        |
| `Cell`              | Failure domain or operational grouping inside an Environment.            |
| `DeploymentTarget`  | Concrete cluster where an Application can run.                           |
| `Deployment`        | Live-state record of an Application on a DeploymentTarget.               |
| `Promotion`         | Requested movement of a Release into one or more targets.                |
| `PromotionPolicy`   | Gate set required by a Promotion.                                        |
| `SyncPlan`          | Binding from an Application to DeploymentTargets.                        |
| `SyncRun`           | Single reconciliation attempt by the Sync Engine.                        |
| `HealthCheck`       | Health evaluation for an Application on a target.                        |
| `FreezeWindow`      | Window during which promotions to a scope are blocked.                   |
| `Approval`          | Approver decision on a Promotion.                                        |

## MVP roadmap

- **MVP 0** — Read-only replacement UI. Application/topology registry, Git
  desired-state read, cluster live-state read, matrix, health, deployed version,
  resource tree.
- **MVP 1** — Keleustes-managed sync for kustomize/helm/raw manifest deployments.
- **MVP 2** — Releases and promotions, Git mutation, approval gates, basic
  policy.
- **MVP 3** — Enterprise topology: cells, regions, waves, freeze windows,
  change-ticket integration.
- **MVP 4** — Policy and audit acceptable for regulated environments.

## Building

Tooling is pinned in `tools/` and orchestrated through Task. Bootstrap and run:

```bash
cd tools && go mod tidy   # one-time
go -C tools tool task --list
go -C tools tool task ci  # full local CI (lint, test, staticcheck, vuln, build)
```

A non-exhaustive task surface:

```bash
go -C tools tool task manifests        # regenerate CRDs + RBAC
go -C tools tool task generate         # regenerate deepcopy implementations
go -C tools tool task build            # build cmd/manager
go -C tools tool task build-ctl        # build cmd/keleustesctl
go -C tools tool task run              # run the manager against current kubeconfig
go -C tools tool task install          # install CRDs into the current cluster
go -C tools tool task deploy           # deploy the manager into the current cluster
```

## Public positioning

> Existing GitOps tools split reconciliation, promotion, topology, and
> visibility across separate products. Keleustes provides a unified open-source
> control plane for GitOps delivery without requiring organizations to adopt a
> commercial SaaS control layer.

## License

MIT. See `LICENSE` and `REUSE.toml`.
