<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Render Contract & Inventory Model

- **Status:** Draft — 2026-05-17
- **Linear:** SKA-320 (this plan); SKA-339 (Sync Engine wrapper) and SKA-340 (Render Engine) consume the contract defined here.
- **Promotes into:** a future ADR co-located with ADR 0006. Until then, this document is authoritative for any code that touches the Render boundary.
- **Supersedes:** [engine plan §8 action 6](./2026-05-engine-boundaries-and-technology-integration.md) (the "Render Contract & Inventory Model" deep-dive it asked for).
- **Related:** [ADR 0001](../adr/0001-plugin-extension-model.md) (§6 — Render is not a plugin surface), [ADR 0003](../adr/0003-git-source-of-truth.md) (Git invariant), [ADR 0005](../adr/0005-distributed-runtime.md) (object-storage cache, agent execution), [ADR 0006](../adr/0006-engine-boundaries.md) (engine boundaries; `gitops-engine` containment rule), [gitops-engine spike report](./2026-05-gitops-engine-spike.md) (handoff Q3 / Q4).

## 1. Purpose and Scope

This plan pins down the Go types and behavioral contract that sit on the
Render boundary — the seam between Keleustes-authored rendering and the
rest of the system (Sync Engine, Diff Engine, agent transport, audit).

Every later story that touches manifest production or pruning will build
on these types. Without this contract:

- The Sync Engine wrapper (SKA-339) would have to invent the shape of
  what it consumes, and the shape would drift.
- The Render Engine (SKA-340) would have nothing to test against beyond
  "produces some objects."
- The agent transport (SKA-321 deep-dive, MVP 2) would not know what
  payload shape it must carry from hub to agent.
- The audit pipeline would lack a render-source trace it can persist.

**In scope:** `RenderRequest`, `RenderResult`, the `Inventory` shape,
pruning rules, content-addressing for the render cache, the handoff
into `gitops-engine`, failure-mode contracts.

**Out of scope:** Specific renderer implementations (Kustomize, Helm,
raw — those land with SKA-340). Plugin surfaces (Render is explicitly
not pluggable — ADR 0001 §6). Apply-side semantics (sync waves, hooks,
SSA field manager — those belong to the Sync Engine plan once it is
written).

## 2. Where the Render Boundary Sits

```
                                  ┌───────────────────────────────┐
                                  │ internal/render               │
                                  │   - Renderer (Kustomize/Helm/ │
                                  │     raw)                      │
                                  │   - inventory extraction      │
   Application + Release +        │                               │
   DeploymentTarget + Revision    │   RenderRequest → RenderResult│
   (typed inputs from CRDs and    │   (pure function; no cluster  │
   Source revision)               │    reads, no Git fetch here)  │
   ────────────────────────────►  │                               │
                                  └─────────────┬─────────────────┘
                                                │
                                                ▼
                              ┌──────────────────────────────────────┐
                              │ internal/sync (wrapper around        │
                              │ gitops-engine, MVP 1)                │
                              │                                       │
                              │   RenderResult.Objects ─► gitops-     │
                              │   engine sync.ReconciliationResult    │
                              │   .Target                             │
                              │                                       │
                              │   pkg/cache live-state ─► .Live       │
                              │   annotation split ─► .Hooks          │
                              └──────────────────────────────────────┘
```

A few invariants drop out of this picture:

- **`internal/render/` never imports `gitops-engine`.** The containment
  rule in ADR 0006 §4 limits engine imports to `internal/sync/`,
  `internal/diff/`, `internal/health/`, `internal/kube/`. Render is on
  the list of packages *protected* by that rule, not on it.
- **Render is a pure function.** Same inputs in → same `RenderResult`
  out, including the same content hash. No cluster reads, no Git
  fetches, no clock reads beyond a single canonical timestamp supplied
  by the caller. This is what makes `(inputs) → hash → object-storage
  cache key` work.
- **The flat object list crosses the boundary.** Hook splitting,
  annotation translation, and live-state correlation all happen on the
  Sync Engine side. Render does not know about hooks.

## 3. `RenderRequest`

```go
// internal/render/types.go
package render

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    keleustesv1alpha1 "github.com/skaphos/keleustes/api/v1alpha1"
)

// RenderRequest is the complete input to a single render call.
// It is a snapshot — the caller has already resolved the live
// Application, Release, and DeploymentTarget objects and is asking
// Render to produce manifests for that exact tuple.
type RenderRequest struct {
    // Application is the resolved Application CR. The caller passes
    // the full object (not a ref) so Render does not perform any
    // Kubernetes reads of its own; this keeps Render a pure function.
    Application keleustesv1alpha1.Application

    // Release is the resolved Release CR pinned for this render.
    // Release.Spec.Artifacts is the source of every concrete version
    // string injected into the rendered output.
    Release keleustesv1alpha1.Release

    // Target captures the destination's identity and the small set of
    // facts the renderer is allowed to consult about it. Anything not
    // in TargetContext is unavailable to renderers by design — values
    // overrides come from Git (Application.Spec.SourceRefs and the
    // Release), never from the live cluster.
    Target TargetContext

    // Revision is the source commit SHA or content digest of the
    // manifest source that this render call is materializing. It is
    // recorded verbatim in RenderResult.Trace and contributes to the
    // content hash.
    //
    // Required and immutable for a given render.
    Revision string

    // ValueSources is the ordered list of Git-backed values overlays
    // applied during rendering. Each entry is a SourceRef + path inside
    // that source. Order matters; later entries override earlier ones
    // by renderer-specific rules (helm: deep merge; kustomize: patch).
    ValueSources []ValueSource

    // Determinism pins the inputs that would otherwise be implicit —
    // the renderer version, the canonical timestamp, the Helm chart
    // schema version, etc. Two RenderRequests with identical
    // Determinism blocks MUST produce identical content hashes.
    Determinism DeterminismInputs
}

// TargetContext is the set of facts a renderer may consult about the
// destination. Anything outside this struct is off-limits — the
// renderer must not call out to the cluster.
type TargetContext struct {
    // Target is the resolved DeploymentTarget CR.
    Target keleustesv1alpha1.DeploymentTarget

    // ClusterFacts is a small, hub-curated set of properties about
    // the target cluster. Populated by the SyncRun controller before
    // calling Render; never by the renderer itself.
    ClusterFacts ClusterFacts
}

// ClusterFacts is a deliberately narrow projection of cluster
// metadata. Adding fields here requires explicit review — every field
// becomes part of the content hash and a new way for renders to
// non-determine.
type ClusterFacts struct {
    // KubernetesVersion is the discovered server version of the
    // target (e.g., "v1.32.4"). Lets Helm chart capability gates
    // resolve deterministically without a live API call from the
    // renderer.
    KubernetesVersion string

    // APICapabilities is the set of group/version/kind tuples the
    // target advertises, captured by the controller at SyncRun
    // creation time. Sorted; hashed as part of Determinism.
    APICapabilities []GroupVersionKind
}

// GroupVersionKind is the canonical key for capability matching.
type GroupVersionKind struct {
    Group   string
    Version string
    Kind    string
}

// ValueSource pins a single Git-backed values overlay.
type ValueSource struct {
    // SourceRef is the name (namespace/name) of the Source CR the
    // overlay is read from.
    SourceRef string

    // Path inside the source repository or chart.
    Path string

    // Revision is the source commit SHA or chart digest. Independent
    // of RenderRequest.Revision because an Application can mix
    // sources (a chart at one digest, values overlays at others).
    Revision string
}

// DeterminismInputs makes implicit render inputs explicit so the
// content hash is honest.
type DeterminismInputs struct {
    // RendererVersion is the Render package's semantic version string
    // (e.g., "render/v0.3.1"). A bump invalidates every cached
    // RenderResult, by design.
    RendererVersion string

    // RenderTime is the canonical timestamp the renderer may stamp
    // into rendered objects (Helm "Release.Time" equivalent). Set by
    // the caller; commonly the SyncRun.Status.StartedAt.
    RenderTime metav1.Time

    // HelmKubeVersion overrides Helm's `--kube-version`. When empty,
    // falls back to ClusterFacts.KubernetesVersion.
    HelmKubeVersion string
}
```

A few things this shape encodes on purpose:

- **No `*rest.Config`, no `kubernetes.Interface`, no `discovery.Client`.**
  Render cannot reach the cluster. The caller pre-collects the small
  set of facts Render is allowed to see (`ClusterFacts`) at SyncRun
  creation time.
- **No `map[string]any` for values.** Values are a list of pointers to
  Git-backed source overlays, not raw blobs. This forces every value
  that influenced a render to be a trail back to Git (ADR 0003).
- **`Determinism` is not optional.** A render with no `RendererVersion`
  set should fail validation, not silently hash to a different value
  than the next caller's render.

## 4. `RenderResult`

```go
// RenderResult is the complete output of a single render call.
type RenderResult struct {
    // Objects is the flat list of rendered Kubernetes objects, in the
    // order the renderer emitted them. The Sync Engine wrapper hands
    // this list (after annotation translation) to gitops-engine as
    // ReconciliationResult.Target. Hook objects stay in this list —
    // gitops-engine's splitHooks separates them by annotation.
    //
    // Each object is *unstructured.Unstructured; typed clients are
    // built by callers that need them.
    Objects []RenderedObject

    // Inventory is the materialized inventory of every object in
    // Objects. Built alongside the render so callers never need to
    // re-walk the object list to know what should be pruned.
    Inventory Inventory

    // Warnings is non-fatal render output a human should see: a chart
    // emitted nothing for an enabled flag, a kustomization included
    // a deprecated patch shape, a referenced ValueSource was empty.
    // Warnings do not flip the render to failed.
    Warnings []RenderWarning

    // ContentHash is the deterministic hash of the request that
    // produced this result (see §6). Same RenderRequest → same
    // ContentHash. Used as the object-storage cache key.
    ContentHash string

    // Trace records, per emitted object, which renderer input
    // produced it: which Kustomization layer, which Helm chart
    // template, which raw manifest file. Required for the UI's
    // "where did this object come from?" view and for audit.
    Trace []RenderTraceEntry
}

// RenderedObject pairs an unstructured object with its inventory key
// so callers do not need to recompute the key from labels.
type RenderedObject struct {
    Object *unstructured.Unstructured
    Key    ResourceKey
}

// RenderWarning is one human-facing message about a render.
type RenderWarning struct {
    // Source identifies the renderer (kustomize|helm|raw) that
    // emitted the warning.
    Source string
    // Message is the human-readable message.
    Message string
    // Subject is an optional ResourceKey when the warning is about a
    // specific object.
    Subject *ResourceKey
}

// RenderTraceEntry records the provenance of a single rendered object.
type RenderTraceEntry struct {
    Object ResourceKey
    // Origin describes the renderer-specific input file or chart
    // template that emitted this object (e.g., "charts/web/templates/
    // deployment.yaml" or "overlays/prod/kustomization.yaml").
    Origin string
    // SourceRef is the name of the Source CR whose ValueSource (or
    // BasePath) emitted this object. Empty for raw manifests inlined
    // in the Application's primary source.
    SourceRef string
}
```

`ResourceKey` is borrowed from `gitops-engine`'s
`pkg/utils/kube.ResourceKey` shape verbatim:

```go
type ResourceKey struct {
    Group     string
    Kind      string
    Namespace string
    Name      string
}
```

We re-declare it in `internal/inventory/` rather than aliasing the
upstream type, because the containment rule (ADR 0006 §4) forbids
`internal/render/` from importing `gitops-engine`. The two definitions
must stay structurally identical; the Sync Engine wrapper converts
between them with a trivial copy.

## 5. Inventory Model

### 5.1 Why inventory exists

Inventory answers three questions the Sync Engine cannot answer from
just the rendered list:

1. **What should we prune?** — "Last run we wrote these objects;
   this run we are not writing some of them; therefore delete the
   ones that disappeared."
2. **Who owns this live object?** — Two Applications might render
   into the same namespace. The cluster cache sees one
   `ConfigMap/example`; without inventory we cannot say which
   Application owns it.
3. **Has the desired state actually changed?** — A render that
   produces structurally identical output to last time should not
   trigger a SyncRun.

Inventory is therefore the durable side of "what is Keleustes
responsible for on this target", separate from the transient
RenderResult.

### 5.2 `Inventory` type

```go
// internal/inventory/inventory.go
package inventory

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Inventory is the set of objects an Application owns on a
// DeploymentTarget. It is the source of truth for pruning and for
// "is this object ours?" answers.
type Inventory struct {
    // Owner identifies which Application + Release + Target this
    // inventory belongs to. Encoded verbatim into the OwnerLabels
    // applied to every owned object.
    Owner Owner

    // Entries is the per-object inventory record, keyed by
    // ResourceKey. Order is not significant; callers should not
    // depend on map iteration order.
    Entries map[ResourceKey]Entry

    // RecordedAt is when this inventory snapshot was produced. Used
    // for cache TTL and for "last applied" UI surfaces.
    RecordedAt metav1.Time
}

// Owner is the {Application, Release, Target} triple expressed as
// the label values Keleustes stamps onto every owned object.
type Owner struct {
    // Application is the namespace/name of the Application CR.
    Application string
    // Release is the namespace/name of the Release CR pinned for
    // this inventory.
    Release string
    // Target is the namespace/name of the DeploymentTarget CR.
    Target string
}

// Entry is a single object's inventory record.
type Entry struct {
    // ContentHash is the canonical hash of the object's spec at
    // render time. Lets the Sync Engine short-circuit a no-op apply.
    ContentHash string

    // Hook is true when the object carries a Keleustes or Argo CD
    // hook annotation. Hook resources are reported in the inventory
    // so audit knows they existed, but the prune set excludes them
    // (gitops-engine manages hook deletion via hook-delete-policy).
    Hook bool

    // Skip is true when the object carries
    // `keleustes.skaphos.io/prune=false`. The Sync Engine treats
    // Skip=true as "leave alone forever after first apply": no
    // prune, no diff, no health roll-up.
    Skip bool
}
```

### 5.3 Owner labels stamped on every object

Render must inject the following labels onto every emitted object
(both into `metadata.labels` and into any pod template `selector`
where the object is a workload — same convention Argo CD uses for
its `app.kubernetes.io/instance`):

| Label key                                       | Value                                                         |
|-------------------------------------------------|---------------------------------------------------------------|
| `app.kubernetes.io/managed-by`                  | `keleustes`                                                   |
| `keleustes.skaphos.io/application`              | `<application-namespace>.<application-name>`                  |
| `keleustes.skaphos.io/release`                  | `<release-namespace>.<release-name>`                          |
| `keleustes.skaphos.io/target`                   | `<target-namespace>.<target-name>`                            |
| `keleustes.skaphos.io/inventory-version`        | `v1` (bumped when the label set changes)                      |

`keleustes.skaphos.io/inventory-version` is the migration knob: when
the label set ever changes, the controller honors both the old and
new label set during a transition window. Without it, every existing
deployment would orphan on upgrade.

### 5.4 Where inventory is stored

The inventory snapshot lives next to the `Deployment` CR that
records the apply outcome — *not* on the cluster objects themselves
and *not* in a separate `Inventory` CRD. Two storage strategies,
selected by inventory size:

- **Small (default, ≤ ~500 entries):** inline on
  `Deployment.status.inventory`. Fits the common case; survives any
  cluster-scope event without an extra object.
- **Large (≥ ~500 entries OR ≥ ~256 KiB serialized):** offloaded to
  object storage at
  `inventory/<application-ns>.<application-name>/<deployment-ns>.<deployment-name>.json`,
  with a pointer on `Deployment.status.inventoryRef`. The threshold
  is enforced by the Sync Engine, not by the user.

Why not a dedicated `Inventory` CRD? Reconciler thrash. Inventory
changes every SyncRun; an `Inventory` CRD would have a separate
informer cycle, a separate ResourceVersion arms race, and one extra
admission-webhook hop on every apply. `Deployment` already changes
every SyncRun; piggy-backing on its status field has zero additional
notification cost.

Why not a `ConfigMap`? Because then RBAC for "see the inventory" is
the same as "see this `ConfigMap`", which is too broad for the
multi-tenant model in ADR 0004.

### 5.5 ResourceKey stability

```go
type ResourceKey struct {
    Group     string
    Kind      string
    Namespace string
    Name      string
}
```

Two stability rules:

1. **`Version` is not part of the key.** A CRD that ships
   `apps/v1beta1` and later `apps/v1` Deployments is the same
   object; storage version is a server detail. This is the same call
   `gitops-engine` makes — the contracts line up by design.
2. **Cluster-scoped objects use `Namespace: ""`.** Renderers must
   not put placeholder values like `"-"` or `"<cluster>"`. Empty
   string is the canonical absence.

Implementations of `ResourceKey.String()` for use in logs and label
values follow the form `<group>/<kind>/<namespace>/<name>`, with
`""` for missing fields preserved (so a cluster-scoped object reads
`apiextensions.k8s.io/CustomResourceDefinition//example.com`).

## 6. Content Addressing for the Render Cache

### 6.1 Why content-address

Rendering is the single most expensive operation in MVP 1. Helm
chart rendering across 1K Applications is multiple minutes if cold;
a cache hit collapses that to a lookup. Two consumers benefit:

- **Hub-side Sync Engine:** avoid re-rendering when the Application
  and Release have not changed.
- **Agent-side Sync Engine (MVP 2):** the hub renders, stores the
  result by content hash in object storage (ADR 0005 §11), and the
  agent fetches by hash. Or — equivalently — the agent renders
  locally with the same inputs and gets the same hash, proving it
  read the same Git.

### 6.2 Hash inputs

The `ContentHash` is `sha256(canonical-json(RenderRequest))` after
the request has been normalized:

1. All metadata maps sorted by key.
2. All slices sorted where order is not semantically significant
   (`APICapabilities`, etc.). Slices where order matters
   (`ValueSources`, `Objects` in the result, `RenderTraceEntry`)
   are preserved.
3. Timestamps quantized to seconds. `RenderTime` is included by
   default; an MVP 2 opt-out (`Determinism.OmitRenderTime`) is the
   escape hatch for charts that don't actually stamp it anywhere.
4. The `RendererVersion` is folded in so a renderer bump cleanly
   invalidates the entire cache.

The same canonicalization function is used to compute per-object
`Entry.ContentHash`, scoped to the object's spec (metadata
annotations Kubernetes adds at admission — `kubectl.kubernetes.io/
last-applied-configuration`, etc. — are stripped before hashing).

### 6.3 Cache layout

```
<bucket>/render/<content-hash>.tar.zst   # tarball of YAML objects
<bucket>/render/<content-hash>.meta.json # RenderResult minus Objects
```

- Two files because the metadata is small and read on every cache
  probe; the tarball is large and only read on a hit.
- Cache lifetime: bounded by the most recent `Release.spec` for the
  Application + 7-day grace period. The grace period covers rollback
  windows.
- A cache miss followed by a successful render must publish both
  files atomically (write under a UUID prefix, then rename). Partial
  writes leak storage but do not corrupt subsequent lookups.

### 6.4 Cache audit

Every cache hit and miss is emitted as an audit event
(`render.cache.hit` / `render.cache.miss`) with the ContentHash,
ApplicationRef, ReleaseRef, and TargetRef. Lets the team measure
hit rate and detect non-determinism leaks (two seemingly-identical
requests hashing differently means a renderer is reading something
it shouldn't).

The audit-event schema for these is finalized in SKA-322; the
Render plan reserves the two event types.

## 7. Handoff to `gitops-engine`

Inside `internal/sync/`:

```go
func (e *Engine) buildReconciliation(
    result render.RenderResult,
    cache cache.ClusterCache,
) (sync.ReconciliationResult, error) {
    translated := translateAnnotations(result.Objects)  // §6.3 of ADR 0006

    target := make([]*unstructured.Unstructured, 0, len(translated))
    for _, o := range translated {
        target = append(target, o.Object)
    }

    live, err := cache.FilterByOwner(result.Inventory.Owner)
    if err != nil {
        return sync.ReconciliationResult{}, err
    }

    // gitops-engine's splitHooks separates Target into Target+Hooks
    // by annotation. We pass everything as Target and let the engine
    // split — keeps the seam thin.
    return sync.Reconcile(target, live, e.namespace, e.resInfo), nil
}
```

Three things this codifies:

- **Hooks travel in `Target`.** Render does not separate them.
  `gitops-engine`'s `splitHooks` is the one place hook
  classification lives.
- **`Live` comes from `pkg/cache`.** One cache per `DeploymentTarget`
  (ADR 0006 §4 table; spike report Q4). The Sync Engine wrapper
  asks the cache for objects matching the owner labels.
- **Annotation translation happens at the seam.** Render emits
  `keleustes.skaphos.io/sync-wave` and `…/hook`; the wrapper
  translates to `argocd.argoproj.io/...` only when the engine
  needs the Argo-prefixed names (ADR 0006 §7).

## 8. Pruning Rules

### 8.1 The set-difference rule

```
to_prune = previous_inventory.Entries.Keys() − current_inventory.Entries.Keys()
         − {entries where Skip is true}
         − {entries where Hook is true}
```

`to_prune` is computed on the hub side from two inventory snapshots
(previous = current `Deployment.status.inventory`; current = the
just-produced `RenderResult.Inventory`). The result is handed to
`gitops-engine`'s sync with a prune flag per object.

### 8.2 Hand-off between Applications

When an object moves from Application A to Application B (rare but
real — extraction of a service into its own Application), pruning
must not delete it.

Mechanism:

1. Application B's render emits the object with B's owner labels.
2. The Sync Engine compares B's inventory against A's inventory
   (the cluster cache reports the live object with A's labels
   still attached).
3. On observing the object as a hand-off candidate, the wrapper
   issues an SSA patch that updates only the owner labels (one
   field manager — `keleustes-handoff`), then proceeds with
   B's normal apply.
4. A's next render will see the object as "no longer in my live set
   under my labels" and will *not* schedule a prune (set-difference
   only prunes objects whose labels still claim A).

Hand-off is **explicit**, not magical: it requires both Applications
to be in the same Project (ADR 0004) and a one-time
`keleustes.skaphos.io/handoff-from=<A-ns>.<A-name>` annotation on
the moving objects in B's render. Without the annotation the object
is treated as a normal apply and A's prune set still contains it —
the controller will refuse the SyncRun with `HandoffConflict` until
the operator confirms intent.

### 8.3 CRD-and-instance ordering

A render that emits both a CRD and instances of that CRD must apply
the CRD first and prune in the opposite order. `gitops-engine`'s
sync-wave annotation is the existing knob (Argo's convention is
`apiextensions.k8s.io/CustomResourceDefinition` at wave −1); we
adopt the same default. Renderers may override per-object.

The corner case is *prune* ordering: if both the CRD and its
instances drop out of the new inventory, instances must be deleted
first, otherwise the CRD's `Established` condition is removed
while live instances still exist and finalizers wedge.

The Sync Engine implements this as: prune by descending sync-wave
(opposite of apply order). Renderers do not need to do anything
special.

### 8.4 Skip rules

An object with `keleustes.skaphos.io/prune=false`:

- Is recorded in inventory with `Skip: true`.
- Is excluded from the prune set even when it drops out of the next
  render.
- Is excluded from drift detection (Diff Engine treats it as
  read-only).
- Is included in audit and UI ("this object is owned but
  intentionally not reconciled").

Common uses: `PersistentVolumeClaim` with `Retain` policy,
operator-managed `Secret`s the Application installs once and
delegates to a controller.

### 8.5 Deletion propagation

The Sync Engine uses `foreground` propagation for `Namespace`s and
CRDs, `background` for everything else. Foreground is necessary so
the cluster's garbage collector tears down children before the
parent disappears — without it, pruning a `Namespace` leaves
orphaned objects discoverable for hours.

## 9. Failure Modes

### 9.1 Render OOM

A render exceeding its per-call memory budget (default 512 MiB,
tunable per Application class) is killed with `RenderFailed:
ResourceExhausted`. No partial output is cached. The Sync Engine
publishes a `RenderFailed` condition with the budget that was
exceeded. The next SyncRun retries at most twice before backing off
exponentially. No automatic budget bumping — the operator must
intervene because a chart hitting 512 MiB is almost always a chart
bug.

### 9.2 Render timeout

Render runs under a `context.Context` with a default 5-minute
deadline. Cancellation cascades:

- Helm template subprocess receives `SIGTERM` then `SIGKILL` after
  10s.
- Kustomize build (in-process, library-only per ADR 0006 §5)
  honors the context check at every file read.

Timeout produces `RenderFailed: DeadlineExceeded`. Same retry
shape as OOM.

### 9.3 Invalid manifests

Render succeeded but produced objects that fail server-side
dry-run validation. This is *not* `RenderFailed`. Semantics:

- The render output is cached normally (the bytes are real).
- `RenderResult.Warnings` lists each invalid object with the
  validation message.
- The Sync Engine surfaces `RenderInvalid` (not `RenderFailed`) as
  the condition.
- The SyncRun stays in `Rendering` until the next render: the
  operator presumably fixes Git, the next render produces valid
  output, and the SyncRun continues. No automatic retry on
  `RenderInvalid` — Git is the queue.

The distinction matters because operations runbooks treat them
differently: `RenderFailed` is "Keleustes had a problem";
`RenderInvalid` is "your manifests are wrong."

### 9.4 Missing CRD at apply time

Not a render failure. The Sync Engine handles via:

- Per-Application CRD dependencies declared on
  `Application.spec.dependencies.crds` (ADR 0006 §8).
- Sync-wave ordering when the CRD and instances ship together.
- An explicit `WaitingForCRDs` condition on the SyncRun when an
  unmet dependency holds the apply.

Render itself does not validate that the target cluster has the
right CRDs installed — `APICapabilities` in `ClusterFacts` makes
that information *available* to chart capability gates, but the
contract is "render whatever Git says; let the Sync Engine decide
when to apply."

## 10. Open Questions

1. **Per-object hash collisions.** Two different `unstructured`
   inputs that canonicalize to the same JSON would collide on
   `Entry.ContentHash`. With sha256 over canonicalized JSON,
   collision probability is negligible — but the canonicalization
   itself is non-trivial. The Renderer's canonicalizer must be
   property-tested before MVP 1 lands.

2. **Cluster-scope `APICapabilities` snapshotting.** `ClusterFacts.
   APICapabilities` is captured at SyncRun creation time. If the
   target cluster installs a new CRD between SyncRun creation and
   render, the render does not see it. Two choices:
   (a) accept the staleness and require a re-trigger on CRD events
   (current direction), or
   (b) re-snapshot inside Render. (b) violates "Render is a pure
   function." Open until SKA-340 implementation forces the issue.

3. **Render cache GC.** Per §6.3 the cache is bounded by
   "most-recent Release + 7d." Concretely: a sweeper job walks
   the bucket and deletes entries whose ContentHash does not
   appear in any current Release or Deployment. Sweeper cadence
   and ownership (hub controller? separate Job?) — open.

4. **Inventory storage cutover.** When an Application crosses the
   inline → object-storage inventory threshold, who triggers the
   migration? Options: (a) Sync Engine on every SyncRun checks
   inventory size and migrates if needed; (b) a one-shot
   admission webhook on `Deployment` status updates. (a) is
   simpler and chosen as the default; (b) is the fallback if the
   webhook hop becomes a hot path.

5. **Renderer determinism conformance tests.** A `render/conformance/`
   suite that takes a fixture (Application + Release + Target) and
   runs Render 100× to assert identical `ContentHash`. Belongs to
   SKA-340, but the suite's fixture format is part of the
   contract — should it be defined here? Tentatively yes; will
   land as `internal/render/conformance/` fixtures alongside
   SKA-340 implementation.

## 11. Compliance with prior decisions

| Decision               | This plan honors it by                                                                                                |
|------------------------|------------------------------------------------------------------------------------------------------------------------|
| ADR 0001 §6 (no plugin) | Render boundary types live in `internal/render/`; no extension point exposed; renderers are first-party packages only. |
| ADR 0003 (Git invariant)| `ValueSources` only references `Source` CRs (Git-backed); `ClusterFacts` is narrow and pre-computed by the controller. |
| ADR 0004 (RBAC)        | `Owner.Application` and `Owner.Target` are namespace-qualified so RBAC scoping works against label selectors directly.  |
| ADR 0005 §11 (object store) | Render cache layout uses the bucket layout reserved in the runtime plan; cache audit events flow through JetStream.    |
| ADR 0006 §4 (containment)| `internal/render/` imports no `gitops-engine` package; `internal/sync/` is the one place the conversion happens.      |
| ADR 0006 §5 (library-only)| Renderers use library APIs (Kustomize, Helm SDK); no exec — preserves the deterministic-hash invariant in §6.         |
| ADR 0006 §7 (annotations)| Render emits Keleustes-prefixed; Sync Engine wrapper translates to Argo-prefixed at the gitops-engine seam.           |

## 12. Concrete follow-ups this plan enables

1. **SKA-340 Render Engine** — implements `Renderer` (Kustomize, Helm,
   raw subpackages) against this contract. Acceptance test: every
   fixture in `internal/render/conformance/` produces an identical
   `ContentHash` across 100 runs and across two machines.

2. **SKA-339 Sync Engine wrapping `gitops-engine`** — consumes
   `RenderResult` and `Inventory`, builds `ReconciliationResult`,
   drives `SyncContext.Sync()` / `GetState()`. The phase translator
   already exists in `internal/sync/engine.go` (commit `cf74943`).

3. **New ticket (file as part of SKA-320 closeout): `internal/inventory/`
   package scaffold** with `ResourceKey`, `Inventory`, `Owner`,
   `Entry`, the canonical-JSON canonicalizer, and unit tests for
   the canonicalizer's stability properties.

4. **New ticket: render cache GC sweeper** — answers open question
   §10.3. Blocks at most one Application going to MVP 2; not on the
   MVP 1 critical path.

5. **SKA-322 (Audit Event Schema)** must reserve `render.cache.hit`,
   `render.cache.miss`, `render.failed`, `render.invalid`,
   `render.handoff.refused` event types when it is authored.

6. **The annotation translator referenced in §7 (`internal/sync/translate.go`)**
   is called out in ADR 0006 §7 and needs its own small story.
   Tracked under SKA-339's scope.

---

**When this plan stabilizes**, the §1–§9 sections promote into a
new ADR (likely ADR 0007 — Render boundary contract) co-located
with ADR 0006. The §10 open questions remain in this plan as the
working set until they resolve.
