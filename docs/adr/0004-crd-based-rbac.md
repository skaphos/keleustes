<!--
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
-->

# ADR 0004 — CRD-based RBAC model

- **Status:** Accepted
- **Date:** 2026-05-17
- **Deciders:** Platform Architecture (Skaphos)
- **Linear:** SKA-409
- **Related:** ADR 0001 (Plugin extension model), ADR 0003 (Git invariant)
- **Supersedes:** RBAC portions of `docs/plans/2026-05-rbac-audit-and-git-invariant.md` §4–§5 and §11 questions 1–5

## Context

Argo CD's RBAC model — a single `policy.csv` in a ConfigMap, evaluated by
Casbin — hits a wall at the scale Keleustes is built for. A few specific
failure modes:

- Editing policy is centralized; project admins can't manage RoleBindings
  inside their own project.
- Resource granularity ("can sync this Application to prod-us but not
  prod-eu") is awkward to express.
- Separation of duties (requester ≠ approver) requires external
  scaffolding.
- Emergency elevation requires editing the policy file — there is no
  time-bound break-glass.

`docs/plans/2026-05-rbac-audit-and-git-invariant.md` proposes a CRD-based
RBAC model that lives in Git like every other piece of configuration. This
ADR promotes that proposal into a binding decision and resolves the §11
questions that bear on RBAC (1–5).

## Decision

### 1. Five RBAC CRDs in `keleustes.skaphos.io/v1alpha1`

```
keleustes.skaphos.io/v1alpha1
├── IdentityProvider   — OIDC config; group-claim normalization
├── Role               — named set of verbs scoped to resource kinds
├── RoleBinding        — binds subjects (group/user/SA) to a Role within a scope
├── Project            — tenancy boundary; groups Applications, Sources, Environments,
│                        DeploymentTargets, and a default ApprovalPolicy
└── ApprovalPolicy     — N-of-M, separation-of-duties, break-glass elevation rules
```

`Role` and `RoleBinding` are namespace-scoped by default. Cluster-wide
equivalents — `ClusterRole`, `ClusterRoleBinding` — exist for the operator
itself and for explicitly cluster-scoped resources (`Cell`, cluster
`FreezeWindow`); they are not the default path. Application teams work
exclusively in namespace-scoped objects within their Project.

These CRDs are edited in Git and reconciled like every other Keleustes
resource. **Argo CD's `policy.csv` model is explicitly rejected.**

### 2. `Project` is the delegation boundary

A `Project` carries:

- The set of Applications, Sources, Environments it owns (by name or
  selector).
- The `DeploymentTarget`s it is allowed to deploy to.
- A list of project-admin groups — these groups can create / edit / delete
  `RoleBinding`s within the project scope without admin help.
- A default `ApprovalPolicy` reference.

A Project is the unit a platform team hands to an application team: "here
is your project, here are the targets you can reach, manage your own
RoleBindings inside it."

Cross-project dependencies (Application A in Project P1 depends on
Application B in Project P2) require an explicit grant and are audit-
flagged (per engine-boundaries plan §2.6).

### 3. Layered with native Kubernetes RBAC, not replaced

Keleustes RBAC governs **actions on Keleustes CRDs and the orchestration
workflows around them** (promote, approve, sync, break-glass). Native
Kubernetes RBAC continues to govern **what the operator's own
ServiceAccounts can do inside the cluster**.

- The operator's `Role` / `ClusterRole` markers in `config/rbac/` (via
  `+kubebuilder:rbac`) are unchanged by this ADR.
- A Keleustes `Role` like `promote-payments-prod` does not appear in any
  k8s `ClusterRole`; it is interpreted by the Keleustes API server.
- Customers can still use native k8s RBAC to gate `kubectl` access to
  Keleustes CRDs; the two layers are independent and additive.

### 4. Action verbs

The verb set is defined per resource. The initial alphabet:

| Resource                              | Verbs                                                              |
|---------------------------------------|--------------------------------------------------------------------|
| `Application`                         | `view`, `create`, `edit`, `delete`, `sync`, `pause`, `resume`     |
| `Source`                              | `view`, `create`, `edit`, `delete`, `force-refresh`               |
| `Release`                             | `view`, `create`, `edit`, `delete`                                 |
| `Promotion`                           | `view`, `create`, `approve`, `cancel`                              |
| `PromotionPolicy`                     | `view`, `edit`                                                     |
| `Approval`                            | `view`, `grant`                                                    |
| `FreezeWindow`                        | `view`, `create`, `edit`, `delete`, `override`                     |
| `DeploymentTarget`                    | `view`, `register-agent`, `edit`, `break-glass`                    |
| `Environment` / `Cell`                | `view`, `edit`                                                     |
| `RoleBinding` (within project)        | `view`, `create`, `edit`, `delete`                                 |
| cross-cutting                         | `break-glass`, `view-audit`, `query-state`                         |

New verbs require a CRD change + an ADR amendment. The verb set is
deliberately bounded so the audit shape (per the rbac-audit plan §6.2)
stays interpretable.

Per-verb scope can be: `cluster`, `project: <name>`, `application: <name>`,
`selector: <label-selector>`, or `environment: <name>` (plus sub-scopes).
`project` is the recommended default.

### 5. Identity sources via `IdentityProvider` CRD

| Identity type        | Mechanism                                                                          | Used for                                                              |
|----------------------|-------------------------------------------------------------------------------------|------------------------------------------------------------------------|
| Human users          | OIDC (any provider — Okta, Entra ID, Google Workspace, GitLab, Keycloak, Dex)      | UI access, interactive `keleustesctl`, approval actions                |
| CI / automation      | **OIDC workload identity** by default (GitHub OIDC, GitLab OIDC, cloud workload ID) | API calls from CI/CD pipelines                                         |
|                      | **mTLS client certs** as the air-gapped fallback                                    | Air-gapped CI where OIDC isn't available                               |
| Agents               | NKey + signed JWT (NATS leaf transport; see ADR 0005)                              | Agent → hub                                                            |
| In-cluster controllers | Kubernetes ServiceAccount tokens                                                  | Operator components talking to the apiserver                           |

There are **no Keleustes-issued passwords** and no local user database.
Identity is always federated.

`IdentityProvider` CRDs configure each source. A deployment can have
multiple — common pattern is "humans via Okta, CI via GitHub OIDC." Audit
records carry which `IdentityProvider` authenticated each actor.

This locks in plan §11 question 4: **OIDC workload identity is the default
recommendation for CI; mTLS is the air-gapped fallback.** Long-lived static
secrets are not part of the default path.

### 6. OIDC group-claim normalization

Different IdPs format group claims differently — Azure AD uses object IDs,
GitLab and Okta use slash-paths, Google groups are email-shaped. The
`IdentityProvider` CRD carries a normalization rule that maps incoming
claim values to canonical Keleustes group names:

```yaml
spec:
  groupClaim: groups
  normalize:
    case: lower
    trim: ["roles/", "/"]
    map:
      "aad-1234abcd-platform-eng": "platform-engineering"
```

Normalization happens **once, at authentication time**, and the canonical
form is what `RoleBinding.subjects` references. This locks in plan §11
question 2: normalize on `IdentityProvider`, not at each binding.

### 7. Project ↔ Kubernetes-namespace mapping

Keleustes Projects and Kubernetes namespaces are independent. A Project
can group resources from one namespace (the common case), from multiple
namespaces, or, with explicit configuration, none at all.

The default Project shape, however, is **1:1 with a Kubernetes namespace**:
each Project carries a `defaultNamespace` field, and namespaced Keleustes
CRDs created under that Project land there. Customers who need N:1 or
non-Kubernetes-native groupings opt in by leaving the field empty and
spelling out the namespace per CR.

This locks in plan §11 question 3: **Project is the delegation boundary,
not the Kubernetes namespace, but the default mapping is 1:1** so the two
models coexist cleanly with `kubectl` workflows.

### 8. Separation of duties via `ApprovalPolicy`

`ApprovalPolicy` (per Project, per Promotion gate, or per
`PromotionPolicy`) expresses:

- `requireDistinctActors` — requester and approver must be different
  identities.
- `minApprovers: N` — N-of-M approval.
- `requiredGroups` — at least one approver must be in group X.
- `excludeGroups` — members of group X cannot approve (e.g., the
  requester's direct reports).

The Promotion state machine enforces these. There is no place in code or
CLI that lets you skip them.

### 9. Time-bound RoleBindings and break-glass

`RoleBinding` carries optional fields:

- `validUntil: <RFC3339 timestamp>` — auto-expire without manual
  revocation.
- `reason: <string>` — required for short-lived bindings (≤ 24h).
- `auditTicket: <string>` — incident ticket; recorded on every audit event
  generated under this binding.

Common break-glass pattern: an incident commander self-grants
`break-glass` on a project for 2h via CLI; the grant is itself an audit
event; expiry is automatic; each use of `break-glass` produces its own
audit events tagged with the binding's `auditTicket`.

The break-glass workflow itself is defined by ADR 0003 §4 — this ADR
defines the **permission shape** that gates it.

**Action signing for break-glass** (FIDO2 / hardware-key step-up, plan
§11.10) is deferred — `RoleBinding` time-bounding is sufficient gating for
MVP 0–2. Step-up auth lands in MVP 4 alongside the rest of the
high-assurance work; the `RoleBinding` schema reserves a `requiresStepUp`
field for forward compatibility.

### 10. Default-deny

Subjects with no matching `RoleBinding` get **no access**, not read-only.
Read-only is a verb (`view`) that must be granted explicitly.

This is stricter than Argo CD's default. The cost is one extra `view`
binding per audience; the benefit is that "I forgot to lock that down" is
not a failure mode.

### 11. Policy evaluator: custom-over-CRDs, not Casbin

Casbin is well-trodden but encourages exactly the `policy.csv` pattern
this ADR rejects. The evaluator is a small, custom in-house implementation
that reads `Role` + `RoleBinding` + `Project` + `ApprovalPolicy` CRDs and
answers `(subject, verb, resource, scope) → allow|deny`.

This locks in plan §11 question 1. The implementation is deliberately
kept narrow:

- Decisions are pure functions of CRD state — no embedded language, no
  custom DSL.
- The evaluator runs in-process in the API server; results are cached
  per-request.
- Every decision can be re-executed offline for audit forensics by
  replaying the relevant CRDs as of a given time (which JetStream supports
  per ADR 0005).

If a customer wants Casbin or OPA semantics, they can implement a
`PolicyGate` plugin (ADR 0001) for promotions; that is layered on top of
RBAC, not a replacement for it.

### 12. UI deep-linking with stable identifiers

Every load-bearing resource (Promotion, Approval, audit event) carries a
**ULID** in addition to its Kubernetes `name`. ULIDs survive rename,
namespace move, and re-creation; names do not.

UI URLs prefer the ULID form for deep links to specific Promotions and
audit events. Names remain the human-facing label. This locks in plan §11
question 5.

### 13. Multi-tenant alignment with ADR 0001

ADR 0001 §11 commits to **single-tenant-by-default in v1alpha1**, with
hard cross-tenant enforcement landing in MVP 3. RBAC in this ADR follows
the same line:

- Project-scoped RoleBindings are honored by the API server in v1alpha1
  via standard authz checks.
- Cross-project dependency grants (§2) are validated at admit time but
  *enforced* with full server-side rejection only when ADR 0001's
  multi-tenant enforcement lands.
- The audit envelope carries `actor.identityProvider` and `actor.groups`
  from day one, so the MVP 3 enforcement layer has the inputs it needs
  without breaking earlier installations.

## Consequences

**Positive**

- RBAC scales past the single-policy-file ceiling. Project admins manage
  their own bindings.
- Fine-grained, action-verb-level permissions — `sync`, `promote`,
  `approve`, `break-glass` are distinct.
- Separation of duties and N-of-M approval are first-class, enforced in
  the Promotion state machine.
- Time-bound bindings give break-glass a usable shape without leaving
  permissions lying around.
- All RBAC lives in Git, satisfying ADR 0003.
- Group-claim normalization at `IdentityProvider` keeps the binding
  vocabulary clean across multi-IdP deployments.

**Negative / accepted costs**

- Five new CRDs to learn. The shapes are deliberately small.
- Default-deny is a migration cost for Argo-CD-coming installations that
  relied on permissive defaults. Documented in the migration guide.
- Custom evaluator is engineering we have to maintain. Bounded by keeping
  decisions to pure functions of CRD state.
- Cross-project dependency grants are *advisory* in v1alpha1 — the API
  server logs the boundary crossing but does not block. Hardened in MVP
  3 alongside the rest of multi-tenant.

## Alternatives considered

- **Casbin with policy stored in a ConfigMap.** Rejected: it's the Argo CD
  pattern this ADR is reacting to. Centralized editing, no delegation, no
  time-bounding.
- **Casbin reading from CRDs.** Hybrid that keeps Casbin's policy DSL while
  living in Git. Rejected: the DSL is the part that's hard to audit, and
  the evaluator we'd actually want — pure functions of CRD state — is
  smaller than the Casbin integration.
- **Lean entirely on native Kubernetes RBAC.** Rejected: Kubernetes RBAC
  has no notion of `promote`, `approve`, or `break-glass`; bolting those
  on as Custom Verb names is non-portable and doesn't compose with
  `kubectl auth can-i`.
- **`ClusterRoleBinding` as the default.** Rejected: pushes every team's
  bindings cluster-wide, making delegation impossible. Namespace-scoped
  default; cluster-scoped is the exception.
- **Per-namespace RBAC (no Project layer).** Considered. Rejected because
  a Project naturally groups several namespaces' worth of resources
  (Application + Source + Environment + DeploymentTarget can span
  namespaces) and a single delegation boundary is needed to express
  "this team owns all of these things."

## Compliance and follow-ups

- Plan §11 questions 1 (Casbin vs custom), 2 (group normalization), 3
  (namespace mapping), 4 (CI auth), 5 (UI deep-linking) are resolved
  here.
- Plan §11 questions 8 (audit retention), 9 (external SIEM), 10 (step-up
  auth) overlap with ADR 0005 (distributed runtime, JetStream retention)
  and with ADR 0001 (audit destinations via the plugin model). They are
  not re-decided here.
- New tickets to scaffold each of the five RBAC CRDs in MVP 0 (description
  only — reconcilers come in MVP 1 per the rbac plan §10).
- The custom evaluator lands in MVP 1 alongside the first real `Role` /
  `RoleBinding` reconciler.
- This ADR will be revisited if a real-world customer needs an RBAC
  policy that pure-functions-of-CRDs cannot express, or if multi-tenant
  enforcement (MVP 3) demands schema changes.
