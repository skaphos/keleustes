<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Distributed Runtime Architecture Plan

**Status:** Draft  
**Date:** 2026-05  
**Related:** PROPOSAL §9 (Architecture), §10 (Deployment model), §20 (MVP roadmap)  
**Owner:** Platform Architecture (Skaphos)  
**Intended outcome:** Concrete planning input for future ADRs on runtime decomposition, agent protocol, and multi-region operation. Not a decision record.

---

## 1. Executive Summary

Keleustes must support a credible multi-region, high-DR operating model for large platform teams. The single central controller pattern (even with Redis caching) is insufficient for the target use cases.

This plan explores a **hub + regional agents/runners** model, inspired by mature implementations in the GitOps ecosystem (notably Akuity's approach to Argo CD). The goal is to achieve:

- Regional failure isolation for the control plane.
- Load distribution of expensive work (rendering, Git operations, large-scale sync, promotion evaluation).
- Continued limited operation of regional targets when the central hub is unreachable.
- A clean evolutionary path from a simple monolithic start (MVP 0/1) to a distributed system without major rewrites.

The existing CRD surface (`SyncPlan`, `SyncRun`, `Deployment`, `DeploymentTarget`, `Promotion`) is already well-aligned with a delegated execution model. The runtime and engine boundaries are not.

## 2. Business and Operational Drivers

Platform teams at Skaphos scale need:

- **Disaster Recovery** — Loss of one region (or one availability zone within a cloud) must not take down promotion pipelines or visibility for other regions.
- **Load Distribution** — Rendering, diffing, and reconciliation of hundreds of applications across dozens of clusters generates significant CPU/memory/IO. Centralizing all of it creates hot spots and cost problems.
- **Latency & Data Sovereignty** — Some rendering, policy evaluation, or Git mutation work benefits from being close to the target clusters or regional Git mirrors.
- **Operational Resilience** — During a control-plane outage, teams still need to be able to perform emergency syncs, health checks, and limited promotions in surviving regions.
- **Competitive Parity** — Akuity, Codefresh, and other commercial GitOps platforms already sell "control plane + distributed agents" as a core differentiator. An open-source control plane that cannot articulate a credible path here will be at a disadvantage for enterprise adoption.

Keleustes is more ambitious than pure Argo CD (it owns the promotion state machine, Git mutation, topology, and policy). This makes the decomposition *more* important, not less.

## 3. Current State (Scaffold)

### What exists today

- All 14 CRD types defined with good structure and documentation.
- 6 lightweight controller stubs wired (`Application`, `Environment`, `Cell`, `DeploymentTarget`, `Promotion`, `SyncPlan`).
- `SyncRun` and `Deployment` types already model per-target execution attempts.
- `DeploymentTarget` models remote clusters via `kubeconfigSecretRef` (or in-cluster SA).
- Proposal mentions a future `Keleustes Worker` and "optional later" Redis/NATS.
- One-paragraph "future federation" section describing Hub + Agents (PROPOSAL §10.3).

### What is missing

- No component decomposition beyond "controller + worker + api + ui".
- No agent/runner concept.
- No discussion of how the Sync Engine or Promotion Engine would delegate work.
- No event bus or notification architecture.
- No model for regional control-plane instances or partition behavior.
- Rendering, Git mutation, and heavy policy evaluation are not yet located anywhere in the architecture.

The type system is quietly ahead of the runtime thinking. This is an opportunity.

## 4. Goals for the Distributed Runtime

1. **Regional DR for visibility and limited mutation** — When the central hub is down, regional agents should still be able to report health, serve matrix views (via cached state), and execute pre-approved or emergency syncs.
2. **Scalable execution of expensive work** — Rendering (kustomize/helm), large diffs, Git operations, and policy evaluations can be offloaded to agents or worker pools that are closer to the data or sharded by scope.
3. **Clean ownership boundaries** — The central control plane owns desired state, promotion orchestration, and audit. Regional agents own execution against specific `DeploymentTarget`s.
4. **Evolutionary path** — MVP 0/1 can be largely central. The agent delegation protocol and interfaces are designed early so later MVPs do not require re-architecture.
5. **Explainability preserved** — Every `SyncRun` and `Promotion` phase transition must still be traceable to a Git commit, render output, policy result, and actor/approver, even when work was executed regionally.
6. **Git remains the source of truth** — Agents never become autonomous in a way that violates the "mutate Git, not cluster" rule for normal flows.

## 5. Constraints & Non-Goals

**Constraints** (from PROPOSAL and CLAUDE.md):
- CRDs remain the source of truth for active desired/control state.
- The PROPOSAL originally called for Postgres as the durable store for history, audit, UI cache, and promotion timeline. **This plan supersedes that choice** — see §11 — in favor of a no-RDBMS-on-the-critical-path model (NATS JetStream + object storage + DuckDB-derived layer) driven by the restorable-from-zero requirement.
- Reconcilers and engines must be idempotent and bounded.
- Application deploys mutate Git, not cluster state (except explicit break-glass).
- Initial sync engine is intentionally constrained (kustomize/helm/raw paths).
- Kubernetes-native does not mean "only in-cluster".

**Non-goals for this planning exercise**:
- Full active-active multi-master CRD reconciliation (extremely complex; not required for the first distributed version).
- Building a general-purpose workflow engine.
- Supporting arbitrary plugins in the initial Sync Engine.

## 6. Proposed Component Model

```
                  +-------------------------------------------+
                  |              Keleustes Hub                |
                  |  (Management cluster or regional control) |
                  +-------------------------------------------+
                                 |          ^
                  +--------------+----------+--------------+
                  |              |          |              |
                  v              v          v              v
            +-----------+  +-----------+  +-----------+  +-----------+
            |   API     |  |  Core     |  | Promotion |  |  Source   |
            |  Server   |  | Recon-    |  |  Engine   |  |  Engine   |
            | (REST)    |  | cilers    |  |           |  |           |
            +-----------+  +-----------+  +-----------+  +-----------+
                                 |               ^
                                 | (delegation)  |
                                 v               |
                          +-------------+        |
                          |   Event Bus | <------+
                          | (NATS/etc.) |
                          +-------------+
                                 ^
                                 | regional replication / federation
                +----------------+----------------+
                |                |                |
                v                v                v
         +-------------+  +-------------+  +-------------+
         | Regional    |  | Regional    |  | Regional    |
         | Agent /     |  | Agent /     |  | Agent /     |
         | Runner      |  | Runner      |  | Runner      |
         | (Cluster A) |  | (Cluster B) |  | (Region C)  |
         +-------------+  +-------------+  +-------------+
                 |                |                |
                 v                v                v
            Target Clusters  Target Clusters  Target Clusters
            (render, SSA,    (render, SSA,    (render, SSA,
             health, local    health, local    health, local
             SyncRuns)        SyncRuns)        SyncRuns)
```

**Core Hub Components** (can be co-located or split):
- **API Server** — Public REST (primary) + future gRPC. Owns authz, query routing, command surface.
- **Core Reconciliation** — Lightweight controller-runtime reconcilers for the topology and planning CRDs (`Application`, `SyncPlan`, `Promotion`, etc.).
- **Promotion Engine** — Owns the promotion state machine, policy evaluation orchestration, and Git mutation requests.
- **Source Engine** — Watches external sources and emits revision events.
- **Worker Pool** (optional early) — Async jobs for heavy rendering, diffing, evidence collection.

**Regional Agents / Runners**:
- Deployed in or near target clusters/regions.
- Claim or are assigned `SyncRun` work for `DeploymentTarget`s they are responsible for.
- Execute rendering (when delegated), server-side apply, inventory tracking, health checks, and `Deployment` status reporting.
- Can participate in limited local promotion steps (e.g., pre-approved rollbacks or emergency sync) when hub is unreachable.
- Report `HealthCheck` and `Deployment` state back to the hub (via event bus or direct API).
- Built on top of [`gitops-engine`](https://github.com/argoproj/gitops-engine) for the per-target cluster cache, diff with normalizers, resource health, and manifest-level sync mechanics (SSA, waves, hooks, prune). See the engine-boundaries plan §2.5 for what we adopt vs. retain.
- Reach the hub via an outbound-only transport (default: NATS leaf node; see §7.4). The agent never listens for inbound connections.

**Cross-cutting**:
- **Event Bus + Durable Log** (**NATS JetStream**, committed) — Decouples components, enables reliable regional fan-out, and serves as the durable append-only event/audit log (see §11).
- **Webhook Receivers** — Stateless public-facing pods that validate provider signatures and publish normalized events to JetStream. Horizontally scaled behind a public Gateway (see §7.5).
- **Object Storage** (S3 / GCS / Azure Blob / MinIO) — Content-addressed home for rendered manifests, evidence bundles, and JetStream cold-archive segments. No backups needed (provider-native replication; content addressing makes writes idempotent).
- **Hot Index Layer** — NATS KV (built on JetStream) for current values (agent presence, locks, recent `Deployment` snapshots). Always rebuildable from JetStream replay.
- **Derived Query Layer** — DuckDB-over-parquet on object storage for UI matrix and analytics queries; rebuilt periodically from JetStream replay; no backups.
- **OIDC / Authz Service** — Central (or regionally mirrored) for consistent identity and fine-grained promotion approval.
- **No relational database on the critical path.** State that cannot be reconstructed from CRDs + Git + JetStream does not exist — see §11.

## 7. Hub + Regional Agent Pattern (Detailed)

### 7.1 When the Hub is Healthy (Normal Operation)

- Central Promotion Engine decides *what* should be promoted and to which `DeploymentTarget`s.
- It creates or updates `SyncPlan` objects.
- The Core Reconciliation layer (or a dedicated SyncPlan controller) creates `SyncRun` objects.
- Regional agents watch for `SyncRun` objects assigned to their `DeploymentTarget`s (via label selectors or explicit affinity on the `DeploymentTarget`).
- Agents perform the render (if the render work was delegated), apply, and update the `SyncRun` through its phases (`Rendering` → `Applying` → `Verifying` → terminal).
- Agents write `Deployment` records and `HealthCheck` results.
- All material changes flow back through the event bus to JetStream (for audit/timeline; durable, replayable) and the API layer (for live status).

### 7.2 During Hub Partition or Outage

- Regional agents continue to execute already-approved `SyncRun`s and report health.
- Agents can be configured with a "local autonomy" policy for specific `FreezeWindow` or emergency scenarios (e.g., "if hub unreachable for > 15 min and change is pre-approved, allow direct commit rollback").
- Visibility (matrix, blockers, last known good) is served from regional NATS KV snapshots + DuckDB parquet shards on regional object storage.
- New promotions and complex cross-region waves are blocked or queued until the hub returns.
- On reconnection, agents reconcile any local drift against the central `SyncPlan`/`Promotion` state.

This model gives operators a useful "read-mostly + limited mutation" experience during control-plane incidents — far better than a fully centralized system that goes dark.

### 7.3 Agent Identity and Security

- Agents authenticate to the hub using workload identity or mTLS.
- `DeploymentTarget` can carry an `agentRef` or affinity rules in later versions.
- Agents only have permissions to act on the `DeploymentTarget`s they own (scoped RBAC or claims).
- Identity material is bound to the transport (see §7.4): an NKey + signed JWT for the default NATS leaf transport; mTLS client certs or SPIFFE SVIDs for alternates.

### 7.4 Cross-Connect Transport (Agent ↔ Hub Connectivity)

The hub and regional agents will run in different networks: different cloud regions, different cloud providers, on-prem datacenters, and (for managed offerings) customer VPCs. This is conceptually a **Tailscale-shaped problem** — identity-bound, NAT-traversing connectivity between many endpoints — but Keleustes cannot mandate Tailscale (or any other overlay-network vendor) without losing a large slice of the enterprise audience. The transport has to look and feel like Tailscale (outbound-only from the agent, strong identity, multi-region reachability) without being Tailscale.

#### Constraints we accept

- **Outbound-only from the agent.** Many enterprise networks allow agents to dial out (often only HTTPS, often only through a forward proxy) but disallow inbound. The transport must work with no listening port on the agent side.
- **Strong end-to-end identity.** Every message must be attributable to a specific agent identity, which is in turn bound to the `DeploymentTarget` set the agent is permitted to act on. mTLS, NKey + JWT, SPIFFE SVIDs, or cloud workload identity tokens are all acceptable shapes.
- **Durable and replayable.** On reconnection after a partition, an agent must catch up on missed work and the hub must catch up on missed status without losing events.
- **No vendor lock-in.** A customer with Tailscale, Teleport, Cloudflare Tunnel, or a bastion/SSH pattern should be able to drop in their existing transport. A greenfield customer should get a sensible default with zero extra infrastructure.

#### Recommended approach: NATS leaf nodes as default, pluggable transport interface

**Default transport: NATS leaf nodes connecting outbound to the hub's NATS cluster.**

- The hub runs a NATS cluster with JetStream as the event bus (§6, §11).
- Each regional agent embeds a NATS *leaf node* that dials the hub cluster outbound over TLS (port 4222, or 443 with WebSocket transport for stricter networks).
- Leaf authentication uses NKeys + signed JWTs; JWT scope mirrors the agent's allowed `DeploymentTarget` set (subject permissions, stream permissions).
- JetStream provides the durable, replayable streams the partition story needs — both for work assignments outbound and results/events back.
- Multi-region: the hub can be a NATS supercluster; agents always reach the nearest cluster member, and the cluster handles regional fan-out internally.

```
                +-------- Hub Region --------+
                |   NATS cluster + JetStream |
                |   API Server / gRPC        |
                +--------------+-------------+
                               ^
                  outbound TLS | (mTLS / NKey JWT)
                               | (no inbound listener on agent)
              +----------------+---------------+
              |                |                |
        +-----v------+   +-----v------+   +-----v------+
        | Region A   |   | Region B   |   | On-prem    |
        | NATS leaf  |   | NATS leaf  |   | NATS leaf  |
        | Agent      |   | Agent      |   | Agent      |
        +-----+------+   +-----+------+   +-----+------+
              |                |                |
         target clusters  target clusters  target clusters
```

**Why NATS leaf over plain gRPC streaming or a bespoke protocol:**
- Outbound-only connection model is native; no extra plumbing.
- Subject ACLs and replayable streams are already there.
- We need the event bus regardless; reusing it as the agent transport avoids running a second wire.
- The leaf is a library import in the agent binary, not a separate process for the operator to manage.
- Operationally well-understood: NATS has a multi-year track record in this exact shape (Synadia leaf nodes, Argo Events, etc.).

**Transport interface for pluggability.**

The agent-side code talks to a Keleustes-defined Go interface (sketch: `internal/agent/transport.Transport`) with verbs like `ClaimWork`, `PublishEvent`, `StreamLargePayload`, `Heartbeat`. The default implementation wires through the embedded NATS leaf. Alternate implementations enable:

- **Bring-your-own overlay** — Tailscale, Teleport, Twingate, Cloudflare Tunnel, OpenZiti: the customer terminates connectivity on a private hub endpoint and the agent transport just dials a local address. Keleustes does not depend on the overlay; the overlay is invisible to Keleustes.
- **gRPC bidirectional streaming** — For environments where NATS is not acceptable (regulatory, ops familiarity).
- **HTTP/2 long-poll** — For maximally constrained networks that only allow conventional HTTPS through an inspecting proxy.

Keleustes ships and supports the NATS-leaf transport. Alternate transports are an extension point, not a maintained compatibility matrix.

#### Failure modes

- **Agent loses connection** — retries with backoff; JetStream replay catches it up on reconnect; no work is lost.
- **Hub NATS cluster partition** — leaf reconnects to a surviving member of the supercluster.
- **Agent never reconnects** — hub marks affected `DeploymentTarget`s with an `AgentUnreachable` condition after a configurable grace period; UI surfaces this prominently; promotions affecting that target are blocked.
- **Forward-proxy environments** — leaf supports WebSocket-over-TLS on 443, which traverses most enterprise proxies that allow generic outbound HTTPS.
- **Compromised agent credential** — NKey + JWT is revocable centrally; revocation propagates via the NATS account JWT update mechanism, no cluster restart required.

#### Open transport questions (collected; resolved in §13)

- WebSocket-on-443 as default vs. native 4222.
- Whether to ship a second-class but officially-supported gRPC transport in MVP 3 for customers who reject NATS.
- How to model the transport choice in the `DeploymentTarget` or in a new `Agent` CR (likely the latter — agents are first-class enough to deserve their own resource).

### 7.5 External Exposure Surface (Gateway API + Tiered Exposure)

Keleustes is a **modern-Kubernetes** project. Exposure is via [Gateway API v1](https://gateway-api.sigs.k8s.io/) (`gateway.networking.k8s.io/v1`, GA), **not** legacy `Ingress`. This is a hard rule; we will not invest in `Ingress` shims even for short-term compatibility.

Different parts of the hub have different exposure requirements and identity models. Conflating them into a single Gateway is wrong both **operationally** (a webhook flood would degrade UI access) and from a **security** standpoint (webhook receivers have no business sharing a listener — or a trusted identity header — with the UI).

#### Exposure tiers

| Tier | Components | Identity model | Typical fronting |
|------|------------|----------------|------------------|
| **Public internet** | Git / OCI / Helm provider webhook receivers (`/webhooks/github`, `/webhooks/gitlab`, `/webhooks/azuredevops`, `/webhooks/oci`, ...) | Per-request provider HMAC / signature verification in our code. **No OIDC.** | Bare public `Gateway` with TLS; rate limiting + optional provider IP allowlists. |
| **Internal users (UI)** | UI static assets + UI-bound API endpoints | Cloud IAP / Identity-Aware Proxy (Google IAP, AAD App Proxy, Cloudflare Access, Pomerium, oauth2-proxy, ...) terminates identity at the gateway; backend trusts the IAP-injected header. | GKE Gateway with IAP; Envoy Gateway with oauth2-proxy ExtAuth; Contour with Pomerium; etc. |
| **Internal / API** | Public REST / gRPC API for `keleustesctl`, CI/CD integrations | OIDC (any provider) for humans; workload identity / mTLS for CI/CD. | Same IAP gateway with a separate listener, **or** a dedicated internal gateway with NetworkPolicy isolation. |
| **Agent transport** | NATS leaf endpoint (default transport, §7.4) | NKey + JWT terminated by **NATS itself**. | Dedicated `Gateway` with `TLSRoute` passthrough, **or** a standalone `Service type=LoadBalancer`. The gateway here is a transport, not an auth boundary. |
| **Cluster-internal observability** | Prometheus `/metrics`, profiling endpoints | NetworkPolicy + cluster-internal only. | **No Gateway** — `ClusterIP` Service with NetworkPolicy. |

#### Why separate Gateways per tier

- **Blast radius.** A webhook-flood DoS must not take down the UI. A compromised webhook signature path must not give path-traversal into UI auth.
- **Auth model mismatch.** IAP-fronted traffic injects identity headers the backend trusts; webhook traffic must not be able to forge those headers. Different listeners with different policies is the simplest enforcement.
- **TLS policy.** Webhook endpoints often need stable public hostnames and provider-acceptable certs; UI hostnames can move freely and benefit from stricter cipher policy.
- **Independent scaling.** Webhook receivers are spiky and stateless; the UI is bursty-but-cacheable; the API is steady. Different Gateways let them scale (and fail over) independently.

#### Gateway API specifics we commit to

- **API version:** Gateway API v1 only. No `extensions/v1beta1 Ingress`, no `networking.k8s.io/v1 Ingress`.
- **Resources used:** `GatewayClass`, `Gateway`, `HTTPRoute`, `GRPCRoute`, `TLSRoute` (for the agent transport listener), `ReferenceGrant` (cross-namespace backends), `BackendTLSPolicy` where backend-side mTLS is required.
- **Controller-agnostic.** Keleustes does **not** ship a Gateway controller and does **not** assume one. We provide `Gateway` / `HTTPRoute` samples and integration recipes for at least: Envoy Gateway, Contour, Istio, GKE Gateway, AKS Application Gateway, Cilium Gateway. Customers run what they already run.
- **Webhook receivers.** Live in `internal/webhooks/<provider>/` (see engine-boundaries plan §3). Each provider has a dedicated path under one public `HTTPRoute`. Signature validation is **per-provider, in our code** — never delegated to the gateway, because no gateway controller validates GitHub HMAC out of the box and we should not bind to one that does.
- **IAP integration.** Out of scope for Keleustes to ship the IAP itself; we ship documented integration recipes per common provider. The backend trusts the IAP-injected user/email header **only** on the IAP-fronted listener (enforced by checking the listener's configured trust header — not by trusting any inbound `X-Authenticated-*` header globally).

#### What this changes for the component model (§6)

The hub decomposes into at least:

- **Webhook receiver** — small public-facing HTTP server. Stateless. Validates provider signatures, publishes a normalized revision event onto the bus. Horizontally scaled independently.
- **UI** — separate Deployment, fronted by the IAP-tier Gateway.
- **API server** — REST/gRPC. Co-located with the webhook receiver in a single binary in MVP 0/1 is acceptable (different listeners, same process); split when scale or independent deploy cadence justifies it.
- **NATS** — its own Service/Gateway listener (§7.4).
- **Existing engines** — unchanged; consume bus events.

This decomposition is friendly to the regional/agent model: webhook receivers are region-agnostic and can run multiple instances behind a global anycast Gateway; the UI is the same; the API server scales like a normal HTTP service.

## 8. Mapping to Existing Types

The current types already provide excellent seams:

| Type            | Current Role                          | Role in Distributed Model                          | Notes |
|-----------------|---------------------------------------|----------------------------------------------------|-------|
| `SyncPlan`      | Binds Application to DeploymentTargets | Central declaration of desired reconciliation scope | Hub owns creation/update |
| `SyncRun`       | Execution record of one attempt       | The primary delegation unit. Agents claim & execute | Phases (Rendering, Applying...) are perfect for distributed handoff |
| `Deployment`    | Live state on a target                | Written by the agent that performed the sync       | Regional source of truth for "what is actually running here" |
| `DeploymentTarget` | Where an app can run               | Carries execution affinity / preferred agent in future | Already models remote kubeconfig |
| `Promotion`     | Requested movement of a Release       | Central orchestration; can delegate execution steps | Promotion phases remain hub-owned |
| `HealthCheck`   | Health evaluation                     | Primarily produced by regional agents              | Can be aggregated centrally |

This alignment is a strong signal that the type design was done with distribution in mind, even if the runtime was not.

## 9. Phased Rollout Aligned to MVPs

**MVP 0 (Read-only replacement)**
- All work central.
- `SyncPlan`/`SyncRun` stubs only.
- Agents not required; the pattern is designed but not implemented.

**MVP 1 (Constrained Sync)**
- First real Sync Engine, built on top of `gitops-engine` (`pkg/cache`, `pkg/sync`, `pkg/diff`, `pkg/health`) — see engine-boundaries plan §2.5.
- Rendering and apply still happen in the central worker/controller.
- Introduce NATS (with JetStream) inside the hub for the worker job queue. Agents do not yet connect to it — but the transport interface (§7.4) is defined.
- Introduce the **agent protocol design** and a minimal agent that can report `Deployment` + `HealthCheck` (read path only). This proves the delegation boundary without risking mutation.

**MVP 2 (Releases + Promotions)**
- Promotion Engine + basic Git mutation.
- `SyncRun` execution can be delegated for a subset of `DeploymentTarget`s (opt-in via annotation or `DeploymentTarget` field).
- Event bus introduced.
- Regional agents can perform rendering and apply for their targets.

**MVP 3 (Enterprise Topology)**
- Full regional agent deployment model.
- Agents participate in waves and can execute limited local promotions during partitions.
- Regional NATS KV + object-storage replication patterns defined; JetStream supercluster spans regions.
- Multi-region matrix and blast-radius views.

**MVP 4 (Policy & Audit)**
- Policy evaluation can be sharded (some policies run centrally for consistency, others regionally for data locality).
- Full audit trail works across hub/agent boundaries with cryptographic linking (future).

## 10. Key Interfaces & Contracts to Define Early

Before writing significant Sync or Promotion engine code, we should define (even if not fully implement):

1. **SyncRun Execution Protocol** — How an agent claims a `SyncRun`, reports progress through phases, and surfaces render/apply results.
2. **Render Request / Response** — Input (Application + Release + manifest config + target context) → Output (rendered manifests, inventory, warnings). This can be a local call early, then a remote procedure call to an agent or worker.
3. **Agent Registration & Heartbeat** — How agents announce themselves and which `DeploymentTarget`s they are responsible for.
4. **Regional State Snapshot** — What minimal state an agent must hold locally to operate during a hub outage (last known `SyncPlan` versions, approved emergency promotion tokens, etc.).
5. **Event Schema** for the bus (promotion transitions, `SyncRun` phase changes, source revisions).

Defining these interfaces in MVP 1 (even as internal Go interfaces + later gRPC/HTTP) will dramatically reduce future refactoring.

## 11. Data, Consistency, and Restorability

### 11.1 Hard constraint: restorable from zero, no backups

Keleustes must be **restorable from zero without restoring a backup file**. Any storage layer whose loss requires a `pg_dump`-style restore is disqualified from the critical path. Operationally this matters because Keleustes carries the promotion state machine for thousands of applications across thousands of repositories; an additional backup/restore discipline on top of that is unacceptable.

The rule that drops out: **state that cannot be reconstructed from CRDs + Git + JetStream does not exist.**

### 11.2 Storage tiers

| Tier | Technology | Purpose | Critical-path? | Recovery |
|------|-----------|---------|----------------|----------|
| Authoritative active state | **CRDs (etcd)** | Desired state, control state, topology | Yes | Re-applied from the Git config repo (GitOps the operator itself). |
| Authoritative history | **Git repos** (user-managed) | Manifest history, release tags, promotion commits | Yes | User's Git provider is the system of record; nothing to back up on our side. |
| Event / audit log | **NATS JetStream** durable streams | Promotion events, SyncRun phase transitions, source revisions, approval events, agent events | Yes | Replicated R≥3 across NATS cluster members; cold archive to object storage on a rolling window; replay from archive on catastrophic NATS loss. |
| Hot indexes / presence | **NATS KV** (on JetStream) | Agent presence, leader locks, current `Deployment` snapshots per target | No | Rebuildable from JetStream replay or live agent reports. |
| Large content-addressed artifacts | **Object storage** (S3 / GCS / Azure Blob / MinIO) | Rendered manifests (keyed by content hash), evidence bundles, JetStream archive segments, large diffs | Cache (mostly); archive (yes) | Provider-native bucket replication. Rendered manifests are pure cache — re-rendered on miss. |
| Derived analytics / UI matrix | **DuckDB over parquet on object storage** | UI matrix, promotion timeline, ad-hoc cross-Application queries | No (fully derived) | Periodically rebuilt from JetStream replay. No schema migrations. No backups. Wipe-and-rebuild in minutes. |

**Explicitly disqualified from the critical path:** PostgreSQL, MariaDB, MySQL, Oracle, SQL Server, and any other RDBMS as source-of-truth. A customer may run a derived SQL consumer side-car against the JetStream audit stream for BI, but Keleustes core does not depend on it.

See engine-boundaries plan §5.5 for the package-level mapping.

### 11.3 Consistency model

- **CRDs** — apiserver consistency; eventual to controllers via watch.
- **JetStream** — at-least-once delivery within a consumer group; ordering guaranteed per subject; replay is idempotent because consumers are designed to be idempotent.
- **NATS KV** — last-writer-wins per key; for cases where this is wrong (concurrent updates to a `Deployment` snapshot from competing agents), the model is "an agent owns its targets; only its writes are accepted." Enforced by JWT subject permissions, not optimistic concurrency.
- **Object storage** — content-addressed writes are idempotent; non-content-addressed writes (archive segments) use generation numbers / preconditions.

During partitions, the system must be explicit about **which writer is authoritative**. The rule for `Deployment` and `HealthCheck`: **the owning agent is always authoritative for its targets.** Conflicting hub-side writes during a partition are dropped on reconnect; the agent's view wins.

### 11.4 Recovery from zero (target time budgets, indicative)

| Loss scenario | Recovery procedure | Target |
|---------------|--------------------|--------|
| Operator pod lost | Pod restart | seconds |
| CRDs lost, config repo intact | `kubectl apply -k` from config repo | minutes |
| NATS JetStream lost, archive intact | Restore latest archive segments; replay | tens of minutes |
| JetStream lost, no archive | Reconstruct live state from CRDs + agent reports; **historical audit beyond CRD `.status` is gone** (explicit, documented limit) | minutes for live; historical loss is accepted |
| Object storage lost | Cache regenerates on demand; parquet regenerates on next rebuild | hours for full warm-up; nothing permanently lost |
| Cluster lost | Re-bootstrap cluster, then recover as above | hours |

These targets must be **demonstrated** before MVP 2 closes — not assumed.

The recovery runbook is short by design: redeploy the operator, apply the config repo, wait. There is no step that requires finding a recent `pg_dump`.

## 11.5 Horizontal Scaling and Sharding

**Concrete scale target.** The initial Skaphos production environment is **~500 Applications across ~450 repositories today, projected to 2000+ repositories in a few months**. Horizontal scalability is a **day-one requirement**, not a later refactor. Single-replica components and single-shard reconcile loops are disallowed in MVP 1; the only acceptable scale-up axis is *more pods*, *more shards*, *more agents* — never *bigger pods*.

### Scale targets per MVP

| MVP | Target Applications | Target repositories | Target `DeploymentTarget`s | Notes |
|-----|---------------------|---------------------|----------------------------|-------|
| MVP 0 | 100 | 100 | 10 | Read-only; proves topology types and observability. |
| MVP 1 | **1,000** | **1,000** | 50 | Matches near-term Skaphos load with headroom. All hub components horizontally scalable. |
| MVP 2 | **2,500** | **2,500** | 200 | Matches projected Skaphos scale. Agent execution introduced; sharded controllers required. |
| MVP 3 | 10,000 | 10,000 | 500+ | Enterprise platform. Multi-region agents; JetStream supercluster; object-storage lifecycle policies for archive; DuckDB parquet sharding for matrix queries. |
| MVP 4 | 10,000+ | 10,000+ | 500+ | Same as MVP 3 with full policy evaluation at scale. |

These targets must be **demonstrated via benchmark before the MVP is closed**, not assumed.

### Sharding model per component

| Component | Sharding key | How it scales | Notes |
|-----------|--------------|---------------|-------|
| Webhook receivers | None (stateless) | More pods behind a public Gateway; HPA on QPS/CPU | Validated event published to JetStream subject for the provider; downstream consumers are sharded. |
| Source watchers | `Source` name → consistent hash | JetStream consumer group with N consumers, durable per-shard; new shard = new consumer joins the group | Webhooks drive steady state; scheduled polls handle bootstrap and drift detection only. Per-provider rate limits applied per shard. |
| Render workers | None (pool); content hash for cache | Pool consumes from JetStream render queue; results stored in object storage keyed by `(Application, Release, target context)` content hash, with NATS KV as the hot index | Cache dedupes work across promotions to many targets from the same release. |
| `Application` / `SyncPlan` controllers | `Application` name → hash slice | Sharded controllers (N replicas, each owns a hash slice via a filter predicate). Same shape Argo CD's ApplicationSet uses. | Single-replica leader-elected is acceptable up to ~1K objects per type; above that, sharded is required. Commit to a sharder library by MVP 2. |
| `Source` controller | `Source` name → hash slice | Same pattern | Larger surface than `Application`; sharded earlier. |
| `Promotion` controller | `Promotion` name | Workqueue-keyed; only one worker per Promotion at a time, but many Promotions concurrent | Per-Promotion work is bounded in time; single-writer is safe and simplifies state. |
| Sync execution | `DeploymentTarget` ownership (which agent owns it) | More agents = more parallel target reconciliation | MVP 1: central. MVP 2+: per-target agent claim. |
| Cluster cache | One cache per `DeploymentTarget` | Lives on the agent that owns the target; RAM scales with target count per agent, not globally | A central hub holding caches for hundreds of targets is the wrong shape; this is one of the primary motivators for agents. |
| API server (read) | None (stateless) | More pods; backed by NATS KV for hot snapshots + DuckDB-on-parquet for matrix queries | UI matrix served from materialized parquet snapshots, not aggregated on demand at this cardinality. |
| API server (write) | None (stateless) | Writes go to CRDs (apiserver) or publish to JetStream (audit/events) — both handle their own concurrency | — |
| NATS / JetStream | Subject partitioning + cluster | Hub NATS cluster sized per total subject throughput; JetStream streams partitioned by Application or Source where ordering allows | Replication factor R ≥ 3 in production. Carries event log, audit, agent transport. |
| JetStream archive | Time-segmented | Rolling segments to object storage; lifecycle policies on the bucket | Default: 30 days hot in JetStream, archive thereafter. Restorable by replaying segments. |
| Object storage | Provider-managed (sharded by hash) | Provider-native scaling; content-addressed writes are idempotent | Required for content-addressed rendered manifests and JetStream archive. No backups (replication handles durability). |
| NATS KV | Bucket per concern (presence, leaderlocks, deployment-snapshots) | Sharded by key inside JetStream | Always rebuildable from JetStream replay or live reports. |
| DuckDB query layer | Parquet shard per Application / per time-window | Rebuild jobs run as worker pool consumers of JetStream | No live server; queries open parquet on object storage on demand. Wipe-and-rebuild in minutes. |

### Hot loops we must not write

- ❌ **N+1 reconciles** over `Source` or `Application` on a hot path.
- ❌ **Synchronous render** inside a controller `Reconcile` — must be queued to render workers.
- ❌ **Cluster-cache aggregation across all targets on the hub** — caches live on agents.
- ❌ **UI fan-out queries** that scan all Promotions or all SyncRuns — backed by DuckDB parquet snapshots and NATS KV for live state.
- ❌ **Per-repo Git polling** in steady state — webhooks drive updates; polls are drift-detection only.

### Required benchmarks per MVP exit gate

- p95 reconcile latency under steady-state load at the MVP's target cardinality.
- Webhook receiver throughput: sustained events/sec without backpressure into JetStream.
- Render worker pool throughput: renders/sec with realistic Helm chart complexity.
- JetStream sustained publish throughput at audit cardinality (events/sec, p99 publish latency).
- Memory budget per hub pod, per agent pod, per render worker pod — documented for ops sizing.

### What stays constant under scale

- CRDs remain the source of truth for active desired/control state; their cardinality grows linearly with Applications and Sources, which is fine for the apiserver.
- Reconcilers remain bounded and idempotent (CLAUDE.md guardrail).
- Application deploys remain Git-mutating, not cluster-mutating.

## 12. Failure Modes & Partition Behavior (Critical)

This section will be expanded in the ADR that follows this plan. Key scenarios to model:

- Hub unreachable from one region only.
- Hub completely down.
- Single agent down (others continue).
- Git unreachable from hub but reachable from regional agent (interesting case for Git mutation).
- JetStream cluster member loss (R≥3 absorbs single-node loss; behavior at quorum loss).
- Object-storage region failure (degraded archive; cache regenerates).
- Stale DuckDB parquet during a JetStream catch-up window.

The plan is to produce a "Partition Behavior Matrix" as input to the eventual ADR.

## 13. Open Questions & Future ADR Candidates

> **Partially resolved.** Q14 (sharded vs. leader-elected controllers, which library, when to commit) is answered by [`2026-05-sharded-controller-pattern.md`](./2026-05-sharded-controller-pattern.md) (SKA-328, active interim contract). Q15 (JetStream retention vs. archive cadence) and Q18 (subject / stream layout for JetStream) are answered by [`2026-05-jetstream-subject-and-stream-layout.md`](./2026-05-jetstream-subject-and-stream-layout.md) (SKA-324, active interim contract). The bullets below are retained for archaeology — the live answers live in the SKA-324 / SKA-328 plans and in [docs/DECISIONS.md](../DECISIONS.md).

1. Should the first agent implementation be **in-cluster sidecar** (simple) or **out-of-cluster dedicated deployment** (more flexible for air-gapped)?
2. Rendering location strategy: always central first, then opt-in delegation, or designed as pluggable from the start?
3. How much "local autonomy" do we give agents for emergency operations, and how is that expressed in `PromotionPolicy` or `FreezeWindow`?
4. Do we need a lightweight "regional control plane" mode (a smaller hub per region) or is pure agent + central hub sufficient?
5. Event bus technology — **working assumption is NATS with JetStream** (see §7.4). Remaining decision: whether agents reach it via NATS leaf nodes (recommended default), via a Keleustes-defined gRPC protocol that internally publishes to NATS, or via a pluggable transport interface from day one.
6. OIDC/Authz service — do we run Dex, use a cloud OIDC provider directly, or build a thin Keleustes-specific layer?
7. How do we handle Git mutation when the mutation must be performed from a regional agent (some Git providers have regional considerations)?
8. **Transport pluggability timing.** Do we commit to one transport (NATS leaf) for MVP 1–3 and revisit later, or invest in a transport interface early to accommodate Tailscale / Teleport / Cloudflare Tunnel / gRPC as first-class alternates?
9. **Agent identity model.** Introduce an `Agent` CR (with its own registration, transport, and `DeploymentTarget` scope) or fold agent identity into `DeploymentTarget` annotations? An `Agent` CR is cleaner long-term but adds another type to the surface.
10. **`gitops-engine` adoption scope and pinning.** Which packages do we vendor in MVP 1 (the obvious set is `pkg/cache`, `pkg/sync`, `pkg/diff`, `pkg/health`, `pkg/utils/kube`), how do we pin them, and how do we contain the dependency footprint in the agent binary? See engine-boundaries plan §2.5 and §7.
11. **Single binary vs. split Deployments for webhook receiver / API / UI assets.** Single binary with separate listeners is simpler in MVP 0/1; split is operationally cleaner and matches independent scaling needs once production load arrives. Decide explicit MVP for the split.
12. **`TLSRoute` passthrough vs. standalone LoadBalancer for NATS leaf endpoint.** `TLSRoute` keeps everything inside Gateway API and one set of operator skills; standalone Service is simpler today and avoids depending on a Gateway controller that may not implement `TLSRoute` cleanly.
13. **Gateway controller test matrix.** Which controllers do we treat as first-class (sample overlays, smoke tests in CI), which as documented-but-untested? Realistic options for first-class: Envoy Gateway + one cloud-managed (GKE Gateway or AKS Application Gateway).
14. **Sharded vs. leader-elected controllers** at 2K+ Application / Source cardinality (see §11.5). Single-leader controller-runtime is fine to ~1K objects per type but starts to hurt above that. When do we commit to sharded controllers and which library / pattern?
15. **JetStream retention vs. archive cadence.** Default proposal: 30 days hot in JetStream, rolling segments to object storage thereafter. Needs benchmarking at MVP 2 cardinality.
16. **Freshness of the DuckDB / parquet derived layer.** Options: rebuild every N minutes (simple, possibly stale); tail JetStream into a writeable parquet (fresher, more code); serve live state from JetStream + NATS KV for the last N hours and parquet for older windows (probably the right answer; cost is real complexity).
17. **Acceptable historical loss on catastrophic JetStream + archive loss.** If both go, what audit history are we willing to declare unrecoverable beyond what CRD `.status` carries? Default: anything not still resident in a CRD's status history is lost. This is an explicit, documented bound — not a surprise.
18. **Subject / stream layout for JetStream** at 2K+ Applications. One stream per Application is too many; one stream for everything is a bottleneck. Probably partitioned by Application-hash prefix; needs a concrete proposal before MVP 2.
19. **Customer-side derived SQL consumer** (if we publish one). Reference implementation that consumes the JetStream audit stream into Postgres or a data warehouse for BI integrations. Owned by the customer, not Keleustes core — but we should publish a reference so customers don't reinvent it badly.

These will become focused ADRs once the overall direction is accepted.

---

## 14. References

- PROPOSAL.md (especially §9, §10, §11, §20)
- Argo CD architecture and Akuity agent/runner model (external reference)
- [`gitops-engine`](https://github.com/argoproj/gitops-engine) — shared sync/diff/health/cache library
- [Gateway API](https://gateway-api.sigs.k8s.io/) — Kubernetes networking
- [NATS JetStream](https://docs.nats.io/nats-concepts/jetstream) — durable event log + KV
- Existing types: `SyncPlan`, `SyncRun`, `Deployment`, `DeploymentTarget`, `Promotion`
- CLAUDE.md engineering guardrails (idempotency, Git-as-source-of-truth, explainability)
- Engine-boundaries plan §2.5 (`gitops-engine` adoption), §5.5 (storage tiers), §5.9 (Gateway API)

---

**Next steps after review of this plan**

- ~~Agree on high-level direction (hub + agents as the target model).~~ — landed as [ADR 0005](../adr/0005-distributed-runtime.md); §13 questions 1–9, 11–19 are resolved there. Question 10 (`gitops-engine` adoption) belongs to ADR 0006.
- Produce a follow-on tactical plan for the MVP 1 agent read-path proof-of-concept.
- Begin defining the SyncRun execution interface.

This document will be updated as discussion progresses. Significant decisions will be captured in `docs/adr/`.
