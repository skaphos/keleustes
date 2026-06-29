<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Audit Event Schema (Versioned)

- **Status:** Draft — 2026-05-17
- **Linear:** SKA-322 (this plan). Blocks SKA-332 (MVP 0 audit envelope emission), SKA-347 (MVP 1 audit pipeline JetStream end-to-end), SKA-385 (MVP 4 hash-chained audit). Related: SKA-320 (Render Contract) reserved five `render.*` event types here; SKA-324 (JetStream subject and stream layout) will pick up the stream side.
- **Promotes into:** a future ADR co-located with ADR 0004. Until then, this document is authoritative for any code that writes or reads audit events.
- **Supersedes:** [RBAC plan §6.2](./2026-05-rbac-audit-and-git-invariant.md) (envelope JSON sketch — formalized and extended here), and the audit-shape paragraph in the same plan's §11 next-steps ("Define the audit event Protobuf / JSON schema and version it…").
- **Related:** [ADR 0003](../adr/0003-git-source-of-truth-invariant.md) (Git invariant — audit is the only path that records non-Git mutations), [ADR 0004](../adr/0004-crd-based-rbac.md) (RBAC — ULID requirement, actor identity), [ADR 0005](../adr/0005-distributed-runtime.md) (JetStream + object storage tiers; "audit history beyond CRD-status reach is not recoverable" if both lost), [ADR 0006](../adr/0006-engine-boundaries.md) (engine boundaries — each engine emits its own audit events from its own package), [ADR 0008](../adr/0008-resource-identity-model.md) (resource identity — origin of `action.subject.ulid`), [SKA-320 Render Contract](./2026-05-render-contract-and-inventory-model.md) (`render.*` event types).

## 1. Purpose and Scope

This plan pins down the audit event format that every Keleustes
producer writes and every Keleustes consumer reads — for years. The
envelope shape, the versioning rules, the actor/redaction rules, and
the persistence contract land here once and stay frozen except via
the explicit additive/breaking-change protocol in §5.

**Why this is load-bearing for everything else.** Twelve future
stories will write into the audit stream and four will read from it.
If we get the envelope wrong now, every one of them has to migrate;
if we get it right now, additions are cheap forever.

**In scope:**

- The wire format (JSON canonical on the bus; CBOR option at-rest).
- The envelope: fields that are present on every event regardless of
  type.
- The per-verb extension mechanism: how `action.verb=promote` gets a
  typed `payload` block without breaking the envelope contract.
- Versioning policy (additive forever, deprecation lane, breaking-
  change protocol).
- Actor normalization across IdPs.
- Snapshot policy (when `before`/`after` is captured, what gets
  redacted, what `evidence` is allowed to carry).
- Correlation IDs (`eventId`, `requestId`, `sessionId`,
  `traceparent`).
- Event-type registry: initial canonical set with reservation
  protocol for the open MVPs.
- Producer/consumer contracts and failure modes.

**Out of scope:**

- The JetStream subject hierarchy and stream partitioning shape
  (SKA-324). This plan specifies *what* gets persisted; SKA-324
  specifies *where* in JetStream and how it is partitioned across
  Application-hash prefixes.
- Hash-chaining for tamper-evidence (SKA-385, MVP 4). The schema
  reserves field positions (§5.4) but does not implement.
- The SIEM exporter implementation (RBAC plan §11 open question 9;
  ships in `contrib/`).
- DuckDB-on-parquet rebuild logic (ADR 0005 §11; consumer of the
  schema, not author).

## 2. Why the Stable Envelope Wins

Argo CD's per-controller log emission is the cautionary tale: every
component invented its own message shape, and every downstream
consumer (the UI, the metrics scraper, the CLI history command, the
SIEM exporters customers wrote) had to deserialize N different
formats. The compounding cost is real — when a thirteenth event
type lands, it is twelve consumers' worth of *deserialize logic* to
update.

Keleustes' choice: one envelope, one canonical encoding, one set of
correlation IDs. Producers serialize once; consumers deserialize
once. New event types add a new entry to the `payload` discriminated
union; they do **not** add new top-level fields.

Three properties drop out of this commitment:

- **Reads cost the deserializer no more in year 5 than in year 1.**
- **Adding an event type is a producer-side change.** A consumer
  that does not care about the new payload just iterates past it.
- **Schema drift is detectable.** `schemaVersion` plus the
  registered enum of `action.verb` values gives every consumer a
  no-surprises envelope.

## 3. The Envelope

Every Keleustes audit event, regardless of verb, has this shape:

```jsonc
{
  "schemaVersion": "audit/v1",        // §5 — bumps on breaking change

  // ---- correlation (§9) -----------------------------------------
  "eventId":   "01HQ8FRVHX2BJW2N4Z9KZ8P6XK", // ULID; primary key
  "occurredAt": "2026-05-17T14:32:11.482Z",  // RFC 3339 UTC; producer wall clock
  "recordedAt": "2026-05-17T14:32:11.491Z",  // RFC 3339 UTC; broker (JetStream) ack time
  "requestId":  "f3e1c2d0-7b8a-4e5f-9c1d-8a7b6c5d4e3f", // UUIDv4; one per API call
  "sessionId":  "01HQ8FRGW0K3JNR1E5D7V8YQM2",          // ULID; one per CLI/UI session
  "traceparent": "00-...-...-01",                       // W3C trace context (optional)

  // ---- who (§6) -------------------------------------------------
  "actor": {
    "type":             "human",            // enum — §6.2
    "subject":          "alice@example.com",// normalized — §6.3
    "subjectId":        "okta|01HQ7…",      // immutable IdP-issued id
    "identityProvider": "okta-prod",        // name of IdentityProvider CR
    "groups":           ["sre", "payments"],
    "delegatedFrom":    null                // §6.5 — for system actions on behalf of a user
  },

  // ---- what -----------------------------------------------------
  "action": {
    "verb":  "promote",                     // registered enum — §13
    "scope": "project:payments",            // RBAC scope of the action
    "subject": {                            // the object the verb is being applied to
      "apiGroup":  "keleustes.skaphos.io",
      "kind":      "Promotion",
      "namespace": "payments",
      "name":      "checkout-api-to-prod-2026-05-15", // natural key — addressing only
      "ulid":      "01HQ8FQA7Z4M2N1P3K9F8X6Y7B" // durable id — ADR 0008; origin in SKA-324 §4.5
    }
  },

  // ---- intent (free-text, required for human-triggered mutations) -
  "intent": "Patch CVE-2026-29181 in checkout-api before EOD.",

  // ---- context ---------------------------------------------------
  "context": {
    "sourceIp":    "10.42.7.13",            // string; v4 or v6
    "userAgent":   "keleustesctl/0.5.0",
    "auditTicket": "INC-12345",             // optional; required when actor is break-glass
    "clusterName": "hub-us-east-1",         // which Keleustes deployment emitted this
    "shard":       "shard-a3"               // hub shard id (MVP 2+)
  },

  // ---- result ----------------------------------------------------
  "result": {
    "outcome": "success",                   // enum: success | denied | error | partial
    "reason":  "",                          // human-readable; required for denied/error/partial
    "before":  null,                        // §8 — typed snapshot, redacted, optional
    "after":   { "...": "..." }             // §8 — typed snapshot, redacted, optional
  },

  // ---- per-verb payload (§7) -------------------------------------
  "payload": {
    "@type": "promote.v1",                  // discriminator; matches verb registry
    "from":  "staging",
    "to":    "prod",
    "release": {
      "ref":    "checkout-api/release/2026-05-15.1",
      "digest": "sha256:9a8b…",
      "ulid":   "01HQ8FRT9DC4VV1MX2N7K1P8YQ"
    },
    "approvalRefs": [
      { "name": "bob-approval-1", "ulid": "01HQ8FR…" }
    ]
  },

  // ---- evidence (§8.3) -------------------------------------------
  "evidence": [
    {
      "kind":      "render-source",
      "hash":      "sha256:5e6f…",
      "objectRef": "render/sha256:5e6f….meta.json",
      "store":     "object"
    }
  ]
}
```

A few rules about the envelope that are not negotiable:

- **`schemaVersion` is mandatory** and is the only field that may
  change without a coordinated migration (§5).
- **Top-level keys are closed.** Producers must not add new
  top-level keys. New information goes in `payload`, `evidence`, or
  `context`.
- **`payload.@type` is mandatory** when `payload` is present. The
  discriminator pattern is what keeps consumers honest — they
  switch on `@type` and ignore unknown values rather than crashing.
- **Every monetary, dimensional, or interval field is a string in
  ISO units.** Timestamps are RFC 3339 with `Z`; durations are
  ISO 8601 `P…`. No epoch seconds, no fractional days. This is the
  one place we trade compactness for unambiguity.
- **`evidence` entries are pointers, not blobs.** Always. See §8.3.

## 4. Wire Format

### 4.1 Decision: canonical JSON on the bus

The canonical wire format is **UTF-8 JSON, canonicalized per
RFC 8785 (JSON Canonicalization Scheme)**. Same encoding on
JetStream subjects, in the UI's gRPC stream, in CLI output, in the
object-storage archive's metadata.

Rationale:

1. **Operators read this directly.** `nats stream view
   keleustes.audit` should produce something a human can scan
   without a `protoc` round-trip.
2. **Schema is enforced at the producer.** A Go struct with
   `json:"…,omitempty"` tags plus a single
   `audit.MarshalCanonical(event)` function gives us deterministic
   output. Consumers that want a typed view re-parse into the same
   Go struct.
3. **One format means one set of forward-compat bugs.** A second
   wire format multiplies migration cost without buying capability
   we have a present need for.

Argo CD's component-by-component drift was the lesson — picking one
format and enforcing it at one chokepoint is the durable win.

### 4.2 CBOR as the at-rest option (MVP 3+)

For the object-storage cold archive, we *may* re-encode to CBOR
when archiving multi-million-event segments (size ratio is typically
~0.6 of canonical JSON, and the rolled-up consumer is the DuckDB
rebuild job which does not require human eyeballs). This is
deferred to MVP 3 — the DuckDB rebuild work — and is purely an
archive-side concern. The JetStream wire format stays JSON forever.

If we adopt CBOR at-rest, the archive segment filename carries the
encoding (`…/segments/2026-05.cbor` vs `…/segments/2026-05.json`)
and consumers branch on suffix. No envelope changes.

### 4.3 What about Protobuf?

Considered and rejected as the primary format. Producer ergonomics
matter: every engine emits audit events from its own package, and
forcing each one to depend on a generated `*.pb.go` so it can write
to JetStream is a per-engine build-graph weight we do not need. The
secondary concern is operator readability (§4.1.1).

We keep Protobuf reserved for two specific futures:

- **Cross-region replication.** If JetStream supercluster links
  become bandwidth-constrained at MVP 4 scale, we may add a
  binary-encoded replication path between regions. This is a
  transport-encoding concern, not a schema concern.
- **Customer-side SIEM exporter performance.** If a customer's
  SIEM pipeline benchmarks the JSON parse step as the bottleneck,
  the reference exporter (`contrib/`) can offer a `--cbor` flag.
  Same source schema, different encoding at the exporter boundary.

Neither future changes the on-bus encoding.

## 5. Versioning Policy

### 5.1 Additive forever

The default and only routine operation is **additive**:

- Add a new optional field anywhere except as a new top-level key.
- Add a new `action.verb` (must register in §13).
- Add a new `payload.@type` (must register in §13).
- Add a new `evidence.kind` (must register in §8.3).
- Add a new `actor.type`, `result.outcome`, etc., enum value.

These changes do **not** bump `schemaVersion`. A consumer pinned to
`audit/v1` that encounters an unknown verb or payload type ignores
the payload and processes the envelope normally.

### 5.2 Required fields are forever

A field listed as required in the envelope (`schemaVersion`,
`eventId`, `occurredAt`, `actor.type`, `actor.subject`,
`action.verb`, `action.scope`, `result.outcome`) cannot become
optional. A field that is optional today cannot become required
without a `schemaVersion` bump.

### 5.3 Breaking changes — the deprecation lane

When a breaking change is genuinely needed:

1. **Reserve** the new field (or rename) in the spec but mark it
   `experimental: true` for one MVP cycle.
2. **Producers** emit both old and new fields for one MVP cycle
   under the existing `schemaVersion`.
3. **Consumers** update to read the new field while still falling
   back to the old.
4. **Bump** `schemaVersion` to `audit/v2`; producers stop emitting
   the old field. The deprecation cycle is at least 6 months
   regardless of MVP cadence.
5. **Old segments** in object storage are never rewritten. The
   `audit/v1` decoder is preserved as a frozen package
   (`internal/audit/v1`) for replay forever.

The `schemaVersion` bump rule is conservative on purpose: every
bump is a migration day for every SIEM consumer customers run.

### 5.4 Reserved fields

The envelope reserves these field positions for future use without
counting as adding a new top-level key:

| Field           | Reserved for                                       | Lands in       |
|-----------------|----------------------------------------------------|----------------|
| `chain.prev`    | Hash-chained audit (SKA-385)                       | MVP 4          |
| `chain.signature` | Detached signature over `{envelope, chain.prev}` | MVP 4          |
| `partition`     | Sharded-controller-friendly hash prefix for SKA-324 | MVP 1         |
| `tenantId`      | Per-tenant IdP isolation (MVP 4)                   | MVP 4          |

A producer running today must not write these keys. A consumer
running today must accept them when they appear (they will be
ignored by `audit/v1`-only parsers, which is the desired
behavior).

## 6. Actor Normalization

### 6.1 Why this needs explicit rules

Customers run many IdPs. Azure AD groups are `/`-separated paths.
GitHub OIDC groups are dotted. Okta groups have user-display vs
canonical names. CI workload identities (GitHub `repo:org/repo`,
GitLab `project_path:org/group`, cloud IAM service accounts) all
emit different subject formats.

If we record raw IdP output, every UI query like "what did Alice
do?" becomes a per-IdP join in user code. We normalize at the
producer boundary so consumers see one shape.

### 6.2 `actor.type` (closed enum)

```
human   — a person, authenticated via an OIDC IdP marked humans=true
ci      — automation, authenticated via OIDC workload identity or SA
agent   — a Keleustes agent (NKey-authenticated)
system  — Keleustes hub internal (controllers running as themselves)
```

`system` is a real value, not a hack. When the Sync Engine prunes a
resource on a scheduled SyncRun, the audit event has
`actor.type=system`, `actor.subject=keleustes-sync-engine`. This is
how the UI distinguishes "Alice ran a sync" from "the controller
reconciled."

### 6.3 `actor.subject` normalization

Producer responsibilities:

| `actor.type` | `subject` shape                                          | Example                                |
|--------------|----------------------------------------------------------|----------------------------------------|
| `human`      | RFC 5322 email when available, else `<idp-name>:<id>`    | `alice@example.com`                    |
| `ci`         | `<provider>:<workload-id>` (lowercase, no path slashes)  | `github-actions:org/repo@main`         |
| `agent`      | `agent:<deploymentTarget>.<agentInstance>`               | `agent:prod-us-east.agent-7`           |
| `system`     | `keleustes-<engine>` (lowercase, hyphenated)             | `keleustes-sync-engine`                |

`actor.subjectId` carries the IdP-immutable identifier when the
display `subject` can change (email rename, GitHub username change).
For `agent` and `system`, `subjectId` is the same as `subject`.

### 6.4 `actor.groups`

Always present (may be empty). Group claims are mapped through the
`IdentityProvider` CR's claim-mapping rules into a canonical
Keleustes group set (lowercase, no slashes, ASCII). The original
IdP group strings are *not* preserved in the audit envelope — they
live on the `IdentityProvider` CR's audit history if needed.

This is the one place we accept lossy normalization. Customers
needing the raw IdP claim should write a custom audit consumer that
joins back to the IdP — but the canonical group form is what UI
queries and RBAC evaluators consume.

### 6.5 `actor.delegatedFrom` — system-on-behalf-of-human

When a controller acts because a human triggered it earlier, the
controller's audit event carries `actor.delegatedFrom` with the
originator's actor block. Example: an Approval is granted (human
event), and the Promotion advances 30s later (system event with
`delegatedFrom = <bob's actor>`). The UI uses this to render the
Promotion timeline as a single causal thread.

`delegatedFrom` is recursive (a system action that delegates from a
system action that delegates from a human is possible but rare). The
schema permits one level of nesting; deeper nesting requires
`requestId` correlation instead.

## 7. Per-Verb Payloads — the Verb Registry

### 7.1 Discriminated union via `payload.@type`

Every `action.verb` has zero or one registered payload types. The
`payload.@type` string is the discriminator. Examples:

```jsonc
"action": { "verb": "promote", ... },
"payload": { "@type": "promote.v1", "from": "staging", "to": "prod", ... }
```

If `payload` is absent, the consumer treats the event as
envelope-only (sufficient for many simple verbs like `view`).

### 7.2 The payload type registry

Payload types are versioned independently of the envelope. A bump
from `promote.v1` to `promote.v2` follows the §5.3 deprecation lane
but does not require an envelope `schemaVersion` bump — payloads are
the additive extension surface, the envelope is the frozen contract.

The initial registered payload types land in §13.

### 7.3 Unknown payload handling

A consumer encountering `payload.@type=foobar.v9` it does not
understand:

- **Logs at INFO**, not WARN.
- **Records the event in its index** with `payload` retained as
  opaque JSON.
- **Does not refuse to process the envelope.**

This is what makes the registry a producer-side concern. A new
event type can roll out across producers and only the consumers
that *want* to render it need to update.

### 7.4 Producer-side validation

Producers must validate their own payload against the registered
schema before publishing. The `internal/audit/` package exposes
`audit.Emit(ctx, envelope, payload)` which fails the build if
`payload` does not satisfy the schema for `envelope.action.verb`.
This catches drift at compile time instead of at log-query time.

## 8. Snapshots, Redaction, and Evidence

### 8.1 `result.before` / `result.after`

For state-changing actions, the envelope carries before/after
snapshots of the affected object — but with explicit rules about
when and how.

**Inclusion rules:**

- **Always include** for `result.outcome ∈ {success, partial}` on
  state-mutating verbs (`create`, `edit`, `delete`, `promote`,
  `approve`, `sync`, `break-glass`).
- **`before` is null** for `create`. **`after` is null** for
  `delete`. **`before` only** for `denied` / `error`.
- **Snapshot is the rendered/normalized form**, not the user
  input. For a CRD edit, that means the object after admission
  webhooks ran but before reconciler mutation.

**Size cap:** 64 KiB serialized per snapshot. If the object exceeds
the cap, the snapshot is replaced with a pointer:

```jsonc
"before": {
  "@oversize": true,
  "objectRef": "audit/snapshots/<event-id>.before.json",
  "store":     "object",
  "hash":      "sha256:..."
}
```

The full object is uploaded to object storage at emission time. This
preserves the audit trail without exploding the JetStream payload.

### 8.2 Redaction rules

Some object fields are **never** allowed to appear in `before` /
`after`:

| Class                                | Examples                                                    | Treatment                                       |
|--------------------------------------|-------------------------------------------------------------|--------------------------------------------------|
| Secret bytes                          | `Secret.data.*`, `Secret.stringData.*`                       | Replace with `"@redacted":"secret-bytes"`        |
| Token bodies                          | `*.token`, `*.bearerToken`, `Authorization` header value     | Replace with `"@redacted":"bearer-token"`        |
| TLS material                          | `tls.crt`, `tls.key`, certificate PEM blocks                 | Replace with `"@redacted":"tls-material"`        |
| Webhook receiver secrets              | `Source.spec.webhook.secret`                                 | Replace with `"@redacted":"webhook-secret"`      |
| Plugin-declared sensitive             | Anything matching the `keleustes.skaphos.io/sensitive=true` annotation on the field's owning CRD | Replace with `"@redacted":"plugin-sensitive"` |

Redacted fields are replaced **in place** so the snapshot's
structure is preserved (a consumer can still see the `Secret` had a
`data.password` key, just not the value).

The redacted-key list is maintained centrally in
`internal/audit/redaction/rules.go` and is the same code path used by
the UI's "view object" surface (so what users see and what audit
records are the same redaction). A change to the redaction list is
itself audited (`system.config.changed` with payload describing the
diff).

What is **never redacted**: digests, content hashes, ULIDs, object
names, namespaces, labels, generations, conditions. Pointers and
identifiers are part of the auditable trail by design.

### 8.3 `evidence` entries

`evidence` is the schema's escape hatch for "this event happened in
relation to a large artifact." Each entry is a pointer to something
stored elsewhere — never an inlined blob.

Registered evidence kinds:

| `kind`               | Points at                                                       |
|----------------------|------------------------------------------------------------------|
| `render-source`      | A `RenderResult` meta.json in object storage (SKA-320 §6.3)      |
| `manifest-bundle`    | The tarball of rendered objects in object storage                |
| `git-commit`         | A Git commit URL produced by the Git Mutation Engine             |
| `pr-link`            | A pull request URL                                                |
| `signature`          | A cosign signature artifact (digest + URL)                       |
| `sbom`               | An SBOM artifact                                                  |
| `policy-evaluation`  | A policy engine result blob in object storage                    |
| `diff`               | A diff result blob in object storage                             |

Each entry has the same shape:

```jsonc
{
  "kind":      "render-source",
  "hash":      "sha256:5e6f…",
  "objectRef": "render/sha256:5e6f….meta.json",
  "store":     "object",   // enum: object | git | external
  "label":     "RenderResult metadata for promote (optional)"
}
```

Evidence pointers must resolve from a hub or a consumer with
object-storage read access. The full evidence retrieval flow lives
in §12 of the runtime plan and is out of scope here.

## 9. Correlation IDs

| ID            | Lifetime                | Generator                              | Used for                                                                                       |
|---------------|-------------------------|----------------------------------------|------------------------------------------------------------------------------------------------|
| `eventId`     | one event               | producer; ULID                         | primary key in NATS KV; URL deep link; idempotency on consumer rewrites                         |
| `requestId`   | one API call            | API server, first hop; UUIDv4          | correlates the human-triggered API call with all downstream system events it caused              |
| `sessionId`   | one CLI/UI session      | CLI: random ULID at session start; UI: cookie | correlates events the same user generated across multiple API calls in a sitting              |
| `traceparent` | distributed trace span | W3C trace context (ADR 0002 default)   | OpenTelemetry trace linking — optional; present when the originator passed trace context        |
| `subject.ulid`| object lifetime         | the affected resource itself (ADR 0004) | "show me everything that happened to this Promotion"                                            |

**Rules:**

- `eventId` is set exactly once and never changes after publication
  (it is the JetStream subject suffix).
- `requestId` is set by the *first* hop that touches the action
  (API server, CLI command). Engines that act because the API server
  enqueued work propagate the same `requestId` through their own
  emissions.
- `sessionId` is propagated by the CLI and UI on every API call.
  Pure system events (controller reconciles without a human trigger)
  have `sessionId` set to the empty string.
- `traceparent` is optional but recommended for any HTTP-initiated
  request. When absent, the audit consumer can still correlate via
  `requestId`.

## 10. Persistence — What the Schema Demands

> **Satisfied by [`2026-05-jetstream-subject-and-stream-layout.md`](./2026-05-jetstream-subject-and-stream-layout.md) (SKA-324, active interim contract).** Every demand listed below has a concrete answer in that plan: §5 spells out the `keleustes-audit` stream (30 d hot, R≥3, file storage, `discard: old`); §6 spells out the `audit-index` NATS KV bucket keyed by `<subject.ulid>/<eventId>` with a 7 d TTL; §7 spells out the object-storage archive layout (`<bucket>/audit/segments/<YYYY-MM>/<segment-id>.json`); §4.4 resolves §15 Q1 below (partition value = `subject.ulid`-derived shard for events with subjects, literal `"cluster"` for system events).

This plan does **not** specify the JetStream subject hierarchy or
stream partitioning shape — that is SKA-324's job. What this plan
demands of SKA-324:

1. **One canonical durable stream** — `keleustes.audit` per the RBAC
   plan §6.3. Per-shard fan-out subjects (per ADR 0005 §11.2) may
   prefix the stream but every event lands in one stream.
2. **30-day hot retention** in JetStream per ADR 0005 §2; rolling
   segments to object storage thereafter.
3. **Per-resource secondary index in NATS KV** (`audit-index`),
   keyed by `subject.ulid`, with a 7-day window for fast
   per-resource lookups (RBAC plan §6.3).
4. **Object-storage archive layout**:
   `<bucket>/audit/segments/<YYYY-MM>/<segment-id>.json` (or `.cbor`
   per §4.2). Segment boundary on UTC midnight or 256 MiB,
   whichever comes first.
5. **No deletes.** Audit is append-only. JetStream retention drops
   *hot* events on TTL; the archive keeps them.

## 11. Producer Contracts

### 11.1 Write-then-act

For state-changing verbs, the producer must publish the audit event
**before** the action takes visible effect. Concretely: the audit
emit returns `(eventId, error)`, and the producer must hold off on
its side effect until the emit succeeds.

The exception is *post-hoc* events (sync result, agent-side
operations) where the action and the audit emission happen on the
same boundary. In that case audit publication is part of the same
SSA write or NATS request; partial failure is recovered by the
controller's idempotent retry, which re-emits with the same
`requestId` and a fresh `eventId`.

### 11.2 Emit interface (Go)

```go
// internal/audit/emit.go
package audit

type Envelope struct { /* §3 fields */ }

type Payload interface {
    // AuditType returns the registered @type discriminator
    // (e.g., "promote.v1"). Implemented by generated payload types.
    AuditType() string
}

// Emit publishes one audit event. Returns the assigned eventId.
// Implementations: JetStream (production), in-memory (tests),
// stdout (MVP 0 logs-only mode per RBAC plan §10).
type Emitter interface {
    Emit(ctx context.Context, env Envelope, payload Payload) (string, error)
}
```

Every engine takes an `audit.Emitter` in its constructor — never a
package-level singleton. This keeps tests honest and keeps the
agent (MVP 2+) able to swap to a forwarding emitter that batches
to the hub.

### 11.3 Ordering and idempotency

- **Per-resource ordering:** consumers cannot assume events for the
  same `subject.ulid` arrive in `occurredAt` order. JetStream
  reorders under retry. Consumers sort by `occurredAt` when
  rendering and break ties by `eventId` (ULIDs are monotonic).
- **Idempotency:** a consumer must accept duplicate `eventId`
  silently (overwrite-or-ignore is acceptable; refuse-and-error is
  not). Producers do not guarantee exactly-once.

### 11.4 Failure modes

- **Emitter unavailable.** State-changing verbs MUST refuse to act
  if the audit emit fails (write-then-act, §11.1). Surface as a
  failed CRD reconcile with `AuditUnavailable` condition. The
  controller backs off and retries.
- **Oversized envelope.** Producers must enforce the 64 KiB
  snapshot cap (§8.1) and the 256 KiB total envelope cap before
  emit. Oversized payloads spill `payload`/`evidence` to object
  storage with pointers.
- **Schema-validation failure** at emit time (a producer ships
  the wrong `payload.@type`): emit returns an error and the
  controller logs at ERROR. No partial-event publication.

## 12. Consumer Contracts

### 12.1 The four canonical consumers

| Consumer                                  | Reads from                  | Latency target | Notes                                          |
|-------------------------------------------|------------------------------|----------------|------------------------------------------------|
| API server (`Audit.Query` / `Audit.Watch`)| JetStream + KV index         | sub-second     | live tail and recent-history queries           |
| UI per-resource audit tab                 | API server                   | sub-second     | renders Promotion timelines, Application history |
| DuckDB rebuild job                        | JetStream + object archive   | minutes        | feeds matrix analytical queries (ADR 0005)     |
| SIEM exporter (`contrib/`)                | JetStream tail               | seconds        | customer-operated; never blocks the producer    |

### 12.2 Schema-version handling

Consumers carry a *minimum* `schemaVersion` they accept and a
*maximum*. Within that window:

- Events with unknown `payload.@type`: store the payload as opaque
  JSON; surface the envelope.
- Events with a `schemaVersion` newer than the consumer's max:
  store but do not interpret. Log at INFO.
- Events with a `schemaVersion` older than the consumer's min: the
  consumer carries the frozen decoder for every prior version
  (per §5.3). This is the cost of the breaking-change protocol.

### 12.3 Backfill from the archive

A new consumer (a freshly added SIEM exporter, a rebuilt UI
indexer) requests a JetStream consumer cursor plus an
object-storage segment list, reads from the archive forward to
where JetStream still has events, then switches to live tail. The
schema is identical across both sources — `audit/v1` is `audit/v1`
whether read from JetStream or from object storage.

## 13. Event-Type Registry — Initial Set

The registry below is the canonical set for MVPs 0–2. Each entry is
`verb` + optional `payload.@type` + whether it requires `intent`.

### 13.1 CRD lifecycle (every Keleustes CRD)

Auto-emitted by the API server admission hook for every state-
changing write to a `keleustes.skaphos.io/v1alpha1` object.

| verb     | payload type        | intent required | notes                                                      |
|----------|---------------------|-----------------|-------------------------------------------------------------|
| `create` | `crd.write.v1`      | no              | `before=null`, `after=<object>`                            |
| `edit`   | `crd.write.v1`      | yes (UI), no (controller) | `before`/`after` both populated                  |
| `delete` | `crd.write.v1`      | yes             | `before=<object>`, `after=null`                            |
| `view`   | (none — envelope only) | no            | emitted for `view-audit` actions and sensitive resource reads |

`crd.write.v1` payload carries the JSON patch between `before` and
`after` for compact downstream rendering.

### 13.2 RBAC / Identity

| verb                | payload type             | intent required | notes                                  |
|---------------------|--------------------------|-----------------|----------------------------------------|
| `grant`             | `rolebinding.grant.v1`   | yes             | the RoleBinding being applied          |
| `revoke`            | `rolebinding.revoke.v1`  | yes             | binding removal                        |
| `binding-expired`   | `rolebinding.expired.v1` | no              | auto-emitted at validUntil             |
| `edit-role`         | `role.edit.v1`           | yes             | Role spec diff                         |
| `idp-added`         | `idp.added.v1`           | yes             | IdentityProvider created                |
| `idp-removed`       | `idp.removed.v1`         | yes             | IdentityProvider deleted                |
| `denied`            | `access.denied.v1`       | no              | RBAC refusal — `subject` is the resource that was denied; full request in payload |

### 13.3 Promotion

| verb                  | payload type                 | intent required |
|-----------------------|------------------------------|-----------------|
| `promote`             | `promote.v1`                 | yes             |
| `promotion-cancelled` | `promotion.cancelled.v1`     | yes             |
| `promotion-advanced`  | `promotion.advanced.v1`      | no (system)     |
| `promotion-completed` | `promotion.completed.v1`     | no (system)     |
| `approve`             | `approval.granted.v1`        | yes             |
| `deny-approval`       | `approval.denied.v1`         | yes             |
| `approval-expired`    | `approval.expired.v1`        | no (system)     |

### 13.4 Sync

| verb                | payload type             | intent required | notes                                                                |
|---------------------|--------------------------|-----------------|----------------------------------------------------------------------|
| `sync-started`      | `sync.started.v1`        | no              | SyncRun phase entered Running                                        |
| `sync-applied`      | `sync.applied.v1`        | no              | one or more objects applied; payload carries the per-object result list |
| `sync-pruned`       | `sync.pruned.v1`         | no              | objects pruned; carries the inventory diff (SKA-320 §8.1)             |
| `sync-failed`       | `sync.failed.v1`         | no              | terminal failure with reason                                          |
| `sync-completed`    | `sync.completed.v1`      | no              | terminal success                                                      |
| `sync-skipped`      | `sync.skipped.v1`        | no              | dependencies unmet or freeze window in effect                         |

### 13.5 Render (reserved by SKA-320)

| verb                  | payload type              | intent required |
|-----------------------|---------------------------|-----------------|
| `render-cache-hit`    | `render.cache.hit.v1`     | no              |
| `render-cache-miss`   | `render.cache.miss.v1`    | no              |
| `render-failed`       | `render.failed.v1`        | no              |
| `render-invalid`      | `render.invalid.v1`       | no              |
| `render-handoff-refused` | `render.handoff.refused.v1` | no          |

### 13.6 Git mutation

| verb                  | payload type            | intent required | notes                                |
|-----------------------|-------------------------|-----------------|--------------------------------------|
| `git-commit-opened`   | `git.commit.v1`         | no              | one commit pushed to a Git repo      |
| `git-pr-opened`       | `git.pr.opened.v1`      | no              | provider PR/MR URL in evidence       |
| `git-pr-merged`       | `git.pr.merged.v1`      | no              |                                      |
| `git-mutation-failed` | `git.mutation.failed.v1`| no              |                                      |

### 13.7 Break-glass

| verb                       | payload type              | intent required | notes                                                                              |
|----------------------------|---------------------------|-----------------|-------------------------------------------------------------------------------------|
| `break-glass`              | `breakglass.applied.v1`   | **yes**, plus `context.auditTicket` required | direct cluster mutation                            |
| `break-glass-pr-opened`    | `breakglass.pr.v1`        | no              | follow-up PR opening the change to Git (RBAC plan §3)                              |
| `break-glass-drift-resolved` | `breakglass.resolved.v1` | yes             | when the PR merges or the explicit revert lands                                    |

### 13.8 Agent

| verb                  | payload type             | intent required | notes                                                            |
|-----------------------|--------------------------|-----------------|------------------------------------------------------------------|
| `agent-registered`    | `agent.registered.v1`    | no              | agent joined; carries NKey fingerprint                            |
| `agent-claimed-work`  | `agent.claimed.v1`       | no              | agent took ownership of a SyncRun                                 |
| `agent-dropped-work`  | `agent.dropped.v1`       | no              | work returned to the queue (timeout, disconnect)                  |
| `agent-disconnected`  | `agent.disconnected.v1`  | no              |                                                                  |

Heartbeats are **not** audited. They are presence signals, not
actions. Aggregated agent health is on a separate metrics path
(ADR 0002).

### 13.9 System / configuration

| verb                | payload type             | intent required | notes                                                                  |
|---------------------|--------------------------|-----------------|------------------------------------------------------------------------|
| `config-changed`    | `system.config.v1`       | no              | operator-level config (FreezeWindow, redaction rules, etc.)             |
| `controller-elected`| `system.election.v1`     | no              | leader-election transition (one event per transition, not per heartbeat) |
| `migration-applied` | `system.migration.v1`    | no              | a schema-version migration ran (informational)                          |

### 13.10 Adding a new verb

1. Add the row to the registry (here, with a PR amending this plan
   until the plan promotes into an ADR).
2. Define the payload type in `internal/audit/payloads/<verb>.go`.
3. Implement `AuditType() string` and `Validate() error` on the
   payload struct.
4. Update producers; consumers are not required to update.

## 14. Failure Modes

| Failure                                          | Producer behavior                                          | Consumer behavior                                  |
|--------------------------------------------------|------------------------------------------------------------|----------------------------------------------------|
| Emitter unavailable, state-changing verb         | Refuse to act; `AuditUnavailable` condition; backoff       | n/a — no event was published                       |
| Emitter unavailable, observational verb (`view`) | Log at WARN; skip the audit emit                            | n/a — no event was published                       |
| Oversized envelope                                | Spill snapshots/payload to object storage (§8.1 oversize)   | Resolves the `@oversize` pointer when materializing |
| Producer ships wrong `payload.@type`             | Build-time validation fails; emit returns error             | Stores envelope, opaque payload, logs at INFO       |
| Consumer encounters unknown `payload.@type`      | n/a                                                         | Stores opaque; logs at INFO                         |
| Consumer encounters unknown `schemaVersion` (newer) | n/a                                                       | Stores; does not interpret; logs at INFO            |
| JetStream stream lost AND archive lost           | Producer keeps emitting; new events land in fresh stream    | Historical query returns "audit unavailable for ts<X"; ADR 0005 §4 |

## 15. Open Questions

1. **`partition` value semantics.** §5.4 reserves `partition` for
   SKA-324 to use as a sharded-controller-friendly hash prefix. The
   open question: should the partition value be derived from
   `subject.ulid` (consistent per-resource — good for fan-out) or
   `eventId` (uniform distribution — better for load balance). Both
   have plausible cases; defer to SKA-324 implementation.

2. **`requestId` propagation through Git Mutation.** When the Git
   Mutation Engine opens a PR in response to a Promotion, the PR
   description should carry the originating `requestId` so that a
   reviewer reading the PR can pull up the Keleustes audit trail.
   Mechanism is straightforward (commit-trailer or PR-body field);
   the open question is exact placement and whether the
   `requestId` survives downstream Git rewriting (rebases, squash
   merges). Tracked under the Git Mutation Engine plan.

3. **`actor.delegatedFrom` depth.** §6.5 caps nesting at one
   level. A multi-hop system→system→human chain falls back to
   `requestId` correlation. Whether the chain depth is sufficient
   for promotion-with-batch-approvals (where one human approval
   gates many follow-up system actions) needs validation against
   real MVP 2 promotion flows. May relax to depth 2 if the UI
   timeline grows awkward.

4. **CBOR-at-rest decision deadline.** §4.2 reserves CBOR for the
   archive. The benchmark gate is "JetStream → DuckDB rebuild
   takes > 5min for a typical 24h segment." Until we hit that,
   stay on JSON. Decision deferred to MVP 3 implementation.

5. **Redaction rule changes are themselves audit events
   (§8.2).** The chicken-and-egg case: how do you redact the
   `before`/`after` of the redaction-rule change without
   permanently losing what the previous rule looked like? The
   pragmatic answer is "snapshot the rule file content as
   payload; never redact rule-config content even if it matches
   the redaction patterns." Confirm with security review before
   MVP 0 lands.

## 16. Compliance with Prior Decisions

| Decision                              | This plan honors it by                                                                                            |
|---------------------------------------|-------------------------------------------------------------------------------------------------------------------|
| ADR 0003 (Git invariant)              | `break-glass.*` verbs are first-class with mandatory `intent` + `auditTicket`; PR-opened event closes the loop.    |
| ADR 0004 (RBAC) §247-252 (ULID)       | `eventId`, `sessionId`, and `subject.ulid` are ULIDs; surface in URL deep links.                                  |
| ADR 0004 (RBAC) §6.2 envelope         | Envelope (§3) is the formalization of the RBAC plan's JSON sketch; field positions preserved.                      |
| ADR 0005 (no RDBMS, JetStream)        | Persistence (§10) is JetStream + object storage + NATS KV; no RDBMS dependency.                                    |
| ADR 0005 §4 (acceptable historical loss)| Failure mode "stream lost AND archive lost" explicitly returns "audit unavailable for ts<X" rather than crashing. |
| ADR 0006 §4 (containment rule)        | `internal/audit/` is a Keleustes-owned package; `gitops-engine` is never imported here.                            |
| ADR 0002 (OpenTelemetry default)      | `traceparent` is the W3C trace context; producers populate it when present, consumers MAY index on it.             |
| ADR 0001 (plugin extension model)     | `AuditDestination` is a plugin surface (per ADR 0001); plugins receive envelopes in this schema's `audit/v1` form. |

## 17. Concrete Follow-ups

1. **SKA-332 (MVP 0 audit envelope emission)** — implements
   `internal/audit/Emit` + the API-server admission hook. Uses the
   stdout/log emitter for MVP 0; the JetStream emitter lands in
   MVP 1 (SKA-347).
2. **SKA-347 (MVP 1 audit pipeline JetStream end-to-end)** —
   wires the JetStream emitter, NATS KV index population, archive
   segmenter. Consumes the subject layout from SKA-324.
3. **SKA-324 (JetStream subject and stream layout design)** —
   takes the demands in §10 of this plan and turns them into the
   stream + subject hierarchy.
4. **SKA-385 (MVP 4 hash-chained audit)** — fills in the
   `chain.prev` and `chain.signature` reserved fields. The schema
   already has slots; SKA-385 specifies the hash function (likely
   SHA-256 over canonical JSON), the chain entry point, and the
   signature key management.
5. **New ticket (file as part of SKA-322 closeout): payload-type
   generator.** A `go generate`-driven generator over the §13
   registry that emits the Go payload structs + their `AuditType()` /
   `Validate()` methods, plus a JSON-schema file per type for the
   UI and SIEM exporter to consume. Defers hand-keeping registry
   sync across producers and the docs.
6. **New ticket: `internal/audit/redaction/` package** — the
   centralized redaction rule list referenced in §8.2. Owned by
   the platform team; rule additions require a security review
   sign-off recorded as an audit event.
7. **PROPOSAL.md** has audit references that pre-date the
   envelope. Cross-link or supersede during SKA-325 (PROPOSAL +
   CLAUDE refresh).

---

**When this plan stabilizes** (after SKA-324 has consumed §10 and
at least one engine has implemented `audit.Emit`), §1–§14 promote
into a new ADR (likely ADR 0007 — Audit envelope and event registry,
or sequenced after the Render Contract ADR if that lands first).
The §13 verb registry continues to grow under the additive protocol
in §5.1; the envelope itself does not.
