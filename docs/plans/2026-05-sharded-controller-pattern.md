<!--
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
-->

# Sharded Controller Pattern

- **Status:** Draft — 2026-05-17 (Decision)
- **Linear:** SKA-328. Blocks SKA-349 (MVP 1 sharded controller foundation — predicate filter ready).
- **Promotes into:** a future ADR co-located with ADR 0005. Until then, authoritative for any code that wires a sharded controller.
- **Resolves:** [`docs/plans/2026-05-distributed-runtime-architecture.md`](./2026-05-distributed-runtime-architecture.md) §13 Q14 (sharded vs. leader-elected controllers, which library / pattern, when to commit).
- **Related:** [ADR 0005](../adr/0005-distributed-runtime.md) §11.5 (the sharding model table this plan implements), [ADR 0006](../adr/0006-engine-boundaries.md) §4 (containment rule — we are *not* vendoring argo-cd's sharder code), [SKA-324 JetStream Layout](./2026-05-jetstream-subject-and-stream-layout.md) §4 (the same xxhash64 partition function we reuse for controller sharding), [SKA-324](./2026-05-jetstream-subject-and-stream-layout.md) §6 (`controller-locks` NATS KV bucket — the leader-election primitive this plan consumes).

## 1. Purpose and Scope

ADR 0005 §11.5 named sharded controllers as a day-one requirement
and pointed at "the shape Argo CD's ApplicationSet uses." This plan
picks the concrete pattern, the coordination primitive, and the
controllers that will use them — answering distributed runtime plan
§13 Q14 in one shot.

**In scope:**

- The sharder pattern (custom predicate filter, not vendored
  argo-cd code).
- The coordination primitive (NATS KV `controller-locks` bucket
  from SKA-324 §6).
- The shard list (which controllers shard, which stay
  single-leader).
- The co-shard policy for derived resources (SyncPlan / SyncRun /
  Deployment hash on the parent Application name).
- The reference Go implementation sketch.
- The operational story: adding/removing replicas, partition-count
  growth, leader failover behavior.
- Per-MVP timeline.

**Out of scope:**

- The JetStream subject-side mechanics (already in SKA-324).
- Per-shard publish-rate monitoring (SKA-324 §13 Q1 open question).
- Multi-region controller fleets (treated as one logical fleet per
  SKA-324 §10; geographic placement is operator concern).

## 2. Decision (Short Form)

1. **Pattern:** custom controller-runtime predicate filter; each
   replica owns a hash slice. ~150 LOC; lives in
   `internal/sharder/`.
2. **Coordination:** NATS KV `controller-locks` bucket (per SKA-324
   §6). Replicas race to claim shard IDs; 30 s lease,
   heartbeat-extended. No ConfigMap-based static map; no
   leader-elected central assigner.
3. **Sharded controllers (4):** `Application`, `Source`, `SyncPlan`,
   render worker pool. Everything else (`Promotion`, `Approval`,
   `HealthCheck`, `FreezeWindow`, `Deployment` reconciler,
   `Cell`/`Environment`/`DeploymentTarget`) stays single-leader at
   MVP 1–2 scale.
4. **Co-shard policy:** `SyncPlan`, `SyncRun`, and `Deployment`
   compute their shard ID from the **parent `Application`'s name**,
   not from their own name. Per-Application locality preserved.
5. **Partition-count growth:** two-fleet transition window
   (matching SKA-324 §4.3 producer/consumer pattern). New fleet at
   the new partition count is rolled in, old fleet drains, no
   single-pod dual-ownership.
6. **MVP timeline:** MVP 1 ships the predicate + NATS KV claim
   logic with `partitionCount=1` (sharded foundation, single shard
   active). MVP 2 bumps to `partitionCount=16` via the two-fleet
   transition. MVP 3 to 32 or 64 per benchmark.

## 3. Context

### 3.1 Why not single-leader controller-runtime?

The controller-runtime leader-election story is excellent up to
~1 K objects per type per controller pod (informed by Argo CD's
historical scale and our own benchmarks against synthetic load).
Above that, three pressures compound:

1. **Watch fan-out.** Every replica's informer holds every object;
   memory and steady-state CPU both grow linearly.
2. **Workqueue depth.** One leader, one workqueue, one drain
   rate — the rate-limit at the bottom is the cluster throughput.
3. **Recovery time.** Pod restart re-syncs every object from the
   apiserver; at 2.5 K Applications this is a multi-second pause
   on every leader change.

Projected MVP 2 cardinality is 2 500 Applications, 2 500 Sources,
200 DeploymentTargets (ADR 0005 §11.5 table). Single-leader works
at 1 K but breaks at 2.5 K. Foundation in MVP 1, on by MVP 2.

### 3.2 Why not vendor argo-cd's sharder code?

Argo CD's ApplicationSet sharder is battle-tested and approximately
the right shape. We do **not** vendor it because:

- ADR 0006 §4 (containment rule) restricts argo-cd imports to
  `gitops-engine` packages inside four `internal/` directories.
  The sharder lives in argo-cd's `applicationset` controller, not
  in `gitops-engine` — importing it from `internal/sharder/` would
  open the containment rule.
- The actual logic is ~150 LOC. Vendoring 2 K LOC of supporting
  code to access 150 LOC is the wrong cost shape.
- Our coordination primitive (NATS KV) is different from argo-cd's
  (ConfigMap-backed shard map). Adapting argo-cd's coordinator to
  NATS KV is bigger than rewriting from scratch.

We use argo-cd's *shape* — predicate filter on hash slice, one
replica per shard — without taking its code.

### 3.3 Why NATS KV `controller-locks`?

SKA-324 §6 already commits us to a `controller-locks` bucket for
per-shard leader election. Reusing it for the sharded-controller
claim is a free reuse:

- The bucket already exists in the operational picture.
- The bucket's `<controllerName>/<shardId>` key shape, 30 s TTL,
  and heartbeat-extension semantics are exactly what shard claim
  needs.
- No new dependencies on Lease-based coordination, no ConfigMap
  drift between replicas and operator config.

A ConfigMap-based shard map (the original ApplicationSet pattern)
was considered and rejected: it requires operator action to add or
remove replicas, and a stale map after a scale event is a
silent-failure mode. NATS KV claim is dynamic, the bucket TTL
handles the operator-forgot-to-update case automatically.

## 4. The Predicate

### 4.1 The filter

Each controller's `Watches(...)` builder takes a predicate that
returns true only when the resource belongs to this pod's owned
shard slice.

```go
// internal/sharder/predicate.go
package sharder

import (
    "sigs.k8s.io/controller-runtime/pkg/event"
    "sigs.k8s.io/controller-runtime/pkg/predicate"
)

// Predicate returns a controller-runtime predicate that admits an
// object only when its shard (derived per ShardKeyFn) is claimed by
// this pod via the Sharder.
func (s *Sharder) Predicate() predicate.Predicate {
    return predicate.NewPredicateFuncs(func(obj client.Object) bool {
        key := s.shardKey(obj)
        return s.ownsShard(key)
    })
}
```

Where `shardKey` is the function that decides which resource
attribute the shard ID derives from — for `Application` it's
`obj.GetName()`; for `SyncPlan`/`SyncRun`/`Deployment` it's
`obj.spec.application` (the parent Application name), per §6.

`ownsShard(key)` returns true when `xxhash64(kind + "/" + key) %
partitionCount` is in the set of shard IDs this pod currently
holds (the §5 claim).

The predicate is the entire mechanism. No central dispatcher, no
shard-router service. Pods that don't own a shard for an event
silently ignore the event; pods that own a shard process it.
Idempotent reconcilers + the bounded re-hash on rebalance (§7)
make brief double-coverage during failover harmless.

### 4.2 The shard-key function (per controller type)

```go
// internal/sharder/keys.go
package sharder

// ApplicationShardKey derives a shard key from an Application.
func ApplicationShardKey(o client.Object) string { return o.GetName() }

// SourceShardKey derives a shard key from a Source.
func SourceShardKey(o client.Object) string { return o.GetName() }

// SyncPlanShardKey derives the shard key from the SyncPlan's
// parent Application name (co-shard per §6).
func SyncPlanShardKey(o client.Object) string {
    sp := o.(*keleustesv1alpha1.SyncPlan)
    return sp.Spec.Application
}

// SyncRunShardKey reads the Spec.PlanRef → SyncPlan → parent
// Application chain via informer indexes (not API calls).
func SyncRunShardKey(o client.Object, idx PlanIndex) string {
    sr := o.(*keleustesv1alpha1.SyncRun)
    plan, ok := idx.Get(sr.Spec.PlanRef.Name)
    if !ok {
        return "" // orphan — handled below
    }
    return plan.Spec.Application
}
```

Orphan handling: when the parent reference cannot be resolved
(SyncPlan not yet visible to the informer; SyncRun spec.planRef
points at a deleted SyncPlan), the predicate returns false on
every pod. The CRD finalizer is responsible for cleanup; orphaned
SyncRuns are not silently dropped — they show up in the per-shard
controller's `OrphanedResources` metric and trip an alert if the
count grows.

## 5. The Claim — Using NATS KV `controller-locks`

### 5.1 Claim protocol

On startup, every controller pod runs:

```go
// internal/sharder/claim.go (sketch)
func (s *Sharder) ClaimLoop(ctx context.Context) error {
    kv := s.kv // *nats.KeyValue, bound to controller-locks bucket
    desired := s.desiredShardCount() // partitionCount

    for {
        s.tryClaimUnowned(ctx, kv, desired)
        s.heartbeatOwned(ctx, kv)

        select {
        case <-ctx.Done():
            s.releaseAll(kv)
            return ctx.Err()
        case <-time.After(s.claimInterval): // default 10s
        }
    }
}

func (s *Sharder) tryClaimUnowned(ctx context.Context, kv nats.KeyValue, n uint64) {
    for shard := uint64(0); shard < n; shard++ {
        if s.owns(shard) {
            continue
        }
        key := fmt.Sprintf("%s/shard-%02x", s.controllerName, shard)
        // CAS-claim: only set if key is absent or TTL expired.
        _, err := kv.Create(key, s.holderToken())
        if err == nil {
            s.markOwned(shard)
        }
        // err is normal — another pod won the race or holds the lease.
    }
}

func (s *Sharder) heartbeatOwned(ctx context.Context, kv nats.KeyValue) {
    for shard := range s.owned {
        key := fmt.Sprintf("%s/shard-%02x", s.controllerName, shard)
        // Update extends TTL; if the entry has been deleted (we lost
        // it during a partition), we fail and re-claim on the next loop.
        _, err := kv.Update(key, s.holderToken(), s.expectedRevision(shard))
        if err != nil {
            s.markLost(shard)
        }
    }
}
```

### 5.2 What `holderToken()` carries

```go
type Holder struct {
    PodName      string    `json:"podName"`
    PodNamespace string    `json:"podNamespace"`
    PodIP        string    `json:"podIp"`
    StartedAt    time.Time `json:"startedAt"`
}
```

Operators can `nats kv view controller-locks` and instantly see
which pod owns which shard. Useful for incident debugging.

### 5.3 Claim invariants

- **A pod never owns more shards than `maxShardsPerPod`** (default
  `ceil(partitionCount / minReplicas)`). When a pod tries to claim
  beyond this cap, it skips; another replica will catch up.
- **A shard not claimed within `unclaimedAlertWindow` (default 60 s)
  triggers a Prometheus alert.** Single-pod-fleet brownouts shouldn't
  hide.
- **`releaseAll()` runs on graceful shutdown.** The lease's natural
  TTL (30 s) covers ungraceful exits.

### 5.4 Predicate observation of claim changes

The predicate (§4.1) reads from a `sync.Map` inside the Sharder
struct; the claim loop updates that map. Controller-runtime's
informer event handlers call the predicate per-event, so claim
changes take effect at the next event — no informer reset, no
re-watch.

A pod that just *lost* a shard may continue processing in-flight
reconciles for that shard's resources for up to one reconcile
period. Since reconcilers are idempotent (CLAUDE.md guardrail),
brief overlap with the new owner is harmless.

## 6. The Co-Shard Policy

### 6.1 The principle

`SyncPlan`, `SyncRun`, and `Deployment` shard on the **parent
`Application` name**, not on their own. The shard ID for an
Application X and every CR that derives from X is the same.

This preserves the SKA-324 §2 "per-resource locality" principle:
every event about X and X's derivatives lands on the same shard's
JetStream subjects and the same controller pod's informer.

### 6.2 Concrete derivations

| Resource     | Shard key                                                  | Why                                                |
|--------------|------------------------------------------------------------|----------------------------------------------------|
| `Application`| `obj.GetName()`                                            | The root.                                          |
| `Source`     | `obj.GetName()`                                            | An independent root; not derived from Application. |
| `SyncPlan`   | `obj.Spec.Application`                                     | Direct parent reference.                           |
| `SyncRun`    | `Lookup(obj.Spec.PlanRef).Spec.Application`                | Via informer index — no API call.                  |
| `Deployment` | `obj.Spec.Application`                                     | Direct parent reference.                           |
| `Promotion`  | n/a (single-leader)                                        | Workqueue-per-Promotion already serializes.        |
| Render work  | `RenderRequest.Application.GetName()`                      | Per SKA-320 §3 — already keys by Application.       |

### 6.3 The skew tradeoff

A pathological Application with 200 `DeploymentTarget`s (200
`SyncPlan`s) piles work on one shard. We accept this:

- **Detect:** the per-shard publish-rate monitor (SKA-324 §13 Q1)
  also tracks per-shard reconcile rate. A shard exceeding 4× the
  median trips an alert.
- **Mitigate:** bump `partitionCount` (the §7 two-fleet transition).
  Doubling the count halves the per-shard load on average — but
  the *skewed* Application stays on one shard, so we don't fix
  this case by partitioning.
- **The real fix is rare.** If a single Application's
  DeploymentTarget count exceeds 200 in practice (it shouldn't —
  that's a topology smell), the operator should split the
  Application into multiple Applications. Split-Application is
  cheap; cross-shard fan-out for one Application is not.

### 6.4 Why not shard each derived resource independently?

Independent sharding (each SyncPlan hashes by its own name) gives
more uniform load. The cost:

- Application X's status change has to fan out across multiple
  shards (every shard owning one of X's SyncPlans).
- The audit trail for "everything Application X did this hour"
  becomes a cross-shard query.
- The render cache locality (SKA-320 §6) is broken — each shard's
  Render worker pool has its own cache view.

The co-shard pattern wins on every locality axis. The load-skew
loss is mitigated by partition-count adjustment and, in the
pathological case, by splitting the offending Application.

## 7. The Shard List

### 7.1 Sharded controllers

| Controller            | Resource                | Shard key (per §6)                              | MVP-1 count | MVP-2 count |
|-----------------------|-------------------------|--------------------------------------------------|-------------:|-------------:|
| `application-controller` | `Application`        | `obj.GetName()`                                  |            1 |           16 |
| `source-controller`      | `Source`             | `obj.GetName()`                                  |            1 |           16 |
| `syncplan-controller`    | `SyncPlan`           | `Spec.Application` (co-shard)                    |            1 |           16 |
| Render worker pool       | `RenderRequest`      | `Application.GetName()` (co-shard with parent)   |            1 |           16 |

`SyncRun` and `Deployment` are reconciled by the `syncplan-controller`
fleet — they share the same predicate via the co-shard rule and ride
on the same shard claim. This is a deliberate choice to reduce the
number of distinct shard claims per pod (one per top-level
controller, not one per CRD type).

### 7.2 Single-leader controllers (not sharded)

| Controller                     | Resource(s)                       | Why not sharded                                                                                   |
|--------------------------------|-----------------------------------|----------------------------------------------------------------------------------------------------|
| `promotion-controller`         | `Promotion`, `Approval`           | Workqueue-per-Promotion already serializes concurrent reconciles. 10–100 concurrent Promotions at MVP 2 scale fits one leader easily. |
| `healthcheck-controller`       | `HealthCheck`                     | Low cardinality; tied to per-Application aggregation already handled by sharded `syncplan-controller`. |
| `freezewindow-controller`      | `FreezeWindow`                    | Low cardinality.                                                                                   |
| `topology-controller`          | `Cell`, `Environment`, `DeploymentTarget` | Low cardinality; changes are operator-driven and infrequent.                                |
| `idp-controller`               | `IdentityProvider`                | Cluster-scoped, very low cardinality.                                                              |
| `rbac-controller`              | `Role`, `RoleBinding`, `Project`, `ApprovalPolicy` | Medium cardinality but bursty; fits one leader at MVP 2 scale.                            |

If any of these hits the 1 K-object pain point at MVP 3 scale, add
sharding by the §4 + §5 protocol; no schema change needed.

### 7.3 Render worker pool

The render pool is sharded but not via the predicate filter pattern
— renders are dispatched through JetStream (`keleustes.events` per
SKA-324 §3), and each worker claims its shard's render queue via
the same NATS KV controller-locks bucket. The `ownsShard()` check
runs on JetStream message pull instead of on a CRD event.

This is the one place the §4 predicate doesn't apply directly. The
underlying claim and rebalance mechanism is identical.

## 8. Operational Story

### 8.1 Initial deploy

```
kubectl scale deployment/application-controller --replicas=16
```

- All 16 pods start.
- Each runs the claim loop (§5.1).
- They race for shard IDs `shard-00` through `shard-0f` in the
  `controller-locks` NATS KV bucket.
- Within ~2 reclaim intervals (≈ 20 s), every shard is claimed.
- Each pod's predicate (§4.1) admits events for its owned shard(s)
  only. Reconciles begin.

### 8.2 Pod restart (rolling deploy or evict)

- Pod loses its lease 30 s after stopping heartbeats (or
  immediately on `SIGTERM` via `releaseAll()`).
- Surviving pods see the shard as unclaimed on their next claim
  loop.
- One pod wins the CAS race; the predicate update flows through
  within the next event.
- The new owner replays in-flight reconciles via its informer;
  idempotency makes this harmless.

### 8.3 Scaling out (15 → 16 replicas)

- New pod starts; runs the claim loop.
- All 16 shards are owned by the existing 15 pods (some pod owns
  ≥ 2 shards under `maxShardsPerPod`).
- The new pod finds no unowned shard to CAS into; it sits in the
  loop with no owned shards. **This is not a deadlock —** when any
  existing pod restarts (or its lease expires), the new pod
  claims.
- To rebalance proactively: roll the existing fleet; the new pod
  picks up shards as the rolling restart frees them.

### 8.4 Scaling in (16 → 8 replicas)

- 8 pods receive `SIGTERM`; their `releaseAll()` drops their
  shards.
- The remaining 8 pods see 8 unclaimed shards on their next loop
  and claim them (subject to `maxShardsPerPod`).
- Within one reclaim interval, every shard is owned again — each
  pod now owns 2 shards.

### 8.5 Partition-count growth (16 → 32)

This is the two-fleet transition (matching SKA-324 §4.3):

1. **Add streams at the new count.** Operator config (`config/nats/
   streams/*.yaml`) gains `keleustes-events-32`; producers continue
   publishing to `-16`.
2. **Deploy a second controller fleet** with
   `--partition-count=32` and a distinct `--controller-name`
   (e.g., `application-controller-v32`). New fleet claims shards
   `shard-00` through `shard-1f` in the `application-controller-v32/...`
   key space — no collision with the old fleet's keys.
3. **Cutover:** roll producers to publish to `-32` subjects.
   Existing in-flight events on `-16` drain.
4. **Decommission the old fleet** (`kubectl scale … --replicas=0`)
   after the old streams' retention expires (7 d for events).
5. **Old streams** are deleted from operator config.

Window size: 7 days (events stream retention). During the window,
both fleets run; pod count doubles. Trivial at this scale.

### 8.6 Single-shard brownout

If the pod holding `shard-0a` is killed AND the JetStream `update`
to release the lease lags, `shard-0a`'s events stack up in the
JetStream durable consumer for up to 30 s (the lease TTL). On
expiry, another pod claims; the consumer cursor is intact;
processing resumes from where it stopped.

Reconcile latency for the affected Applications spikes to 30 s
during this window. Acceptable; the alternative (synchronous
fast-failover) would require quorum coordination on every event,
which we deliberately avoided.

### 8.7 Operator-visible state

```
kubectl get applications -A -o wide                # standard
nats kv view controller-locks                      # which pod owns which shard
nats stream info keleustes-events-16               # consumer lag per shard
keleustesctl sharder status                         # cross-controller summary (UI mirror)
```

`keleustesctl sharder status` is a new MVP 1 CLI verb that prints
per-controller shard ownership, lease ages, and per-shard reconcile
QPS. Avoids requiring operators to learn `nats kv` for routine
introspection.

## 9. Reference Implementation Skeleton

The full plumbing lives in `internal/sharder/`:

```
internal/sharder/
├── sharder.go         # Sharder struct, lifecycle
├── claim.go           # ClaimLoop + tryClaimUnowned + heartbeatOwned
├── predicate.go       # Predicate() returning controller-runtime predicate.Predicate
├── keys.go            # Per-resource ShardKeyFn implementations
├── metrics.go         # Prometheus metrics: owned_shards, claim_failures, predicate_admit_total
├── status.go          # status renderer for keleustesctl sharder status
└── sharder_test.go    # unit tests + table-driven predicate tests
```

Constructor pattern (used by every sharded controller's
`SetupWithManager`):

```go
sharder, err := sharder.New(sharder.Options{
    ControllerName:  "application-controller",
    PartitionCount:  cfg.Sharder.PartitionCount, // 1 in MVP 1, 16 in MVP 2
    KV:              natsClient.KeyValue("controller-locks"),
    ShardKey:        sharder.ApplicationShardKey,
    PodName:         os.Getenv("POD_NAME"),
    PodNamespace:    os.Getenv("POD_NAMESPACE"),
    PodIP:           os.Getenv("POD_IP"),
    HeartbeatPeriod: 10 * time.Second,
    LeaseTTL:        30 * time.Second,
    MaxShardsPerPod: 0, // 0 = ceil(partitionCount/minReplicas) default
})
if err != nil { return err }
go sharder.ClaimLoop(ctx)

return ctrl.NewControllerManagedBy(mgr).
    For(&keleustesv1alpha1.Application{}).
    WithEventFilter(sharder.Predicate()).
    Complete(r)
```

That's the entire integration surface for a sharded controller:
construct, start the claim loop, set the predicate. ~5 LOC at the
caller; ~150 LOC inside `internal/sharder/`.

## 10. Per-MVP Timeline

| MVP | What ships                                                                                  | Notes                                                                                                   |
|-----|---------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------|
| 0   | Nothing — single-leader controller-runtime is fine at MVP 0 scale.                          | The `internal/sharder/` package is not yet imported.                                                     |
| **1** | `internal/sharder/` package ready; all four target controllers wired through the predicate; `partitionCount=1` in production config (single shard active). | MVP 1 exit criterion: "Sharded controller foundation (predicate filter ready, even if running single-shard)." |
| 2   | `partitionCount=16` via the §8.5 two-fleet transition. Per-shard publish-rate monitor (SKA-324 §13 Q1) live; `keleustesctl sharder status` CLI verb shipped. | First MVP requiring multi-shard in production.                                                            |
| 3   | `partitionCount=32` or `=64` per benchmark; additional controllers (e.g., RBAC) shard if the 1 K-per-controller line is crossed.                          | Multi-region adds geographic placement (handled by NATS supercluster per SKA-324 §10).                    |
| 4   | No changes expected from sharding standpoint; per-tenant IdP isolation may add per-tenant shard sets.                                                       | If isolation forces per-tenant shard claims, the §5 bucket layout already accommodates via the key prefix. |

## 11. Failure Modes

| Failure                                                          | Behavior                                                                                                                |
|------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------|
| Single controller pod loss                                       | Lease expires in ≤ 30 s; surviving pod claims; predicate flips; reconciles resume. ≤ 30 s brownout for affected shard.   |
| Two pods simultaneously believe they own shard X                 | Brief (≤ one reconcile period). Idempotent reconciles ensure no harm — the worst case is duplicate SSA applies (no-op). |
| NATS KV bucket lost                                              | All pods lose their shard ownership; predicate admits nothing; reconciles pause. Alerts trigger. Recovery: rebuild bucket from JetStream replay (per SKA-324 §6 — KV is JetStream-backed). |
| Partition-count config drift (one pod has 16, another has 32)    | Pods using count 32 try to claim `shard-10..1f` in `controller-locks` for the same controller name — keys not in use, claims succeed but no producer writes those subjects. No traffic; no harm. Confidence check: alert if shard `shard-XY` has been claimed for ≥ 1 h with zero processed events. |
| Sharder predicate library bug admits 0% of events                | Reconcile QPS goes to zero; `OrphanedResources` metric grows; alert. Roll back deployment.                                |
| All controller pods of a single shard down simultaneously        | That shard's events queue in JetStream; consumer cursor preserved. When pod recovers (manual `kubectl scale` or HPA), processing resumes from cursor. Acceptable degradation; not data loss. |
| Pathological Application skew (200 SyncPlans on one shard)       | Per-shard reconcile-rate alert fires; partition-count bump does not help (skew stays on one shard); operator splits the offending Application into N smaller Applications. See §6.3. |

## 12. Open Questions

1. **`maxShardsPerPod` calculation when pod count drops below
   minReplicas.** Today: ceiling against `minReplicas`. If the
   fleet is scaled below `minReplicas` (incident-time intervention),
   some shards have no candidate owners until pods restart. Default
   `minReplicas` for sharded controllers should match
   `partitionCount` to keep this edge case rare; confirm in the
   chart's `values.yaml` at MVP 1 implementation.

2. **Cross-controller shard alignment.** All four sharded
   controllers (`application`, `source`, `syncplan`, render-pool)
   use the same `partitionCount`. Should we encourage
   `application-controller` pod K and `syncplan-controller` pod K
   to be the same Kubernetes node (anti-affinity to spread, or
   *pro*-affinity to co-locate for cache locality)? Defer until
   real benchmark numbers exist; default to anti-affinity (spread)
   for HA reasons.

3. **Predicate evaluation cost per event.** xxhash64 over a short
   string is single-digit nanoseconds. At 10 K events/sec across
   all shards, the predicate cost is negligible. Validate at MVP 2
   benchmark; consider caching the shard ID on the object's
   `cache.SharedIndexInformer` key if hot-path measurements demand
   it.

4. **Static-vs-dynamic-shard-count discovery.** Today the partition
   count is operator config (passed as a flag / env var). A future
   refinement: pods discover the count from a ConfigMap watched
   by all replicas, eliminating the two-fleet transition need by
   propagating count changes via watch. The cost is a new
   ConfigMap to keep correct. Not adopted yet — two-fleet
   transitions are infrequent and the static-config approach is
   easier to reason about.

5. **HPA on sharded controllers.** HPA on CPU/memory is fine for
   per-pod scaling within a partition. But scaling the *fleet*
   (adding pods beyond `partitionCount`) gives pods with no shard
   to own — they idle (per §8.3). Default the chart's HPA max to
   `partitionCount` and document the rule. Open question: should
   the chart enforce this via a webhook?

## 13. Compliance with Prior Decisions

| Decision                                          | This plan honors it by                                                                                                          |
|---------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------|
| ADR 0005 §11.5 (sharded controllers required)     | Four controllers shard at MVP 2; the others stay single-leader by the same reasoning (workqueue-per-resource serializes).        |
| ADR 0006 §4 (containment rule)                    | `internal/sharder/` imports zero argo-cd packages. We borrow the *shape* (predicate filter on hash slice) not the *code*.        |
| SKA-324 §4 (xxhash64 partition function)          | Reused verbatim: `xxhash64(kind + "/" + key) % partitionCount`. The same hash that decides JetStream partitions decides controller shards. |
| SKA-324 §6 (`controller-locks` NATS KV bucket)    | This plan is the primary consumer of that bucket; the bucket's key shape was designed for this exact purpose.                    |
| SKA-324 §4.3 (partition-growth protocol)          | The §8.5 two-fleet transition is the controller-side mirror of SKA-324's producer/consumer-side protocol.                        |
| SKA-322 §6 (`actor.type=system` for controllers)  | Sharded controllers emit audit events with `actor.subject=keleustes-application-controller` etc., matching SKA-322's §6.3 table. |
| CLAUDE.md (reconcilers must be idempotent)        | The brief double-coverage during failover (§8.2) is harmless precisely because of this guardrail.                                |
| Distributed runtime plan §13 Q14 (the open Q)     | Resolved end-to-end: pattern (custom predicate), library (none — internal), coordination (NATS KV), timing (MVP 1 foundation, MVP 2 active). |

## 14. Concrete Follow-ups

1. **SKA-349 (MVP 1 sharded controller foundation)** —
   implements `internal/sharder/` end-to-end and wires the four
   sharded controllers through it with `partitionCount=1`.
2. **New ticket: MVP 2 `partitionCount=16` cutover plan** — the
   two-fleet transition runbook (§8.5) executed for real on MVP 2
   deploy.
3. **New ticket: per-shard publish-rate monitor** (SKA-324 §13 Q1
   + this plan §6.3 + §12.2). Single ticket covers both
   per-shard JetStream rate and per-shard reconcile rate.
4. **New ticket: `keleustesctl sharder status`** subcommand. MVP
   1 — ships alongside SKA-349.
5. **New ticket: Helm chart wiring** — chart values for
   `partitionCount`, `minReplicas`, `maxReplicas`,
   `claimInterval`, `leaseTTL`. HPA template caps `maxReplicas` at
   `partitionCount` per §12.5.
6. **Update `docs/DECISIONS.md`** — add this plan to the active
   interim contracts table (handled in the same commit as this
   plan).

---

**When this plan stabilizes** (after SKA-349 lands and the MVP 2
two-fleet transition has been exercised at least once on real
load), §1–§11 promote into a new ADR co-located with ADR 0005 —
likely ADR 0011 (Render → 0007, Audit → 0008, RBAC shapes → 0009,
JetStream layout → 0010, Sharded controllers → 0011). §12 open
questions remain in this plan until resolved.
