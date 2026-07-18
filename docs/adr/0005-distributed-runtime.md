<!--
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
-->

# ADR 0005 — Distributed runtime: hub + regional agents, NATS JetStream as canonical bus, no RDBMS on the critical path

- **Status:** Accepted
- **Date:** 2026-05-17
- **Deciders:** Platform Architecture (Skaphos)
- **Linear:** SKA-410
- **Related:** ADR 0002 (Observability), ADR 0003 (Git invariant), ADR 0004 (RBAC), ADR 0006 (Engine boundaries)
- **Supersedes:** `docs/plans/2026-05-distributed-runtime-architecture.md` §13 (open questions)

## Context

Keleustes is built for platform teams running 2,500+ Applications today
and 10,000+ in the near term (PROPOSAL §19). A single central controller —
even with caching — does not survive that cardinality, cannot deliver
regional DR, and cannot keep working when its hub is unreachable.

`docs/plans/2026-05-distributed-runtime-architecture.md` proposes a
**hub + regional agents** runtime topology with NATS JetStream as the
canonical event/state bus and an explicit no-RDBMS-on-the-critical-path
storage model. The plan's §13 lists 19 open questions; this ADR resolves
the ones that bear on storage, transport, scaling, and exposure.

Decisions made here are deliberately *upstream* of the first agent
implementation: changing them after MVP 2 ships an agent is materially
more expensive than committing now.

## Decision

### 1. Hub + regional-agent topology

```
                   +----------- Hub region -----------+
                   |   API server / engines           |
                   |   NATS cluster + JetStream       |
                   |   Object storage (durable archive)|
                   +-----------+----------------------+
                               ^
              outbound TLS     |  (no inbound listener on agent)
              NKey + JWT       |
              +----------------+----------------+
              |                |                |
        +-----v------+   +-----v------+   +-----v------+
        | Region A   |   | Region B   |   | On-prem    |
        | Agent      |   | Agent      |   | Agent      |
        +-----+------+   +-----+------+   +-----+------+
              |                |                |
         target clusters  target clusters  target clusters
```

- The **hub** holds the API server, controllers, the canonical JetStream
  stream, and the durable archive. It owns desired state, Promotion
  orchestration, audit emission, and Git mutation.
- **Regional agents** are deployed per region, near target clusters. They
  claim `SyncRun`s for the `DeploymentTarget`s they own, execute render +
  apply, and report `Deployment` and `HealthCheck` state back.
- Agents are **first-class Kubernetes Deployments**, not sidecars. This
  locks in plan §13 question 1: out-of-cluster (i.e., not co-located in
  the workload pod), deployed in or near target clusters. The agent
  binary may run inside the target cluster or in a regional management
  cluster; it is not a sidecar of any workload.
- Each agent **claims work** for explicit `DeploymentTarget` ownership;
  the hub never pushes work to an agent that hasn't claimed the target.
- **No regional hub** in MVP 0–3 (plan §13 question 4). Pure agent +
  central hub is sufficient at the 10K-Application target; the
  complexity of a per-region hub is reserved for a later ADR if
  geographic latency or sovereignty requirements demand it.

### 2. NATS JetStream is the canonical event/state bus

JetStream is the single durable event log for the control plane:

- Source revisions, SyncRun phase transitions, Promotion phase changes,
  approval events, agent events, audit envelopes — all flow through
  JetStream.
- Subjects are partitioned by Application-hash prefix (plan §13 question
  18). One stream per Application is too many; one stream for everything
  is a bottleneck; hash-prefix partitioning gives bounded fan-out per
  consumer group.
- Replication factor R ≥ 3 in production. Multi-region: a NATS
  supercluster spans regions; agents always reach the nearest member.
- **Retention:** 30 days hot in JetStream, rolling segments to object
  storage thereafter (plan §13 question 15). Tunable per stream; the
  default targets typical audit-window needs.

### 3. No RDBMS on the critical path

**Hard constraint:** Keleustes must be restorable from zero without
restoring a backup file. Any storage layer whose loss requires a
`pg_dump`-style restore is **disqualified** from the critical path.

State that cannot be reconstructed from CRDs + Git + JetStream does not
exist. The storage tiers:

| Tier                              | Technology                            | Recovery                                              |
|-----------------------------------|---------------------------------------|--------------------------------------------------------|
| Authoritative active state        | CRDs (etcd)                          | Re-applied from the Git config repo                    |
| Authoritative history             | User's Git repos                     | The user's Git provider is the system of record        |
| Event / audit log                 | NATS JetStream durable streams       | R≥3 + cold archive in object storage; replay restores  |
| Hot indexes / presence            | NATS KV (on JetStream)               | Rebuildable from JetStream replay or live agent reports|
| Large content-addressed artifacts | Object storage (S3/GCS/Azure/MinIO)  | Provider-native replication; rendered manifests are pure cache |
| Derived analytics / UI matrix     | DuckDB over parquet on object storage| Periodically rebuilt from JetStream replay             |

**Explicitly disqualified from the critical path:** PostgreSQL, MariaDB,
MySQL, Oracle, SQL Server, or any other RDBMS as source-of-truth. A
customer may run a derived SQL consumer side-car against the JetStream
audit stream for BI; Keleustes core does not depend on it.

A **reference SQL consumer** ships in `contrib/` (plan §13 question 19)
so customers don't reinvent it badly. It is out-of-tree, unowned by core,
and not a runtime dependency.

### 4. Acceptable historical loss

The recovery contract is explicit: in the catastrophic case where
JetStream **and** its object-storage archive are both lost, **audit history
that is not still resident in a CRD's `.status` is gone.** Live state
reconstructs from CRDs + live agent reports in minutes; the historical
audit beyond CRD-status reach is not recoverable.

This is plan §13 question 17, locked in. It is an explicit, documented
bound — not a surprise — and is consistent with the "no backups" stance.

### 5. Cross-connect transport: NATS leaf default, interface from day one

The default transport between agents and the hub is **NATS leaf nodes**
dialing the hub's NATS cluster outbound over TLS:

- Outbound-only from the agent (no listener on the agent side).
- NKey + signed JWT authentication; subject and stream permissions scoped
  to the agent's allowed `DeploymentTarget` set.
- JetStream provides durable, replayable streams for work assignments and
  results.
- WebSocket-over-TLS on port 443 is supported for forward-proxy
  environments.

Plan §13 question 5 (event bus tech) and §13 question 8 (transport
pluggability timing) are locked together:

- **NATS leaf is the only transport Keleustes ships and supports in
  MVP 1–3.**
- **The agent-side code talks to a Go interface** (`internal/agent/transport.Transport`)
  from day one, so that alternate transports (Tailscale, Teleport,
  Cloudflare Tunnel, gRPC bidi-stream, HTTP/2 long-poll) are an extension
  point rather than a refactor.
- Customers wiring an overlay terminate connectivity on a private hub
  endpoint; the agent dials a local address. Keleustes does not depend on
  the overlay.

### 6. Agent identity is a first-class `Agent` CR

A new CRD `Agent` in `keleustes.skaphos.io/v1alpha1` carries:

- Agent name and region.
- Transport configuration (which transport, endpoint, auth material
  references).
- The set of `DeploymentTarget`s this agent is allowed to act on.
- Status: presence (heartbeat), claimed targets, last-seen, version.

`DeploymentTarget.spec.agentRef` (or affinity selectors) names the agent
that owns the target. The hub will not assign a `SyncRun` to an agent
that does not own the target.

This locks in plan §13 question 9: an `Agent` CR, not annotations on
`DeploymentTarget`. The cost is one more CRD; the benefit is that agent
identity, transport, and scope are reviewable in Git and observable like
every other Keleustes resource.

### 7. Tiered exposure via Gateway API v1

Gateway API v1 (`gateway.networking.k8s.io/v1`) is the only supported
networking surface. **No `Ingress`** — not even as a fallback.

Different parts of the hub get separate `Gateway`s to enforce blast-
radius and auth-model separation:

| Tier                              | Components                             | Auth model                                              | Fronting                                                          |
|-----------------------------------|----------------------------------------|----------------------------------------------------------|--------------------------------------------------------------------|
| Public internet                   | Webhook receivers (GitHub/GitLab/ADO/OCI) | Per-request provider HMAC; no OIDC                       | Bare public `Gateway` with TLS; rate limiting                      |
| Internal users (UI)               | UI assets + UI-bound API endpoints    | Customer IAP (Google IAP, AAD App Proxy, Cloudflare Access, Pomerium, oauth2-proxy) | IAP-fronted Gateway; backend trusts the listener-configured header |
| Internal / API                    | REST/gRPC for `keleustesctl`, CI     | OIDC (humans), workload identity / mTLS (CI)            | Same IAP gateway with a separate listener, or a dedicated internal Gateway |
| Agent transport                   | NATS leaf endpoint                    | NKey + JWT terminated by NATS itself                     | **Standalone `Service type=LoadBalancer`** in v1alpha1; revisit `TLSRoute` once a customer's controller supports it cleanly |
| Cluster-internal observability    | `/metrics`, profiling                 | NetworkPolicy + cluster-internal only                    | No Gateway — `ClusterIP` + NetworkPolicy                           |

Locked-in choices:

- **Agent NATS endpoint uses a standalone `LoadBalancer` Service**
  (plan §13 question 12). `TLSRoute` passthrough is the cleaner long-
  term shape but depends on Gateway controllers implementing it
  consistently, which is not yet the case. Standalone LB ships now; we
  migrate per-controller as `TLSRoute` matures.
- **First-class Gateway controllers** are **Envoy Gateway** and **GKE
  Gateway** (plan §13 question 13) — sample overlays, smoke tests in CI.
  Contour, Istio, AKS Application Gateway, Cilium Gateway are
  documented-but-untested. Customers run what they already run.
- **IAP integration recipes** ship for Google IAP, AAD App Proxy,
  Cloudflare Access, Pomerium, oauth2-proxy. Keleustes does not ship an
  IAP itself.

### 8. OIDC / authz service

The API server integrates **directly with the customer's OIDC provider**
via the `IdentityProvider` CRD (ADR 0004 §5). Keleustes does not ship Dex
as a required component (plan §13 question 6) — customers who want it
deploy it themselves. The reference architecture covers Okta, Entra ID,
Google Workspace, GitLab, Keycloak, and Dex equally.

### 9. Webhook receiver deployment shape

**MVP 0/1:** single binary with separate listeners for API + webhook
receivers (different ports, same process). Simpler to deploy, fewer
moving parts before public exposure goes live.

**MVP 2:** webhook receivers split into a separate `Deployment` when
public exposure goes live. Independent scaling for spiky webhook traffic;
independent deploy cadence; smaller blast radius for a webhook-flood DoS.

This locks in plan §13 question 11.

### 10. Horizontal scaling and sharding

Single-replica components and single-shard reconcile loops are
**disallowed in MVP 1**. The only acceptable scale-up axis is more pods,
more shards, more agents — never bigger pods.

| Component                             | Sharding key                                | Notes                                                                               |
|---------------------------------------|---------------------------------------------|-------------------------------------------------------------------------------------|
| Webhook receivers                     | None (stateless)                            | HPA on QPS/CPU                                                                       |
| Source watchers                       | `Source` name → consistent hash             | JetStream consumer group per shard; rate limits applied per shard                    |
| Render workers                        | None (pool); content-hash for cache         | Pool consumes from JetStream render queue; results in object storage by content hash |
| `Application` / `SyncPlan` controllers| `Application` name → hash slice             | **Sharded by MVP 2.** Single-leader is acceptable to ~1K; sharded required above.    |
| `Source` controller                   | `Source` name → hash slice                  | Sharded earlier — larger surface.                                                     |
| `Promotion` controller                | `Promotion` name                            | Workqueue-keyed; one worker per Promotion at a time, many Promotions concurrent.    |
| Sync execution                        | `DeploymentTarget` ownership (which agent)  | MVP 1: central. MVP 2+: per-target agent claim.                                       |
| Cluster cache                         | One per `DeploymentTarget`                  | Lives on the owning agent. Never aggregated on the hub.                              |
| API server (read)                     | None (stateless)                            | Hot snapshots from NATS KV; matrix queries from DuckDB-on-parquet.                   |
| NATS / JetStream                      | Subject partitioning + cluster              | R ≥ 3 in production.                                                                  |

Plan §13 question 14 is locked: **commit to sharded controllers at MVP 2
for `Application` and `Source`.** The sharder library/pattern is chosen
in MVP 1 implementation (initial preference: a controller-runtime
predicate-filter shard, modeled on Argo CD's ApplicationSet pattern).

**Hot loops we will not write:** N+1 reconciles over `Source` or
`Application` on a hot path; synchronous render inside a controller's
`Reconcile`; cluster-cache aggregation across all targets on the hub;
UI fan-out queries that scan all Promotions or all SyncRuns; per-repo Git
polling in steady state.

**Benchmark gates** are required at each MVP exit (MVP 1: 1K Apps; MVP 2:
2.5K Apps; MVP 3: 10K Apps). Demonstrated, not assumed.

### 11. Render and Git mutation distribution

- **Rendering** stays **central in MVP 1**, with delegation to agents
  becoming an opt-in flag in MVP 2 (plan §13 question 2). The render
  cache (content-addressed in object storage) means central-vs-agent
  rendering produces byte-identical artifacts.
- **Git mutation** is **hub-preferred and hub-default** (plan §13
  question 7). Regional agents may perform limited Git mutation when
  explicitly authorized per `PromotionPolicy` or `DeploymentTarget` — the
  use case is regional emergency rollback when the hub is unreachable.
  Most installations will never enable it.

### 12. Local autonomy for emergency operations

Agents are configurable for **limited local autonomy** when the hub is
partitioned:

- They continue to execute already-approved `SyncRun`s.
- They emit `HealthCheck` and `Deployment` records to local NATS KV
  snapshots.
- They may, with explicit per-`PromotionPolicy` or per-`FreezeWindow`
  configuration, perform pre-approved rollbacks or emergency syncs.

**No emergency promotions originate at the agent in MVP 1–2.** Agents
catch up with the hub on reconnection; the hub remains the authority for
new Promotion decisions. Expanding autonomy is gated on operational
experience (plan §13 question 3).

### 13. DuckDB freshness model

Plan §13 question 16: live state served from JetStream + NATS KV for the
**last 4 hours**, DuckDB-on-parquet for older windows. Parquet shards are
rebuilt on a rolling cadence (default 5-minute increments per
Application-hash prefix). The UI's matrix queries automatically select
the right layer based on the queried time window.

This is the third option from the plan ("live + parquet hybrid"), chosen
because it's the only one that meets both freshness and cost targets at
10K Applications.

### 14. Reference SQL consumer

A reference Postgres consumer that reads the JetStream audit stream and
exposes a SQL surface for BI / external integrations ships under
`contrib/` (plan §13 question 19). It is **owned by the customer**, not
the core, and is explicitly not on the critical path. Its existence
makes "I need SQL for BI" a documented escape valve rather than a
pressure to add an RDBMS upstream.

## Consequences

**Positive**

- Regional DR is real: agents continue to execute already-approved work
  during hub partition.
- 10K-Application scale is achievable through more pods / more shards /
  more agents — no bigger-pods scaling.
- Restorability from zero is a one-page runbook: redeploy operator, apply
  config repo, wait. No `pg_dump`-restore step exists.
- NATS leaf-node transport is enterprise-friendly: outbound-only, NAT-
  traversing, identity-bound. Works through forward proxies on 443.
- `Agent` CR keeps agent identity, scope, and transport reviewable in
  Git, consistent with ADR 0003.
- Tiered Gateway API exposure prevents a webhook-flood DoS from taking
  down the UI.

**Negative / accepted costs**

- One catastrophic-loss class is documented as unrecoverable
  (JetStream + archive both gone, historical audit beyond CRD `.status`
  is lost). This is explicit and bounded.
- NATS is a hard dependency in production. Customers who reject NATS get
  the documented gRPC-bidi transport option in a later MVP via the
  pluggable transport interface, but lose the durable-stream story unless
  they bring an equivalent.
- DuckDB-on-parquet replaces SQL ergonomics for analytics. The reference
  SQL consumer in `contrib/` covers customers who need SQL semantics for
  BI.
- Sharded controllers add complexity at MVP 2; tested via the benchmark
  harness gating MVP 2 exit.
- The `Agent` CR is one more resource for platform teams to learn. The
  schema is small and the lifecycle is mechanical (register, heartbeat,
  claim, expire).

## Alternatives considered

- **Central-only with regional caching.** Argo CD's historical shape.
  Rejected: cannot deliver regional DR, single-region scaling ceiling
  hits at the 2.5K-Application target.
- **Active-active multi-master control plane.** Significantly more
  complex. Rejected for v1alpha1 — not required to meet the DR and scale
  goals.
- **Per-region hubs (federation between hubs).** Stronger sovereignty
  story. Rejected for MVP 0–3 — pure hub + agents covers the use cases;
  per-region hubs are reserved for a future ADR if geographic
  requirements emerge.
- **PostgreSQL as durable state.** Disqualified by the no-backups
  constraint and the recovery-from-zero requirement.
- **Kafka instead of NATS JetStream.** Kafka is more familiar to many
  ops teams but has a heavier operational footprint, no native leaf-node
  story for agents, and a separate KV story is required. JetStream covers
  the bus, KV, and agent-transport needs in one binary.
- **`DeploymentTarget` annotations for agent identity** instead of an
  `Agent` CR. Smaller surface but conflates transport, identity, and
  target ownership on a resource whose primary concern is the workload
  target — eventually painful.
- **`TLSRoute` passthrough as default** for the agent endpoint. Cleaner
  long-term; depends on Gateway controllers implementing it consistently,
  which isn't yet the case across the first-class controllers. Migrate
  per-controller as support matures.

## Compliance and follow-ups

- Plan §13 questions 1, 2, 3, 4, 5, 6, 7, 8, 9, 11, 12, 13, 14, 15, 16,
  17, 18, 19 are resolved here. Question 10 (`gitops-engine` adoption)
  belongs to ADR 0006.
- The `Agent` CRD scaffold (description-only) belongs in MVP 0; the
  reconciler arrives in MVP 1 alongside the first agent read-path
  proof-of-concept.
- The sharded-controller pattern selection is an MVP 1 deliverable,
  gating MVP 2 exit.
- The benchmark harness (plan §13 question 13 cross-reference) is
  funded engineering work; without it, MVP exits cannot be gated on
  cardinality.
- The reference SQL consumer in `contrib/` is a separate ticket; not
  required for MVP 0–2.
- This ADR will be revisited if a per-region hub becomes necessary, if a
  customer pushes for a non-NATS transport as first-class, or if the
  cardinality benchmarks at MVP 2 / MVP 3 reveal a sharding or
  partitioning shape that doesn't match this ADR.
