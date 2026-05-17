<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Engine Boundaries and Technology Integration Plan

**Status:** Draft  
**Date:** 2026-05  
**Related:** PROPOSAL §9 (Architecture & Engines), §11 (Sync engine approach), distributed-runtime-architecture.md plan  
**Owner:** Platform Architecture  
**Purpose:** Define internal Go package structure, engine ownership boundaries, and the first-cut technology integration map so the codebase scales cleanly from scaffold → MVP 1 central deployment → distributed hub/agent model.

---

## 1. Why This Matters Now

The current `internal/controller/` contains only thin stubs. As soon as we write real logic for rendering, reconciliation, promotion state machines, Git mutation, policy evaluation, etc., we risk creating a large, tangled package that is hard to test, hard to distribute to agents, and violates the "small functions, clear names, low cognitive load" guardrail in CLAUDE.md.

We have named engines in the proposal (Source, Sync, Promotion, Git Mutation, Policy, Health, Diff, plus Worker). We need explicit **ownership boundaries** and **package structure** before any of them grow beyond a few hundred lines.

Additionally, every external technology (Git provider SDK, Helm, Kustomize, Helmfile, cosign, Trivy, NATS, DuckDB, etc.) must be assigned to an engine (or a shared supporting package) with a clear decision on:

- Hub-only vs. agent-deployable
- Library vs. exec (security, size, reproducibility)
- When the integration is introduced (MVP timeline)

This plan starts that work.

## 2. Named Engines (from PROPOSAL §9)

| Engine              | Responsibility Summary                                      | Primary Owner of Which Types / Outputs          | Distribution Notes |
|---------------------|-------------------------------------------------------------|--------------------------------------------------|--------------------|
| **Source Engine**   | Watch Git, OCI, Helm repos, container images; resolve versions, verify digests, provenance, SBOMs; emit revision events | `Source` CR, new revision events | Can be sharded by source type; some verification work benefits from being near registries. Webhook reception (public-facing) is a separate concern in `internal/webhooks/<provider>/` — see §3 and runtime plan §7.5. Steady-state revision detection is webhook-driven; scheduled polls are drift-detection only. |
| **Render** (new, cross-cutting) | Produce rendered manifests + inventory from Application + Release + manifest config | Rendered manifests, inventory, warnings | The hottest and most security-sensitive path. Must be cleanly callable from hub or agent |
| **Sync Engine**     | Reconcile rendered manifests to targets via SSA, prune via inventory, produce `SyncRun` lifecycle, write `Deployment` records, basic health; honor cross-Application dependency declarations (§2.6) | `SyncPlan`, `SyncRun`, `Deployment` | The most expensive engine. Primary candidate for regional agent execution. Cross-Application dependency ordering is part of the Sync Engine's scope, not a separate engine. |
| **Promotion Engine**| Own the full promotion state machine, orchestrate policy gates, drive Git mutation requests, wave execution | `Promotion`, `Approval`, `PromotionPolicy` | Mostly central (orchestration + audit), but can delegate some execution steps |
| **Git Mutation Engine** | Perform structured updates (kustomize image, helm values, plain YAML, JSON6902) and create PRs or direct commits | Git provider interactions | Hub-preferred for audit; regional agents may need limited mutation capability in DR scenarios |
| **Policy Engine**   | Execute native gates + external policy tools; produce evidence | Policy results attached to Promotions and SyncRuns | Some policies (vuln scanning) are CPU-heavy and good candidates for worker/agent offload |
| **Health Engine**   | Aggregate resource health, sync health, promotion health, target health | `HealthCheck`, conditions on `Deployment` | Agents are the natural producers for per-target health |
| **Diff Engine**     | Desired vs live, release vs release, rendered manifest diff, policy diff, ignore rules, secret redaction | Diff artifacts (stored or transient) | Heavy; natural fit for worker pool or delegated to agents |
| **Worker / Job Pool** | Async execution of expensive, non-latency-sensitive tasks (deep render, large diffs, evidence bundles) | Background jobs | Can be central workers or regional agent background tasks |

**Key insight:** `Render` is not called out as a top-level engine in the original proposal but is the critical shared abstraction that almost every other engine depends on. Treating it as a first-class, well-isolated package from day one is essential.

**Second key insight:** Several of the engines above — specifically Sync, Diff, and Health, plus the per-target cluster cache that the agent needs — have proven open-source implementations in [`gitops-engine`](https://github.com/argoproj/gitops-engine) (the shared library extracted by Argo CD and used in parts by Flux). We should build on top of it rather than reimplement, both to inherit years of battle-testing and to focus our engineering on what is genuinely novel about Keleustes (Promotion, Git Mutation, multi-target topology, distributed runtime). See §2.5.

## 2.5 Reuse: `gitops-engine`

[`github.com/argoproj/gitops-engine`](https://github.com/argoproj/gitops-engine) (Apache-2.0) is the shared sync/diff/health/cache library extracted from Argo CD; pieces of it are also used by Flux. Adopting it shifts the scope of several engines significantly and frees engineering bandwidth for the parts of Keleustes that no upstream library covers.

### What we adopt

| `gitops-engine` package | Used by Keleustes engine | What it gives us |
|-------------------------|--------------------------|-------------------|
| `pkg/cache` (cluster cache) | Sync Engine, Health Engine, Diff Engine (agent-side) | In-memory, watch-backed view of every resource in a target cluster, with owner-reference traversal. Avoids us building a cluster cache + index from scratch. |
| `pkg/diff` (+ `pkg/diff/normalizers`) | Diff Engine | Three-way diff between desired / live / last-applied, with field normalizers (status, generation, defaulted fields, known types). |
| `pkg/sync` | Sync Engine | Server-side apply, sync waves (annotation-driven ordering), hooks (Pre/Sync/Post/Fail), prune by inventory, structured sync results. |
| `pkg/health` | Health Engine | Built-in health assessment for Deployment, StatefulSet, DaemonSet, Job, Pod, PVC, Service, Ingress, HPA, ReplicaSet, ReplicationController; Lua-based custom health checks. |
| `pkg/utils/kube` | shared (`kube/`) | Resource key generation, GVK helpers, SSA field-manager wiring. |

### What it does NOT cover (still on us)

- **Rendering** — Kustomize/Helm/raw execution. `gitops-engine` consumes already-rendered manifests; the Render Engine remains entirely ours.
- **Source watching / resolution** — Git, OCI, Helm repo polling and digest/signature verification. Flux's `source-controller` is the reference here; we will likely vendor or re-implement narrowly.
- **Promotion state machine** — Releases, Approvals, `PromotionPolicy` gates, waves across `DeploymentTarget`s — entirely Keleustes territory.
- **Git Mutation Engine** — Structured updates to manifest repos (kustomize image, helm values, JSON6902, plain YAML) and PR/commit creation — entirely ours.
- **Multi-cluster orchestration** — `SyncPlan` → `SyncRun` per-target fan-out, claim semantics for agents, regional execution affinity — ours. `gitops-engine` is fundamentally single-app, single-cluster.
- **Topology** — `Application`, `Environment`, `Cell`, `DeploymentTarget`, `Promotion` — these are Keleustes CRDs with no analogue upstream.

### Impact on engine scope

This narrows several engines materially:

- **Sync Engine** becomes mostly an *orchestrator*: `SyncPlan` → `SyncRun` lifecycle, per-target `gitops-engine` invocation, result surfacing into our CRDs, agent-side execution wrapper. The manifest-level sync mechanics (SSA, waves, hooks, prune) are `gitops-engine`'s job.
- **Diff Engine** becomes a thin wrapper around `pkg/diff` plus our own normalizers for what `gitops-engine` doesn't model (secret redaction, policy diffs, release-vs-release).
- **Health Engine** becomes a wrapper around `pkg/health` plus aggregation across `Deployment` records and Lua extensions for Skaphos-specific kinds.
- **Render Engine** is unaffected — `gitops-engine` is downstream of rendering.

### Risks and watch-points

1. **Dependency footprint.** `gitops-engine` drags in significant `k8s.io/*` modules. Agent binary size and CVE surface must be measured; a slimmer build-tag set for the agent may be needed.
2. **API churn.** `gitops-engine`'s public API has changed at major versions. Pin tightly and treat upgrades as deliberate, tested events.
3. **Argo-isms.** Some sync semantics (e.g., hook annotations, `argocd.argoproj.io/sync-wave`) come straight from Argo CD. We must decide whether to expose those annotations verbatim on `Application` specs (lowest migration friction) or translate them to Keleustes-prefixed equivalents (cleaner ownership). See §7.
4. **Single-cluster assumption.** `pkg/cache` is built around one cluster per cache instance. Per-`DeploymentTarget` caches are the natural fit; cache lifecycle must be bound to `DeploymentTarget` lifecycle and not leaked.
5. **License & lineage.** Apache-2.0 is compatible with our MIT licensing. Attribution must be preserved (NOTICE file, headers where required).

This decision is one of the more consequential ones in the plan and warrants an early ADR.

## 2.6 The Dependency Model (Cross-Application Ordering)

**Concrete pain to fix:** in Argo CD today, when "addon Applications" require CRDs to be installed and ready before consumer Applications can sync, auto-sync fails repeatedly and a human has to manually sync things in the right order to break the deadlock. This is wrong; the system should know the order and wait, not fail.

### Design

Dependencies are declared on the `Application` CR (not a separate `Dependency` CRD — that's an unnecessary level of indirection):

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: Application
spec:
  dependencies:
    applications:
      - applicationRef:
          name: cert-manager
          namespace: addons
        scope: same-target           # or: any-target, namedTarget: <DeploymentTarget name>
        waitFor: Healthy             # or: Ready, Synced, custom condition name
        timeout: 10m
        onTimeout: block             # or: warn, proceed
    crds:
      - name: certificates.cert-manager.io
        waitForCondition: Established
      - name: clusterissuers.cert-manager.io
```

### Behavior

- **`SyncPlan` evaluates dependencies before generating `SyncRun`s.** If any dependency is unmet, the SyncPlan enters `WaitingForDependencies` with a `DependenciesUnmet` condition listing exactly what is unmet and why.
- **Auto-sync waits, does not fail.** When dependencies become satisfied (e.g., the dependency's `Deployment.status.healthy == true` event lands on JetStream), the SyncPlan automatically proceeds. No manual `keleustesctl sync` needed to break the deadlock.
- **CRD dependencies** are first-class because they are the single most common cause of the addon-bootstrap pain. The Sync Engine checks the target cluster's `CustomResourceDefinitions` for `Established` before allowing dependent Applications to sync.
- **Cross-target scope.** `scope: same-target` (default) requires the dependency to be healthy on the same `DeploymentTarget` the dependent is being synced to. `any-target` allows any healthy instance (useful for "needs a control-plane component running somewhere"). `namedTarget` allows explicit cross-cluster dependencies (rarely needed; flagged in audit).
- **Bootstrap waves.** From a cold start (e.g., new cluster bootstrap), the engine computes a topological order across all Applications targeting that cluster and rolls them up in dependency order without manual intervention.
- **Cycle detection at admit time.** A validating webhook on `Application` rejects creates/updates that would introduce a cycle in the dependency graph (per-target evaluation).
- **Promotion-aware.** When the Promotion Engine fans out a Release to multiple Applications, the Promotion respects the dependency order: producers go before consumers, with health gates between waves.

### Out of scope (deliberately)

- ❌ **Arbitrary DAG executors.** This is dependency ordering, not workflow orchestration. Argo Workflows / Tekton are not in our scope.
- ❌ **Reverse dependencies / unsync ordering.** Pruning of a producer before consumers are unsynced is handled by ownership/finalizers, not by an inverted dependency graph.
- ❌ **Cross-tenant dependencies.** Dependencies must live within a single `Project` scope (RBAC plan §5). Cross-project deps require an explicit grant and are audit-flagged.

### Engine ownership

- The Sync Engine owns dependency evaluation and SyncPlan blocking.
- The Promotion Engine consumes dependency information to order Promotion waves.
- The Health Engine produces the `Healthy` / `Ready` signals that dependencies wait on.
- The webhook receivers do not need to know about dependencies.

This stays inside the existing engine boundaries (§2) — no new top-level engine, no new top-level package.

## 3. Proposed Internal Package Structure

Target layout (after MVP 1, before heavy distribution):

```
internal/
├── api/                     # Future: REST/gRPC server, handlers, authz middleware (or client library)
├── cli/                     # Existing keleustesctl command tree (keep thin — delegates to engines)
├── controller/              # ONLY thin coordinator reconcilers
│   ├── application_controller.go
│   ├── syncplan_controller.go
│   ├── promotion_controller.go
│   └── ...                  # Each should be < ~150 LOC, no business logic
├── sync/                    # Sync Engine (core reconciliation logic)
│   ├── engine.go            # SyncEngine interface + main orchestration
│   ├── plan.go
│   ├── run.go
│   ├── applier.go           # SSA + inventory + prune
│   ├── health.go            # basic resource health during sync
│   └── ...
├── promotion/               # Promotion Engine + state machine
│   ├── engine.go
│   ├── state_machine.go     # All 12 phases, transitions, blockers
│   ├── policy_evaluator.go
│   ├── mutation_request.go
│   └── ...
├── source/                  # Source Engine
│   ├── engine.go
│   ├── resolver.go
│   ├── verifier.go          # digest, cosign, provenance
│   └── ...
├── mutation/                # Git Mutation Engine (name chosen for brevity)
│   ├── engine.go
│   ├── github.go
│   ├── gitlab.go
│   ├── azuredevops.go
│   ├── structured_update.go # kustomize/helm/yaml/JSON6902 helpers
│   └── ...
├── policy/                  # Policy Engine
│   ├── engine.go
│   ├── native_gates.go
│   ├── external/            # adapters for Trivy, Grype, OPA, etc.
│   └── evidence.go
├── health/                  # Health Engine (aggregation + custom checks)
│   ├── engine.go
│   ├── resource_health.go
│   └── ...
├── diff/                    # Diff Engine
│   ├── engine.go
│   ├── manifest_diff.go
│   ├── release_diff.go
│   ├── normalizers/
│   └── ...
├── render/                  # Critical shared package — DO NOT put inside sync/
│   ├── renderer.go          # RenderRequest → RenderResult + Inventory
│   ├── kustomize/
│   ├── helm/
│   ├── raw/
│   └── inventory.go         # Kubernetes object inventory extraction
├── inventory/               # Common inventory types and label/owner conventions (used by Sync + others)
├── events/                  # Event bus abstractions + NATS implementation
│   ├── bus.go
│   ├── nats/
│   └── schemas/
├── agent/                   # Agent protocol, registration, work claiming, local autonomy
│   ├── protocol.go
│   ├── registration.go
│   └── autonomy.go
├── store/                   # Durable storage abstractions (see §5.5 — no SQL on the critical path)
│   ├── jetstream/           # durable event/audit streams; NATS KV for hot indexes
│   ├── object/              # content-addressed object storage (rendered manifests, evidence bundles, archives)
│   └── cache/               # in-process / NATS KV hot caches (rebuildable from canonical sources)
├── webhooks/                # Public-facing webhook receivers (provider-specific HMAC validation)
│   ├── github/
│   ├── gitlab/
│   ├── azuredevops/
│   └── oci/
├── git/                     # Low-level Git provider clients (thin wrappers around SDKs)
│   ├── github/
│   ├── gitlab/
│   └── ...
├── kube/                    # Kubernetes client helpers, SSA field managers, dynamic client wrappers
│   └── ...
└── util/                    # Small, genuinely shared utilities (avoid dumping ground)
```

**Rules enforced by this structure:**

- `controller/` only coordinates and updates CRD status. All real work is delegated to an engine package.
- `render/` is the only package allowed to know about kustomize/helm rendering details.
- No engine package imports another engine's internal types directly (use well-defined interfaces or event schemas).
- `mutation/` and `git/` are separated so regional agents can take a minimal dependency if needed.
- `events/` and `store/` are the main integration seams for distribution.
- `gitops-engine` is imported **only** by `sync/`, `diff/`, `health/`, and `kube/`. Other packages access its capabilities through Keleustes-defined interfaces exported from those packages. This keeps the blast radius of a future fork, version pin, or replacement contained.

**`gitops-engine` integration shape.** `internal/sync/`, `internal/diff/`, `internal/health/`, and the per-`DeploymentTarget` cluster cache are built on top of `gitops-engine` packages (see §2.5). The Keleustes packages remain in our module so we can extend, add custom normalizers, swap implementations, or fork without disrupting consumers — the boundary between "ours" and "theirs" lives *inside* each package, not at the package boundary.

## 4. Engine Ownership & Dependency Rules

**Clear "owns" statements** (examples):

- **Sync Engine owns**: Creating and driving `SyncRun` objects through their phases, writing `Deployment` records, deciding prune sets, producing per-target inventory.
- **Promotion Engine owns**: The `Promotion` phase machine, blocker calculation, approval orchestration, requesting Git mutations.
- **Render owns**: Turning an Application's manifest spec + a specific Release into a set of rendered objects + inventory. It does **not** own applying them or deciding when to render.
- **Git Mutation Engine owns**: Structured mutation logic + provider-specific PR/commit creation. It does **not** decide *when* a mutation should happen (Promotion Engine) or what the desired image tag is (Source/Release).
- **Policy Engine owns**: Evaluation + evidence production. It is called by Promotion Engine and (in future) Sync Engine.

**Dependency direction** (acyclic):

- Engines may depend on `render/`, `inventory/`, `events/`, `store/`, `kube/`, `git/`.
- Engines may **not** depend on each other except through narrow interfaces or the event bus.
- `controller/` depends on engines (not the other way around).
- `cli/` and future `api/` talk to engines via clean facades or the event/query layer.

This structure makes it feasible to later extract a subset of packages into an agent binary with a smaller dependency set.

## 5. Integrated Technologies — First-Cut Map

This section **starts** the technology integration planning. It is deliberately incomplete; new entries should be added via PR with justification against the engine boundaries above.

### 5.1 Rendering & Manifest Technologies

| Technology       | Version / Library                  | Primary Owner     | Hub / Agent | Library vs Exec | MVP Intro | Risk / Notes |
|------------------|------------------------------------|-------------------|-------------|-----------------|-----------|--------------|
| Kustomize        | `sigs.k8s.io/kustomize/api` + `kyaml` | `render/kustomize` | Both       | Library        | MVP 1    | Preferred. Avoid exec for reproducibility and security. |
| Helm             | `helm.sh/helm/v3` (pkg/action, pkg/chart) | `render/helm`     | Both       | Library        | MVP 1    | Use the Go libraries, not the CLI binary. Chart repo handling needs care. |
| Helmfile         | Helmfile binary (exec) **or** custom Go composition over Helm SDK | `render/helmfile` | Both | Likely exec initially | MVP 2    | Helmfile composes multiple Helm releases. No Go library exists; either exec the `helmfile` binary in a sandbox, or define a Keleustes-specific composition format that renders via our Helm library. Recommend exec in MVP 2; revisit. |
| Raw manifests    | `k8s.io/apimachinery` + yaml       | `render/raw`      | Both       | Library        | MVP 1    | Simple directory walk + decoding. |
| Inventory extraction | `k8s.io/apimachinery` + unstructured | `render` + `inventory` | Both | Library     | MVP 1    | Must be deterministic for pruning. |

**Decision record needed soon:** Do we shell out to `helm template` / `kustomize build` in any fallback path, or is pure-library only acceptable?

**Note:** `gitops-engine` is *downstream* of rendering — it consumes rendered manifests but does not render. The Render Engine remains entirely ours.

### 5.2 Git & Mutation Providers

| Provider         | Library                                      | Primary Owner     | Hub / Agent     | Notes |
|------------------|----------------------------------------------|-------------------|-----------------|-------|
| GitHub           | `github.com/google/go-github/v60`            | `mutation/github` + `git/github` | Hub primary; Agent limited | Use App installation tokens or PATs scoped per Application. |
| GitLab           | `gitlab.com/gitlab-org/api/client-go`        | `mutation/gitlab` | Hub primary     | — |
| Azure DevOps     | `github.com/microsoft/azure-devops-go` or REST | `mutation/azure` | Hub primary     | Work item integration also lives here. |
| Gitea            | `code.gitea.io/sdk/gitea`                    | `mutation/gitea`  | Hub primary     | Lower priority. |

**Structured update helpers** live in `mutation/` (not per-provider) so the same "update image tag in kustomization.yaml" logic is reused.

### 5.3 Policy, Security & Provenance

| Technology       | Purpose                              | Primary Owner          | Hub / Agent | Library | MVP | Notes |
|------------------|--------------------------------------|------------------------|-------------|---------|-----|-------|
| cosign           | Signature verification               | `source/` or `policy/` | Both       | Library (`sigstore/cosign`) | MVP 2 | Verification can be expensive; cache results. |
| Trivy            | Vulnerability scanning               | `policy/external/trivy` | Agent-friendly | CLI or library | MVP 3/4 | CLI may be simpler for SBOM + vuln; increases agent image size. |
| Grype            | Alternative vuln scanner             | `policy/external/grype` | Agent-friendly | — | MVP 4 | — |
| OPA / Gatekeeper | Policy as code                       | `policy/external/opa`  | Both       | Library | MVP 3 | Rego evaluation cost must be bounded. |
| Kyverno          | Policy checks                        | `policy/external/kyverno` | Hub preferred | — | Later | Heavy dependency. |
| SLSA / GUAC      | Provenance & supply chain            | `source/` + `policy/`  | Hub         | — | MVP 4 | Early integration via attestation verification. |
| SBOM (SPDX / CycloneDX) | SBOM presence + basic checks | `source/` or `policy/` | Both | Various parsers | MVP 2 | — |

**Important boundary:** Verification that happens at *source resolution time* (e.g., "is this image signed?") belongs in Source Engine. Policy evaluation against a Promotion or Deployment belongs in Policy Engine. The evidence format must be common.

### 5.4 Event Bus & Async

| Technology | Purpose | Owner | Hub / Agent | MVP | Notes |
|------------|---------|-------|-------------|-----|-------|
| NATS (JetStream) | Reliable eventing, work queues, regional fan-out | `events/nats` | Both (agents connect outbound) | MVP 1 (for workers), MVP 2 (for agents) | Strong multi-region story, durable streams, replay. Recommended first choice. |
| Redis Streams / PubSub | Alternative or supplementary | `events/` or `store/cache` | — | Later | Only if NATS proves insufficient. |

### 5.5 Storage & Cache

**Hard constraint: no relational database on the critical path.** Keleustes must be **restorable from zero without backups**. Any storage layer whose loss requires a `pg_dump`-style restore is disqualified. State that cannot be reconstructed from canonical sources (CRDs, Git, JetStream) does not exist.

This constraint flows from operational reality: a system carrying the promotion state machine for 2000+ repos cannot also be the system that requires a separate backup/restore discipline. Recovery is "redeploy the operator and replay," not "find a recent dump."

#### Storage tiers

| Tier | Technology | Purpose | Critical-path? | Recovery |
|------|-----------|---------|----------------|----------|
| **Authoritative state** | **CRDs (etcd)** | Desired state, control state, topology | Yes | Re-applied from the Git config repo (GitOps the operator itself). |
| **Authoritative history** | **Git repos** (user-managed) | Manifest history, release tags, promotion commits | Yes (for what's in Git) | User's Git provider is the system of record; nothing to back up on our side. |
| **Event / audit log** | **NATS JetStream** durable streams | Promotion events, SyncRun phase transitions, source revisions, approval events, agent events | Yes | Replicated R≥3 across NATS cluster members in normal ops. Cold archive to object storage on a rolling window; replay from archive on catastrophic NATS loss. |
| **Hot indexes / presence** | **NATS KV** (built on JetStream) | Agent presence, leader locks, current `Deployment` snapshots per target, recent matrix snapshots | No | Rebuildable from JetStream replay or live agent reports. |
| **Large content-addressed artifacts** | **Object storage** (S3 / GCS / Azure Blob / MinIO) | Rendered manifests (keyed by content hash), evidence bundles, JetStream archive segments, large diffs | Mostly no (cache); yes for archive segments | Replicated via bucket replication (provider-native). Rendered manifests are pure cache — re-rendered on miss. |
| **In-process hot cache** | Go in-memory + optional Redis | Per-pod LRU for hot rendered-manifest digests, render result memoization within a single request | No | Dies with the pod; cold starts are slower. |
| **Derived analytics / UI matrix** | **DuckDB over parquet on object storage** | UI matrix, promotion timeline visualisations, ad-hoc cross-Application queries | No (fully derived) | Periodically rebuilt from JetStream replay. No schema migrations. No backups. Wipe-and-rebuild in minutes. |

**Why this composition works:**

- **CRDs** carry the active state; etcd already replicates inside the management cluster, and the *contents* are reconstructable from the Git config repo (the operator's own manifests, samples, and topology CRs live in Git).
- **JetStream** is durable, append-only, replicated, and replayable — exactly what an audit log needs. Subject sharding handles fan-out at scale.
- **Object storage** is the right home for large immutable artifacts; content addressing means writes are idempotent and dedup is free.
- **DuckDB on parquet** gives us a real SQL query layer for UI/analytics without a SQL **server** to operate, back up, or recover. The parquet files are rebuilt from JetStream replay; if a parquet shard is lost, regenerate it.
- **NATS KV** covers the few "current value" use cases (agent presence, locks) where event-replay is the wrong shape. It is itself backed by JetStream, so it inherits the same durability/replay story.

#### What we explicitly do NOT use

- ❌ **PostgreSQL / MariaDB / MySQL / Oracle / SQL Server as source-of-truth.** Disqualified by the no-backups constraint. May be used as a **derived** read replica by a customer integration outside the core path, but the core never depends on it.
- ❌ **etcd as a generic KV** outside the apiserver — etcd serves CRDs, full stop; no operator-owned keys.
- ❌ **Blob columns in any database** — large content goes to object storage by hash.

#### When SQL would be acceptable

A derived, externally-managed Postgres (or similar) that consumes the JetStream audit stream and exposes a SQL surface for BI / external integrations is acceptable as a **side-car deployment owned by the customer**, not as a Keleustes core dependency. We may publish a reference consumer; we do not operate or require it.

#### Recovery from zero (target time budgets, indicative)

| Loss scenario | Recovery procedure | Target wall-clock |
|---------------|--------------------|-----|
| Operator pod lost, all state intact | Pod restart | seconds |
| CRDs lost, config repo intact | `kubectl apply -k` from config repo | minutes |
| NATS JetStream lost, object storage archive intact | Restore latest archive segments; replay | tens of minutes |
| JetStream lost, no archive | Reconstruct from CRD state + live agent reports; accept historical audit loss prior to recovery point | minutes for live state; historical audit beyond CRD `.status` history is gone (this is an explicit, documented limit, not a surprise) |
| Object storage lost | Cache regenerates on demand; analytics parquet regenerates on next rebuild | hours for full warm-up; nothing is permanently lost |
| Cluster lost | Re-bootstrap cluster, then recover as above | hours |

These targets must be **demonstrated** before MVP 2 closes, not assumed.

### 5.6 Kubernetes Client & Apply

- Use `sigs.k8s.io/controller-runtime` + dynamic client for the hub.
- For agents: either a lightweight controller-runtime client or raw `client-go` to keep the agent binary smaller.
- Explicit field manager names per engine/operation (critical for SSA correctness and drift detection).
- Server-side apply is non-negotiable.

### 5.7 OIDC / Authentication

- Central OIDC handling (Dex or direct provider integration) for the API Server and CLI.
- Workload identity (Kubernetes ServiceAccount tokens, SPIRE, cloud IAM) for agent-to-hub communication.
- No new auth system in Keleustes itself initially.

### 5.8 Sync, Diff, Health, Cluster Cache (via `gitops-engine`)

This subsection captures the `gitops-engine` (§2.5) packages in the same map shape as the rest of §5, so the technology inventory stays in one place.

| Capability | `gitops-engine` package | Keleustes owner | Hub / Agent | MVP | Notes |
|------------|------------------------|-----------------|-------------|-----|-------|
| Cluster cache | `pkg/cache` | `sync/` (agent-side) | Agent (one per `DeploymentTarget`) | MVP 1 | Bound to `DeploymentTarget` lifecycle; not cluster-wide. |
| Three-way diff + normalizers | `pkg/diff` (+ `pkg/diff/normalizers`) | `diff/` | Both | MVP 1 | Extend with our own normalizers for secret redaction and policy diffs. |
| SSA + sync waves + hooks + prune | `pkg/sync` | `sync/` | Both | MVP 1 | Decide which Argo CD annotations (`argocd.argoproj.io/sync-wave`, hooks) we honor verbatim vs. translate. See §7. |
| Resource health assessment | `pkg/health` | `health/` | Both | MVP 1 | Custom Lua checks for Skaphos-specific kinds added per-team. |
| Resource key / GVK helpers | `pkg/utils/kube` | `kube/` | Both | MVP 1 | Shared utility. |

**Containment rule:** `gitops-engine` is imported only inside `internal/sync/`, `internal/diff/`, `internal/health/`, and `internal/kube/`. Other packages call Keleustes-defined interfaces, not `gitops-engine` types. (Repeated from §4 because it is load-bearing.)

### 5.9 Networking / Exposure (Gateway API)

Keleustes commits to [Gateway API v1](https://gateway-api.sigs.k8s.io/) (`gateway.networking.k8s.io/v1`) — **no `Ingress`**. Different parts of the hub use different `Gateway` resources to enforce blast-radius and auth-model separation; the full tiering is in the runtime plan §7.5.

| Capability | Tech | Primary owner | MVP | Notes |
|------------|------|---------------|-----|-------|
| API version | Gateway API v1 | `config/` overlays | MVP 1 | `GatewayClass` choice is customer-provided. |
| Public webhook reception | `HTTPRoute` on a public `Gateway` → `internal/webhooks/<provider>/` | `webhooks/` | MVP 2 (paired with real Source Engine) | Per-provider path; signature validation **in code**, not at gateway. |
| UI exposure | `HTTPRoute` on an IAP-fronted `Gateway` | external (customer-provided IAP) | MVP 3+ | Documented integration recipes for Google IAP, AAD App Proxy, Cloudflare Access, oauth2-proxy, Pomerium. |
| API exposure | `HTTPRoute` / `GRPCRoute` | external | MVP 1 (initial), MVP 2 (hardened) | OIDC for humans, workload identity / mTLS for CI/CD. |
| Agent transport listener | `TLSRoute` passthrough (or standalone `Service type=LoadBalancer`) | `events/nats/` | MVP 2 | Gateway is transport-only; auth done by NATS. See runtime plan §7.4. |
| Backend mTLS | `BackendTLSPolicy` | `config/` | MVP 3 | Where the controller chain supports it. |

**Controller-agnostic.** Keleustes ships `Gateway` / `HTTPRoute` samples but does not ship a Gateway controller. First-class test matrix to be decided (likely Envoy Gateway + one cloud-managed). Documented-only for the rest.

**Hard rule: no legacy `Ingress`.** Not even as a fallback. New deployments are Gateway API only.

## 6. Technology Introduction Timeline (First Pass)

Each MVP also carries an explicit **scale target** (see runtime plan §11.5) that must be demonstrated before the MVP closes — not assumed.

- **MVP 0** (100 Apps / 100 repos): None (read-only, no heavy engines). Gateway API skeleton in place.
- **MVP 1** (~1,000 Apps / ~1,000 repos): `gitops-engine` (`pkg/cache`, `pkg/sync`, `pkg/diff`, `pkg/health`, `pkg/utils/kube`); Kustomize + Helm libraries; raw; basic inventory; **NATS with JetStream** as event/audit log (hub-internal only — agents do not yet connect); **object storage** for content-addressed rendered manifests and JetStream cold archive; Gateway API v1 for the API listener; `client-go` / controller-runtime SSA. **No SQL.**
- **MVP 2** (~2,500 Apps / ~2,500 repos — matches projected Skaphos scale): GitHub mutation first (go-github), cosign verification, basic native policy gates; NATS opened to regional agents via leaf nodes (runtime plan §7.4); webhook receivers online behind public Gateway; **NATS KV** for hot indexes; sharded controllers (Application, Source) required.
- **MVP 3** (~10,000 Apps / ~10,000 repos): GitLab + Azure DevOps, Trivy/Grype integration, regional agent support for rendering + apply, optional second transport (gRPC) for customers who reject NATS; **DuckDB-on-object-storage** for UI matrix / analytics queries (rebuilt from JetStream replay).
- **MVP 4**: Full policy integrations (OPA, SLSA, GUAC), advanced diff normalizers, full multi-region story.

**What is conspicuously absent from this timeline:** PostgreSQL / MariaDB / any RDBMS as a core dependency. By design — see §5.5.

## 7. Open Decisions & Risks

1. **Render as library only?** Exec fallback increases attack surface and makes reproducible builds harder.
2. **Agent binary size & base image.** If we embed `gitops-engine` + Trivy + Helm + Kustomize + multiple Git SDKs, the agent image becomes large. Consider a plugin or sidecar model for heavy policy tools, and a slimmer build-tag set for the agent (no Git-mutation providers, no policy externals).
3. **Git mutation from agents.** Some organizations will never allow agents to write to Git. The autonomy model must be configurable per `PromotionPolicy` / `DeploymentTarget`.
4. **Helm chart repository authentication** in a distributed world (how does an agent in a restricted network reach a private chart repo?).
5. **Dependency pinning strategy** for all the above libraries (use `tools/` pattern + Renovate or similar).
6. **`gitops-engine` version pinning and upgrade cadence.** Major version bumps have historically been disruptive. Pin tightly; gate upgrades behind targeted regression coverage; revisit at each MVP boundary, not opportunistically.
7. **Argo CD annotation compatibility.** Do we accept `argocd.argoproj.io/sync-wave` and hook annotations directly on user manifests (lowest migration friction) or define `keleustes.skaphos.io/...` equivalents (cleaner ownership, no surprise inheritance of Argo behaviors we don't intend to support)? A hybrid (accept Argo annotations, translate internally, prefer ours in new docs) is probably the right answer but should be explicit.
8. **Cross-engine bypass of the containment rule.** If, say, the Promotion Engine ever needs raw diff output from `gitops-engine` rather than our normalized form, do we widen the `diff/` interface or allow the import? Default answer is widen the interface; cases that argue otherwise need a written exception.
9. **JetStream retention and archive cadence.** How long do we keep events in JetStream itself vs. rolled to object-storage archive segments? Default proposal: 30 days hot in JetStream, rolling archive thereafter. Tunable per stream. Needs benchmark data.
10. **DuckDB rebuild cadence.** For UI matrix and timeline views, how fresh does the parquet snapshot need to be? Options: rebuild every N minutes (simple, possibly stale), tail JetStream into a writeable parquet (fresher, more code), serve "live" from JetStream + KV for the last N hours and parquet for older. Probably the third, but cost is real complexity.
11. **Webhook receiver deployment shape.** Single binary with separate listeners (simpler MVP) vs. separate `Deployment` per concern (cleaner scaling, independent deploy cadence). Recommend single binary in MVP 1, split webhooks out in MVP 2 when public exposure goes live.
12. **Sharded controller pattern choice.** Pick a sharder by MVP 2 — e.g., the controller-runtime sharder pattern Argo CD ApplicationSet uses, or a Keleustes-specific predicate-based shard filter. Not glamorous but blocks 2K+ Application throughput.
13. **Scale benchmark harness.** We need a synthetic load generator (N fake Applications, N fake repos, simulated webhook bursts, simulated agent fan-out) to gate MVP exits. Building this is engineering work that must be funded explicitly.
14. **Dependency declaration ergonomics** (§2.6). How explicit does the user need to be about CRD dependencies? Options: (a) fully explicit `spec.dependencies.crds`, (b) auto-inferred from the Application's manifest references (we scan the manifests and discover which CRDs they consume), (c) hybrid. Auto-inference is friendlier but can be wrong; explicit is verbose but unambiguous. Probably hybrid with auto-inference as a suggestion the user accepts.
15. **Cross-shard dependency coordination** at sharded-controller scale (runtime plan §11.5). When Application A is owned by shard 1 and depends on Application B owned by shard 2, the dependency-satisfied event must cross shards. JetStream subjects are the natural carrier; needs concrete design before MVP 2.

## 8. Recommended Immediate Next Actions

1. Accept the package structure above as the target for the first real engine work.
2. **Spike: vendor `gitops-engine`** in a throwaway branch. Measure (a) module graph delta and binary size impact for both `manager` and a hypothetical `agent`, (b) ease of wrapping `pkg/sync` with our `SyncRun` lifecycle, (c) whether `pkg/cache` instantiates cleanly per `DeploymentTarget` with bounded resource use. Report results into the eventual ADR.
3. Create `internal/render/` as the first non-trivial Keleustes-owned package (even a stub that just returns "not implemented" with the right request/response types). This forces all future sync/promotion code to go through the correct seam.
4. Write the `RenderRequest` / `RenderResult` types + inventory extraction logic as a pure function with tests.
5. Choose NATS as the event bus officially and add it to the technology map (already reflected here; needs to be reflected in PROPOSAL and an ADR).
6. For the next planning document, produce a **"Render Contract & Inventory Model"** deep-dive (interfaces, what exactly is captured in inventory, how pruning works, how to handle CRDs and custom resources) and a parallel **"Agent Transport Interface"** sketch aligned with runtime plan §7.4.

---

When sections of this plan stabilize (especially the package layout and the Render boundary), they should be promoted into one or more ADRs.

> **Promoted:** [ADR 0006 — Engine boundaries and `gitops-engine` reuse](../adr/0006-engine-boundaries.md) resolves §7 questions 1–8 and 14–15. §7 questions 9–13 are resolved in [ADR 0005](../adr/0005-distributed-runtime.md).

This document will be updated as technology choices are made and as the distributed runtime model matures.
