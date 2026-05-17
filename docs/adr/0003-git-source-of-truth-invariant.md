<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# ADR 0003 — Git-source-of-truth invariant

- **Status:** Accepted
- **Date:** 2026-05-17
- **Deciders:** Platform Architecture (Skaphos)
- **Linear:** SKA-408
- **Supersedes:** the relevant portions of `docs/plans/2026-05-rbac-audit-and-git-invariant.md` §3, §6, §9, §11

## Context

Argo CD's two largest unforced operational errors at scale are
**parameter overrides** and **edit-live-resource** — features that let an
operator change deployed state in ways that have no record in any Git
repository. Once those affordances exist, "what is deployed?" stops being
answerable from Git alone; reproducibility, disaster recovery, and audit
all degrade.

Keleustes is built around the opposite stance: every byte of desired state
on every target is derived from a commit in a Git repository the user
controls. This stance is referenced informally in CLAUDE.md / AGENTS.md and
described in `docs/plans/2026-05-rbac-audit-and-git-invariant.md` §3, but
it is the single most load-bearing invariant in the architecture. Every
engine, every CRD, every CLI verb has to comply. This ADR makes the rule
binding and resolves the plan's §11 questions that relate to it.

## Decision

### 1. The hard rule

> Every byte of desired state for every Application on every target is
> derived deterministically from a commit in a Git repository the user
> controls. Keleustes does not store, mutate, or apply any desired state
> that is not in Git.

This is **non-negotiable** for normal operation. Break-glass (§4) is the
single sanctioned exception and is itself made auditable through Git.

### 2. What the rule forbids

The following patterns are **explicitly forbidden** anywhere in the
operator, the CLI, the UI, or any subsystem:

- **Parameter overrides on the `Application` CR.** No `spec.source.helm.parameters`, no `spec.kustomize.images`, no equivalent field that lives on the CR and isn't sourced from a values file or kustomization in Git. The CRD shape must not even *offer* such fields.
- **CLI / UI "set" verbs that change desired state.** `keleustesctl app set --image …` does not exist. The corresponding workflow is `keleustesctl promote --bump-image …`, which **creates a Git commit** that performs the change.
- **"Edit live resource" affordances in the UI.** The UI is read + actions-that-write-Git. It is not a `kubectl edit` front-end.
- **In-cluster `kubectl apply` of mutated manifests by Keleustes.** Any mutation must round-trip through Git first.
- **Helm `--set` style invocations whose arguments are not in Git.** All Helm values come from values files in Git.
- **Helmfile state files outside Git.** Helmfile support depth (plan §11.6) does not change the invariant — Helmfile state files live in Git; the engine that consumes them reads from a Git checkout.

### 3. What the rule guarantees

Every sync decision must be reconstructable from exactly five inputs:

1. The **Git commit** the desired state was derived from.
2. The **render output** (content-addressed, cached in object storage).
3. The **apply result** (server-side apply outcome, recorded on `SyncRun`).
4. The **inventory** (the set of resources the SyncRun owns).
5. The **health state** at the time of the decision.

If a decision cannot be explained from those five inputs, the system has a
bug.

### 4. Break-glass: the single sanctioned exception

Break-glass is **not** "bypass the rule." Break-glass is "make a direct
cluster mutation when Git is unreachable or speed beats round-trip, and
make the action auditable and reconcilable after the fact." It is the
only sanctioned path that touches a cluster outside Git.

**Required shape:**

1. The actor holds the `break-glass` action permission, typically as a
   time-bound `RoleBinding` (RBAC ADR §5.6; see ADR 0004).
2. The actor invokes the break-glass workflow explicitly
   (`keleustesctl break-glass apply …` or a UI button with a destructive-
   action confirmation).
3. Keleustes records the intent in the audit stream **before** applying.
4. Keleustes applies the change using a dedicated SSA field-manager
   (`keleustes-break-glass`) so the mutation is attributable.
5. Keleustes opens a PR against the config repo capturing exactly what was
   applied, with a `break-glass: <timestamp> <actor> <reason>` trailer.
6. The next normal reconcile detects drift between Git and live and
   surfaces it as a `BreakGlassDrift` condition on the affected `Application`
   and `Deployment`.
7. **The drift condition does not auto-clear, and Keleustes does not
   auto-revert.** A human resolves it by either merging the PR (Git catches
   up with reality) or by issuing an explicit `keleustesctl break-glass
   revert …` (reality catches up with Git). This locks in the plan §11.7
   lean: explicit revert, not silent auto-revert.

This is the only sanctioned path that touches the cluster outside Git, and
it leaves a record that itself becomes Git history.

### 5. Enforcement is structural, not policy

The invariant is enforced through **engine design**, not through after-the-
fact policy gates:

| Engine                  | Enforcement responsibility                                                                 |
|-------------------------|---------------------------------------------------------------------------------------------|
| API server              | CRD schemas have no fields that would accept out-of-band overrides.                         |
| Promotion Engine        | The only way to advance a Release is to request a Git mutation; Promotion never applies directly. |
| Git Mutation Engine     | Every mutation is a commit; every commit carries actor + audit-event-ID metadata.           |
| Sync Engine             | Applies only what was rendered from a Git commit; rejects manifests with a foreign field-manager prefix; emits audit on every SyncRun phase change. |
| Source Engine           | New revisions are *proposed* desired state — they don't deploy until a Promotion accepts them and commits to the config repo. |
| Webhook receivers       | Audit every receipt + provider-validation result; never bypass the Promotion path.          |
| Agents                  | Same constraints as the hub; emit audit per action; never autonomously commit outside the sanctioned workflows. |

Policy gates (ADR 0001's `PolicyGate` surface) are additive — they can
*deny* a Git-driven change. They cannot grant a change that didn't come
through Git in the first place.

### 6. UI shape

The UI is a read surface plus a small bounded set of action surfaces.
Action buttons do exactly one of three things:

1. Issue a state-machine transition on a CRD (e.g., approve a Promotion).
2. Open a PR against the config repo (e.g., "Promote" bumps a release tag).
3. Invoke break-glass under the §4 safeguards.

There is **no** "edit values inline and apply" button. There is **no**
"override parameters" panel. The CRD schemas are the contract that
prevents these from being added later by accident.

### 7. Render inputs and the invariant

All inputs to rendering — Kustomize patches, Helm values files, Helmfile
state files, raw YAML — come from Git. Even cluster-discovered values
(current image tags observed by the Source Engine) are committed to Git
before they can drive an apply. Render output is content-addressed and
cached in object storage, so the rendered manifests for a given
`(Application, Release, target)` tuple are stable and reproducible.

If rendering depends on something not in Git, that's a bug.

## Consequences

**Positive**

- **Bit-perfect reproducibility.** Any historical desired state is
  recreatable by checking out the relevant commits.
- **Trivial disaster recovery.** Re-bootstrap from Git; no "find the
  overrides" step.
- **Simple audit story.** "What was deployed?" → "what was in the manifest
  at commit X." No reconstruction needed.
- **Eliminates the Argo CD parameter-override and edit-live trap classes
  before they can ship.** The CRD shape prevents them; the engine design
  prevents them; the UI design prevents them.
- Break-glass remains a real, usable escape hatch — but it is observable,
  bounded, and self-healing through Git.

**Negative / accepted costs**

- Every change goes through Git. Latency-sensitive operators who want
  one-click rollback do not get a direct apply path; they get a one-click
  promotion that commits the rollback to Git. The latency cost is real
  (seconds, not milliseconds) and is accepted.
- The break-glass drift condition does not auto-clear. Operators must
  consciously resolve it (merge or revert). This is intentional friction
  to keep break-glass from becoming a routine path.
- Some workflows that feel natural in Argo CD (parameter overrides for
  short-term experimentation, edit-live for quick patches) have no
  equivalent here. Customers migrating from Argo CD will need to learn the
  Git-mutating workflows.

## Alternatives considered

- **Allow parameter overrides as a UI feature, gated by RBAC.** Rejected:
  the trap is the existence of the affordance, not the access control. Once
  it exists, it gets used, and "what was deployed?" stops being answerable
  from Git.
- **Allow edit-live with auto-revert on next reconcile.** Rejected: even
  with auto-revert, the window where Git and live disagree is large enough
  to matter, and the affordance encourages exactly the wrong reflex.
- **Auto-revert on break-glass when the PR is rejected.** Rejected: silent
  auto-revert is itself an out-of-band cluster mutation. The plan §11.7
  lean is explicit revert; a human reviewing a rejected break-glass PR
  knows whether reality should change or whether the proposed Git state
  should be revised. The system should not guess.
- **Allow short-lived in-Application overrides with a TTL.** Rejected:
  this is the parameter-override pattern with a timer; it has all the same
  reproducibility problems while it's live and surprises operators when
  it expires.

## Compliance and follow-ups

- ADR 0004 (CRD-based RBAC) defines the `break-glass` permission shape
  and time-bound RoleBinding mechanics referenced in §4.
- The CRD shape for `Application`, `Source`, `Release`, etc. must not
  introduce override fields. New PRs that touch these CRDs are reviewed
  against this ADR.
- A `keleustes break-glass apply` + `keleustes break-glass revert` pair
  belongs in the `keleustesctl` command tree (PROPOSAL §17).
- The `BreakGlassDrift` condition is a first-class status condition on
  affected resources; engines that surface conditions must include it in
  their condition catalog.
- Plan §11 question 6 (Helmfile support depth) is locked: Helmfile state
  files live in Git like every other render input; depth of feature
  support is a separate decision but the invariant is fixed.
- Plan §11 question 7 (drift handling after break-glass) is locked:
  explicit revert, no silent auto-revert.
- This ADR will be revisited only if a new failure class emerges that the
  five-input explainability model (§3) cannot cover — in which case the
  fix is to extend the inputs, not to relax the invariant.
