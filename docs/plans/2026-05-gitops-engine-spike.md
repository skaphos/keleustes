<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Spike Report — gitops-engine Adoption (SKA-327)

- **Date:** 2026-05-17
- **Branch:** `shawnstratton/ska-327-gitops-engine-adoption-spike` (throwaway)
- **Reports into:** [ADR 0006](../adr/0006-engine-boundaries.md) §4 (`gitops-engine` reuse) — validates the empirical cost the ADR accepted in advance.
- **Linear:** SKA-327
- **Verdict:** **Adopt vanilla upstream; accept the k8s.io ≤ v0.34 ceiling as steady state.** Confirms ADR 0006 §4 in principle. ⚠️ *Verdict revised 2026-05-17 (afternoon) — original verdict was "soft fork + upstream PR + 90-day check"; see [Update](#update-2026-05-17-afternoon--soft-fork-abandoned) below and the corresponding [ADR 0006 amendment](../adr/0006-engine-boundaries.md#2026-05-17-afternoon--soft-fork-strategy-abandoned).*

## Update 2026-05-17 (afternoon) — soft fork abandoned

The original verdict ("Adopt with a soft fork + upstream PR + 90-day check") was reversed within hours of landing this report.

**Why.** This report named one v2beta call site (`pkg/health/health_hpa.go`) as the proximate cause of the k8s.io ≤ v0.34 ceiling and proposed an upstream PR + skaphos mirror to remove it. A second, independent site exists: `pkg/utils/kube/scheme/scheme.go` blanket-registers Kubernetes API groups via `_ "k8s.io/kubernetes/pkg/apis/autoscaling/install"`, which itself registers `v2beta1` and `v2beta2` types. The `pkg/sync` cluster cache initialization reaches this path on any non-trivial use of the engine. The ~50 LOC upstream PR removes the *direct* import in `pkg/health` but leaves the scheme-install path intact — and the scheme-install path is the load-bearing one for the ceiling. Resolving it would require restructuring how the engine registers schemes, well outside the scope of a small upstream patch.

**Decision.** Treat the k8s.io ≤ v0.34 / controller-runtime ≤ v0.22 ceiling as a **steady-state constraint**. Pin to vanilla upstream pseudo-versions. The `skaphos/argo-cd` mirror and the 90-day clock are withdrawn.

**Follow-ups status after the reversal:**

| # | Follow-up | Status |
|---|---|---|
| 1 | ADR 0006 amendment | Done — `b8576bc` + the 2026-05-17 (afternoon) amendment |
| 2 | Upstream PR to drop v2beta imports | **Withdrawn** — does not lift the ceiling on its own |
| 3 | `skaphos/argo-cd-gitops-engine` mirror | **Withdrawn** — SKA-418 closed as superseded |
| 4 | MVP 1 cluster-cache warm-up measurement | Still open |
| 5 | NOTICE / THIRD_PARTY_LICENSES tooling | Done — `2bb1589` |
| 6 | 90-day escalation calendar reminder | **Withdrawn** with the soft-fork strategy |

The empirical results below (module-graph delta, binary-size impact, wrap ergonomics, cache instantiation, annotation exposure) are unchanged — they were measured against the vanilla upstream engine; the fork patch did not move the numbers.

## TL;DR

`github.com/argoproj/gitops-engine` was archived on 2025-09-24 and migrated under `github.com/argoproj/argo-cd` as a sub-module at `gitops-engine/`. The canonical import path is now `github.com/argoproj/argo-cd/gitops-engine/...`. Adopting it is workable and Keleustes still benefits from the battle-tested SSA / sync-wave / hooks / cluster-cache / health surfaces, but adoption costs are higher than ADR 0006 §4 budgeted:

1. **k8s.io version ceiling.** Upstream pins `k8s.io/* v0.34.0` and `pkg/health/health_hpa.go` still imports `autoscaling/v2beta1`/`v2beta2` — packages removed from `k8s.io/api` at v0.35. Consuming the engine forces Keleustes to pin `k8s.io ≤ v0.34` and cascade-downgrade `controller-runtime v0.24 → v0.22`.
2. **Mandatory `replace` block.** Upstream's go.mod uses `require k8s.io/* v0.0.0` paired with `replace` directives (the standard `k8s.io/kubernetes`-consumer pattern). `replace` directives in dependencies do **not** propagate; every consumer must duplicate the ~30-line block.
3. **Maintenance posture.** No SemVer tag since `v0.7.3` (Aug 2022); issues/PRs now route through argo-cd's tracker where they compete with internal Argo CD roadmap. Treat as a frozen-ish baseline, not a living library.

**Recommendation:** Adopt via a small maintained fork that drops the dead `autoscaling/v2beta{1,2}` imports, send the cleanup PR upstream, and revisit if upstream is unresponsive after 90 days.

## What was measured

The spike vendored `github.com/argoproj/argo-cd/gitops-engine@master`
(`v0.0.0-20260515214037-a39953d21f51`, 2026-05-15), wrote a 200-line
wrapper in `internal/sync/spike/`, and built four binaries.

### Q1. Module-graph delta

```
                              modules total    k8s.io modules
baseline (k8s 0.36, no engine)            165                13
spike    (k8s 0.34, engine wired)         251                35
delta                                     +86 (+52%)         +22 (+169%)
```

The `k8s.io/*` blow-up is the dominant share. Origin: `gitops-engine` depends on `k8s.io/kubernetes`, which transitively requires almost every k8s.io staging module. The corresponding `replace` block in our go.mod has 31 entries.

### Q2. Binary-size impact

Built with `CGO_ENABLED=0 -trimpath -ldflags='-s -w'` (release profile).

| Binary | amd64 | arm64 |
|---|---:|---:|
| `manager` baseline (k8s 0.36, no engine) | 32.26 MB | 30.02 MB |
| `manager` + k8s downgrade only (no engine) | 34.26 MB | 32.05 MB |
| `manager` + engine wired in | **43.93 MB** | **41.16 MB** |
| `spike-agent` (slim agent profile) | 35.53 MB | 33.36 MB |

Deltas relative to baseline (amd64):

| Component | Δ MB | Δ % |
|---|---:|---:|
| k8s.io v0.36 → v0.34 + controller-runtime v0.24 → v0.22 | +2.00 | +6.2% |
| `gitops-engine` (`pkg/{cache,sync,diff,health,kube}`) | +9.67 | +28% |
| **Combined cost vs baseline** | **+11.67** | **+36%** |

The slim agent profile is comparable to (slightly larger than) the hub
binary baseline. Per ADR 0006 §9 the agent excludes Git provider SDKs
and external policy plugins; the engine accounts for the remainder.

### Q3. Wrap ergonomics (`pkg/sync.Sync`)

The full wrapper lives in `internal/sync/spike/spike.go` and compiles
against the engine. Key constructor:

```go
sync.NewSyncContext(
    revision string,
    reconciliationResult sync.ReconciliationResult, // Live / Target / Hooks
    restCfg, rawCfg *rest.Config,
    kctl kubeutil.Kubectl,                           // &kubeutil.KubectlCmd{} is fine
    namespace string,
    opts ...sync.SyncOpt,
) (SyncContext, func() /* cleanup */, error)
```

`SyncContext` exposes three methods that map cleanly onto the Keleustes
`SyncRun` phase machine:

| `gitops-engine` API | Keleustes mapping |
|---|---|
| `Sync()` (non-blocking, idempotent) | one tick of the SyncRun reconcile loop |
| `GetState() → (OperationPhase, msg, []ResourceSyncResult)` | populate `SyncRun.status` + emit per-resource `Deployment` records |
| `Terminate()` | reconciler responds to `SyncRun` deletion / cancel |

`OperationPhase` enum (`Running` / `Succeeded` / `Failed` / `Error` /
`Terminating`) maps 1:1 to `SyncRunPhase` with one synthetic addition
(`Pending` for "not yet ticked"). See `mapPhase` in
`internal/sync/spike/spike.go`.

**Conclusion:** API surface fits without contortions. The per-resource
result type carries `(Group, Kind, Namespace, Name, Status, Message)`
— sufficient for `Deployment.status.resources[]` without leaking
engine types across the reconciler boundary (containment rule per ADR
0006 §4 holds).

### Q4. `pkg/cache` per-DeploymentTarget instantiation

```go
cache.NewClusterCache(
    cfg *rest.Config,
    opts ...cache.UpdateSettingsFunc,   // SetNamespaces, SetLogr, SetResourceHandlers, …
) cache.ClusterCache
```

Per-target instantiation is a one-liner; the cache is bounded by the
namespaces it watches. `SetNamespaces(nil)` is cluster-scope;
`SetNamespaces([]string{…})` restricts to a subset. The cache owns its
own watch lifecycle; closing the parent `rest.Config` is sufficient
shutdown.

**RAM bound** is governed by the number of objects in the watched
namespaces, not by config; the engine does not surface a hard memory
cap. For the projected scale targets (DeploymentTarget = single cluster
or single namespace set), a per-DT cache is the correct ownership unit
— matches ADR 0005's per-target agent topology.

**Warm-up latency** was not measured against a live cluster in this
spike; flagged for the MVP 1 implementation ticket. Order of magnitude
is "one full initial list across each watched GVR" — likely seconds for
typical workload-cluster sizes, but worth a real measurement before the
agent claims work.

### Q5. Argo annotation exposure

`pkg/sync/common/types.go` defines:

```go
AnnotationSyncWave            = "argocd.argoproj.io/sync-wave"
AnnotationKeyHook             = "argocd.argoproj.io/hook"
AnnotationKeyHookDeletePolicy = "argocd.argoproj.io/hook-delete-policy"
```

`pkg/sync` reads these directly from manifests. ADR 0006 §7's hybrid
strategy (accept both `argocd.argoproj.io/*` and
`keleustes.skaphos.io/*` prefixed forms; Keleustes wins when both
present) is implementable as a manifest-mutation pass in front of
`NewSyncContext`. The spike implements this in `translateAnnotations`
(spike.go); pattern is a copy-on-modify map walk per object, O(n) in
manifest count, zero engine surface area.

## Findings beyond the ticket questions

### F1. Repository archived; canonical import path changed

`github.com/argoproj/gitops-engine` was archived 2025-09-24 and
migrated into `github.com/argoproj/argo-cd` under `gitops-engine/`.
ADR 0006 §4's table cites the old path. Two updates needed:

- The ADR's import-path reference will be wrong on every page that
  cites `github.com/argoproj/gitops-engine`. Update or supersede.
- The Apache-2.0 attribution should reference argo-cd's `LICENSE` and
  `NOTICE` files going forward, not the archived repo's.

### F2. k8s.io ≤ v0.34 ceiling (load-bearing)

`pkg/health/health_hpa.go:9-10`:

```go
autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
```

These API versions were removed from Kubernetes itself in 1.25 (Aug
2022) and 1.26 (Dec 2022). They survived in `k8s.io/api` as type stubs
through v0.34; removed in v0.35. As a result:

- Keleustes must pin `k8s.io/* ≤ v0.34.0`.
- This cascades to `controller-runtime ≤ v0.22.0` (matching pair).
- Every upstream Kubernetes minor we let drift past v0.34 widens the
  gap until upstream argo-cd bumps.

### F3. Mandatory `replace` block

The engine's go.mod uses the standard `k8s.io/kubernetes`-consumer
pattern: `require k8s.io/* v0.0.0` + a `replace` block pinning every
staging module. `replace` directives in dependencies are ignored — the
consuming module must duplicate the block.

Concretely: 31 `replace` lines were added to Keleustes' go.mod. This is
already documented inside the file. The block must be maintained
alongside the engine's pin.

### F4. Maintenance posture (sentiment with concrete evidence)

- **No SemVer tag since v0.7.3 (Aug 2022).** Pseudo-versions are the
  only consumption path.
- **Dead-code references** to long-removed APIs survive (F2). Indicates
  the maintainers are not optimizing the module for external consumers.
- **Issues/PRs now route through argo-cd's tracker.** External
  contributions compete with Argo CD's internal roadmap.
- **The repo was titled "Democratizing GitOps".** Archiving it under
  that title is the loudest signal.

This does not block adoption — argo-cd as a whole receives k8s.io bumps
(latest sub-module pseudo-version is dated 2026-05-15). But it sets the
expectation: treat the dependency as a frozen-ish baseline, not a
living library.

## Decision: adoption strategy

> ⚠️ *This section's chosen option ("soft fork + upstream PR + 90-day check") was reversed within hours of the report landing. The "Use as-is against vanilla upstream" decision that briefly replaced it has itself been **superseded by [ADR 0007](../adr/0007-hard-fork-gitops-engine.md)** (hard-fork to `github.com/skaphos/gitops-engine`, extraction under SKA-430). See also [Update 2026-05-17 (afternoon)](#update-2026-05-17-afternoon--soft-fork-abandoned) and the [ADR 0006 amendment](../adr/0006-engine-boundaries.md#2026-05-17-afternoon--soft-fork-strategy-abandoned). The original analysis below is retained for historical record.*

Three options were considered before the spike:

| Option | Cost | Differentiation gain | Maintenance burden |
|---|---|---|---|
| Use as-is | k8s.io v0.34 ceiling forever | None | None |
| Full fork | Own 20–30K LOC | None (engine not differentiating) | High, forever |
| Write our own | 6–12 months added before MVP 1 sync | None (Keleustes wins on Promotion + multi-target + agent) | None upstream, all internal |

**Chosen: a fourth option — soft fork + upstream PR + 90-day check.**

Mechanic:

1. **Send an upstream PR** to drop the `autoscaling/v2beta{1,2}` imports
   from `pkg/health/health_hpa.go`. This is `~50 lines` of dead code
   removal; high chance of upstream acceptance because k8s 1.25/1.26
   removed those API versions at runtime years ago. The PR also fixes
   the `k8s.io/api ≤ v0.34` ceiling for everyone downstream.
2. **While the PR is open**, point Keleustes' go.mod at our fork via
   `replace github.com/argoproj/argo-cd/gitops-engine => github.com/skaphos/argo-cd-gitops-engine vX`.
   Carry only the patch that the upstream PR proposes; rebase on every
   upstream commit.
3. **90-day escalation trigger.** If the upstream PR has not been
   acted on within 90 days of opening, revisit. At that point the
   maintenance-posture argument (F4) becomes load-bearing; switching to
   a hard fork or replacing `pkg/health` with a Keleustes-owned health
   engine becomes more defensible.

ADR 0006 §4's containment rule (`gitops-engine` imports only inside
`internal/{sync,diff,health,kube}/`) limits the blast radius either way.

## Concrete follow-ups

1. **ADR 0006 amendment / supersession.** Update §4 to:
   - cite the new import path (`github.com/argoproj/argo-cd/gitops-engine`)
   - record the k8s.io ≤ v0.34 ceiling and the `replace`-block
     requirement
   - reference this spike report and the soft-fork strategy
2. **Open the upstream PR.** Repo:
   `github.com/argoproj/argo-cd`, path: `gitops-engine/pkg/health/`.
   Drop `autoscalingv2beta1` + `autoscalingv2beta2` blocks from
   `health_hpa.go` and the corresponding test fixtures.
3. **Set up `skaphos/argo-cd-gitops-engine` mirror** with one branch
   carrying the cleanup patch. Update Keleustes' go.mod `replace` line
   to point at it.
4. **MVP 1 ticket: real cluster-cache warm-up measurement.** Spike did
   not exercise `EnsureSynced()` against a live cluster. Get a number
   for typical-workload-cluster sizes.
5. **NOTICE / THIRD_PARTY_LICENSES tooling.** Aggregate Apache-2.0
   attributions at build time. Recommended tool: `go-licenses report`
   bundled into the manager build step in `Taskfile.yml`. Required for
   compliance per ADR 0006 §4 (license attribution clause).
6. **Track the 90-day escalation deadline** as a calendar reminder
   keyed to the upstream PR open date.

## Reverting the spike

The branch carries the following throwaway changes:

- `internal/sync/spike/` — wrapper proving Q3 + Q5
- `cmd/spike-agent/` — main exercising Q1 + Q2
- `cmd/manager/spike_import.go` — blank import to measure hub-side delta
- `go.mod` / `go.sum` — engine require + 31-line `replace` block + k8s + controller-runtime downgrade
- `bin/spike/` — compiled artefacts

Only this report (`docs/plans/2026-05-gitops-engine-spike.md`) is
intended to land on `main`. The rest of the branch is discarded once
the report merges and follow-ups (1)–(6) above are filed.
