<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# RBAC, Audit, and the Git-Source-of-Truth Invariant

**Status:** Draft
**Date:** 2026-05
**Related:** PROPOSAL §9 (Architecture), §14 (Promotion Policy), §16 (UI), §17 (CLI); distributed-runtime-architecture.md; engine-boundaries-and-technology-integration.md
**Owner:** Platform Architecture (Skaphos)
**Purpose:** Define identity, RBAC, user-action audit, and the inviolable rule that all desired state lives in Git. This plan exists to ensure Keleustes does not repeat Argo CD's mistakes around parameter overrides, in-tool edits, and policy-as-config-file.

---

## 1. The Three Things This Plan Is About

1. **Identity & RBAC** that scales past Argo CD's policy-file model — CRD-based, delegable per project/team, with action-level granularity, separation of duties, approval gates, and time-bound break-glass.
2. **User-action audit** as a first-class citizen — every action that affects state is recorded into the JetStream audit log with actor, intent, target, result, and request ID. Queryable from the central UI.
3. **The Git-source-of-truth invariant** — no feature in Keleustes may create or mutate desired state outside Git. The Argo CD "parameter override," "sync with overridden parameters," and "edit live manifest" patterns are explicitly forbidden. Break-glass is a defined, audited, time-bounded workflow that itself commits its trace to Git.

These three are tightly coupled: RBAC controls *who* can act, audit records *what* they did, and the Git invariant defines *what an action even is* — namely, something that produces a Git commit (or a CRD change that is itself in Git).

## 2. What Argo CD Got Right, and Where It Hurts at Scale

**Right:** OIDC integration with group claims; project-scoped resources; Casbin-based policy language; AppProject as a delegation boundary.

**Hurts at our scale:**

- **One big policy file.** RBAC policy as a single `policy.csv` (in a ConfigMap) becomes unmanageable at hundreds of teams. Editing it is a centralized bottleneck.
- **No delegated RBAC.** A team cannot manage its own RoleBindings within its own project without admin help. Either admins gate everything, or you give too much power.
- **Limited resource granularity.** "Can sync this application" is expressible; "can sync this application to prod-us but not prod-eu" is awkward.
- **No first-class separation of duties.** Same identity can request and approve a promotion unless you wire something external.
- **No time-bound break-glass.** Either you have the permission or you don't; emergency elevation requires editing the policy file.
- **Parameter overrides as a UI feature.** The biggest unforced error. You can set `app.spec.source.helm.parameters` or use `argocd app set --parameter` to apply a value that does not exist in any Git repo. Now the deployed state cannot be reproduced from Git alone. This is the trap.
- **"Edit live resource."** The Argo CD UI lets you patch live cluster resources. The next reconcile undoes it (eventually), but in the meantime you've broken the invariant.

Keleustes must outperform on the first six and must absolutely refuse the last two.

## 3. The Git-Source-of-Truth Invariant (Hard Rule)

**Rule:** *Every byte of desired state for every Application on every target is derived deterministically from a commit in a Git repository the user controls. Keleustes does not store, mutate, or apply any desired state that is not in Git.*

### What this forbids

- ❌ **Parameter overrides in CRDs.** No `spec.source.helm.parameters`, no `spec.kustomize.images`, no equivalents that live in the `Application` CR and aren't in Git. Values come from a values file or a kustomization in Git. Period.
- ❌ **CLI / UI "set" verbs that change desired state.** `keleustesctl app set --image foo:bar` does not exist. The equivalent workflow is `keleustesctl promote --bump-image foo:bar` which **creates a Git commit** that does the change.
- ❌ **"Edit live resource" affordances in the UI.** The UI is read + actions-that-write-Git. It is not a `kubectl edit` front-end.
- ❌ **In-cluster `kubectl apply` of mutated manifests by Keleustes.** Any mutation must round-trip through Git first.
- ❌ **Helm `--set` style invocations whose arguments are not in Git.** All Helm values come from values files in Git. Promotion is a values-file edit + commit.

### What break-glass looks like (the *one* sanctioned exception)

Break-glass is **not** "let me bypass the rule." Break-glass is **"let me make a direct cluster mutation when Git is unreachable or speed is more important than the round-trip, while ensuring the action is auditable and reconcilable."**

Concrete shape:
1. User holds the `break-glass` action permission (RBAC; usually time-bound — see §5.6).
2. User invokes the break-glass workflow (`keleustesctl break-glass apply ...` or UI button with confirmation).
3. Keleustes records the intent in the audit stream **before** applying.
4. Keleustes applies the change with a dedicated field-manager (`keleustes-break-glass`).
5. Keleustes opens a PR against the config repo capturing what was applied (`break-glass: <timestamp> <actor> <reason>`).
6. The next normal reconcile detects drift between Git and live and surfaces it as a `BreakGlassDrift` condition until the PR is merged (reconciling Git with reality) or reverted (reconciling reality with Git).
7. The condition does not auto-clear; a human resolves it.

This is the only path that touches the cluster outside Git, and it leaves a record that itself becomes Git history.

### What this enables

- **Bit-perfect reproducibility.** Any historical desired state can be recreated by checking out the relevant commits.
- **Trivial disaster recovery.** Re-bootstrap from Git; no "find the last set of overrides" step.
- **Simple audit story.** "What was deployed?" → "what was in the manifest at commit X." No reconstruction needed.
- **Real GitOps, not Git-also.**

## 4. Identity

### 4.1 Identity sources

| Identity type | Mechanism | Used for |
|---------------|-----------|----------|
| **Human users** | OIDC (any provider — Okta, Entra ID, Google Workspace, GitLab, Keycloak, Dex) | UI access, `keleustesctl` interactive use, approval actions |
| **CI / automation** | OIDC workload identity (GitHub OIDC, GitLab OIDC, cloud workload identity) **or** mTLS client certs **or** Kubernetes ServiceAccount tokens | API calls from CI/CD pipelines to trigger promotions, fetch state, etc. |
| **Agents** | NKey + signed JWT (NATS leaf transport; runtime plan §7.4) | Agent → hub communication |
| **In-cluster controllers** | Kubernetes ServiceAccount tokens | Operator components talking to apiserver |

**No Keleustes-issued passwords.** No local user database. Identity is always federated.

### 4.2 Group claims

OIDC group claims (`groups`, `roles`, custom) are the primary attribute used in RoleBindings. The mapping from OIDC claim → Keleustes group is per-deployment config (an OIDC `IdentityProvider` CRD; see §5.2). Customers manage groups in their IdP, not in Keleustes.

### 4.3 Multiple IdPs

A deployment can have multiple `IdentityProvider` resources — common pattern is "humans via Okta, CI via GitHub OIDC." Audit records carry which IdP authenticated the actor.

## 5. RBAC

### 5.1 Design goals

- **Expressed as CRDs**, not config files. RoleBindings live in the config Git repo alongside everything else.
- **Delegable per project.** A project admin can create RoleBindings within their project scope without touching cluster-wide config.
- **Action-verb level.** Not just "read" / "write" — distinct verbs for `view`, `sync`, `promote`, `approve`, `cancel-promotion`, `edit-source`, `edit-policy`, `break-glass`, etc.
- **Scoped tightly.** A binding can target an Application, a set of Applications via label selector, an Environment, a Cell, a DeploymentTarget, or a Project.
- **Separation of duties.** A configurable constraint that "requester != approver" on a given Promotion. Multi-actor approval (N-of-M) for sensitive promotions.
- **Time-bound.** Bindings can carry `validUntil`; auto-expire without manual revocation.
- **Auditable.** Every grant, revoke, and role change is itself an audit event.

### 5.2 New CRDs (proposed)

```
keleustes.skaphos.io/v1alpha1
├── IdentityProvider     # OIDC config; group claim mapping
├── Role                 # named set of verbs + resource scopes
├── RoleBinding          # binds subjects (group/user/SA) to a Role within a scope
├── Project              # delegation boundary; groups Applications, Sources, Environments
└── ApprovalPolicy       # N-of-M, separation-of-duties, break-glass elevation rules
```

These are designed to be edited in Git and reconciled like every other Keleustes CRD. The Argo CD `policy.csv` model is *explicitly rejected*.

### 5.3 Verbs (initial set)

| Resource | Verbs |
|----------|-------|
| `Application` | `view`, `create`, `edit`, `delete`, `sync`, `pause`, `resume` |
| `Source` | `view`, `create`, `edit`, `delete`, `force-refresh` |
| `Release` | `view`, `create`, `edit`, `delete` |
| `Promotion` | `view`, `create`, `approve`, `cancel` |
| `PromotionPolicy` | `view`, `edit` |
| `Approval` | `view`, `grant` |
| `FreezeWindow` | `view`, `create`, `edit`, `delete`, `override` |
| `DeploymentTarget` | `view`, `register-agent`, `edit`, `break-glass` |
| `Environment` / `Cell` | `view`, `edit` |
| `RoleBinding` (within project) | `view`, `create`, `edit`, `delete` |
| (cross-cutting) | `break-glass`, `view-audit`, `query-state` |

Per resource, scope can be:
- `cluster` (rare; admin only)
- `project: <name>` (recommended default)
- `application: <name>` (very narrow)
- `selector: <label-selector>` (e.g., `team=payments`, `tier=prod`)
- `environment: <name>` + sub-scopes

### 5.4 Project as the delegation boundary

A `Project` carries:
- A set of Applications, Sources, Environments (by name or selector)
- A set of `DeploymentTarget`s the project is allowed to deploy to
- A list of project-admin groups (these groups can create RoleBindings within the project scope)
- Default `ApprovalPolicy` reference

This is the unit a platform team hands to an application team: "here is your project, here are the targets you can reach, manage your own RoleBindings inside it."

### 5.5 Separation of duties & multi-actor approval

`ApprovalPolicy` (per Project, per Promotion gate, or per `PromotionPolicy`) expresses:
- `requireDistinctActors`: requester and approver must be different identities
- `minApprovers: N`
- `requiredGroups`: at least one approver must be in group X
- `excludeGroups`: members of group X cannot approve (e.g., the requester's direct reports)

The Promotion state machine enforces these — there is no place in code or CLI that lets you skip them.

### 5.6 Time-bound break-glass

`RoleBinding` can carry:
- `validUntil: <timestamp>`
- `reason: <string>` (required for short-lived bindings)
- `auditTicket: <string>` (e.g., link to an incident ticket — recorded with the audit event)

Common pattern: an incident commander self-grants `break-glass` on a project for 2 hours via a CLI command; the grant itself is an audit event; the expiry is automatic; the use of `break-glass` produces its own audit events.

### 5.7 Default-deny

Subjects with no matching RoleBinding get **no access**, not read-only. Read-only is a verb (`view`) that must be granted explicitly. This is more restrictive than Argo CD's default; we accept that tradeoff for correctness.

## 6. Audit

### 6.1 What gets audited

| Category | Examples |
|----------|----------|
| **Identity events** | login (when surfaced by IdP), OIDC token refresh, agent registration, agent disconnection |
| **RBAC changes** | Role / RoleBinding / Project / ApprovalPolicy create / edit / delete |
| **User actions on CRDs** | every API write that creates, updates, or deletes a Keleustes CRD — actor, before, after, request ID |
| **Promotion lifecycle** | request, gate evaluations, approval grants, phase transitions, cancellation, completion |
| **Sync lifecycle** | SyncRun phase transitions, applied resource counts, errors |
| **Git mutations** | every Git commit / PR Keleustes opens, including break-glass-driven ones |
| **Source events** | new revision detected, signature verification result |
| **Break-glass usage** | every break-glass invocation, what was applied, by whom, with what reason |
| **Configuration changes** | any change to operator config (IdentityProvider, FreezeWindow, etc.) |

### 6.2 Audit event shape

```json
{
  "eventId": "ulid-of-event",
  "occurredAt": "2026-05-15T18:42:13.123Z",
  "actor": {
    "type": "human|ci|agent|system",
    "subject": "user@example.com or service-account/foo",
    "identityProvider": "okta|github-oidc|...",
    "groups": ["sre", "payments"],
    "sessionId": "..."
  },
  "action": {
    "verb": "promote|sync|approve|break-glass|edit-role|...",
    "resource": {
      "kind": "Promotion",
      "namespace": "payments",
      "name": "checkout-api-to-prod-2026-05-15"
    },
    "scope": "project:payments"
  },
  "intent": "short free-text reason from the actor when required (approve, break-glass)",
  "context": {
    "requestId": "uuid",
    "sourceIP": "1.2.3.4",
    "userAgent": "keleustesctl/0.5.0",
    "auditTicket": "INC-12345"
  },
  "result": {
    "outcome": "success|denied|error",
    "reason": "...",
    "before": "optional resource snapshot",
    "after":  "optional resource snapshot"
  }
}
```

This shape is stable across all event types — different `action.verb` values, same envelope. Producers serialize once; consumers (UI, exporters, BI side-cars) deserialize once.

### 6.3 Where audit lives

- **JetStream durable stream `keleustes.audit`** is the source of truth (see runtime plan §11).
- **NATS KV `audit-index`** holds the recent (e.g., last 7 days) per-resource index for "what happened to this Promotion?" queries.
- **Object storage** archive of older audit segments (provider-replicated, no backups).
- **DuckDB parquet shards** rebuilt periodically from the audit stream for UI matrix views ("show me everything done in project X this month").

This composition gives sub-second "recent events" queries from NATS KV, multi-hour replayable history from JetStream, and fast analytical queries from DuckDB — all without an RDBMS.

### 6.4 Audit guarantees

- **Write-then-act.** For state-changing actions, the audit event is published **before** the action takes effect (or with the same event-log commit as the CRD change, where atomicity is feasible). This prevents "the system did something but didn't tell anyone" failure modes.
- **Tamper-evident.** Audit events are append-only in JetStream; the only way to "edit" history is to publish a correcting event, which is itself audited.
- **Cryptographic linking (later).** MVP 4 candidate: hash-chain audit events so any gap is detectable. Not in scope before then.

### 6.5 Surfacing audit in the UI and CLI

- UI: per-resource audit tab (Promotion timeline, Application history, Project activity); cross-project audit search for users with `view-audit`.
- CLI: `keleustesctl audit query --resource application/foo --since 24h`, `keleustesctl audit who --did promote --since 7d`.
- API: gRPC `Audit.Query(...)` and `Audit.Watch(...)`.

## 7. Central UI: Querying State

The user's requirement is the ability to query state from a central UI. The model:

- **Live state** — from the API server, served from CRDs (apiserver cache) and NATS KV (cross-target snapshots). Latency: sub-second.
- **Recent history** — from JetStream replay over the last N hours. Latency: seconds.
- **Historical / analytics** — from DuckDB on parquet (audit + state snapshots over time). Latency: seconds for typical matrix queries.
- **Cross-region** — UI talks to its local hub; for multi-region deployments, regional UIs aggregate or a "primary" UI proxies (decision deferred — runtime plan §13).

The UI is a read surface plus a small set of action surfaces (`approve`, `cancel`, `pause`, `resume`, `promote`, `break-glass`). It is **not** an editor for desired state. Every action button does one of:
1. Issue a state-machine transition on a CRD (e.g., approve a Promotion).
2. Open a PR against the config repo (e.g., bump a release version → "Promote").
3. Invoke break-glass (with the safeguards in §3).

There is no "edit values inline and apply" button. There is no "override parameters" panel.

## 8. Render Source Support

User requirement: kustomize, helm, helmfile, raw manifests. See engine-boundaries plan §5.1 for the technology mapping. Restatement of the rule that connects rendering to this plan:

- **All inputs to rendering come from Git.** Values files, kustomization patches, helmfile state files, raw YAML — all in Git.
- **No runtime-supplied values that aren't in Git.** Even cluster-discovered values (current image tags, etc.) for promotion are derived from Source events and committed back before being applied.
- **Render output is content-addressed and cached** (object storage), so the rendered manifests for a given (Application, Release, target) tuple are stable and reproducible.

This is the rendering side of the Git invariant: if rendering depends on something not in Git, that's a bug.

## 9. Engine Implications (cross-cutting)

| Engine | What it must do |
|--------|-----------------|
| **API server** | Authenticate every request (OIDC / mTLS / SA token); resolve groups; evaluate RBAC; emit audit event on every state-changing call. |
| **Promotion Engine** | Enforce `ApprovalPolicy` (separation of duties, N-of-M). Refuse to advance if approvals don't meet policy. Emit audit on every gate evaluation. |
| **Git Mutation Engine** | Every commit/PR it opens carries the originating actor and audit-event-ID in commit metadata (trailer or PR body). Break-glass commits are clearly tagged. |
| **Sync Engine** | Refuses to apply manifests with a different field-manager prefix than expected (prevents sneaky mutations). Emits audit on every SyncRun phase change. |
| **Source Engine** | New revisions are not desired state — they propose desired state. Until a Promotion accepts them and commits to the config repo, they don't deploy. |
| **Policy Engine** | Audit policy evaluation results; never silently overrule a policy. |
| **Webhook Receivers** | Audit webhook receipt + provider validation result. |
| **Agent** | Emits audit events for every action it takes on a target (apply, prune, health check) tagged with which `SyncRun` and which actor authorized it. |

## 10. Phased Rollout

- **MVP 0**: `IdentityProvider` CRD scaffold; OIDC against a single IdP; default-deny RBAC with a single `admin` Role; audit envelope defined and emitted for CRD writes (even if just to logs).
- **MVP 1**: `Role` / `RoleBinding` / `Project` CRDs and reconcilers; verb set; CLI auth; audit stream wired into JetStream; UI read-only with audit tab.
- **MVP 2**: `ApprovalPolicy` CRD; separation of duties enforced in Promotion; break-glass workflow implemented end-to-end with Git PR + drift surfacing; multi-IdP support; UI action surfaces (approve, cancel, promote).
- **MVP 3**: Delegated project admin; time-bound bindings; cross-region UI; DuckDB-backed audit queries; full audit export reference consumer for SIEM.
- **MVP 4**: Hash-chained audit events; FIDO2 / step-up auth for break-glass; per-tenant IdP isolation.

## 11. Open Decisions & Future ADRs

1. **Casbin or in-house policy evaluator?** Casbin is well-trodden but encourages the policy-file pattern we are rejecting. Custom evaluator over CRDs is cleaner but is engineering. Probably custom over CRDs, kept narrow.
2. **OIDC group claim normalization.** How do we handle groups with `/` (Azure AD), nested groups, dotted names? Normalize to a canonical form on IdentityProvider config.
3. **Per-namespace vs. per-project RBAC isolation.** Project is our delegation boundary, but Kubernetes namespaces are also a thing. How they map (1:1, N:1, or independent) matters for `kubectl` co-existence.
4. **Service-account token vs. mTLS for CI**. OIDC workload identity (GitHub, GitLab) is becoming standard and avoids long-lived secrets. Default recommendation should be OIDC; allow mTLS for air-gapped CI.
5. **UI deep-linking to specific Promotions and audit events** — requires stable URLs that survive resource rename. Use ULIDs/UUIDs in addition to names.
6. **Helmfile support depth** (see engine-boundaries plan §5.1 and §7). The Git invariant still applies — Helmfile state files live in Git.
7. **Drift handling after break-glass.** If a break-glass-driven PR is rejected by the human reviewer, do we automatically revert the cluster change, leave it as a permanent `BreakGlassDrift` condition, or require an explicit "revert" action? Default proposal: require explicit revert.
8. **Audit retention.** How long do we keep audit events hot in JetStream (default proposal: 90 days), in object-storage archive (years), in DuckDB analytics (queryable indefinitely from archive)? Compliance regimes (SOC2, ISO 27001) will set the floor.
9. **External audit export.** Customers running SOC2/ISO/PCI will want to push audit events into a SIEM (Splunk, Sumo Logic, Datadog, Elastic). Provide a reference exporter that reads JetStream and pushes via vendor SDKs. **Out of core**; in `contrib/`.
10. **Action signing for break-glass.** Should break-glass invocations require a signed token (e.g., from a hardware key), not just an RBAC permission? Step up auth for the most dangerous action.

## 12. References

- PROPOSAL.md §9, §14, §16, §17
- distributed-runtime-architecture.md (especially §11 storage tiers, §7.4 agent transport)
- engine-boundaries-and-technology-integration.md (especially §5.1 rendering, §5.5 storage)
- Argo CD RBAC documentation (external; what we are deliberately improving on)
- CLAUDE.md engineering guardrails (Git-as-source-of-truth)

---

**Next steps after review of this plan**

- ~~Confirm the Git-source-of-truth invariant as a non-negotiable design constraint (suitable for an early ADR).~~ — landed as [ADR 0003](../adr/0003-git-source-of-truth-invariant.md); §11 questions 6 and 7 are resolved there.
- Decide on the policy evaluator question (§11.1) — Casbin vs. custom.
- Sketch `IdentityProvider`, `Role`, `RoleBinding`, `Project`, `ApprovalPolicy` CRD shapes for MVP 1 scaffolding.
- Define the audit event Protobuf / JSON schema and version it (it will be load-bearing for years).
- Begin a tactical plan for the MVP 1 audit pipeline (CRD write → JetStream publish → NATS KV index → UI tab).

This document will be updated as the RBAC model and audit pipeline mature. Significant decisions will be captured in `docs/adr/`.
