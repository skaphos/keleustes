<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# ADR 0006 — Engine boundaries and `gitops-engine` reuse

- **Status:** Accepted — amended 2026-05-17 (twice: SKA-327 spike findings, then soft-fork reversal — see Amendments below). The afternoon amendment's *Decision* paragraph (k8s.io ceiling as steady-state) is **superseded by [ADR 0007](./0007-hard-fork-gitops-engine.md)** (hard-fork to `skaphos/gitops-engine`).
- **Date:** 2026-05-17
- **Deciders:** Platform Architecture (Skaphos)
- **Linear:** SKA-411, SKA-327 (adoption spike)
- **Related:** ADR 0001 (Plugin extension model), ADR 0003 (Git invariant), ADR 0005 (Distributed runtime)
- **Supersedes:** `docs/plans/2026-05-engine-boundaries-and-technology-integration.md` §7 questions 1–8 and 14–15 (plan questions 9–13 are resolved in ADR 0005)
- **See also:** `docs/plans/2026-05-gitops-engine-spike.md` (SKA-327 spike report — empirical validation of §4)

## Amendments

### 2026-05-17 (afternoon) — Soft-fork strategy abandoned

The soft-fork half of the earlier amendment's §4 (below) is reversed. Keleustes now consumes a **vanilla upstream pseudo-version** of `github.com/argoproj/argo-cd/gitops-engine` (`v0.0.0-20260515214037-a39953d21f51` at the time of this amendment). The `skaphos/argo-cd` mirror, the upstream PR plan (argo-cd#27887), and the 90-day escalation trigger are all withdrawn. SKA-418 — the ticket that wired the `replace` directive at the soft-fork — is closed as superseded.

> **Superseded by [ADR 0007](./0007-hard-fork-gitops-engine.md).** Keleustes now consumes the Skaphos-owned fork `github.com/skaphos/gitops-engine` (extracted from `argo-cd/gitops-engine/` via filter-repo), not the vanilla upstream pseudo-version. The `skaphos/argo-cd` GitHub fork was renamed in place to claim the new module name (SKA-430 carries the extraction).

**What changed the calculus.** The earlier amendment named only one v2beta call site (`pkg/health/health_hpa.go`). Implementation work surfaced a second, independent site: `pkg/utils/kube/scheme/scheme.go` blanket-registers Kubernetes API groups via `_ "k8s.io/kubernetes/pkg/apis/autoscaling/install"`, which itself registers `v2beta1` and `v2beta2` types. This path is reached through the `pkg/sync` cluster cache initialization, so any non-trivial use of the engine traverses it. The ~50 LOC cleanup PR upstream removes the *direct* import in `pkg/health` but leaves the scheme-install path intact — and the scheme-install path is the load-bearing one for the k8s.io ≤ v0.34 ceiling. Resolving it would require restructuring how the engine registers schemes, well outside the scope of a small upstream patch.

**Decision.** Treat the k8s.io ≤ v0.34 / controller-runtime ≤ v0.22 ceiling as a **steady-state constraint**, not a temporary one. The earlier amendment's "catch-up review" framing for pinning cadence (§4) still applies — but the trigger for catching up shifts from "the upstream PR lands" to "upstream restructures the scheme registration" or "Keleustes' need to consume k8s.io v0.35+ becomes load-bearing enough to justify a hard fork or a Keleustes-owned health engine." Neither is on the near-term roadmap.

> **Superseded by [ADR 0007](./0007-hard-fork-gitops-engine.md).** The ceiling is no longer steady-state. Owning the fork moves the scheme-registration refactor from a wait-on-upstream task to a Skaphos-internal commit (SKA-421 rescoped against the fork); the v0.34 `replace` block in Keleustes' `go.mod` stays until SKA-421 lands but is now in-tree work, not a permanent constraint.

**What is unchanged.** The containment rule (§4), license attribution (§4), the mandatory duplicated `replace` block in `go.mod` (Amendments §3 / §10), the agent build profile (§9), engine boundaries, render policy, Git-provider policy, and annotation policy all remain as written. The dependency graph cost recorded in the SKA-327 spike report (+86 modules, +11.67 MB binary) is paid against the vanilla upstream pin, not the fork — the fork did not change those numbers.

**Compliance updates carried by this amendment:**

- Spike report's *Decision: adoption strategy* section is annotated with a pointer to this amendment.
- Spike report's follow-ups (2) "open the upstream PR" and (3) "set up skaphos/argo-cd-gitops-engine mirror" and (6) "track 90-day escalation deadline" are withdrawn. Follow-ups (1) ADR amendment, (4) cluster-cache warm-up measurement, (5) NOTICE / THIRD_PARTY_LICENSES tooling are unchanged.
- `docs/licenses.md` and `NOTICE` already point at `argo-cd` upstream, not the skaphos fork — no change required.

### 2026-05-17 — SKA-327 spike findings

The MVP 1 spike enumerated in *Compliance and follow-ups* below was executed (`docs/plans/2026-05-gitops-engine-spike.md`). The spike confirmed §4 in principle but produced four constraints the original decision did not anticipate. These constraints **refine** §4 — they do not overturn it.

1. **Canonical import path changed.** `github.com/argoproj/gitops-engine` was archived on 2025-09-24 and migrated into `github.com/argoproj/argo-cd` as a sub-module. The new path is **`github.com/argoproj/argo-cd/gitops-engine`** (its own go.mod under the argo-cd monorepo). All references in §4 below should be read against this new path; the old path resolves to an archived repository with no future commits. License attribution moves with the path: NOTICE / LICENSE references now point at the argo-cd repository.

2. **k8s.io ≤ v0.34 ceiling.** Upstream pins `k8s.io/* v0.34.0` and `pkg/health/health_hpa.go` directly imports `k8s.io/api/autoscaling/v2beta1` and `v2beta2` — packages removed from `k8s.io/api` at v0.35. Adopting the engine therefore forces Keleustes to pin `k8s.io ≤ v0.34` and cascade-downgrade `controller-runtime ≤ v0.22`. The "MVP-boundary pinning" cadence in §4 is **catch-up review, not stay-current review** until upstream moves.

3. **Mandatory `replace` block.** The engine's upstream go.mod uses `require k8s.io/* v0.0.0` paired with `replace` directives (standard `k8s.io/kubernetes`-consumer pattern). `replace` directives in dependencies do not propagate; every consumer must duplicate the ~30-line block in its own go.mod. This is a one-time cost but a permanent obligation. §10's dependency-pinning strategy must include this block alongside the per-module pins.

4. **Soft-fork adoption strategy (overrides "vanilla upstream" implication of §4).** ⚠️ *Superseded by the 2026-05-17 (afternoon) amendment above — the soft-fork strategy below was abandoned once a second, independent v2beta call site in `pkg/utils/kube/scheme` was discovered. Retained here as historical record.* The dead-code `autoscaling/v2beta{1,2}` imports in `pkg/health/health_hpa.go` (~50 lines) are the proximate cause of the k8s.io ceiling. Strategy:

   - **Send an upstream PR** dropping the dead imports. High-leverage cleanup; benefits every downstream consumer.
   - **While the PR is open**, point Keleustes' go.mod at a `skaphos/argo-cd-gitops-engine` mirror carrying only the cleanup patch via `replace github.com/argoproj/argo-cd/gitops-engine => github.com/skaphos/argo-cd-gitops-engine vX`. Rebase on every upstream commit.
   - **90-day escalation trigger.** If the upstream PR has not been acted on within 90 days of opening, revisit. At that point the maintenance-posture signal (no SemVer tag since v0.7.3 in Aug 2022; issues now route through argo-cd's tracker; dead-code references survive for years) becomes load-bearing; a hard fork or a Keleustes-owned health engine becomes more defensible.

The containment rule in §4 (gitops-engine imports only inside `internal/{sync,diff,health,kube}/`) remains intact and continues to limit blast radius under either the soft-fork or any later escalation. **No changes to package layout, engine boundaries, render policy, Git-provider policy, annotation policy, or dependency model are required.**

## Context

`internal/controller/` today is thin reconciler stubs. As soon as real
engines arrive — Source, Sync, Render, Promotion, Git Mutation, Policy,
Health, Diff — there is a clear risk of a single tangled package that
violates the low-cognitive-load guardrail in CLAUDE.md, is hard to test,
and resists later distribution to agents.

The plan
`docs/plans/2026-05-engine-boundaries-and-technology-integration.md`
proposes explicit package boundaries, a containment rule around
`gitops-engine` reuse, and a first-cut technology integration map. This
ADR pins those decisions before MVP 1 engine work begins.

## Decision

### 1. Seven engines plus a shared `Render` package

Keleustes has seven engines and one critical cross-cutting package:

| Engine                | Lives under                  | Owns                                                                  |
|-----------------------|------------------------------|------------------------------------------------------------------------|
| **Source Engine**     | `internal/source/`           | `Source` CR; revision events                                          |
| **Sync Engine**       | `internal/sync/`             | `SyncPlan`, `SyncRun`, `Deployment`; per-target reconciliation lifecycle |
| **Promotion Engine**  | `internal/promotion/`        | `Promotion`, `Approval`, `PromotionPolicy`; the state machine        |
| **Git Mutation Engine**| `internal/mutation/`        | Structured updates + provider PR/commit creation                      |
| **Policy Engine**     | `internal/policy/`           | Native gates + external policy plugin invocation; evidence            |
| **Health Engine**     | `internal/health/`           | `HealthCheck`; resource health aggregation                            |
| **Diff Engine**       | `internal/diff/`             | Desired vs live, release vs release; ignore rules; redaction         |
| **Render** (shared)   | `internal/render/`           | `(Application, Release, target context) → rendered manifests + inventory` |

`Render` is **not** a top-level engine but is the most-shared,
security-critical package every other engine depends on. Treating it as a
first-class, isolated package from day one is non-negotiable.

`internal/controller/` continues to hold **only thin coordinator
reconcilers** (≤ ~150 LOC each); business logic delegates to an engine
package.

### 2. Package layout

The target layout after MVP 1 engine work, before heavy distribution:

```
internal/
├── api/                  # future REST/gRPC server, handlers, authz middleware
├── cli/                  # keleustesctl tree (thin — delegates to engines)
├── controller/           # thin coordinator reconcilers (no business logic)
├── source/               # Source Engine
├── sync/                 # Sync Engine
├── promotion/            # Promotion Engine + state machine
├── mutation/             # Git Mutation Engine
├── policy/               # Policy Engine + plugin dispatcher (per ADR 0001)
├── health/               # Health Engine
├── diff/                 # Diff Engine
├── render/               # Render (Kustomize / Helm / Helmfile / raw)
├── inventory/            # shared inventory types and conventions
├── plugins/              # shared envelope + dispatcher for ADR 0001 plugin surfaces
├── events/               # event bus abstractions + NATS implementation
├── agent/                # agent protocol, transport interface, work claiming
├── store/                # JetStream / NATS KV / object storage adapters
├── webhooks/             # per-provider public webhook receivers
├── git/                  # thin Git provider clients
├── kube/                 # k8s client helpers, SSA field-manager wiring
└── util/                 # small, genuinely shared utilities
```

### 3. Engine ownership and dependency rules

**Strict acyclic dependencies:**

- Engines may depend on `render/`, `inventory/`, `events/`, `store/`,
  `kube/`, `git/`, `plugins/`.
- Engines may **not** depend on each other except through narrow
  interfaces or the event bus.
- `controller/` depends on engines (not the other way around).
- `cli/` and the future `api/` talk to engines via clean facades or via
  the event/query layer.

**Concrete "owns" statements:**

- **Sync Engine** owns: creating and driving `SyncRun` objects through
  their phases, writing `Deployment` records, deciding prune sets,
  producing per-target inventory.
- **Promotion Engine** owns: the `Promotion` phase machine, blocker
  calculation, approval orchestration, requesting Git mutations. It does
  not apply manifests and does not call `git push` directly.
- **Render** owns: turning an Application's manifest spec + a specific
  Release into a set of rendered objects + inventory. It does **not**
  own applying them or deciding when to render.
- **Git Mutation Engine** owns: structured mutation logic + provider-
  specific PR/commit creation. It does **not** decide *when* a mutation
  should happen (Promotion does) or what the desired image tag is
  (Source / Release).
- **Policy Engine** owns: evaluation + evidence production. Called by
  Promotion Engine (gate evaluation) and the Sync Engine (where
  applicable).

### 4. `gitops-engine` reuse — yes, with a containment rule

Keleustes reuses `gitops-engine` (Apache-2.0) — now distributed as the
[`gitops-engine` sub-module of argo-cd](https://github.com/argoproj/argo-cd/tree/master/gitops-engine),
canonical import path `github.com/argoproj/argo-cd/gitops-engine` (see
Amendments above for the path-change history) — for the parts the
GitOps ecosystem has already battle-tested:

| `gitops-engine` package          | Used by                  | What it gives us                                                                 |
|----------------------------------|---------------------------|-----------------------------------------------------------------------------------|
| `pkg/cache` (cluster cache)      | `sync/` (agent-side)     | Per-`DeploymentTarget` watch-backed view of every resource, with owner-ref traversal |
| `pkg/diff` (+ `normalizers`)     | `diff/`                  | Three-way diff with field normalizers                                            |
| `pkg/sync`                       | `sync/`                  | SSA, sync waves, hooks (Pre/Sync/Post/Fail), prune by inventory                  |
| `pkg/health`                     | `health/`                | Built-in resource health checks + Lua extension                                  |
| `pkg/utils/kube`                 | `kube/`                  | Resource key generation, GVK helpers, SSA field-manager wiring                   |

**`gitops-engine` is imported only inside `internal/sync/`,
`internal/diff/`, `internal/health/`, and `internal/kube/`.** Other
packages access its capabilities through Keleustes-defined interfaces
exported from those packages. This is the **containment rule** — load-
bearing because it limits the blast radius of a future fork, version
pin, or replacement.

**Pinning and upgrade cadence** (plan §7 question 6): pin tightly in
`go.mod`; gate upgrades behind targeted regression coverage; revisit at
each MVP boundary, not opportunistically. Per the 2026-05-17 amendment,
this cadence is **catch-up review**, not stay-current review, until
upstream moves past k8s.io v0.34 and the dead-code dependencies in
`pkg/health` are resolved. License attribution is preserved in `NOTICE`
and headers per Apache-2.0 + MIT compatibility (plan §2.5 risk 5);
attribution now references the argo-cd repository, not the archived
gitops-engine repository.

**Containment-rule bypass policy** (plan §7 question 8): if a caller
needs raw `gitops-engine` output rather than the normalized form,
**widen the Keleustes-side interface in `diff/` / `sync/` / `health/`**.
Direct imports from other packages require a written exception in the
PR description, justified against this ADR. Default answer is "widen the
interface."

### 5. Render technology stack — library-only

`render/` ships with first-class subpackages for:

- **Kustomize** — `sigs.k8s.io/kustomize/api` + `kyaml`. Preferred.
- **Helm** — `helm.sh/helm/v3` (pkg/action, pkg/chart). Library-only; no
  `helm` CLI invocation.
- **Raw manifests** — `k8s.io/apimachinery` + yaml decoders.
- **Helmfile** (MVP 2) — exec the `helmfile` binary in a sandbox **only**
  if a pure-Go composition over the Helm SDK proves impractical.

**Library-only is the default** (plan §7 question 1). Reasons:
reproducibility (exec output depends on binary version + environment),
security (no shelling out), and explainability (Render's request/response
types are typed Go, not shell streams). Helmfile is the one exception
and is the candidate we revisit first if execution shape becomes a
performance or correctness issue.

`Render` is **not** a plugin surface (ADR 0001). Render is on the
security-critical hot path; tampering would defeat every downstream
control. Reserved for a future RFC if ever.

### 6. Git provider integration via an in-tree interface

A Keleustes-defined `GitProvider` interface lives in `internal/git/`.
First-class implementations:

- **GitHub** — `github.com/google/go-github/v60`, MVP 2
- **GitLab** — `gitlab.com/gitlab-org/api/client-go`, MVP 3
- **Azure DevOps** — `github.com/microsoft/azure-devops-go` or REST,
  MVP 3
- **Gitea** — `code.gitea.io/sdk/gitea`, later

External Git providers can implement the interface in a customer fork.
Git provider is **not** a plugin surface (ADR 0001) — the security model
and the Git mutation engine's correctness assumptions are too tightly
coupled to expose over webhooks.

Structured update helpers (image-tag bump, values-file edit, JSON6902
patch) live in `mutation/` once and are reused by every provider.

### 7. Argo CD annotation compatibility — hybrid

Plan §7 question 7: Keleustes **accepts Argo CD's sync-wave and hook
annotations on user manifests** (`argocd.argoproj.io/sync-wave`,
`argocd.argoproj.io/hook`, `argocd.argoproj.io/hook-delete-policy`) and
**translates them internally** to Keleustes equivalents. Reasons:

- Lowest migration friction for Argo CD customers — existing manifests
  work.
- The semantics map cleanly into `gitops-engine`'s `pkg/sync` because
  that's exactly where the annotations originate.
- We get to write Keleustes-prefixed equivalents
  (`keleustes.skaphos.io/sync-wave`, etc.) as the **preferred form in
  new documentation**, so over time the ecosystem converges on our
  prefix without forcing migrations.

When both are present, the Keleustes-prefixed annotation wins. When
neither is present, default behavior applies. Annotations Keleustes does
not understand are passed through unchanged.

### 8. Cross-Application dependency ordering

Plan §2.6 (dependency model) is in-scope for the **Sync Engine**, not a
new top-level engine. Dependencies are declared on the `Application` CR
(not a separate `Dependency` CRD):

```yaml
spec:
  dependencies:
    applications:
      - applicationRef: { name: cert-manager, namespace: addons }
        scope:    same-target          # same-target | any-target | namedTarget: <name>
        waitFor:  Healthy
        timeout:  10m
        onTimeout: block               # block | warn | proceed
    crds:
      - name: certificates.cert-manager.io
        waitForCondition: Established
```

Behavior:

- `SyncPlan` evaluates dependencies before generating `SyncRun`s. Unmet
  dependencies put the SyncPlan into `WaitingForDependencies` with a
  condition listing exactly what is unmet.
- **Auto-sync waits — it does not fail.** When dependencies become
  satisfied (via JetStream events), the SyncPlan automatically proceeds.
  This eliminates the Argo CD "addon stuck, auto-sync keeps failing, a
  human has to manually sync in the right order" pain.
- **CRD dependencies** are first-class — checked against the target
  cluster's `CustomResourceDefinitions` for `Established` before
  dependent Applications sync.
- **Cycle detection at admit time** via a validating webhook on
  `Application`.
- **Promotion-aware:** when the Promotion Engine fans out a Release to
  multiple Applications, the wave order respects dependencies.

**Declaration ergonomics** (plan §7 question 14): hybrid. Users declare
dependencies explicitly; a Keleustes lint command (`keleustesctl lint
dependencies`) scans manifests and **suggests** CRD references it found
in the rendered output. Suggestions are advisory; the user accepts them
into the spec.

**Cross-shard coordination** (plan §7 question 15): satisfaction events
travel on JetStream subjects keyed by the providing Application's hash
prefix; consumer Applications subscribe to the relevant subjects. This
makes cross-shard waits work without any direct shard-to-shard call.
Concrete subject layout lands with the MVP 2 sharded-controller design.

### 9. Agent build profile

Plan §7 question 2: the agent binary uses a **slim build-tag set**:

- **Included:** `render/`, `sync/`, `diff/`, `health/`, `kube/`,
  `inventory/`, `events/nats/`, `agent/`, `gitops-engine` packages.
- **Excluded:** `mutation/<provider>/` (no Git provider SDKs),
  `policy/external/` (no Trivy/Grype/OPA in the binary; webhook plugins
  reach external services).

Result: a substantially smaller agent image and CVE surface. The agent
participates in Git mutation only via opt-in PromotionPolicy
configuration (ADR 0005 §11), and only by **requesting** a mutation from
the hub — the agent never embeds a Git provider SDK.

### 10. Dependency pinning strategy

Plan §7 question 5: the existing `tools/` pattern is the source of truth
for all build-time tooling versions. Library dependencies pinned via
`go.mod`. Renovate (or equivalent) opens PRs for updates; CI gates the
merge. `gitops-engine` upgrades follow the §4 cadence (MVP-boundary
review).

Per the 2026-05-17 amendment, Keleustes' go.mod must carry a
**duplicated `replace` block** for every `k8s.io/*` staging module,
mirroring upstream gitops-engine. This is the standard
`k8s.io/kubernetes`-consumer pattern; the block is maintained as one
unit alongside the engine pin. The Renovate configuration must keep the
block aligned with the engine pseudo-version on every bump.

### 11. Helm chart repository authentication in a distributed world

Plan §7 question 4: chart repo credentials live on the `Source` CR
(referenced via `Secret`s). When rendering is delegated to a regional
agent, the agent fetches the same secrets through its own
`ServiceAccount` (per ADR 0004 RBAC). The agent's NKey + JWT
(ADR 0005 §5) scopes access to only the namespaces it owns; cross-
namespace chart repos require explicit grant.

For private chart repos in air-gapped regions, the customer mirrors the
chart repo into the region; the `Source` CR points at the regional
mirror.

## Consequences

**Positive**

- Clear engine boundaries make every reconciler small, testable, and
  amenable to later extraction into an agent binary.
- `gitops-engine` reuse is free battle-testing for the apply / diff /
  health / cache surfaces — engineering bandwidth goes to Promotion,
  Git Mutation, multi-target topology, distributed runtime, which is
  what's actually novel about Keleustes.
- Containment rule keeps the `gitops-engine` dependency contained to
  four packages. Future fork, version pin, or replacement has a small
  blast radius.
- Library-only Render keeps the security-critical hot path reproducible
  and audit-friendly.
- Argo CD annotation compatibility means existing manifests work on
  first install — a huge migration accelerator.
- Cross-Application dependency ordering eliminates the "addon bootstrap
  deadlock" pain class.

**Negative / accepted costs**

- `gitops-engine` drags in significant `k8s.io/*` modules. Agent binary
  size is mitigated by the slim build-tag set, but every dependency
  update costs MVP-boundary review time.
- Helmfile in MVP 2 may have to exec the binary if a pure-Go composition
  proves impractical — a known impedance with the library-only
  preference.
- The hybrid Argo-annotation policy means manifests can carry two sets
  of annotations during migration; documented but a real source of
  confusion if not handled carefully in the docs.
- Cross-shard dependency satisfaction depends on the MVP 2 JetStream
  subject layout being designed correctly the first time. Concretely
  designed before MVP 2 implementation work begins.

## Alternatives considered

- **Reimplement sync / diff / health / cluster cache from scratch.**
  Rejected: years of battle-testing in `gitops-engine`; reimplementing
  delivers no novel capability and consumes engineering bandwidth that
  belongs to Promotion + multi-target.
- **Fork `gitops-engine` into Keleustes.** Reserved as an escape hatch
  if upstream becomes unresponsive or moves in an incompatible
  direction. Not the default — vendoring without a maintenance plan is
  worse than depending on the upstream.
- **Render as a plugin surface.** Rejected per ADR 0001 — Render is on
  the security-critical hot path; tampering would defeat downstream
  controls.
- **Git provider as a plugin surface.** Rejected — Git mutation
  correctness depends on tight coupling with the Git Mutation Engine's
  invariants; a webhook plugin can't carry the necessary contract.
- **Keleustes-only annotation prefix (no Argo CD compatibility).**
  Cleaner ownership; rejected because migration friction is a meaningful
  factor in adoption and Argo's annotation semantics map cleanly through
  `gitops-engine` already.
- **Separate `Dependency` CRD** instead of `Application.spec.dependencies`.
  Rejected as unnecessary indirection — the dependency is naturally a
  property of the consuming Application.
- **Cluster-cache aggregation on the hub.** Rejected — the per-target
  cache lives on the owning agent (ADR 0005); a hub holding caches for
  hundreds of targets is the wrong shape and is one of the primary
  motivators for the agent model.

## Compliance and follow-ups

- Plan §7 questions 1 (render library-only), 2 (agent binary), 3 (Git
  mutation from agents), 4 (chart repo auth), 5 (dependency pinning),
  6 (`gitops-engine` pinning), 7 (Argo annotation compatibility), 8
  (containment-rule bypass), 14 (dependency declaration ergonomics),
  and 15 (cross-shard dependency coordination) are resolved here. Plan
  §7 questions 9 (JetStream retention), 10 (DuckDB cadence), 11
  (webhook receiver shape), 12 (sharded controller pattern), and 13
  (scale benchmark harness) are resolved in ADR 0005.
- The `internal/render/` package scaffold (request/response types +
  inventory extraction with stub returns) is the first non-trivial
  Keleustes-owned package to land — file a ticket for MVP 1.
- The `pkg/plugins/` shared envelope + dispatcher (ADR 0001) lands when
  the first engine that consumes plugins (Source / Promotion / Audit)
  arrives.
- The MVP 1 spike that vendored `gitops-engine` into a throwaway
  branch — measuring binary size, module graph delta for `manager` and
  a hypothetical `agent`, per-`DeploymentTarget` cache instantiation,
  wrap ergonomics, and annotation exposure — **landed on 2026-05-17 as
  `docs/plans/2026-05-gitops-engine-spike.md` (SKA-327).** Headline
  empirical results: +86 modules total (+52%), +22 k8s.io modules
  (+169%), +11.67 MB / +36% manager binary delta (amd64). The spike
  confirmed the adoption and produced the four refinements captured in
  the Amendments section above.
- Argo CD annotation translation lives in `internal/sync/translate.go`
  and is covered by table-driven tests.
- This ADR will be revisited if `gitops-engine` upstream becomes
  unworkable, if Render exec-fallback proves necessary, if a Git
  provider requires capabilities that the in-tree interface cannot
  express, or if the cross-shard dependency design fails the MVP 2
  benchmark.
