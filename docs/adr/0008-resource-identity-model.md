<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# ADR 0008 — Resource identity: natural key for addressing, durable engine-side ULID

- **Status:** Accepted
- **Date:** 2026-06-28
- **Deciders:** Platform Architecture (Skaphos)
- **Linear:** none dedicated — specifies an assumption shared by SKA-322 (audit
  envelope) and SKA-324 (JetStream/KV layout); implementation tickets to follow.
- **Related:** [ADR 0003](./0003-git-source-of-truth-invariant.md) (Git
  source-of-truth invariant), [ADR 0004](./0004-crd-based-rbac.md) (RBAC —
  `subject.ulid` stable-id requirement), [ADR 0005](./0005-distributed-runtime.md)
  (no-RDBMS state model — NATS KV hot indexes), [SKA-322 audit envelope](../plans/2026-05-audit-event-schema.md)
  (`action.subject.ulid`), [SKA-324 JetStream/KV layout](../plans/2026-05-jetstream-subject-and-stream-layout.md)
  (§4.5 resolution, `resource-identity` and `audit-index` buckets).
- **Refines:** ADR 0003 (records that derived identity is *not* written to Git,
  closing a "where does the ULID live" gap without weakening the Git invariant)
  and ADR 0005 (places the identity registry in the NATS KV tier). Specifies the
  origin of the `subject.ulid` that SKA-322 §6 and SKA-324 §4.4 already assume.

## Context and Problem Statement

The audit envelope (SKA-322 §6) carries `action.subject.ulid` — a stable id for
the resource a verb acts on — and SKA-324 §4.4 keys partitioning and the
`audit-index` KV bucket on that same `subject.ulid`. Both plans **depend on**
every audit-subject resource having a durable identity, but neither says where
that ULID is minted or how it stays durable. Left unspecified, the obvious
choice (key audit by name) silently fragments a resource's history the moment it
is renamed.

Three constraints make this non-trivial:

1. **Kubernetes names are immutable.** There is no rename operation; "renaming"
   an Application is delete-old + create-new. The new object gets a fresh
   `metadata.uid`, so `uid` is not a durable cross-rename identity either. Any
   identity not *carried through* the rename is lost.
2. **We will not write identity back into the user's Git.** Minting a ULID and
   committing it as an annotation into the customer's source repo couples the
   control plane to their repo for bookkeeping, creates commit noise, and races
   their own reconciliation. The operator decision is explicit: the ULID is
   engine state, not desired state. (Consistent with ADR 0003 — desired-state
   mutation goes through Git; *derived* state does not.)
3. **State must be reconstructable from CRDs + Git + JetStream, not Git alone**
   (ADR 0005). So a durable id that lives outside Git is acceptable *iff* it
   lives in the runtime tiers ADR 0005 already sanctions.

The forcing function: pick where the durable resource identity lives and what it
is keyed by, such that it survives the common rename, requires no Git write-back,
and is human-ergonomic for day-to-day use.

## Decision Drivers

- Audit and cross-CRD references must stay continuous across a rename.
- No write-back to the customer's Git repository.
- Humans, `keleustesctl`, and `kubectl` must address resources by name.
- Reconstructable from the ADR 0005 tiers (CRD status + NATS KV + JetStream).
- Renames are rare; do not over-engineer for an uncommon event.

## Considered Options

- **A — Name as the sole identity** (the Argo CD posture: no surrogate id).
- **B — `metadata.uid` as the durable id.**
- **C — ULID written back into the customer's Git** (annotation carried across
  the rename by the human/tooling).
- **D — Engine-minted ULID keyed by source path + target cluster, stored in
  NATS KV, cached in CRD status, never in Git.** *(chosen)*

## Decision Outcome

Chosen option: **D**. It is the only option that delivers audit/cross-ref
continuity across a rename *without* writing to the customer's Git, while keeping
all addressing on the natural key.

### 1. Natural key for addressing, durable ULID underneath (surrogate-key pattern)

All human- and machine-facing interaction addresses resources by **name** — UI
routes, `keleustesctl`, the REST contract (`/applications/{name}`), and
`kubectl`. Separately, every audit-subject resource carries a **durable ULID**:
the `action.subject.ulid` of SKA-322. The UI/CLI never ask a human to type or
read a ULID.

### 2. The ULID is keyed by source path + target cluster

The durable ULID is resolved from `xxhash64(sourcePath + targetCluster)` — the
resource's Git source coordinates (`spec.source` repo + path) plus the cluster it
deploys to, i.e. the deployment unit (the matrix cell). A rename changes
`metadata.name`, not the source path, so the key — and the audit trail keyed on
it — is unchanged.

### 3. Engine-resolved via NATS KV, never written to Git

The reconciler computes the key and looks the ULID up in the `resource-identity`
NATS KV bucket (SKA-324 §6); on a miss it mints a ULID, writes it to the bucket,
and caches it in the resource's `status`. Writing `status` is a normal
controller operation — it is not a mutation of the customer's desired-state Git,
so ADR 0003 holds.

### 4. Rename semantics

- **Rename only** (name changes, same path + cluster) → same key → **same ULID**
  → continuous history. This is the common case and it Just Works.
- **Path move or cluster retarget** → new key → **new ULID** → a new logical
  identity. Carrying continuity across a path move is the team's responsibility,
  not the engine's. This is a deliberate, documented boundary, not a bug.

### 5. Best-effort durability

The mapping is NATS KV (hot) backed by the event log. A control-plane reset that
loses the bucket may re-mint; this is **accepted** — identity is stable in steady
state, not eternal. Where possible the controller re-seeds a lost mapping from
the most recent `audit-index` entry for the key before minting fresh, but this is
a mitigation, not a guarantee.

### Consequences

- **Positive**
  - Audit and cross-CRD references survive the common rename, with no Git
    write-back.
  - Reuses machinery SKA-324 already specifies (NATS KV hot indexes,
    `xxhash64` keying) — one new bucket, no new subsystem.
  - Addressing stays name-based: `kubectl`-native and human-ergonomic.
  - The ULID is reconstructable from the ADR 0005 tiers; Git is never on the
    identity path.
- **Negative**
  - A rename *plus* a path move (or cluster retarget) starts a new identity and
    breaks continuity. It is a sharp edge a team can trip over; mitigated only by
    documentation.
  - Best-effort durability means a catastrophic NATS reset can fragment history.
    The `audit-index` re-seed reduces but does not eliminate this.
  - Two identity concepts (name + ULID) must be kept straight in code and docs.
  - The ULID is intentionally **not** in Git, so Git alone is not a complete
    identity backup — a deliberate trade against ADR 0003 cleanliness.
- **Neutral**
  - The UI/matrix carry the ULID for history-linking but never display or route
    by it.
  - `metadata.uid` remains available as a per-incarnation handle; it is simply
    not the durable cross-rename id.

## Pros and Cons of the Options

### A — Name as sole identity
- Good: simplest; fully k8s-native; zero new machinery.
- Bad: audit history and cross-refs fragment on every rename; the `subject.ulid`
  that SKA-322/324 require would have to be dropped or faked.

### B — `metadata.uid`
- Good: free from the API server; no Git write-back; uniquely identifies an
  object incarnation.
- Bad: a rename (delete-create) mints a new `uid`, so it does **not** survive the
  one event we care about. Solves attribution, not continuity.

### C — ULID written back into the customer's Git
- Good: ULID travels with the resource across any rename or move; reproducible
  from Git alone.
- Bad: the control plane mutates the customer's source repo for bookkeeping —
  commit noise, reconciliation races, and a coupling the operator explicitly
  rejected. Heaviest option.

### D — Engine-minted, path+cluster-keyed ULID in NATS KV *(chosen)*
- Good: survives the common rename, no Git write-back, reuses SKA-324 KV,
  name-based addressing preserved.
- Bad: path move = new identity; best-effort durability; identity not in Git.

## Compliance and follow-ups

1. **`docs/DECISIONS.md`** — add the ADR 0008 row; note that SKA-322 and SKA-324
   now reference it; bump the penciled "Likely ADR 000N" projections for the
   in-flight interim contracts so the roadmap stays honest. *(this PR)*
2. **SKA-324** — §4.5 "Origin of `subject.ulid`" and the `resource-identity` KV
   bucket are added. *(this PR)*
3. **SKA-322** — `action.subject.ulid` annotated with its origin (this ADR /
   SKA-324 §4.5). *(this PR)*
4. **`ui/docs` design spec** — `docs/design/ui-design-spec.md` §2.5 already
   states the natural-key + durable-ULID model; add a `see ADR 0008` pointer when
   PR #26 lands. *(follow-up)*
5. **Implementation tickets (MVP 1)** — reconciler/admission minting + status
   caching of the ULID; immutability enforcement on the cached `status` ULID; the
   `resource-identity` bucket provisioning under SKA-324's KV setup. *(new Linear
   work)*
