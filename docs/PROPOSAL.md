<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Keleustes Proposal (Draft)

Status: `Draft v0.1`
Owner: `Skaphos`
Last updated: `2026-05-14`
Name status: `Settled; collision-reviewed 2026-05-14 (former working name: Pilot)`

---

## 1. Executive summary

Keleustes is a proposed open-source GitOps delivery control plane for Kubernetes platforms.

It combines topology-aware application modeling, GitOps reconciliation, release detection, promotion workflows, policy gates, health, diff, audit history, and developer/operator visibility into a single Kubernetes-native system.

The working thesis is:

```text
Keleustes =
  topology-aware app model
  + GitOps reconciliation
  + promotion workflow
  + policy gates
  + change and audit context
  + developer/operator UI
```

Keleustes is not managed Argo CD, Kargo plus a dashboard, or a Flux UI. It is a delivery control plane that treats application delivery state as a first-class operational model.

The intended replacement scope is the combined operational role many platform teams currently split across:

- Argo CD for application sync, drift detection, health, and UI.
- Kargo for promotion orchestration and environment progression.
- Custom CI scripts for Git mutation and environment promotion.
- Spreadsheets or dashboards for deployment tracking.

Keleustes does not replace Git, Kubernetes, Helm, Kustomize, OCI registries, admission controllers, CI systems, observability stacks, or change-management systems of record.

---

## 2. Naming status

`Keleustes` is the settled product name. It replaces the earlier working name `Pilot`, which was retired due to Kubernetes-ecosystem collision (notably Istio's historical `Pilot` control-plane component and various `KubePilot` projects).

Keleustes (Greek κελευστής) was the officer on a trireme who set the stroke and cadence for the rowers — coordinating movement, pacing, and direction through constrained passage. That maps cleanly to application release movement across environments, cells, regions, and production gates: the system does not power the change, it directs the cadence.

The name also reinforces the Skaphos Greek-nautical register (Skaphos itself is Greek for *vessel/hull*).

Pronunciation: `kuh-LOO-stees` (anglicized); Greek `keh-loo-STEES`.

### Collision review outcome

A collision review was completed on 2026-05-14 across GitHub, CNCF landscape, Artifact Hub, pkg.go.dev, npm, crates.io, PyPI, web, and trademark surfaces. Findings:

- `github.com/keleustes` is a dormant GitHub org (last activity ~2019–2023) holding Kubernetes-adjacent forks (a kustomize fork, armada-operator, cluster-api-provider-airship, capi-yaml-gen). No active project, no published packages on any active registry.
- `github.com/retr0h/keleustes` is an abandoned 2017 xhyve VM tool — unrelated and dormant.
- CNCF landscape, Artifact Hub, npm, crates.io, PyPI, and active Go modules: **no collisions**.
- No company, product, or trademark registered under Keleustes or the Latin transliteration Celeustes.
- Domains `.io`, `.dev`, `.sh`, `.org` returned as available.

The dormant `github.com/keleustes` org is accepted as **search-noise only**, not a semantic collision. To distinguish ownership without ambiguity, Skaphos publishes under the `skaphos/keleustes` repository path. The Latin transliteration `Celeustes` is held as a fallback if the org is ever reactivated by a third party.

---

## 3. Problem statement

Current GitOps delivery systems split the operating model in a way that does not match how larger platform teams actually deploy software.

Argo CD centers on application reconciliation: desired state in Git, live state in Kubernetes, drift detection, sync, health, and UI.

Kargo centers on promotion orchestration: packaging artifact revisions and moving them through stages, often leaving actual deployment to a GitOps agent such as Argo CD.

Flux provides a cleaner controller-oriented architecture through separate source, kustomize, helm, notification, and image automation controllers, but it does not provide the full product surface most developers and operators expect from Argo CD and Kargo together.

The result is a fragmented delivery stack:

```text
Argo CD = sync + UI
Kargo   = promotion context
Flux    = clean controller reconciliation
CI      = ad hoc Git mutation glue
Wiki    = topology and ownership
Ticket  = change approval context
```

That fragmentation makes ordinary delivery questions hard to answer:

```text
What applications exist?
Where are they deployed?
What version is running where?
What should be promoted next?
What blocks promotion?
What changed in Git?
What reconciler applied it?
What policy allowed it?
What failed?
What can we roll back to?
Who owns this application?
```

Keleustes addresses the gap by modeling delivery context directly, not as an afterthought attached to sync or promotion.

---

## 4. Goals

- Provide a Kubernetes-native GitOps delivery control plane for applications across many clusters, environments, regions, and failure domains.
- Replace Argo CD style application sync and visibility for an intentionally constrained initial deployment model.
- Replace Kargo style promotion orchestration with first-class `Release` and `Promotion` resources.
- Model topology explicitly: environments, cells, regions, deployment targets, target groups, waves, active/active, and active/passive.
- Make promotion state inspectable: requested release, source environment, destination targets, policy results, blockers, approvals, Git mutation, sync, verification, and audit trail.
- Make reconciliation explainable: desired Git commit, rendered manifest, live state, diff, apply result, health, inventory, prune decisions, and events.
- Provide a first-class UI that developers and operators can use without falling back to raw Kubernetes objects.
- Provide a powerful CLI for automation, incident response, and UI-independent operation.
- Use CRDs for active control-plane state and Postgres for queryable history, audit, UI cache, and promotion timeline.
- Support read-only degradation: if mutation or sync is disabled, Keleustes should still provide useful inventory, health, drift, and audit visibility.

---

## 5. Non-goals

Do not build these initially:

- Container registry
- CI system
- Secrets manager
- Backstage replacement
- Metrics, logging, or tracing platform
- Service mesh
- Cloud provisioning system
- Generic workflow engine
- Change-management system of record
- Generic YAML automation platform

Keleustes integrates with these systems where appropriate. It should not become them.

Keleustes should also avoid Argo CD plugin sprawl in early versions. The first sync engine should support a small set of manifest patterns well instead of accepting arbitrary execution hooks as a product boundary.

---

## 6. Target users

### 6.1 Platform engineer

Needs to define environments, cells, clusters, policies, promotion paths, and GitOps behavior. Needs to diagnose fleet drift, support rollback, and audit deployment state.

### 6.2 Application developer

Needs to see app status, deployed versions, pending releases, promotion blockers, diffs, ownership, and relevant repo/build/change links. Developers should be able to request promotion without needing cluster-admin level access or deep Kubernetes object knowledge.

### 6.3 Operations engineer

Needs to see pending promotions, approve or reject changes, understand blast radius, pause or resume sync, roll back safely, and attach change context.

### 6.4 Security and compliance

Needs to know what changed, who approved it, which artifact was deployed, whether policy passed, what evidence exists, and how to export the audit trail.

---

## 7. Product boundaries

Keleustes replaces:

- Argo CD application sync and visibility for supported deployment patterns.
- Kargo-style promotion orchestration.
- Custom CI scripts for environment promotion.
- Spreadsheet-driven deployment tracking.
- Ad hoc Git mutation bots.

Keleustes does not replace:

- Git
- Kubernetes
- Helm
- Kustomize
- OCI registries
- CI systems
- Observability stacks
- Admission controllers
- Change-management systems

This boundary is important. Keleustes owns delivery control-plane state. It should not own the entire software delivery ecosystem.

---

## 8. Core concepts

### 8.1 `Application`

An `Application` is the central delivery abstraction. It is not equivalent to an Argo CD `Application` or a Kargo `Warehouse`. It ties ownership, deployable sources, manifest configuration, deployment topology, and status together.

```yaml
apiVersion: keleustes.skaphos.dev/v1alpha1
kind: Application
metadata:
  name: marshaller-api
spec:
  owner:
    team: platform-services
    contact: "#platform-services"
  sourceRefs:
    - name: marshaller-api
  deployment:
    strategy: gitops
    manifest:
      type: kustomize
      repo: github.com/example/platform-state
      basePath: apps/marshaller-api
  topology:
    environments:
      - dev
      - test
      - qa
      - prod
```

### 8.2 `Source`

A `Source` describes a stream of deployable inputs. Sources can represent container images, Git repositories, OCI artifacts, Helm repositories, charts, SBOMs, and provenance metadata.

```yaml
apiVersion: keleustes.skaphos.dev/v1alpha1
kind: Source
metadata:
  name: marshaller-api
spec:
  type: containerImage
  image: ghcr.io/example/marshaller-api
  semver: ">=1.8.0 <2.0.0"
  verify:
    cosign: true
```

### 8.3 `Environment`

An `Environment` defines an ordered lifecycle boundary such as dev, test, qa, or prod.

```yaml
apiVersion: keleustes.skaphos.dev/v1alpha1
kind: Environment
metadata:
  name: prod
spec:
  order: 40
  protected: true
  changeControl:
    required: true
```

### 8.4 `Cell`

A `Cell` represents a failure domain or operational grouping inside an environment.

```yaml
apiVersion: keleustes.skaphos.dev/v1alpha1
kind: Cell
metadata:
  name: guest-prod
spec:
  environment: prod
  purpose: guest-facing
  failureBoundary: guest
  regions:
    - westus2
    - westcentralus
```

### 8.5 `DeploymentTarget`

A `DeploymentTarget` is a concrete place where an application can run.

```yaml
apiVersion: keleustes.skaphos.dev/v1alpha1
kind: DeploymentTarget
metadata:
  name: prod-guest-westus2
spec:
  environment: prod
  cell: guest-prod
  region: westus2
  cluster:
    name: aks-prod-guest-westus2-001
    kubeconfigSecretRef:
      name: aks-prod-guest-westus2
```

### 8.6 `Release`

A `Release` is a deployable collection of pinned artifacts for an application.

```yaml
apiVersion: keleustes.skaphos.dev/v1alpha1
kind: Release
metadata:
  name: marshaller-api-1.8.2
spec:
  application: marshaller-api
  artifacts:
    - type: image
      ref: ghcr.io/example/marshaller-api:1.8.2
      digest: sha256:abc...
    - type: chart
      ref: oci://ghcr.io/example/charts/marshaller-api
      version: 0.12.4
  provenance:
    commit: abc123
    buildUrl: https://github.com/example/marshaller-api/actions/runs/123
    sbomRef: ghcr.io/example/marshaller-api/sbom:1.8.2
```

### 8.7 `Promotion`

A `Promotion` is a requested movement of a `Release` into one or more deployment targets.

```yaml
apiVersion: keleustes.skaphos.dev/v1alpha1
kind: Promotion
metadata:
  name: marshaller-api-1-8-2-to-prod-wave4
spec:
  application: marshaller-api
  release: marshaller-api-1.8.2
  from:
    environment: qa
  to:
    environment: prod
    cells:
      - guest-prod
    regions:
      - westus2
  mode: pullRequest
  change:
    provider: servicenow
    id: CRQ123456
  policyRefs:
    - prod-standard
status:
  phase: Blocked
  blockers:
    - change-record-not-approved
```

Promotion modes:

- `directCommit`
- `pullRequest`
- `manualRecordOnly`
- `dryRun`

Promotion phases:

- `Proposed`
- `Evaluating`
- `Blocked`
- `Approved`
- `MutatingGit`
- `WaitingForMerge`
- `WaitingForSync`
- `Verifying`
- `Succeeded`
- `Failed`
- `RolledBack`
- `Canceled`

### 8.8 `PromotionPolicy`

A `PromotionPolicy` declares required gates for a promotion.

```yaml
apiVersion: keleustes.skaphos.dev/v1alpha1
kind: PromotionPolicy
metadata:
  name: prod-standard
spec:
  required:
    - imageSigned
    - sbomPresent
    - vulnThreshold
    - sourceHealthy
    - targetUnlocked
    - changeApproved
    - ownerApproved
```

---

## 9. Architecture

```text
                         +-------------------------------+
                         |           Keleustes           |
                         | GitOps Delivery Control Plane |
                         +---------------+---------------+
                                        |
      +---------------------------------+---------------------------------+
      |                                 |                                 |
      v                                 v                                 v
+-------------+                 +-------------+                  +-------------+
| Keleustes API   |                 | Keleustes UI    |                  | keleustesctl CLI|
+------+------+                 +------+------+                  +------+------+
       |                               |                                |
       +-------------------------------+--------------------------------+
                                       |
                                       v
                         +-------------------------------+
                         |     Keleustes Controller      |
                         +---------------+---------------+
                                        |
      +---------------------------------+---------------------------------+
      |                                 |                                 |
      v                                 v                                 v
+-------------+                 +-------------+                  +-------------+
| Source      |                 | Sync        |                  | Promotion   |
| Engine      |                 | Engine      |                  | Engine      |
+------+------+                 +------+------+                  +------+------+
       |                               |                                |
       v                               v                                v
+-------------+                 +-------------+                  +-------------+
| Git/OCI/Helm|                 | Kubernetes |                  | Git mutation|
| sources     |                 | clusters   |                  | PR/commit   |
+-------------+                 +-------------+                  +-------------+
```

### 9.1 Components

**Keleustes API**: REST API for UI, CLI, external integrations, and automation. REST should be the public API first. gRPC can be used internally later if it earns its complexity.

**Keleustes UI**: React/TypeScript frontend for application matrix, detail views, promotion timeline, release inventory, environment topology, health, diff, policy blockers, audit, and administration.

**Keleustes Controller**: Go controller-runtime based control plane that reconciles Keleustes CRDs and coordinates source, sync, promotion, policy, and health workflows.

**Keleustes Worker**: Optional asynchronous worker deployment for expensive rendering, diff, Git operations, evidence collection, and future integrations.

**Source Engine**: Fetches and verifies sources, resolves versions, tracks digests, detects new deployable releases, and emits source revision events.

**Sync Engine**: Reconciles supported manifest types to target clusters using server-side apply, inventory tracking, pruning, health checks, finalizers, conditions, events, work queues, and rate limiting.

**Promotion Engine**: Evaluates promotion requests, policy gates, topology, waves, change context, Git mutation state, sync status, and verification.

**Git Mutation Engine**: Applies controlled Git mutations through provider integrations. GitHub should be first, then GitLab, Azure DevOps, and Gitea.

**Policy Engine**: Executes native gates and records policy evidence. Integrations with OPA, Kyverno, Conftest, SLSA, GUAC, OSV, Grype, Trivy, ServiceNow, Jira, and Azure DevOps work items can follow.

**Health Engine**: Computes resource, application, promotion, policy, and target health.

**Diff Engine**: Computes desired-vs-live, release-vs-release, environment-vs-environment, PR mutation, rendered manifest, and policy diffs.

---

## 10. Deployment model

### 10.1 Management cluster mode

One central Keleustes instance manages many clusters.

```text
skaphos-keleustes-system
|- keleustes-api
|- keleustes-controller
|- keleustes-ui
|- keleustes-worker
|- postgres or external DB
`- redis/nats optional later
```

This mode is best for enterprise and platform teams.

### 10.2 Cluster-local mode

Keleustes runs in a single cluster and reconciles local state only.

This mode is best for smaller teams, OSS adoption, and demos. It should not require a management cluster.

### 10.3 Future federation

Long term, Keleustes can support hub/agent federation:

```text
Keleustes Hub
  |- Keleustes Agent: dev cluster
  |- Keleustes Agent: qa cluster
  |- Keleustes Agent: prod-westus2
  `- Keleustes Agent: prod-westcentralus
```

---

## 11. Sync engine approach

Replacing Argo CD means owning sync quality. This is the hardest part of the product and should be staged carefully.

Keleustes should not build a full general-purpose sync engine in v1. It should start with a constrained reconciler:

- Kustomize path deployment
- Helm chart deployment
- Raw manifest path deployment

Initial implementation should use:

- Kubernetes server-side apply
- Explicit field manager names
- Inventory labels and inventory records
- Prune by inventory ownership
- Health checks for common Kubernetes resources
- Finalizers for cleanup
- Structured conditions
- Events for visible reconciliation history
- Rate-limited work queues

Early non-goals:

- Arbitrary config-management plugins
- Arbitrary hooks
- General scripting execution
- Direct cluster mutation for application deploys outside break-glass workflows

Rules:

- Application deploys mutate Git, not cluster state, unless explicitly running a break-glass operation.
- Every sync decision must be explainable from Git commit, render output, apply result, inventory, and health state.
- Every mutation should be tied to a promotion, PR or commit, approver, policy result, and audit record.

---

## 12. Health model

Resource health:

- `Healthy`
- `Progressing`
- `Degraded`
- `Suspended`
- `Missing`
- `Unknown`

Health sources:

- Kubernetes built-in conditions
- Deployment, StatefulSet, and DaemonSet readiness
- Job completion
- Service endpoint readiness
- Ingress and Gateway status
- Optional HTTP synthetic checks
- Custom CEL or Lua health checks later

Application health aggregates:

- Resource health
- Sync health
- Policy health
- Promotion health
- Target health

The UI must make health legible without requiring users to inspect raw Kubernetes objects first.

---

## 13. Diff model

Diffs are a core product requirement.

Required diff types:

- Desired Git vs live cluster
- Current release vs target release
- Environment vs environment
- PR mutation diff
- Rendered manifest diff
- Policy diff

Diff support:

- Ignore rules
- Managed fields suppression
- Status field suppression
- Secret redaction
- CRD-specific normalizers
- Field ownership context where useful

Argo CD users will notice immediately if diff quality is poor. Diff correctness should be treated as core infrastructure, not UI polish.

---

## 14. Git mutation model

Supported mutations:

- Kustomize image update
- Helm values image tag or digest update
- HelmRelease chart version update
- Plain YAML path update
- JSON6902 patch update
- OCI chart version bump

Provider order:

1. GitHub
2. GitLab
3. Azure DevOps
4. Gitea

Rules:

- Never mutate cluster state directly for ordinary application deploys.
- Prefer PRs for protected environments.
- Direct commits are acceptable only where policy allows them.
- Every mutation produces a `Promotion` event.
- Every mutation is linked to a PR or commit, actor, approver, policy result, and change context when present.
- Mutation code must parse structured data. Avoid ad hoc string replacement where YAML, JSON, Helm values, or Kustomize APIs are available.

---

## 15. Policy model

Native checks:

- Image digest pinned
- Cosign signature valid
- SBOM exists
- Vulnerability threshold satisfied
- Source environment healthy
- Target environment not frozen
- Change record approved
- Required approvers present
- No active incident
- Manual hold not set

Later integrations:

- OPA
- Kyverno
- Conftest
- SLSA
- GUAC
- OSV
- Grype
- Trivy
- ServiceNow
- Jira
- Azure DevOps work items

Policy output must be evidence-backed. A blocked promotion should show which gate failed, what evidence was used, when it was evaluated, and what action can unblock it.

---

## 16. UI requirements

The UI is a first-class product surface. Replacing Argo CD with CLI-only purity will fail.

Required screens:

- Application matrix
- Application detail
- Promotion timeline
- Release inventory
- Environment topology
- Cluster and target health
- Resource tree
- Diff view
- Policy and blocker view
- Audit trail
- Admin and settings

Core matrix:

```text
Application        Dev     Test    QA      Prod WUS2   Prod WCU    State
marshaller-api     1.8.2   1.8.2   1.8.1   1.8.0      1.8.0       QA ahead
jetway             2.4.0   2.4.0   2.4.0   2.3.7      2.3.7       Ready
keycloak           25.0.4  25.0.4  25.0.3  Active     Standby     Controlled
```

Application detail should show:

- Current release by environment, cell, and region
- Live health
- Desired Git commit
- Last reconciled commit
- Promotion history
- Pending promotion
- Blocked checks
- Rendered resources
- Diff
- Owner
- Links to repo, build, change record, logs, and runbooks

Operator view should show:

- Promotions awaiting approval
- Production waves
- Frozen targets
- Failed reconciliations
- Drift
- Rollback candidates
- Blast radius

---

## 17. CLI requirements

The standalone CLI is `keleustesctl`.

Examples:

```bash
keleustesctl app list

keleustesctl app get marshaller-api

keleustesctl matrix --env prod

keleustesctl release list marshaller-api

keleustesctl promote marshaller-api \
  --release 1.8.2 \
  --to qa

keleustesctl promote marshaller-api \
  --release 1.8.2 \
  --to prod \
  --cell guest-prod \
  --region westus2 \
  --change CRQ123456 \
  --pr

keleustesctl diff marshaller-api \
  --from qa \
  --to prod

keleustesctl blockers marshaller-api --to prod

keleustesctl rollback marshaller-api \
  --to-release 1.8.0 \
  --env prod \
  --cell guest-prod
```

The CLI should support machine-readable output for automation and should remain capable enough that the UI is not a single point of operational failure.

---

## 18. API requirements

External API should be REST first.

Core endpoints:

```text
GET    /applications
GET    /applications/{name}
GET    /applications/{name}/matrix
GET    /applications/{name}/releases
GET    /applications/{name}/promotions

POST   /promotions
GET    /promotions/{id}
POST   /promotions/{id}/approve
POST   /promotions/{id}/cancel
POST   /promotions/{id}/retry

GET    /targets
GET    /targets/{name}/health
GET    /targets/{name}/drift

GET    /diff
GET    /audit
```

The API should expose product concepts, not raw Kubernetes internals as the only usable interface.

---

## 19. Data model

Use Kubernetes CRDs for active control-plane state and Postgres for queryable history.

```text
Kubernetes CRDs = active desired/control state
Postgres        = queryable history, audit, UI cache, promotion timeline
Object storage  = optional rendered manifests, large diffs, evidence bundles
```

Postgres is justified because audit trails, matrix queries, timeline views, historical release state, and compliance exports are awkward if every query must be reconstructed from live Kubernetes objects.

Initial API group:

```text
keleustes.skaphos.dev
```

Initial namespace:

```text
skaphos-keleustes-system
```

Core CRDs:

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

Avoid names that carry existing product baggage:

- `AppProject`
- `ApplicationSet`
- `Warehouse`
- `Freight`
- `Stage`

---

## 20. MVP roadmap

### MVP 0: Read-only replacement UI

Goal: replace the "I need to look in Argo CD" habit.

Features:

- Application registry
- Environment, cell, and target registry
- Cluster connection
- Read Kubernetes live state
- Read Git desired state
- Show application matrix
- Show application health
- Show deployed versions
- Show resource tree

No Git mutation, automated sync, or promotion engine yet.

### MVP 1: Keleustes-managed sync

Goal: replace Argo CD sync for a constrained set of applications.

Features:

- Git source watcher
- Kustomize path sync
- Helm chart sync
- Raw manifest sync
- Server-side apply
- Prune via inventory labels
- Health calculation
- Drift detection
- Manual sync
- Auto-sync
- Suspend and resume
- Basic diff

### MVP 2: Releases and promotions

Goal: replace the core Kargo promotion model.

Features:

- Release detection from image and chart sources
- `Promotion` object
- Promotion pipeline
- Environment progression
- Git mutation via PR
- Promotion timeline
- Approval gates
- Basic policy checks

### MVP 3: Enterprise topology

Goal: make Keleustes materially better than Argo CD and Kargo for large platform teams.

Features:

- Cells
- Regions
- Waves
- Active/passive target groups
- Production freeze windows
- Change ticket integration
- Rollback candidate selection
- Blast-radius view
- Multi-cluster dashboard

### MVP 4: Policy and audit

Goal: make Keleustes acceptable in regulated enterprise environments.

Features:

- Signed artifact policy
- SBOM policy
- Vulnerability gates
- Approval evidence
- Audit export
- Policy result history
- Change-management integration

---

## 21. Build strategy

Backend:

- Go
- `controller-runtime`
- `client-go` dynamic client
- Kubernetes server-side apply
- Cobra for CLI
- REST API with boring, explicit resources
- OpenTelemetry
- `slog` or zap logging

Frontend:

- React
- TypeScript
- Matrix, timeline, resource tree, and diff views as first-class UI components

Storage:

- Postgres for durable product state
- Kubernetes API for active reconciliation state
- Redis optional later
- NATS optional later

Packaging:

- Helm chart
- OCI chart publication
- Kustomize install manifest
- kind demo environment
- Third-party notice artifacts in release archives
- Runtime `notices` or `licenses` CLI command where practical

---

## 22. Design principles

These should appear in the public README if the project proceeds:

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

---

## 23. Public positioning

Do not lead with vendor criticism. It makes the project sound reactive.

Preferred framing:

> Existing GitOps tools split reconciliation, promotion, topology, and visibility across separate products. Keleustes provides a unified open-source control plane for GitOps delivery without requiring organizations to adopt a commercial SaaS control layer.

Sharper framing:

> Keleustes is for platform teams that want GitOps delivery without handing the control plane to a vendor.

First README pitch:

```markdown
# Skaphos Keleustes

Skaphos Keleustes is an open-source GitOps delivery control plane for Kubernetes platforms.

Keleustes combines application inventory, topology-aware deployment, GitOps reconciliation,
promotion workflows, policy gates, and audit history into a single Kubernetes-native system.

Keleustes is designed for platform teams running many applications across many clusters,
regions, environments, and failure domains.

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
```

The core public message is not "cheaper Argo CD." It is:

> A vendor-neutral GitOps delivery control plane for organizations whose deployment reality is bigger than app -> env -> sync.

---

## 24. Relationship to other Skaphos tools

Keleustes is application delivery control-plane software. It is distinct from Meridian, which manages cluster fleet lifecycle.

Potential relationships:

- Meridian can install Keleustes as part of a management cluster blueprint.
- Keleustes can use Fathom health state as a target or promotion gate, but Fathom has no dependency on Keleustes.
- Keleustes can use Tack for progressive traffic shifting during application or target-group rollout workflows, but Tack remains a standalone traffic primitive.
- Keleustes can rely on Anchor to enforce immutable artifact references at admission time, but Anchor has no dependency on Keleustes.
- Skaphos CLI dispatches to `keleustesctl` as one registered tool.

Keleustes should not absorb these tools. It should compose with them.

---

## 25. External references

- Argo CD documentation: https://argo-cd.readthedocs.io/en/stable/
- Kargo core concepts: https://docs.kargo.io/user-guide/core-concepts/
- Flux GitOps Toolkit components: https://fluxcd.io/flux/components/
