<!--
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
-->

# JetStream Subject and Stream Layout

- **Status:** Draft — 2026-05-17
- **Linear:** SKA-324. Blocks SKA-343 (MVP 1 NATS JetStream introduction, hub-internal). Related: SKA-322 (Audit Event Schema — §10 of that plan demands a layout; this plan satisfies it).
- **Promotes into:** a future ADR co-located with ADR 0005. Until then, authoritative for any code that publishes to or consumes from NATS JetStream.
- **Refines:** [`docs/plans/2026-05-distributed-runtime-architecture.md`](./2026-05-distributed-runtime-architecture.md) §13 questions 15 (retention vs. archive cadence) and 18 (subject / stream layout) — both are resolved here.
- **Related:** [ADR 0005](../adr/0005-distributed-runtime.md) §2 (JetStream is the canonical bus; 30-day hot retention; rolling archive to object storage), [ADR 0005](../adr/0005-distributed-runtime.md) §11.5 (sharding model — this plan supplies the *subject*-side of the picture), [ADR 0006](../adr/0006-engine-boundaries.md) §8 (cross-Application dependency events live on JetStream subjects keyed by Application hash prefix), [SKA-322 Audit Schema](./2026-05-audit-event-schema.md) §10 (one canonical `keleustes.audit` stream; archive segments under `<bucket>/audit/segments/<YYYY-MM>/`).

## 1. Purpose and Scope

ADR 0005 §2 made NATS JetStream the canonical event and audit bus.
Engine plan §2.6 and the distributed runtime plan §11.5 reference
"subject partitioning by Application hash prefix" without saying
*how*. Every story that publishes to or subscribes from JetStream
needs a single, frozen subject convention; without it, every
consumer reinvents subject globbing and we end up with the Argo CD
log-shape problem from a different angle.

**In scope:**

- The subject hierarchy (top-level → event class → scope → key).
- The set of canonical streams, their partitioning, retention,
  storage policy.
- The hash function and partition-count grow strategy.
- NATS KV bucket layout for hot indexes (presence, leader locks,
  per-resource audit index, deployment snapshots).
- Object-storage archive layout (where rolled segments land).
- Consumer group conventions (durable names, deliver-by-subject,
  ack policy, replay semantics).
- Cross-shard dependency-event delivery — the engine plan §2.6 +
  ADR 0006 §8 requirement made concrete.
- Producer-side partition-field population (closes SKA-322 §15
  Q1).
- Multi-region supercluster considerations.
- Failure-mode behavior matrix.

**Out of scope:**

- The JetStream **operational** model (operator manifests, R≥3
  quorum, leader election, node sizing) — that lives in the
  ADR 0002 observability layer + the MVP 1 NATS JetStream
  introduction ticket (SKA-343).
- The SIEM exporter (`contrib/`, RBAC plan §11 question 9).
- DuckDB-on-parquet rebuild logic (consumer of this layout, not
  author).
- The Sharded Controller pattern decision (SKA-328) — this plan
  provides the *subject*-side of the picture (per-shard
  filtering); the *controller-runtime* sharder choice is SKA-328.
- Transport-layer specifics for agent ↔ hub (NATS leaf nodes; ADR
  0005 §5).

## 2. Design Principles

Five rules every decision in this plan is measured against:

1. **One subject convention, one wire format.** A consumer that
   reads from JetStream should be able to parse every subject under
   `keleustes.>` with the same grammar. No special-case prefixes,
   no per-engine subject styles.
2. **Partition-by-content, not by-time.** Stream partitioning keys
   off durable identifiers (Application name, Source name) so a
   per-resource consumer can pin to one partition and never lose
   history when partitions rebalance. Time-based partitioning is
   for *archive segments*, not live streams.
3. **No global fan-out paths.** A consumer interested in one
   Application should be able to subscribe with one wildcard token
   and get exactly that Application's traffic. The opposite —
   subscribe to everything and filter client-side — is forbidden by
   convention (ADR 0005 §11.5 "hot loops we must not write").
4. **Replayability is the recovery primitive.** Every consumer
   class must be able to rebuild its state by replaying the
   relevant streams from a known starting cursor. NATS KV is hot
   index; JetStream is the authoritative history; object storage
   is the archive. If a consumer cannot rebuild from those three,
   the design is wrong.
5. **Producers do not pick streams.** Producers publish to a
   subject; the streams that bind those subjects are operator
   config. This keeps the layout addressable from one place
   (`config/nats/streams/*.yaml`) and lets us repartition without
   touching producer code.

## 3. Subject Grammar

```
keleustes.<event-class>.<scope>.<resource-kind>.<key>[.<sub-key>]*
└─ root ┘ └────┬─────┘ └──┬──┘ └─────┬───────┘ └─┬─┘
              |           |           |           └── ULID / name / shard-id depending on event class
              |           |           └────────────── Lowercase Kubernetes Kind (e.g., "application", "syncrun")
              |           └──────────────────────── partition selector (see §4)
              └──────────────────────────────────── one of the registered event classes (§3.2)
```

### 3.1 Tokens

| Token             | Production                                                                                                       |
|-------------------|------------------------------------------------------------------------------------------------------------------|
| `keleustes`       | Literal root namespace. Reserves the entire `keleustes.>` tree on the deployment's NATS account.                  |
| `<event-class>`   | One of `audit`, `events`, `agent`, `dependency`, `webhook`, `render`, `system`. Closed set; new classes need an amendment. |
| `<scope>`         | `cluster` for cluster-scoped resources; otherwise `<shard-id>` (per §4 partition assignment).                       |
| `<resource-kind>` | Lowercase singular Kubernetes Kind: `application`, `source`, `release`, `promotion`, `syncrun`, `deployment`, etc. |
| `<key>`           | The resource's ULID where one exists (per ADR 0004 §12); otherwise its `<namespace>.<name>`.                       |
| `<sub-key>`       | Optional event-specific further segmentation (e.g., per-SyncRun phase, per-target-id).                              |

Examples:

```
keleustes.audit.shard-3a.promotion.01HQ8FQA7Z4M2N1P3K9F8X6Y7B
keleustes.events.shard-1f.application.01HQ7DEC3W5KMRQ9F8X6Y7B22.status
keleustes.dependency.shard-3a.application.01HQ8FQA7Z4M2N1P3K9F8X6Y7B.satisfied
keleustes.agent.cluster.agent.prod-us-east-7.claim
keleustes.webhook.cluster.source.01HQ7DEC3W5KMRQ9F8X6Y7B22.github
keleustes.render.cluster.application.01HQ7DEC3W5KMRQ9F8X6Y7B22.cache-hit
keleustes.system.cluster.election.controller-application
```

### 3.2 Event classes (closed set)

| Class         | Used for                                                                              | Default stream binding (§5) |
|---------------|----------------------------------------------------------------------------------------|------------------------------|
| `audit`       | Every audit envelope per SKA-322. The class is reserved exclusively for SKA-322 events. | `keleustes-audit`            |
| `events`      | Domain events — `SyncRun` phase changes, `Promotion` state machine transitions, `Source` revision observations. Consumed by controllers and the UI live tail. | `keleustes-events-{N}`       |
| `agent`       | Hub ↔ agent transport: work-claim, heartbeats (subscribed by hub), command (subscribed by agents). | `keleustes-agent`            |
| `dependency`  | Cross-Application dependency satisfaction events. Engine plan §2.6.                  | `keleustes-dependency`       |
| `webhook`     | Validated webhook receipts from providers, fanned out from receiver pods.            | `keleustes-webhook`          |
| `render`      | Render cache hit/miss notifications, render-failed events. Companion to SKA-320 §6.4. | `keleustes-render`           |
| `system`      | Hub-internal: leader election transitions, schema migrations, config-changed events. | `keleustes-system`           |

A new event class is an amendment to this plan plus an ADR-author
sign-off, the same protocol as SKA-322 §13 verbs and ADR 0004 §4
RBAC verb alphabet.

### 3.3 Why the scope token is the shard, not the namespace

ADR 0005 §11.5 sharding partitions resources by *resource name*
(Application/Source name), not by Kubernetes namespace. Putting the
namespace in the subject would couple stream traffic to Kubernetes
multi-tenancy in a way Project-based delegation (ADR 0004) makes
unnecessary. The shard ID is the natural partition: consumers know
which shards they own, subscribe to those, ignore the rest.

For cluster-scoped resources (`Cell`, cluster `FreezeWindow`) and
non-resource events (system, agent transport, webhook receipts
before fan-out), the literal `cluster` token replaces the shard ID.

## 4. Partitioning

### 4.1 The hash function

Producers compute a shard ID for partitioned event classes using
**xxhash64** over the resource's canonical identifier, modulo the
current partition count, formatted as `shard-<hex2>` (two-hex-char
suffix gives 256 possible buckets, comfortable headroom).

```go
// internal/events/partition/shard.go
func ShardFor(resourceKind, key string, partitionCount uint64) string {
    h := xxhash.Sum64String(resourceKind + "/" + key)
    return fmt.Sprintf("shard-%02x", h % partitionCount)
}
```

Properties:

- **Deterministic.** Same input → same shard, across language
  runtimes and across hub restarts. xxhash64 is the canonical
  choice for non-cryptographic content addressing inside the NATS
  ecosystem.
- **Stable across partition-count growth (§4.3).** We use the
  *target* `partitionCount` always, not `len(activeShards)`. New
  shards expand the address space; existing keys keep their
  assignment.
- **Uniform.** xxhash64 distribution is uniform enough that a 16-
  shard layout absorbs typical Application skew without hot
  spots. We will validate empirically at MVP 1 benchmark.

The choice of `resourceKind + "/" + key` (not just `key`) prevents
collision between `application/foo` and `source/foo` — they hash
to different shards, so a per-shard consumer of `application`
traffic is not unnecessarily woken by `source` events.

### 4.2 Default partition counts

| Class         | MVP 1 partition count | MVP 2 partition count | Rationale                                                                                |
|---------------|----------------------:|----------------------:|------------------------------------------------------------------------------------------|
| `audit`       | 16                    | 16                    | Audit volume is high but read by few consumers; small partition count keeps fan-out cheap. |
| `events`      | 16                    | 64                    | Drives shard fan-out for `Application` / `Source` controllers — needs headroom at MVP 2.   |
| `agent`       | 1 (unpartitioned)     | 1                     | Agent transport is per-agent; the resource key already partitions naturally.               |
| `dependency`  | 16                    | 64                    | Dependency satisfaction is read by `SyncPlan` controllers — matches `events` partitioning. |
| `webhook`     | 1                     | 1                     | Webhook receivers are stateless; downstream sharded consumers split by Source name.        |
| `render`      | 1                     | 1                     | Render cache events are read by the cache-GC sweeper (SKA-320 §10.3) and the UI live tail. |
| `system`      | 1                     | 1                     | Low-cardinality.                                                                           |

These are the *initial* values. The benchmark gate at each MVP exit
(distributed runtime plan §11.5) re-validates them; any change is an
amendment.

### 4.3 Growing the partition count

Three rules make repartitioning operationally cheap:

1. **The hash always uses the current configured `partitionCount`,
   never `len(activeShards)`.** Doubling the count from 16 → 32
   means hash outputs that previously fell in `shard-00..shard-0f`
   now fall in `shard-00..shard-1f`; every key remains addressable.
2. **New shards are added in *consumer* config before *producer*
   config flips over.** The transition window has both the old and
   new partition count active; consumers spin up the new partitions'
   durables ahead of any producer writing to them. Then a config
   reload (no restart) on producers swaps the count.
3. **`partitionCount` is a tag on the stream name suffix.** Streams
   binding a partitioned class live at `keleustes-events-16` (or
   `-64`); a repartition creates `keleustes-events-32` and
   eventually retires `-16` after the cutover window. Consumers
   bound to a specific suffix never see ambiguous routing.

This pattern is borrowed from Kafka topic partition expansion and
adapted to JetStream's stream model.

### 4.4 Resolution of SKA-322 §15 Q1 — partition value semantics

SKA-322 §5.4 reserved a `partition` envelope field; §15 Q1 asked
whether the value should derive from `subject.ulid` (consistent
per-resource) or `eventId` (uniform distribution).

**Answer: `subject.ulid` when the event has a subject; the literal
`cluster` token otherwise.**

Rationale:

- Per-resource locality wins under realistic consumer patterns. A
  UI tab subscribed to "events for Promotion X" wants every event
  for that Promotion on one partition; a per-shard controller
  watching its slice of Applications wants the same.
- Uniform-distribution-by-eventId would have been right *if* the
  primary consumer were "give me a uniform sample of all events,"
  which is not a real use case here.
- For events with no subject (system events, leader election
  transitions), `partition = "cluster"` routes them through the
  unpartitioned `keleustes-system` stream. The audit envelope's
  `partition` field carries the literal string so consumers can
  trust the field even when `action.subject` is null.

Producers populate `partition` themselves; the value matches the
`<scope>` token in the subject (§3.1). Consumers should treat the
`partition` field as canonical; the subject token is the routing
expression, the envelope field is the data.

### 4.5 Origin of `subject.ulid` — the resource-identity registry

§4.4 keys partitioning on `subject.ulid`, and the `audit-index`
bucket (§6) keys on it too — but the resource ULID has to come from
somewhere stable, or a rename silently fragments a resource's audit
trail. Resource identity is resolved as follows; the decision and its
rationale are recorded in
[ADR 0008](../adr/0008-resource-identity-model.md).

- **Natural key for addressing, durable ULID underneath.** Humans,
  `keleustesctl`, the REST contract, and `kubectl` address resources
  by name. Separately, every audit-subject resource carries a durable
  ULID — the `subject.ulid` every event above depends on.
- **Keyed by source path + target cluster, not by name.** The ULID is
  resolved from `xxhash64(sourcePath + targetCluster)` — the Git
  source coordinates (`spec.source` repo + path) of the resource and
  the cluster it deploys to, i.e. the deployment unit (the matrix
  cell). A rename changes `metadata.name` but not the path, so the
  ULID — and the audit trail keyed on it — survives. (In Kubernetes a
  rename is delete-old + create-new; the `uid` changes, the path-keyed
  ULID does not.)
- **Engine-resolved, never written to Git.** The reconciler computes
  the key and looks the ULID up in the `resource-identity` KV bucket
  (§6); on a miss it mints a ULID, writes it to the bucket, and caches
  it in the resource's `status`. The user's Git repository is never
  mutated to carry the ULID — consistent with ADR 0003 (the ULID is
  derived runtime state, not desired state).
- **Best-effort durability.** The mapping is NATS KV (hot) backed by
  the event log; a control-plane reset that loses the bucket may
  re-mint, which is accepted. Identity is stable in steady state, not
  eternal.
- **Path move = new identity, by design.** If a team renames *and*
  relocates the source path (or retargets the cluster), the key
  changes and a fresh ULID is minted. Carrying continuity across a
  path move is the team's responsibility, not the engine's — a
  deliberate, documented boundary.

## 5. Streams

Streams are operator-config; consumers and producers never name
them directly. Subjects are the interface.

### 5.1 Stream definitions

| Stream                       | Subjects bound                                  | Retention                | Storage         | Replication | Max age   | Discard policy |
|------------------------------|-------------------------------------------------|--------------------------|-----------------|------------:|-----------|-----------------|
| `keleustes-audit`            | `keleustes.audit.>`                             | Limits (max age)         | File            |           3 | **30 d**  | `old` (drop oldest) |
| `keleustes-events-16`        | `keleustes.events.shard-{00..0f}.>`             | Limits (max age + max bytes) | File        |           3 | **7 d**   | `old`          |
| `keleustes-dependency-16`    | `keleustes.dependency.shard-{00..0f}.>`         | Limits                   | File            |           3 | **7 d**   | `old`          |
| `keleustes-agent`            | `keleustes.agent.>`                             | WorkQueue                | File            |           3 | 1 h       | `old`          |
| `keleustes-webhook`          | `keleustes.webhook.>`                           | Limits                   | File            |           3 | 24 h      | `old`          |
| `keleustes-render`           | `keleustes.render.>`                            | Limits                   | File            |           3 | 7 d       | `old`          |
| `keleustes-system`           | `keleustes.system.>`                            | Limits                   | File            |           3 | 30 d      | `old`          |

Defaults:

- **Replication 3** in production (per ADR 0005 §2). MVP 0 single
  node is acceptable for development.
- **File storage** everywhere. Memory storage tempting for
  `keleustes-agent` (short retention, high throughput) but the
  cost of losing in-flight work-claim state on a JetStream node
  restart outweighs the latency win.
- **Discard `old`** so producers never block at the stream limit;
  archive segmenter (§7) is responsible for ensuring nothing is
  permanently lost before it rolls.

### 5.2 Per-stream retention rationale

| Stream                  | Hot retention | Why                                                                                                       |
|-------------------------|---------------|-------------------------------------------------------------------------------------------------------------|
| `keleustes-audit`       | 30 d          | Matches ADR 0005 §2 audit window. Object-storage archive is authoritative after rollover.                  |
| `keleustes-events`      | 7 d           | Live UI tails operate on the last few minutes; replay-for-controller-rebuild needs at most 7 days history.  |
| `keleustes-dependency`  | 7 d           | Satisfaction events are short-lived (a Promotion either advances or times out within hours).                |
| `keleustes-agent`       | 1 h (WorkQueue) | Work-claim is ephemeral; ack removes the message. WorkQueue retention enforces "drop unacked after 1 h." |
| `keleustes-webhook`     | 24 h          | Webhook receipts have a known dedup window; 24 h covers retries from upstream providers comfortably.         |
| `keleustes-render`      | 7 d           | Cache-event audit (SKA-320 §6.4) needs to outlive a typical Release cycle.                                   |
| `keleustes-system`      | 30 d          | Low volume; long retention is cheap and useful for incident forensics.                                       |

Per the SKA-322 §10 contract, the `keleustes-audit` stream rolls
into the object-storage archive (§7) before retention drops events.
The archive segmenter watches the stream's age proximity to the
30-day limit and rolls 24 h ahead of expiry.

### 5.3 What about a single super-stream?

Considered: bind every `keleustes.>` subject to one giant stream
with mixed retention. Rejected because:

- One stream can carry exactly one retention policy.
- One stream can carry exactly one replication factor.
- One stream's lost-quorum failure mode takes down every consumer
  at once.

Seven streams is the operational unit; per-class isolation is
worth the extra config.

## 6. NATS KV Buckets

JetStream-backed KV stores serve as the hot indexes — the
sub-second lookup layer that consumers hit before falling back to
the durable streams.

| Bucket                | Key shape                                              | Value                                          | TTL    | Used by                                                |
|-----------------------|--------------------------------------------------------|------------------------------------------------|--------|---------------------------------------------------------|
| `audit-index`         | `<subject.ulid>/<eventId>`                             | `{partition, subject, recordedAt, hash}` (≤ 1 KiB) | 7 d    | API server `Audit.Query` per-resource lookups (SKA-322 §10). |
| `agent-presence`      | `<deploymentTargetName>.<agentInstance>`               | `{lastHeartbeat, nkeyFingerprint, version}`    | 5 min  | Hub's "is the agent alive?" check + UI agent health row.  |
| `controller-locks`    | `<controllerName>/<shardId>`                           | `{holder, validUntil}` (≤ 128 B)              | 30 s (heartbeat-extended) | Sharded controller leader election per shard.            |
| `deployment-snapshots`| `<deploymentName>` (one entry per Deployment CR)       | Latest reconciled snapshot — small JSON        | none   | UI matrix live view; alternative to live CRD list at scale. |
| `resource-identity`   | `<xxhash64(sourcePath + targetCluster)>`               | `{ulid, sourcePath, targetCluster, mintedAt}` (≤ 256 B) | none   | Engine resolves the durable `subject.ulid` for audit-subject resources (§4.5, ADR 0008). |
| `webhook-dedup`       | `<provider>/<delivery-id>`                              | `{firstSeen, processed}`                       | 24 h   | Webhook receiver dedup window.                            |

KV TTLs are enforced by the JetStream consumer underneath; consumers
of these KV buckets must not assume long-lived entries.

`audit-index` is the bucket SKA-322 §10 required. Keys carry the
`<subject.ulid>` first so the prefix-scan lookup ("everything that
happened to this Promotion") is the natural KV operation. Per-event
detail still lives in the durable stream — the KV value is a
pointer.

## 7. Object-Storage Archive Layout

Per ADR 0005 §2 and SKA-322 §10, durable streams roll into the
object-storage archive when they approach the hot retention limit.
The archive bucket layout:

```
<bucket>/audit/segments/<YYYY-MM>/<segment-id>.json
<bucket>/audit/segments/<YYYY-MM>/<segment-id>.cbor       # MVP 3+ encoding option per SKA-322 §4.2
<bucket>/events/segments/<YYYY-MM-DD>/<segment-id>.json
<bucket>/dependency/segments/<YYYY-MM-DD>/<segment-id>.json
<bucket>/system/segments/<YYYY-MM>/<segment-id>.json
```

- **`<segment-id>`** is `<stream>-<first-eventId>-<last-eventId>`.
  Two ULIDs make the segment file-name self-describing and
  monotonic.
- **Audit segments** roll on UTC monthly boundaries OR 256 MiB,
  whichever comes first; events/dependency segments roll on UTC
  daily boundaries OR 256 MiB. Audit segments stay smaller per file
  on average (operator-friendly), events have more files at higher
  density (replay-friendly).
- **No mixed-stream segments.** Each subdirectory is one stream.
- **Lifecycle:** the bucket carries a 7-year retention policy
  (compliance floor); operators may set a shorter policy per their
  regime.

The `keleustes-agent`, `keleustes-webhook`, and `keleustes-render`
streams do **not** archive. Their content is either ephemeral
(agent work-claim) or already encoded in audit events (render cache
hit/miss). Archiving them would duplicate state with no recovery
value.

The segmenter is a separate hub process consuming each
to-be-archived stream as a durable JetStream consumer; it writes
the new segment atomically (`s3://…/.tmp/<uuid>` → rename) before
acknowledging messages on the consumer. Partial writes leak
storage; they cannot corrupt the archive.

## 8. Consumer Patterns

### 8.1 Durable consumers (the default)

Every per-shard controller, the audit-pipeline, the agent-bus
listener — they all bind **durable** consumers, named for their
role plus the shard they own:

```
<consumer-name> = <role>.<shard>
                = "application-controller.shard-1f"
                = "audit-archive-segmenter"          # unsharded
                = "syncplan-controller.shard-0a"
```

Durable consumers survive consumer-process restarts; the cursor is
held on the stream. Acks are explicit (`AckExplicit`) for every
class — implicit/auto-ack would collapse the "at-least-once with
replay" guarantee that the cross-shard dependency design (§9)
depends on.

Pull or push? **Pull** for every internal consumer. Pull preserves
backpressure and bounds per-consumer memory; the cost is one
client-side loop per consumer. The exception is the UI live tail
(§8.4) where push is acceptable.

### 8.2 Per-shard subscription pattern

A sharded `Application` controller running on shard `1f` subscribes:

```
keleustes.events.shard-1f.application.>
keleustes.dependency.shard-1f.application.>.satisfied
```

Two subjects, one durable, exact slice. No wildcard fan-out.

A worker pool draining the agent transport:

```
keleustes.agent.cluster.agent.>.claim
```

WorkQueue retention on `keleustes-agent` means the first worker to
ack wins; the others see the message disappear from the queue. No
extra coordination layer needed.

### 8.3 Ephemeral consumers

Reserved for one-off operator scripts and tests. Production code
should not create ephemeral consumers — the cursor is not preserved
across reconnects, which makes incident replay impossible.

The UI live tail (§8.4) is the one exception inside the product
itself.

### 8.4 UI live tail

The UI's per-resource audit tab connects through the API server.
The API server holds **one push consumer per connected UI session**,
ephemeral, bound to:

```
keleustes.audit.<shard-for-subject>.<resource-kind>.<subject.ulid>
keleustes.events.<shard-for-subject>.<resource-kind>.<subject.ulid>
```

The API server resolves `<shard-for-subject>` by running the §4.1
hash on the resource ULID. The UI never sees subject strings; it
queries by `subject.ulid` and the API server does the routing.

### 8.5 Cursor preservation across redeploys

Durable-consumer cursors are JetStream state, not consumer-process
state. A pod restart, redeploy, or migration to a new node is
transparent — the next pull picks up at the last acked sequence.
This is what makes "kubectl rollout restart" of a shard controller
fleet a safe maintenance operation.

If a consumer is *intentionally* recreated under a new name
(rare — e.g., adding a per-shard worker tier), it starts at the
end of the stream by default. To replay from a known starting
sequence, pass `OptStartSeq` on consumer creation; this is the
mechanism the audit consumer uses for catastrophic-loss recovery
(§10).

## 9. Cross-Shard Dependency Events

Engine plan §2.6 and ADR 0006 §8 require: when Application A on
shard 1 depends on Application B on shard 2, A's controller must
receive a "B is healthy" event when one occurs. The dependency
event class (`keleustes.dependency.>`) is the carrier.

### 9.1 Subject convention

Producer (Sync Engine running on shard 2, on behalf of Application B):

```
publish to: keleustes.dependency.shard-2-for-B.application.<B-ulid>.satisfied
```

Subscriber (SyncPlan controller on shard 1, watching Application A
which has `dependencies.applications[].applicationRef.name = B`):

```
subscribe to: keleustes.dependency.shard-2-for-B.application.<B-ulid>.satisfied
```

The crucial bit: **the producer publishes to B's shard, not A's.**
A's controller does the lookup at SyncPlan-evaluation time —
"Application A depends on B → compute shard for B → subscribe to B's
shard for satisfied events." No coordinator, no fan-out registry.

### 9.2 How A's controller discovers B's shard

At SyncPlan reconcile time, the controller:

1. Walks `Application.spec.dependencies.applications[]`.
2. For each dependency, computes `ShardFor("application",
   dep.applicationRef.name, partitionCount)` (§4.1).
3. Creates a durable consumer on the corresponding subject if one
   does not already exist for this (A, B) pair.
4. On every received `satisfied` event, re-evaluates A's
   `WaitingForDependencies` condition.

The (A, B)-pair durable consumer name follows the pattern
`syncplan.<A-namespace>.<A-name>.deps.<B-shard>.<B-name>` — long
but explicit. JetStream durables are cheap; thousands per cluster
are routine.

### 9.3 What about transient consumers?

A pure "subscribe while A is waiting, unsubscribe when satisfied"
ephemeral consumer is appealing but loses the recovery property:
if the SyncPlan controller crashes between subscribing and acking
B's satisfied event, the event is gone on the ephemeral consumer
and A never advances.

Durable consumers + explicit ack solve this. The consumer cleanup
is opportunistic: the SyncPlan controller deletes the (A, B)
durable when A reaches `Succeeded` and no longer depends on B.

### 9.4 Hand-off events for Application moves (SKA-320 §8.2)

The render plan's hand-off pattern (Application B taking ownership
of objects previously owned by Application A) also rides this class:

```
keleustes.dependency.<A-shard>.application.<A-ulid>.handoff-out.<B-ulid>
keleustes.dependency.<B-shard>.application.<B-ulid>.handoff-in.<A-ulid>
```

A is notified of an outgoing hand-off (so its next prune set
excludes the moved objects); B is notified of an incoming hand-off
(so its first apply on those objects uses the right field manager).

## 10. Multi-Region Supercluster

ADR 0005 §2 specifies a NATS supercluster spanning regions in
multi-region deployments. The subject layout above is region-
agnostic by design; the supercluster's leaf-node routing handles
the rest.

Two rules for multi-region:

1. **Streams are *not* per-region.** A `keleustes-audit` stream is
   one logical stream across the supercluster, replicated R≥3 with
   replicas placed in at least two regions (operator-configured
   `placement.tags`). Region loss does not lose audit history.
2. **Per-region consumers bind to the same durable name.** If a
   regional controller fleet is running in `us-east` and a
   secondary fleet exists in `us-west` for failover, both use the
   durable `application-controller.shard-1f`. JetStream's consumer
   coordination ensures only one fleet acks any given message.
   Failover is a matter of starting the secondary fleet; no special
   subject changes.

The exception is the `keleustes-agent` stream, where regional
locality matters (agents connect to the nearest supercluster
member). The agent stream remains one logical stream; subscriptions
happen via leaf-node routing and the supercluster handles the
geographic glue.

## 11. Producer Contracts

### 11.1 Subject construction

Producers use the `internal/events/subject` package, never raw
strings:

```go
// internal/events/subject/subject.go
package subject

// For builds a canonical subject for a partitioned event class.
// Falls back to literal "cluster" scope when key == "".
func For(class Class, kind Kind, key string, suffix ...string) string {
    scope := "cluster"
    if key != "" {
        scope = partition.ShardFor(string(kind), key, currentPartitionCount(class))
    }
    parts := []string{"keleustes", string(class), scope, string(kind), key}
    parts = append(parts, suffix...)
    return strings.Join(parts, ".")
}
```

This forces every subject construction through one function; the
function knows the per-class partition count and the canonical
shard format. Adding a new event class is adding an entry to the
`Class` enum and updating the partition-count table.

### 11.2 Publish ordering and idempotency

JetStream publish acks are **synchronous** by default. State-
changing producers (Sync Engine after a successful apply, Promotion
Engine after a state-machine transition) must wait for the ack
before proceeding. Audit events follow the SKA-322 §11.1
"write-then-act" rule; non-audit events follow the same pattern
where the consumer's eventual processing affects user-visible
state.

Idempotency is the consumer's job. Producers do not deduplicate;
JetStream's `Nats-Msg-Id` header is set to the event ULID when
present, allowing JetStream's built-in dedup (configured per
stream with a `duplicate_window` matching the consumer's expected
ack latency).

### 11.3 Backpressure

The `discard: old` discard policy means producers never block on a
full stream; the cost is silent loss of the oldest events. For
audit (where loss is unacceptable inside the hot window), the
archive segmenter (§7) must roll faster than the retention drop.
A stream-age monitor on `keleustes-audit` alerts at 24 h before
expiry if segmenter lag is detected.

For non-audit streams, `discard: old` is the correct default;
oldest-event loss is preferable to producer-side blocking that
would cascade into reconciler timeouts.

## 12. Failure Modes

| Failure                                           | Behavior                                                                                                                       |
|---------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------|
| Single JetStream node loss (R=3)                  | Stream re-elects leader; consumers reconnect transparently. No data loss.                                                       |
| JetStream quorum loss (e.g., 2 of 3 down)         | Producers block on publish ack; consumers cannot create new consumers. Existing durables keep their cursors. Recovery: restore quorum. |
| Object-storage region failure                     | Archive segmenter retries with backoff; hot streams unaffected. If outage exceeds segmenter lag budget, audit stream begins to lose events on retention drop (alerts trigger). |
| Single consumer pod loss                          | Durable cursor preserved; another pod (or restart of the same) picks up at last acked sequence.                                  |
| Producer publishes to a subject with no bound stream | Publish ack fails with "no stream matching subject." Producers fail loudly — must not silently drop. The subject grammar (§3) forbids unbound subjects. |
| Repartition cutover in flight, mixed clients      | Consumers on the new partition count read both old and new shards transparently; producers on the old count emit to old shards. Cutover window is one full retention period.                                |
| Cross-shard dependency event published but A's controller is down | Event lands on JetStream; durable consumer cursor preserved; A's controller picks up on restart and processes the satisfied event from the recovered cursor. No SyncPlan stuck in WaitingForDependencies forever. |

## 13. Open Questions

1. **`partitionCount` mutation cadence.** §4.3 describes how to grow
   the count safely, but who decides when? Open: a hub-side
   monitor that watches per-shard publish rate and recommends a
   bump when the top shard exceeds 4× the median. Confirm shape
   at MVP 2 benchmark.
2. **xxhash vs. SHA-256 truncation for the partition hash.** xxhash
   is faster but not cryptographic. The threat model here is
   "operator accidentally collides Application names with
   adversarial intent" — implausible. Sticking with xxhash unless
   a benchmark proves otherwise.
3. **Cross-region segmenter ownership.** With R≥3 across regions,
   which region's segmenter actually writes the archive? Default
   answer: leader of the stream owns the segmenter. Edge case:
   leader migration mid-segment. Acceptable: the new leader
   resumes from the last acked sequence; the partial segment in
   `s3://…/.tmp/` is GC'd by the bucket's incomplete-multipart
   policy.
4. **Per-Application audit subjects.** Today a Promotion's audit
   event lands on `keleustes.audit.<promotion-shard>.promotion.
   <ulid>`. A UI tab on the Application that *owns* that Promotion
   would benefit from a second subject like
   `keleustes.audit.<app-shard>.application.<app-ulid>.promotion.
   <ulid>`. Costs: duplication. Benefits: per-Application UI tail
   without a join. Defer until UI load tells us which way is right.
5. **WorkQueue vs Limits retention for `keleustes-dependency`.** A
   satisfied event is consumed by at most a handful of waiting
   controllers and then has no further use. WorkQueue with a
   short retention would be cheaper; Limits is the current choice
   because debug replay is occasionally useful. Revisit at MVP 2.

## 14. Compliance with Prior Decisions

| Decision                                          | This plan honors it by                                                                                                              |
|---------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------|
| ADR 0005 §2 (JetStream canonical, R≥3, 30 d hot)  | `keleustes-audit` stream defaults to those values; archive segmenter rolls before the 30 d expiry.                                  |
| ADR 0005 §11.5 (sharding by resource name)        | `<scope>` token is the resource's shard, computed by xxhash over `<kind>/<name>`. Per-shard subscriptions are one wildcard each.    |
| ADR 0005 §4 (acceptable historical loss)          | Failure-mode table (§12) explicitly handles JetStream + archive both lost — audit beyond CRD `.status` is unrecoverable, by design. |
| ADR 0006 §4 (engine containment)                  | The producer/consumer packages live in `internal/events/`; engine packages depend on the helper, not on raw `nats.go` calls.        |
| ADR 0006 §8 (cross-Application dependencies)      | `keleustes.dependency.>` class is the carrier; producer publishes to *the providing* Application's shard, consumer subscribes there.|
| SKA-322 §10 (audit pipeline demands)              | `keleustes-audit` is the canonical stream; `audit-index` NATS KV holds the 7-day per-resource index; archive layout is `<bucket>/audit/segments/<YYYY-MM>/`.                                                              |
| SKA-322 §5.4 + §15 Q1 (partition field semantics) | Resolved in §4.4: `partition = subject.ulid`-derived shard for events with subjects; literal `"cluster"` for system events.         |
| SKA-320 §6.4 (render cache audit events)          | `keleustes.render.cluster.application.<ulid>.cache-hit` (and `.cache-miss`) ride the `render` class; events also flow through `audit` for the SKA-322 audit envelope. |
| SKA-320 §8.2 (Application hand-off)               | `keleustes.dependency.<A-shard>.application.<A-ulid>.handoff-out.<B-ulid>` is the carrier; same dependency class as the satisfied events. |

## 15. Concrete Follow-ups

1. **SKA-343 (MVP 1 NATS JetStream introduction, hub-internal)** —
   implements the seven streams above as operator-managed
   resources (`config/nats/streams/*.yaml`) plus the
   `internal/events/subject` and `internal/events/partition`
   packages. Audit-stream segmenter lands here too.
2. **New ticket: `internal/events/` package scaffold** — the
   producer/consumer helpers, the subject grammar enforcement, the
   per-class partition-count config loader. Small enough to
   piggy-back on SKA-343.
3. **New ticket: NATS KV bucket scaffolds** (per §6 table) —
   `audit-index`, `agent-presence`, `controller-locks`,
   `deployment-snapshots`, `webhook-dedup`. Owned by the same
   ticket that owns each bucket's primary consumer.
4. **New ticket: archive segmenter for non-audit streams** —
   `keleustes-events`, `keleustes-dependency`, `keleustes-system`
   need segmenters too (audit segmenter is SKA-343 scope).
5. **New ticket: per-shard publish-rate monitor** (open question
   §13.1) — emits a Prometheus metric per shard and an alert when
   skew exceeds the 4× threshold. MVP 2.
6. **`config/nats/streams/`** directory and Helm-chart wiring —
   small, ships with SKA-343.
7. **Update `docs/DECISIONS.md`** — add this plan to the active
   interim contracts table (handled in the same commit as this
   plan).

---

**When this plan stabilizes** (after the MVP 1 benchmark validates
the partition counts and the segmenter has run for at least one
audit-retention window), §1–§12 promote into a new ADR co-located
with ADR 0005 — likely ADR 0010 (Render → 0007, Audit → 0008, RBAC
shapes → 0009, JetStream layout → 0010). §13 open questions remain
in this plan until resolved.
