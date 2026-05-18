<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Value-Change Promotion (`Promotion.spec.changes[]`)

- **Status:** Draft — 2026-05-18
- **Linear:** SKA-432 (this plan). Consumed by SKA-352 (Promotion Engine + state machine), SKA-353 (Git Mutation Engine — GitHub provider), SKA-355 (Native policy gates in Promotion Engine), SKA-359 (ApprovalPolicy CRD + enforcement), SKA-367 (UI action surfaces). Spawns one or more MVP 2 implementation tickets (see §13).
- **Promotes into:** a future ADR co-located with ADR 0003 (Git invariant) once §14 open questions are resolved and the first change-Promotions land against real Applications.
- **Related:** ADR 0003 (Git source-of-truth invariant), ADR 0004 (CRD-based RBAC), ADR 0006 (Engine boundaries), `docs/plans/2026-05-rbac-audit-and-git-invariant.md` §3 (break-glass — the one case that is *not* a change Promotion), `docs/plans/2026-05-operator-crd-integration.md` (SKA-431 — the *kinds* whose values can be changed), `docs/plans/2026-05-audit-event-schema.md` §13.3 (Promotion verbs).
- **Out of scope:** the keleustesctl CLI implementation (lives in SKA-334's scope), the UI form rendering (SKA-335 / SKA-367), the Git Mutation Engine's per-provider PR-API surface (SKA-353). This plan defines the *contract* those tickets implement against.

## 1. Purpose and Scope

`Promotion` today is "promote release X (an artifact) from environment A to environment B." That covers the canonical GitOps flow — a CI run produces a Release, an operator promotes that Release through environments, every transition records audit, applies policy gates, and triggers a SyncRun.

What `Promotion` does **not** cover today is the second-most-common workflow: an operator needs to change a *value* in an environment — `spec.replicas`, `spec.template.spec.containers[name=app].resources.requests.memory`, `spec.template.spec.containers[name=app].env[name=LOG_LEVEL].value`, the cert-manager `Certificate.spec.duration`, a feature flag in a ConfigMap — without (or in addition to) shipping a new release artifact. The change is real desired-state intent; per ADR 0003 it has to round-trip through Git; per ADR 0004 it has to be governed and audited; per ADR 0001 it has to be policy-gated.

There are two unacceptable answers to "how do operators change values":

1. **Free-form Git PRs.** Customer-side scripts edit `kustomize` overlay files and open PRs. Some get reviewed, some don't, some bypass freeze windows entirely, all of them fall outside Keleustes's audit envelope. This is what Argo CD has — a working setup until you ask "did anyone change replicas in prod between 2am and 4am?" and there's no canonical answer.
2. **Live-resource edit UI.** A "edit this YAML" surface that writes directly to the cluster. Per ADR 0003 this is hard-forbidden — the Git invariant collapses if any path bypasses it.

The right answer is to extend `Promotion` to carry structured value diffs in addition to (or instead of) artifact references. A value-change Promotion is an artifact like every other Promotion — governed by `PromotionPolicy`, audited via the §13 verb registry, approved by the same ApprovalPolicy, rolled back via a counter-Promotion. The Git Mutation Engine executes the actual Git mutation. The Sync Engine applies the resulting commit like any other Source revision.

**In scope:**

- The `Promotion.spec.changes[]` shape, including how it composes with the existing artifact-promotion fields.
- Path resolution: how a logical path (`spec.replicas`) becomes a concrete Git file + jsonpath location.
- The `Application.spec.values.schema[]` path-allowlist — what a customer can change, with constraints.
- The Git Mutation Engine handoff: multi-file commits, PR shape, idempotency.
- `PromotionPolicy` gates for change-Promotions.
- Conflict resolution: simultaneous changes targeting the same path.
- Audit envelope verbs.
- UI/CLI affordances at the contract level (what `keleustesctl set` / the UI form *creates*).
- Hard cases — non-Git-expressible changes, multi-environment scope, rollback semantics.
- Phased rollout across MVP 2 / MVP 3.

**Out of scope:**

- Live-resource cluster mutations. Per ADR 0003, break-glass is the single sanctioned exception and lives in its own workflow (SKA-360 — "Break-glass workflow — apply + open PR + drift surfacing"); a value-change Promotion is *never* a break-glass mechanism.
- Schema-version migration of `Application.spec.values.schema[]` itself. Treated like any other API change — versioned via the kubebuilder conversion pipeline; not value-change-Promotion territory.
- Direct mutation of cluster-managed values (CRDs' `.status`, controller-set annotations). Those aren't user-settable; the path-allowlist refuses them.

## 2. Why one machinery, two trigger surfaces

Forking the Promotion machinery into "release Promotions" and "value-change Promotions" would double the state machine surface area, the RBAC scope, the audit verb set, and the test matrix — and quadruple the failure modes when the two need to compose ("promote release X to prod *and* pre-scale the replicas for the launch"). Forking is the path most projects take because the artifact case looks simpler at first; the value case is then bolted on. We're avoiding that.

The Promotion CR already carries:

- The actor (`spec.requestedBy`, populated by the API server from the OIDC subject).
- The scope (`spec.application`, `spec.to.environment`).
- The governance reference (`spec.policyRef` or default-via-Project).
- The audit trail (every state transition emits a `promotion.*` verb per audit-event-schema §13.3).

A value-change Promotion uses every one of these — same actor, same scope, same governance, same audit. The only new field is `spec.changes[]`. Wedging value-change into a *different* CR would duplicate every other field, which is the actual maintenance liability.

The mental model: a Promotion is "an intent to change desired state, scoped to an Application × Environment(s), with the change's content described in `spec`." Release-promotion content is "swap to release X." Value-change content is "set these N paths to these values." Mixed content is "swap to release X *and* override these N paths in the target environment(s)." Same machinery, different content.

## 3. The `Promotion.spec.changes[]` Shape

### 3.1 Schema

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: Promotion
metadata:
  name: checkout-api-bf-warmup-2026-05-18
  namespace: payments
spec:
  application: checkout-api

  # Scope — which environment(s) the change applies to. Replaces
  # the existing `from: / to:` pair when used in changes-only mode;
  # composable with from:/to: in mixed mode (§3.3).
  scope:
    environments: [prod]
    # OR a single environment via the legacy `to.environment` field
    # in mixed mode.

  # Optional artifact promotion (existing). Either of release or
  # changes must be set; both may be set in mixed mode (§3.3).
  release: checkout-api-1.8.2

  # NEW: structured value changes. Each entry is path / from / to /
  # reason. Order is not significant — atomic per Promotion.
  changes:
    - path: spec.replicas
      from: 3
      to: 5
      reason: "Black Friday warm-up capacity"
    - path: spec.template.spec.containers[name=app].resources.requests.memory
      from: 512Mi
      to: 1Gi
      reason: "GC stalls under Black Friday load profile"

  # Actor (populated by the API server admission hook; readers should
  # never trust client-supplied values here — see ADR 0004 §6.3).
  requestedBy:
    type: human
    subject: alice@example.com
    subjectId: okta|01HQ7…
    identityProvider: okta-prod
status:
  # Phase machine (§4). One terminal state per Promotion lifetime.
  phase: Pending | Validating | AwaitingApprovals | InFlight | Merged | Cancelled | Rejected | Failed
  conditions:
    - type: Accepted
      status: "True"
      reason: ChangesValidated
      message: "All 2 changes match the Application's value schema and are within constraints."
    - type: Approved
      status: "False"
      reason: AwaitingApprovers
      message: "1 of 2 required approvers; waiting on @sre-oncall."
  # Reference to the PR the Git Mutation Engine opened. Set once the
  # Promotion enters InFlight phase.
  gitMutation:
    provider: github
    repo: skaphos/marshaller-api-config
    branch: keleustes/promotion-checkout-api-bf-warmup-2026-05-18
    pullRequest: 1234
    commit: ""    # filled in once the PR merges
```

### 3.2 The `change` entry

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `path` | string | yes | The logical path (matches an entry in `Application.spec.values.schema[]` — see §5). Not a JSON Pointer; not a raw file path. Path resolution happens in the Promotion Engine. |
| `from` | scalar / object | **yes** | The current value. Validated against live state at admission time; rejection if it doesn't match (prevents stale-edit races). |
| `to` | scalar / object | yes | The new value. Validated against the schema's `constraints`. |
| `reason` | string | yes for human-actors, no for `system`/`ci` | Free text, audit-bearing. The API server populates from `Promotion.spec.intent` for human actors that omit it. |

`from` is mandatory and validated to prevent the classic "two people edit replicas at the same time, second one clobbers the first" race. If the live value has moved between when the user composed the change and when the Promotion is admitted, the Promotion is rejected with `Failed` phase and `Reason: StaleFromValue`. The user re-issues with the now-current `from`. This is `kubectl apply --field-manager` semantics at the Promotion CR level.

### 3.3 Mixed mode: release + changes

A Promotion may carry both `release` and `changes`. Concrete case: "promote `checkout-api-1.8.2` to prod **and** override the prod-environment `replicas` to 5 because the launch needs warm capacity." The Git Mutation Engine produces a single PR with two semantic changes: the release reference bump and the value override. Atomic by virtue of being one PR / one commit / one Source revision.

The two modes share governance: same `PromotionPolicy`, same approvals, same gates. The audit envelope carries both `payload.release` and `payload.changes` per §8.

### 3.4 Multi-environment scope

```yaml
scope:
  environments: [staging, prod]
changes:
  - path: spec.template.spec.containers[name=app].env[name=FEATURE_FLAG_X].value
    from: "false"
    to: "true"
    reason: "Enable Feature X across staging + prod simultaneously"
```

The Git Mutation Engine produces one PR that touches both environments' files. Atomicity scope is the customer's choice — one Promotion across two envs gives "all or nothing" semantics; two separate Promotions give per-env approvals and freeze-window evaluation.

PromotionPolicy gates may differ per environment (a Promotion targeting `[staging, prod]` evaluates prod's PromotionPolicy gates separately from staging's; approvals roll up — both env's required approvers must approve). When the policies differ, the union applies. When they conflict (e.g., one env requires `imageSigned` for the rolled-forward release, the other doesn't), the stricter wins.

## 4. Phase Machine

Existing phase machine for artifact Promotions extends to value-change with two new phases between `Validating` and `InFlight`:

```text
                          ┌──── (gates fail) ────► Rejected
                          │
   Pending → Validating → AwaitingApprovals → InFlight → Merged
       │         │              │                │
       │         │              ▼                ▼
       │         │          Cancelled          Failed (PR closed without merge,
       │         │                                     conflict, etc.)
       │         ▼
       │     Rejected (schema validation failed)
       │
       ▼
   Cancelled (user-cancelled before any work)
```

Per-phase transitions:

| Phase | Entered when | Exits to | Audit verb |
| --- | --- | --- | --- |
| `Pending` | Promotion CR admitted | `Validating` (immediate) or `Cancelled` (user action) | `promotion-changes-proposed` |
| `Validating` | Promotion Engine starts work | `AwaitingApprovals` (gates ready) or `Rejected` (gates can't pass) | `promotion-changes-validated` |
| `AwaitingApprovals` | Validation passed, approvals not yet sufficient | `InFlight` (approvals met), `Rejected` (denied), or `Cancelled` (timeout / user) | (none — uses `approval.granted.v1` / `approval.denied.v1` per §13.2) |
| `InFlight` | Approvals met; Git Mutation Engine opens PR | `Merged` (PR merged), `Failed` (PR closed without merge or conflict), or `Cancelled` (user cancels with PR open) | `promotion-changes-pr-opened` |
| `Merged` | PR merged into the config-repo's tracked branch | terminal | `promotion-changes-merged` (correlates with the standard `sync-*` verbs that follow once the new Source revision is observed) |

`Validating` does:

1. **Schema validation** — every `change.path` matches a `Application.spec.values.schema[]` entry (§5). Misses → `Rejected: PathNotInSchema`.
2. **Constraints validation** — `change.to` satisfies the schema entry's `constraints` (min/max, regex, enum, type). Misses → `Rejected: ConstraintViolation`.
3. **From-value validation** — `change.from` matches the live value at the resolved Git location. Misses → `Failed: StaleFromValue` (special — operator should re-issue rather than treat as a Reject).
4. **Conflict check** — no other in-flight Promotion targets the same `(application, environment, path)` tuple. Misses → `AwaitingApprovals` with `BlockedBy: <other-promotion-name>` condition; resolves when the other Promotion reaches a terminal state.

## 5. Path Resolution via `Application.spec.values.schema[]`

### 5.1 Why the Application declares the schema

The Promotion CR carries logical paths like `spec.replicas`. The Git Mutation Engine needs to know *where in Git* to make the edit — which file, which jsonpath inside the file. The Application is the only place that knows this — it owns the relationship between its manifests and their per-environment overlay structure.

The schema is also the **allowlist**: any path not in the schema is platform-locked. A customer can't change `spec.template.spec.securityContext.runAsRoot` because the schema doesn't list it. This is how Skaphos enforces the platform-vs-application boundary at the data layer rather than via runtime checks.

### 5.2 Schema entries

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: Application
metadata:
  name: checkout-api
spec:
  # ... existing fields ...
  values:
    schema:
      - logicalPath: spec.replicas
        description: "Number of replica pods."
        scope: per-environment            # per-environment | global
        location:
          # The file pattern, with ${env} substituted by the
          # environment's name. Resolves to one file per
          # change.to value.
          file: env/${env}/replicas.yaml
          # JSON Pointer inside the file. (Yes JSON Pointer here,
          # not jsonpath — pointer is the precise shape RFC 6902
          # uses, which is what the Git Mutation Engine writes.)
          jsonPointer: /replicas
        type: integer
        constraints:
          minimum: 1
          maximum: 100
        # Optional: requires that a change to this path also bumps
        # another path (e.g., changing replicas in prod requires
        # updating the matching HPA min). The Promotion Engine
        # rejects partial updates that don't satisfy.
        coChange: []

      - logicalPath: spec.template.spec.containers[name=app].image
        description: "Container image for the main app container."
        scope: per-environment
        location:
          file: env/${env}/image.yaml
          jsonPointer: /image
        type: string
        constraints:
          pattern: "^ghcr\\.io/skaphos/.*"
        # When the path corresponds to an existing Release reference,
        # the Promotion CR's release: field may carry the same intent
        # in mixed mode (§3.3). The schema entry documents which
        # mode is preferred for this path.
        promotionMode: prefer-release

      - logicalPath: spec.template.spec.containers[name=app].env[name=LOG_LEVEL].value
        description: "Log level for the main app container."
        scope: per-environment
        location:
          file: env/${env}/env.yaml
          jsonPointer: /containers/app/env/LOG_LEVEL
        type: string
        constraints:
          enum: [debug, info, warn, error]
```

### 5.3 Scope: per-environment vs. global

`per-environment` paths exist once per `Environment` the Application is deployed to. The `${env}` substitution in `location.file` is what makes the schema reusable. A change-Promotion that targets `scope.environments: [prod]` rewrites only the prod overlay; targeting `[staging, prod]` rewrites both.

`global` paths exist once per Application across all environments. A change-Promotion to a global path always has `scope.environments` empty (or the special value `[*]` for clarity); the Git Mutation Engine writes the same file regardless of which environment the operator was viewing when they composed the Promotion.

### 5.4 What about CRD values (cert-manager `Certificate.spec.duration`, etc.)

CRD-owned values aren't different in the schema from native-kind values. The schema entry's `logicalPath` is whatever the customer writes in their Kustomize overlay / Helm values — there's no Kubernetes-API-aware logic at this layer. If the customer's overlay structure has the cert-manager `Certificate.spec.duration` at `cert/${env}/duration.yaml`, the schema entry reflects that. The Promotion Engine doesn't need to know what kind owns the value.

This works because path resolution is **file-level**, not **rendered-object-level**. The schema names the source-of-truth location in Git, not the resulting rendered manifest. A `Certificate` rendered from a Helm chart values file is the same as a `Deployment` rendered from a Kustomize patch — both have schema entries pointing at their values files.

The `HealthAssessor` / `DiffNormalizer` plumbing from SKA-431 takes care of post-apply behavior. The schema is upstream of that — it's about *how to compose a Git mutation*, not *how to interpret the resulting cluster state*.

### 5.5 What's NOT in the schema by default

A new Application has an empty `values.schema[]`. Customers add entries deliberately — adding a path means "operators with the right RBAC may change this." The platform-vs-application boundary is the customer's call, not Skaphos's; we ship sensible defaults in the `keleustes-curated` bundle (see SKA-431 §4.4) but Applications opt in via per-app schema population.

A future plan covers how Skaphos templates the schema for common Application archetypes (Deployment-with-HPA, StatefulSet-with-PVC, Helm-chart-with-values, etc.) so customers don't write the schema by hand. Out of scope here.

## 6. Git Mutation Engine Handoff

The Git Mutation Engine (SKA-353) is the component that actually opens and tracks the PR. The Promotion Engine hands it a structured "mutation request" once `InFlight` is reached. Contract:

```go
// in internal/mutation/types.go (MVP 2)

type MutationRequest struct {
    // The Promotion this mutation implements. Audit envelopes for
    // every produced commit carry this as the requestId.
    PromotionRef PromotionReference

    // The config-repo + branch the mutation targets. Resolved from
    // the Application's Source(s) at admission time so the
    // Mutation Engine doesn't have to re-resolve.
    Repo   GitRepoRef
    Branch string

    // The actual content of the mutation. One MutationOp per entry
    // in Promotion.spec.changes[] (plus one additional MutationOp
    // for the release reference bump in mixed mode).
    Ops []MutationOp

    // Auth + author identity. Author is the human/CI actor that
    // triggered the Promotion; committer is the Keleustes service
    // identity. Audit ties the two.
    AuthorIdentity   Identity
    CommitterIdentity Identity
}

type MutationOp struct {
    File         string  // resolved per Application.spec.values.schema[].location
    JSONPointer  string  // resolved
    From         any     // for the conflict check + PR body
    To           any
    Reason       string
}
```

### 6.1 Single PR per Promotion, regardless of MutationOp count

Every `MutationRequest` produces exactly one PR with one commit. The commit message is structured for auditability — first-line summary, body listing each Op:

```
promotion(checkout-api): bf-warmup-2026-05-18

Author: alice@example.com (Promotion: payments/checkout-api-bf-warmup-2026-05-18)
Reason: Black Friday warm-up capacity

Changes:
  env/prod/replicas.yaml: /replicas: 3 → 5
  env/prod/resources.yaml: /containers/app/requests/memory: 512Mi → 1Gi

Refs: SKA-432
```

PR body includes:
- The Promotion CR's name + URL (deep link into the Keleustes UI).
- Per-Op diff preview.
- Approval chain (which `Approval` CRs satisfied which `PromotionPolicy` gate).
- A correlation ID that ties the PR back to the `Promotion.status.gitMutation.pullRequest`.

### 6.2 Idempotency

Re-running the Git Mutation Engine for the same `MutationRequest` (e.g., after a transient API error retried by the Promotion Engine) produces a byte-identical commit. The branch name is deterministic (`keleustes/promotion-<promotion-name>` per §3.1) so the second attempt either finds the existing PR (no-op) or fails-cleanly if the first attempt partially completed.

The Mutation Engine never force-pushes. If the branch exists with content different from what this `MutationRequest` would produce (e.g., a human edited it), the Mutation Engine refuses and transitions the Promotion to `Failed: BranchModified` — operator decides whether to abandon or restart.

### 6.3 PR review = approval, PR merge = sync trigger

`PromotionPolicy` approvals are recorded as `Approval` CRs (existing). The Mutation Engine adds the relevant approvers as **PR reviewers** so the GitHub review UI is a parallel surface for the same governance — an operator can approve in Keleustes UI *or* via GitHub UI; the result is identical (both write `Approval` CRs).

Once `PromotionPolicy` gates pass and required approvals are recorded, the Mutation Engine *auto-merges* the PR (modulo any branch-protection-required reviewers — those are layered on top per the customer's GitHub config). PR merge triggers a Source revision; the Source Engine detects it; the Application reconciles; the Sync Engine drives a SyncRun that applies the new value to the target environment. The Promotion's `Merged` phase aligns with PR merge, not SyncRun completion — the Promotion's *intent* is fulfilled by the merge; downstream rollout is observable but not part of the Promotion's lifecycle.

## 7. PromotionPolicy Gates for Change-Promotions

The existing gate set (per `docs/plans/2026-05-extensibility-plugin-surfaces.md`) — `imageSigned`, `vulnThreshold`, `changeApproved`, `ownerApproved`, `sourceHealthy`, `targetUnlocked`, `imageSigned` — was designed for release-promotion semantics. Most carry over for change-Promotions verbatim:

| Gate | Release-Promotion meaning | Change-Promotion meaning |
| --- | --- | --- |
| `changeApproved` | One distinct approver with the `approver` role | Same |
| `ownerApproved` | Application's `OwnerInfo.team` approver | Same |
| `targetUnlocked` | Target environment not in a freeze window | Same |
| `sourceHealthy` | The release's Source is healthy (revision resolved, signature verified) | n/a (no Release reference in changes-only mode) |
| `imageSigned` | The release's image carries a valid signature | Only checked if the change's `to` value is a container image reference (i.e., the schema entry has `promotionMode: prefer-release` but the customer is using changes-mode for it) |
| `vulnThreshold` | Same condition | Same — only when relevant per the change's `to` value |

Two new gates land with change-Promotions:

- **`changeInAllowlist`** — every `change.path` is present in `Application.spec.values.schema[]` *and* every `change.to` satisfies the schema's constraints. Always evaluated.
- **`changeAtomic`** — for changes that declare `coChange` in their schema entry, the Promotion includes all required co-changes. Prevents partial-update misconfigurations (e.g., changing replicas without updating the HPA's `minReplicas`).

The `PromotionPolicy` CR's `required` field lists which gates apply; customers compose policies per Project / per Environment / per Application as they do today.

## 8. Audit Envelope Verbs

New verbs added to the §13 registry of `docs/plans/2026-05-audit-event-schema.md` §13.3 (Promotion):

| Verb | Payload type | Intent required | Notes |
| --- | --- | --- | --- |
| `promotion-changes-proposed` | `promotion.changes.proposed.v1` | yes | Promotion CR admitted with `spec.changes[]` populated |
| `promotion-changes-validated` | `promotion.changes.validated.v1` | no (system) | Schema + constraints + from-value + conflict checks passed |
| `promotion-changes-rejected` | `promotion.changes.rejected.v1` | no (system) | Validation failed; carries the reason enum |
| `promotion-changes-pr-opened` | `promotion.changes.pr.opened.v1` | no (system) | Git Mutation Engine opened the PR; correlates with `git-pr-opened` per §13.6 |
| `promotion-changes-merged` | `promotion.changes.merged.v1` | no (system) | PR merged; new Source revision pending |
| `promotion-changes-cancelled` | `promotion.changes.cancelled.v1` | yes | User-cancelled (before or after PR open) |
| `promotion-changes-failed` | `promotion.changes.failed.v1` | no (system) | Terminal failure — PR closed without merge, branch modified, etc. |

The `promotion.changes.proposed.v1` payload carries the full `Promotion.spec.changes[]` so SIEM consumers and the audit UI can show "alice@example.com changed prod replicas from 3 to 5 because Black Friday." This is the single most useful audit event in the entire system — it's the "who did what" answer for the operational layer.

Existing `promote` / `promotion-cancelled` / `promotion-advanced` / `promotion-completed` verbs from §13.3 continue to apply to the artifact-promotion subset of change-Promotions in mixed mode.

## 9. Conflict Resolution

Two cases.

### 9.1 Concurrent same-target changes (merge-train)

Promotion A targets `(checkout-api, prod, spec.replicas)` and is in `InFlight`. Promotion B targets the same tuple and is admitted while A is still in flight.

- B enters `AwaitingApprovals` with `Condition[BlockedBy=Promotion/<A's name>]`.
- B's gates evaluate as if A had merged (so approvals can accumulate against B in parallel with A's PR review). The blocker is the merge order, not approval availability.
- When A reaches `Merged` or `Cancelled`:
  - If A merged: B's `change.from` is now stale (it referenced the pre-A value). The Promotion Engine re-evaluates B's from-value check against the new live state — most often this means B is auto-failed with `Failed: StaleFromValue` and the operator re-issues with an updated `from`.
  - If A was cancelled: B's `from` is still valid; B proceeds.

This is the "merge train" pattern from kernel patch management, scaled to value changes. Race-free at the cost of forcing operators to re-state their `from` when another change beats them to the punch — which is exactly what we want, because the alternative is silently clobbering each other.

### 9.2 Cross-application conflicts (out of scope here)

Promotion A targets `checkout-api`'s `replicas`; Promotion B targets `billing-api`'s `replicas`. No conflict — different Applications, different tuples.

Cross-application coordination ("don't promote billing-api until checkout-api is stable") is the cross-Application dependency mechanism per engine-boundaries plan §2.6 + SKA-339. Out of scope for value-change Promotion semantics.

## 10. UI / CLI Affordances (Contract Only)

This plan defines what `keleustesctl set` and the UI form *create* — implementation lives in their respective tickets (SKA-334 / SKA-335 / SKA-367).

### 10.1 CLI

```bash
# Single-path change
keleustesctl set replicas 5 \
  --app=checkout-api \
  --env=prod \
  --reason="Black Friday warm-up"
# → Posts a Promotion CR with:
#     spec.application = checkout-api
#     spec.scope.environments = [prod]
#     spec.changes = [{path: spec.replicas, from: <live>, to: 5,
#                      reason: "Black Friday warm-up"}]

# Multi-path change (atomic)
keleustesctl set \
  --app=checkout-api --env=prod \
  --change=spec.replicas=5 \
  --change="spec.template.spec.containers[name=app].resources.requests.memory=1Gi" \
  --reason="Black Friday warm-up capacity + GC stalls"

# Mixed mode (release + override)
keleustesctl promote \
  --app=checkout-api --release=checkout-api-1.8.2 \
  --from=staging --to=prod \
  --override=spec.replicas=5 \
  --reason="Promotion + warm-up"
```

The CLI:
1. Resolves the path against the Application's `spec.values.schema[]` (refusing if the path isn't allowlisted).
2. Queries the current value (`change.from`) from Git via the Source Engine.
3. Composes the Promotion CR client-side.
4. Posts it to the API server, which runs the admission webhook + emits `promotion-changes-proposed`.

### 10.2 UI

The Application page surfaces an "Override values" panel per environment. The panel reads `Application.spec.values.schema[]` and renders a form with one field per `logicalPath` (typed: number-spinner for `integer`, dropdown for `enum`, text-with-validation for `pattern`). Current values come from the Source Engine's render output. Submit composes a Promotion CR with the changed fields and POSTs it.

The form is **read-only** for fields the user lacks `Role` permission to change at this scope (per ADR 0004's RBAC) — they see the current value but the input is disabled. This makes the platform-vs-application boundary visible without forcing the user to read RBAC YAML.

## 11. Hard Cases

### 11.1 Non-Git-expressible changes

"Restart this StatefulSet now." "Force-delete this stuck namespace." "Drain this node."

These are live-cluster actions with no Git delta. Per ADR 0003 they are **break-glass** territory and route through SKA-360 (the break-glass workflow), not through change-Promotion. Calling this out in the plan so operators don't try to use `spec.changes[]` as an action API and so contributors don't expand `spec.changes[]` to support imperative verbs.

The schema enforces this — there's no `change.action` field, only `path/from/to`. A change that can't be expressed as a JSON Pointer edit on a file in Git can't be expressed in `spec.changes[]`. By design.

### 11.2 Schema additions

The customer wants to make a previously platform-locked value user-settable. They edit `Application.spec.values.schema[]` to add the entry. That edit is itself a change to the Application CR — and the Application CR lives in Git like everything else. So the schema addition goes through the regular Application-update PR flow (which has its own approvals, audits, governance).

There's no "platform admin escape hatch" for schema additions. The Project/Application RBAC governs who can edit the schema entry, the audit envelope records the schema change as a `crd.write.v1` event on the Application, and downstream value-change Promotions can immediately use the newly-allowlisted path on the next reconcile.

### 11.3 Rollback semantics

The customer realizes the value change was wrong and wants to revert. Two paths:

**Counter-Promotion (recommended).** Open a new Promotion with `change.from` and `change.to` swapped, marked `metadata.annotations.keleustes.skaphos.io/rollback-of: <original-promotion-name>`. New audit chain (`promotion-changes-proposed` → ... → `promotion-changes-merged`), new PR, new SyncRun. The original Promotion is left as historical record.

**Git revert (discouraged).** `git revert <commit-from-the-original-PR>` opens a revert PR. The Source Engine picks it up on merge, but no Keleustes-side `Promotion` CR exists for the revert. Audit is incomplete (no `promotion-*` events for the rollback) and policy gates aren't re-checked. **The CLI refuses to do this directly**; operators can do it on the GitHub side, but the docs warn that doing so bypasses governance.

The recommended pattern is implemented by `keleustesctl rollback <promotion-name>` which composes the counter-Promotion automatically.

### 11.4 Drift between change-Promotion intent and actual live state

A Promotion merges. The Source Engine observes the new revision. The Application reconciles. But the SyncRun fails (cluster issue, conflicting field manager, whatever).

The Promotion's lifecycle is **complete at Merged** — the operator's *intent* has been recorded and the change is in Git. Downstream sync failure is observable via the regular `sync-failed` audit event + `SyncRun.status` + Notifier delivery, but doesn't reopen the Promotion. The operator either:

- Diagnoses and fixes the SyncRun, at which point the change applies and is real, or
- Opens a counter-Promotion that reverts the value (per §11.3) if they no longer want the change.

This decouples "the operator intended X" from "X is in production." The audit envelope captures both — `promotion-changes-merged` records intent; `sync-completed` / `sync-failed` records execution.

## 12. Phased Rollout

| MVP | Work in this plan's scope |
| --- | --- |
| **MVP 2** | `Promotion.spec.changes[]` schema lands (extends existing Promotion CRD). `Application.spec.values.schema[]` lands. Promotion Engine validation + Git Mutation Engine handoff for **single-environment** scope and **Kustomize file** resolution. Mixed-mode (release + changes) supported. Merge-train conflict detection works for in-flight Promotions. Audit verbs registered. CLI: `keleustesctl set` for single-path changes. UI: "Override values" form on the Application page. **Initial coverage = native Kubernetes kinds + cert-manager + Argo Rollouts** via Skaphos-curated schema templates. |
| **MVP 3** | **Multi-environment** scope (atomic across two-plus envs in one Promotion). **Helm values file** path resolution. `coChange` cross-path atomicity. Curated schema templates expand to the SKA-431 §4.4 operator list (Crossplane, Cluster API, Tekton, Knative, Istio, External Secrets, Prometheus Operator). |
| **MVP 4** | **Complex constraints** — cross-field rules in the schema (e.g., "replicas can't exceed nodes×16"). **Value-history navigation in UI** — "show me every change to this value over the last 90 days." Promotion-search / Promotion-bisect UX (which Promotion introduced the value that's now causing issues). |

## 13. Concrete Follow-ups

1. **SKA-### — Extend the `Promotion` CRD with `spec.changes[]` and `spec.scope.environments`** (MVP 2). Schema-only ticket; reconciler logic comes next.

2. **SKA-### — Extend the `Application` CRD with `spec.values.schema[]`** (MVP 2). Schema-only.

3. **SKA-### — Promotion Engine path resolution + validation** (MVP 2). The `Validating` phase implementation: schema-match, constraints, from-value, conflict checks.

4. **SKA-### — Git Mutation Engine `MutationRequest` interface + GitHub provider implementation** (MVP 2). Companion to SKA-353 — defines the contract this plan's §6 describes.

5. **SKA-### — `changeInAllowlist` + `changeAtomic` policy gates** (MVP 2). Plugs into the existing gate registry.

6. **SKA-### — Audit verb registration** (MVP 2). Update the audit-event-schema §13.3 registry with the new verbs from §8 above.

7. **SKA-### — `keleustesctl set` + `keleustesctl rollback` commands** (MVP 2). CLI affordances per §10.1.

8. **SKA-### — UI "Override values" form** (MVP 2). UI per §10.2; renders against the Application's schema.

9. **SKA-### — Skaphos-curated schema templates for common Application archetypes** (MVP 2 → MVP 3). Helm-chart-with-values, Kustomize-with-overlay, Deployment-with-HPA, StatefulSet-with-PVC. Lives in `skaphos/keleustes-curated` alongside the HealthAssessor bundle from SKA-431.

10. **PROPOSAL §14 (Git mutation) cross-link.** Add a `> See SKA-432` marker noting that value-change Promotion is the canonical Git-mutation workflow.

11. **DECISIONS.md row.** Plan listed in "Plans that have not yet stabilized" with this PR; promotes to an active interim contract once §14 open questions resolve and the first MVP 2 reconciler scaffolds land.

## 14. Open questions

1. **Can `spec.changes[]` carry a change to a value the schema marks as `promotionMode: prefer-release`?** Spec entry says "use a Release reference for this." Customer says "I want to override it for one Promotion only." Allowed (with a warning), or refused? Lean: allowed with audit annotation; the schema's `promotionMode` is a hint, not a hard rule.

2. **PR auto-merge vs. require-human-merge.** Auto-merge once `PromotionPolicy` gates pass is the convenience win; require-human-merge is the additional safety net some customers want. Probably a per-`PromotionPolicy` boolean (`autoMerge: true|false`); default `true` because the gates already encode the safety. Need real-customer input.

3. **Co-change validation against rendered output vs. against the schema declaration.** `coChange` says "changing replicas requires updating HPA min." We could validate this against the schema (entries declared as `coChange`-related must all appear in the Promotion) or against the rendered output (after render, the resulting manifests satisfy a cross-resource invariant). The first is simpler; the second is correct. Probably start with the first and revisit if customers hit corner cases.

4. **Audit envelope size for large `payload.changes[]`.** The audit envelope's 64 KiB snapshot cap (§8.1 of audit-event-schema) applies to `result.before`/`result.after`. The `payload.changes[]` array could exceed this in pathological cases (a Promotion with 50+ changes). Spill-to-object-storage following the same pattern as oversized snapshots — extend the `@oversize` mechanism to the payload field. Confirm before MVP 2 lands.

5. **What about value changes to in-progress Releases?** Customer wants to override a value in a Release that's mid-promotion through environments (still in staging, not yet in prod). Override targets the Release, not the environment. Schema would need to support `scope: per-release` in addition to `per-environment` and `global`. Defer to MVP 3 — MVP 2 ships `per-environment` and `global` only.

6. **Approval routing for change-Promotions vs. artifact Promotions.** Customers may want different approvers for "promote release" vs. "change value" (a release Promotion may have a stable approval chain — the release engineering team — while a value change touches per-team config). Probably need `ApprovalPolicy.spec.appliesTo: [release | changes | both]` (default `both`). Confirm via customer interviews before MVP 2.

7. **Multi-step value changes ("ramp replicas from 3 to 100 over 30 minutes").** Out of scope — that's a different workflow (autoscaler config, or an ad-hoc operator runbook). A change-Promotion is a single atomic edit. Document the boundary.

8. **What's in the `Promotion.spec.changes[].path` namespace for kinds with multiple instances?** A single Kustomize overlay may contain multiple Deployments. The `logicalPath` `spec.replicas` is ambiguous. Probably the `Application.spec.values.schema[]` entry includes a `targetKindRef` for disambiguation, and the `change.path` carries enough context. Need concrete examples to confirm the schema shape.

## 15. Compliance with Prior Decisions

| Decision | This plan honors it by |
| --- | --- |
| ADR 0003 (Git invariant) | Every value change round-trips through a Git PR via the Mutation Engine. No live-cluster mutation path is added. |
| ADR 0004 (CRD-based RBAC) | `Role`/`RoleBinding` gate who can create change-Promotions; per-path RBAC via the `Application.spec.values.schema[]` entry (a `Role` may grant permission to set some paths but not others, scoped via the Project boundary). |
| ADR 0006 (Engine boundaries) | Path resolution, validation, and Promotion machinery live in `internal/promotion/`. Git Mutation Engine lives in `internal/mutation/`. No gitops-engine import; no cross-engine direct dependency. |
| ADR 0001 (Plugin extension model) | `changeInAllowlist` and `changeAtomic` are native gates, but the gate-evaluation contract is the same one external `PolicyGate` plugins use. A customer could ship a third-party gate that validates the change content against their own policy engine. |
| Audit-event-schema plan §13 | New verbs added to §13.3 (Promotion); payloads follow the discriminated-union pattern in §7.1; redaction rules apply to `change.from`/`change.to` for any path resolving to a sensitive field. |
| RBAC plan §3 (break-glass) | Explicitly distinguished from value-change Promotion in §11.1 — break-glass is for non-Git-expressible actions, change-Promotion is for everything Git can represent. |
| Operator CRD integration plan (SKA-431) | CRD-owned values use the same schema entries as native kinds; the integration plan's `HealthAssessor` / `DiffNormalizer` machinery operates on the post-apply state, downstream of this plan's pre-apply mutation contract. |

---

**When this plan stabilizes** (after §14 open questions resolve, after SKA-352's first reconciler implementation lands, and after at least one real customer Promotion has gone through the merge-train in staging), §1–§13 promote into a new ADR — likely co-located with ADR 0003 (since this plan's core contribution is extending the Git-invariant machinery to value-change semantics). The schema details (`spec.changes[]`, `spec.values.schema[]`) become the durable record; the implementation details and open questions stay in working material.
