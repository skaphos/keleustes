<!--
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
-->

# Operator CRD Integration

- **Status:** Draft — 2026-05-18
- **Linear:** SKA-431 (this plan). Refines SKA-339 (vendor + integrate gitops-engine), SKA-340 (Render Engine), SKA-341 (Sync Engine wrapping gitops-engine). Spawns one or more MVP 1/2 implementation tickets (see §13).
- **Promotes into:** a future ADR co-located with ADR 0001 (plugin model) and ADR 0006 (engine boundaries). Until then, this document is authoritative for any code that touches health assessment, diff normalization, or operator-CRD lifecycle handling.
- **Related:** ADR 0001 (Plugin extension model — webhook-first, declarative CRDs), ADR 0003 (Git invariant), ADR 0006 (Engine boundaries — containment rule), ADR 0007 (Hard-fork gitops-engine; friendly-fork amendment), `docs/plans/2026-05-extensibility-plugin-surfaces.md` (the original plugin-surfaces sketch).
- **Out of scope:** value-change Promotion (`Promotion.spec.changes[]` — extending Promotion to route structured value diffs through the promotion machinery). Tracked separately as `docs/plans/2026-05-value-change-promotion.md` (TBD).

## 1. Purpose and Scope

Keleustes is going to ship CRs of every operator's CRDs that a Skaphos customer runs. cert-manager `Certificate`s, Argo `Rollout`s, Crossplane `Composition`s, Cluster API `Cluster`s, Tekton `PipelineRun`s, Knative `Service`s, customer-grown CRDs no Skaphos engineer has seen — all of them.

For each kind, Keleustes needs to know how to:

1. **Apply and prune** it (rendered output → cluster state).
2. **Diff** it (target vs. live, ignoring fields the operator legitimately mutates).
3. **Assess health** of it (is the CR actually working, not just "applied successfully").

(1) is mostly free from the gitops-engine integration; (2) and (3) are not. This plan covers how Keleustes makes (2) and (3) work for kinds Skaphos has never seen — without becoming the bottleneck that requires a Skaphos PR every time a customer adopts a new operator.

**In scope:**

- CRD discovery (automatic via cluster cache; documented so contributors don't reinvent).
- The `HealthAssessor` CRD surface — CEL expression mode + webhook escape hatch, scoped at cluster / project / application.
- The `DiffNormalizer` CRD surface — same shape, applied to drift detection.
- Skaphos-curated registry of built-in assessors and normalizers for popular operators.
- CRD-owner-shipped rules (operators publish their own assessors alongside their install bundle).
- Customer-shipped rules (override or extend for custom CRDs).
- Precedence and conflict resolution.
- Cross-Application dependency ordering for the operator-install-before-CR-use case.
- Audit envelope verbs and observability story.
- Failure modes.
- Phased rollout.

**Out of scope:**

- Custom-resource **action** invocation (e.g., "restart this Argo Rollout" from the UI). Per ADR 0003 (Git invariant) all desired-state mutation goes through Git; live-cluster actions are break-glass territory. Tracked separately.
- Render-engine extensibility (Helm hooks, Helmfile bases, custom-resource transformers). Render is its own plan (SKA-320).
- Multi-cluster CRD federation (one CRD installed cluster-A, CR consumed in cluster-B). Future.

## 2. Why this is load-bearing for everything else

Argo CD's experience is the warning. The default sync runs apply manifests, prune what's owned, and call it done — but every customer who runs cert-manager, Argo Rollouts, Strimzi, or any Crossplane provider hits the same problem on day three: "the Application says Synced but the system isn't actually working." The fix in Argo CD is `resource.customizations.health.<group>_<kind>` in `argocd-cm` — Lua snippets that customers paste in and hope they got right. Lua, embedded in a ConfigMap, no validation, no audit, no GitOps story for the snippets themselves.

Skaphos can't repeat that pattern. The constraints we already committed to make a different shape mandatory:

- **ADR 0001's plugin model.** Extension is via CRDs validated by the API server, not via ConfigMap snippets. The same shape that governs `Notifier`, `Scanner`, `SignatureVerifier`, `PolicyGate`, and `AuditDestination` governs Health and Diff customization too.
- **ADR 0003's Git invariant.** Customer-shipped rules are Kubernetes objects, which means they're in Git, which means they're auditable and reviewable like everything else.
- **No embedded interpreter in the operator binary.** CEL is already in the Keleustes binary (the `Notifier` XValidation rule uses it). Adding Lua, Starlark, or WASM is a new runtime surface we don't need.

Getting this right at MVP 1 means MVP 2's value-change Promotion doesn't have to retroactively design extensibility into a system that didn't have it.

## 3. CRD Discovery (Automatic)

Keleustes inherits CRD discovery from the gitops-engine cluster cache for free. `pkg/cache.ClusterCache.OnEventHandler` fires when any object (including CRDs themselves) is created, updated, or deleted in the watched cluster. The cache rebuilds its `RESTMapper` whenever a CRD changes; the new kind is immediately syncable.

**Concretely**:

- A customer installs cert-manager into a cluster (via a Keleustes Application that ships the cert-manager manifests).
- gitops-engine's cluster cache fires `EventAdded` for the `Certificate` CRD.
- The cache rebuilds its `RESTMapper`.
- The next reconcile of any Application that ships `Certificate` CRs finds the kind registered and applies them normally.
- Keleustes never needed to be told about cert-manager.

**What this means for Keleustes code**: the Sync Engine doesn't carry a hardcoded kind list. The Render Engine outputs `[]*unstructured.Unstructured` (kind-agnostic). The Diff Engine and Health Engine operate on Unstructured. The only places Keleustes makes kind-specific decisions are inside the assessor/normalizer registries described below — and those are themselves data, not code.

**What this means for the containment rule**: `pkg/cache.ClusterCache` and its event handlers are wrapped in `internal/sync/clustercache/` (or `internal/kube/`) per ADR 0006 §4. The cache's `OnEventHandler` callbacks feed into Keleustes types — the rest of the codebase never sees gitops-engine types crossing a package boundary.

## 4. The `HealthAssessor` Surface

### 4.1 CRD shape

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: HealthAssessor
metadata:
  # Name convention: <group>-<kind>-<variant>. Variant lets multiple
  # assessors target the same kind at different scopes (see §4.6).
  name: cert-manager-io-certificate-default
  # Cluster-scoped — see §4.6.
spec:
  # Which Kubernetes resource kind this assesses. Required.
  target:
    apiGroup: cert-manager.io
    # Empty version means "any version of this group/kind." Specific
    # versions allowed for kinds that need version-specific logic.
    version: ""
    kind: Certificate

  # Exactly one of expression or webhook must be set. CEL XValidation
  # rule enforces the XOR (same pattern as Notifier.spec.endpoint).
  expression:
    # CEL expression. `self` is the resource; `self.status` is the
    # most common entrypoint. Must return a HealthVerdict object
    # (see §4.2 for the shape and the helper-library predicates).
    cel: |
      let ready = self.status.conditions.exists(c,
        c.type == 'Ready' && c.status == 'True');
      let issuing = self.status.conditions.exists(c,
        c.type == 'Issuing' && c.status == 'True');
      ready ? Healthy("ready")
        : issuing ? Progressing("issuing")
        : Degraded("not ready")

  # Alternative: webhook delivery (escape hatch for assessment that
  # needs external lookups — querying a remote system about whether
  # the resource is actually serving traffic, for example).
  webhook:
    url: https://health.platform.internal/assess/certificate
    authSecretRef:
      name: health-webhook-token
      key: token
    timeout: 5s

  # Scope: where this assessor applies (see §4.6).
  scope:
    # Empty means cluster-wide. May be narrowed to Projects or
    # Applications.
    projects: []
    applications: []

  # Provenance: who shipped this assessor. Informational, but
  # populates audit envelopes (see §9).
  provenance:
    source: cert-manager-io                  # operator-shipped
    # alternative: source: skaphos-curated   # skaphos-shipped
    # alternative: source: customer          # customer-shipped
    version: "v1.18.0"                       # operator version this
                                             # assessor is known to
                                             # work against
status:
  # ObservedGeneration + Accepted condition follow the existing
  # reconciler-scaffold pattern.
  conditions:
    - type: Accepted
      status: "True"
      reason: ExpressionValid
      message: "CEL expression compiled; webhook target validates."
```

### 4.2 CEL expression mode (default)

CEL is the default mode for two reasons: it's already in the binary (no new runtime), and most operator health assessments are pure functions of `.status.conditions[]` — exactly what CEL was built for.

The expression returns a **HealthVerdict** — a discriminated union:

| Verdict | Meaning | Maps to Keleustes `HealthState` |
| --- | --- | --- |
| `Healthy(reason)` | Resource is working as intended | `Healthy` |
| `Progressing(reason)` | Resource is converging but not yet done | `Progressing` |
| `Degraded(reason)` | Resource is in a recoverable bad state | `Degraded` |
| `Unhealthy(reason)` | Resource is in a terminal bad state | `Unhealthy` |
| `Suspended(reason)` | Resource is paused intentionally (e.g., Argo Rollout paused) | `Suspended` |
| `Unknown(reason)` | Assessor can't determine — typically a missing status | `Unknown` |

The Keleustes-side CEL environment exposes:

- `self` — the resource as a CEL map.
- `Healthy(string)`, `Progressing(string)`, `Degraded(string)`, `Unhealthy(string)`, `Suspended(string)`, `Unknown(string)` — verdict constructors.
- `now()` — current time, for age-based assessments (e.g., "still Progressing after 10 minutes → Degraded").
- `parseDuration(string)`, `parseTime(string)` — helpers for `.status.conditions[].lastTransitionTime`.
- Standard CEL macros (`.exists`, `.all`, `.filter`, `.map`).

The environment is the same one gitops-engine v0.36.1+ uses internally for any CEL-driven assessment; the Keleustes wrapper adds the `provenance` and `scope` machinery without changing the CEL expression semantics.

**Compilation happens at admission time** via a kubebuilder `XValidation` rule plus a validating webhook that compiles the expression against the resource's `target.apiGroup/version/kind` shape. A CEL expression that doesn't compile is rejected by the API server; the operator never sees malformed assessors at runtime.

### 4.3 Webhook escape hatch

Some assessments can't be done with a pure function of the resource itself:

- Querying a remote system about whether the resource is actually serving traffic.
- Cross-resource health (e.g., "this `VirtualService` is healthy only if at least one of its backend `DestinationRule`s is healthy").
- Latency-sensitive assessment that needs to cache out-of-band data.

For these the assessor declares `spec.webhook`. Keleustes posts a request to the webhook with the resource and expects a HealthVerdict response. The envelope is the same shape ADR 0001 defined for the other plugin surfaces (`Notifier`, `Scanner`, etc.) — JSON over HTTPS with mTLS or shared-token auth.

**Failure semantics** (see §10) — webhook 5xx, timeout, or malformed response all map to `Unknown` plus an audit event. Health doesn't fail-closed on webhook outage; the customer's operator team would not thank us for blocking sync on their flaky health endpoint.

### 4.4 The Skaphos-curated registry

Skaphos ships a curated registry of `HealthAssessor`s for popular operators that don't ship their own. The registry lives in a separate Helm chart / kustomize bundle at `skaphos/keleustes-curated` (filed as a follow-up under SKA-431 closeout — separate repo from `skaphos/keleustes` itself, so customers can opt into it independently and so Skaphos can iterate the rules without operator releases).

Initial coverage:

| Operator | Kinds | Notes |
| --- | --- | --- |
| cert-manager | `Certificate`, `Issuer`, `ClusterIssuer`, `Order`, `CertificateRequest` | Reference the `Ready`/`Issuing` condition pattern |
| Argo Rollouts | `Rollout`, `AnalysisRun` | The `Phase` field is the primary signal; pause states map to `Suspended` |
| Argo CD itself | `Application`, `AppProject` | Yes, we assess Argo's CRs too — common in transitional deployments where Argo and Keleustes coexist |
| Crossplane | `Composition`, `CompositeResourceDefinition`, claim resources | `Synced` + `Ready` conditions per the Crossplane spec |
| Cluster API | `Cluster`, `Machine`, `MachineDeployment`, `KubeadmControlPlane` | `Phase` field plus `Ready` condition |
| Tekton | `PipelineRun`, `TaskRun` | `Conditions[Succeeded]` — Tekton uses Knative-style conditions |
| Knative Serving | `Service`, `Revision`, `Route`, `Configuration` | `Ready` condition |
| Istio | `VirtualService`, `DestinationRule`, `Gateway`, `AuthorizationPolicy` | Mostly `Reconciled` condition; some require cross-resource checks (webhook-based) |
| External Secrets | `ExternalSecret`, `ClusterSecretStore` | `Ready` + `Synced` |
| Prometheus Operator | `Prometheus`, `Alertmanager`, `ServiceMonitor`, `PodMonitor`, `PrometheusRule` | StatefulSet-like for the workload kinds; static `Healthy` for the rule kinds (CRD validation is the assessment) |

The bundle is versioned independently. Customers opt in by deploying it as a sub-Application of their Keleustes hub install. Skaphos iterates as operators evolve.

### 4.5 CRD-owner-shipped assessors

Operators publish their own `HealthAssessor` CR alongside their install bundle. cert-manager's `helm/cert-manager` chart, for example, adds a `HealthAssessor` CR for `Certificate` to its `templates/` — installed by default, customer-disablable via Helm values.

For an operator publisher this is a one-paragraph contributor doc: "If your operator's CRs are used by Skaphos consumers, ship a `HealthAssessor` per CRD. Here's the schema link, here's the CEL helper library, here's the test recipe." The `provenance.source` field marks these as operator-shipped (informational only; doesn't change runtime behavior).

This is the load-bearing distribution model. Without it Skaphos becomes the choke point for every new operator a customer adopts.

### 4.6 Scope: Cluster / Project / Application

A `HealthAssessor` declares its scope via `spec.scope`:

- **Cluster scope** (default — `scope.projects` and `scope.applications` both empty): applies to every Application that ships the target kind.
- **Project scope** (`scope.projects: [payments, identity]`): applies only to Applications belonging to those Projects (per ADR 0004's `Project` boundary).
- **Application scope** (`scope.applications: [checkout-api, billing-api]`): applies only to those Applications.

Project and Application scopes are how customers override built-in or operator-shipped assessors for specific contexts. The classic case: "for the `payments` project, our cert-manager `Certificate` is only healthy if the issuance webhook also reports it valid" — a project-scoped assessor with `spec.webhook` calling the customer's signing infrastructure.

The `HealthAssessor` itself is cluster-scoped as a Kubernetes resource (it's metadata about how to assess, not a per-application data record). Lifecycle is decoupled from the Applications it covers.

## 5. The `DiffNormalizer` Surface

### 5.1 Why diff needs custom rules

Many operators mutate the objects they own — controllers patch in fields, defaulters add annotations, mutating admission webhooks rewrite spec values. The naive diff "target manifest vs. live cluster object" shows drift on every reconcile for any of:

- `metadata.annotations.kubectl.kubernetes.io/last-applied-configuration` (kubectl shim, deprecated but still everywhere).
- `metadata.managedFields[]` (server-side-apply tracking — Keleustes itself writes these).
- `spec.clusterIP` on `Service` (assigned by the API server, never in the manifest).
- `spec.template.metadata.annotations` rewrites by service-mesh injectors.
- Custom-resource `status` subresources (controllers write status; users don't).
- Operator-side defaulting (cert-manager fills in `spec.duration: 2160h` if unset; Argo `Rollout` adds `spec.strategy.canary.steps[].pause` defaults).

Without normalization, every drift detector reports false positives forever. With ad-hoc per-kind hacks in Keleustes code, the maintenance cost compounds with every new operator.

gitops-engine ships a built-in `pkg/diff.Diff` that handles the kubectl/last-applied-configuration noise and a few other standard cases. Anything beyond that (per-operator field rewrites) needs Keleustes-side extension.

### 5.2 CRD shape

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: DiffNormalizer
metadata:
  name: cert-manager-io-certificate-default
spec:
  # Same shape as HealthAssessor.spec.target.
  target:
    apiGroup: cert-manager.io
    version: ""
    kind: Certificate

  # Exactly one of expression or webhook (XOR enforced via CEL XValidation).
  expression:
    # The CEL expression returns a list of JSON Pointer paths to
    # ignore (the gitops-engine diff library accepts these as
    # "fields to mask before diffing").
    ignorePaths: |
      [
        "/metadata/annotations/cert-manager.io~1issuer-name",
        "/spec/duration",   # operator-defaulted if user omitted
        "/status",          # entire status subresource; never user-managed
      ]
    # OR a more structured ignore-rules form for cross-cutting cases:
    ignoreRules:
      - jsonPointer: "/spec/template/metadata/annotations"
        # Only ignore keys matching this prefix — the rest are real
        # user intent.
        keyPrefix: "linkerd.io/"

  webhook:
    url: https://normalize.platform.internal/diff/certificate
    authSecretRef: { name: diff-normalizer-token, key: token }
    timeout: 3s

  # Same scope model as HealthAssessor.
  scope:
    projects: []
    applications: []

  provenance:
    source: cert-manager-io
    version: "v1.18.0"
status:
  # Same condition shape.
```

### 5.3 What the normalizer does

The diff path becomes:

1. Render produces `target` (desired Unstructured).
2. Cluster cache produces `live` (current Unstructured).
3. Diff engine consults the `DiffNormalizer` registry for the target's kind. Most-specific scope wins (§7).
4. The matched normalizer's `ignorePaths` / `ignoreRules` mask both `target` and `live` (so the same fields are removed from both before comparison).
5. `pkg/diff.Diff` runs on the masked pair.
6. If the result is empty, no drift. Otherwise drift is real.

The Skaphos-curated registry ships normalizers for the same operator list as the HealthAssessor registry. CRD owners ship their own. Customers override.

### 5.4 The interaction with server-side apply

Keleustes uses server-side apply with field manager `keleustes` (per the gitops-engine integration design — `WithServerSideApplyManager("keleustes")`). SSA gives us free conflict detection on fields Keleustes owns vs. fields the operator owns: if the operator patched a field Keleustes also tries to set, the SSA conflict is the signal.

For most kinds this means **fewer** custom diff rules are needed than under client-side apply. The `managedFields[]` machinery is the source of truth for "who owns what." The `DiffNormalizer` surface still exists for cases SSA doesn't cover — primarily user-visible defaulting (the cert-manager `spec.duration: 2160h` case) and operator-side computed fields (CIDR assignments on `Service`).

## 6. Cross-Application Dependency Ordering

The "install the operator before using its CRs" case is engine-boundaries plan §2.6 — already in the design. Repeating the contract here because it's important for the operator-CRD story:

- `Application.spec.dependsOn[]` is a list of references to other Applications or to specific CRD-established sentinels.
- The Sync Engine refuses to reconcile an Application until its dependencies are `Healthy` (per HealthCheck aggregation — which uses the assessors defined above).
- A dependency on `crd:certificates.cert-manager.io` resolves to "wait until that CRD is established on the target cluster," not "wait for a specific Application."

This is what makes the operator-install pattern work: ship one Application that installs cert-manager (CRDs + controller Deployment), declare `dependsOn` from every Application that uses `Certificate` CRs. Keleustes doesn't deploy the consumers until cert-manager is genuinely ready — and "ready" is defined by the cert-manager-shipped `HealthAssessor`, not by Skaphos hardcoding.

The dependency-establishment events feed back into the same audit envelope and Notifier delivery as other promotion-machinery events; nothing new here from the audit side.

## 7. Distribution, Loading, and Precedence

### 7.1 Where assessors and normalizers come from

Three sources, in increasing precedence (most-specific wins):

1. **Skaphos curated bundle** (`skaphos/keleustes-curated` repo, separate Helm chart). Customers opt in by deploying it as a sub-Application of their hub install. Default scope is cluster-wide.

2. **CRD-owner-shipped** alongside the operator install. Same Application that installs cert-manager also ships the `HealthAssessor`s for cert-manager's CRDs. Default scope is cluster-wide. Operator publishers maintain these.

3. **Customer-shipped** for their own custom CRDs, or to override an upstream assessor for a specific Project/Application. Scoped narrowly. Customer owns lifecycle.

In addition there's a hardcoded **built-in default** for any kind with no assessor: check `.status.conditions[?(@.type=='Ready' && @.status=='True')]` and return `Healthy` if found, `Progressing` if there's a `Ready=False`, `Unknown` otherwise. Works for ~80% of well-formed CRDs that follow Kubernetes API conventions.

### 7.2 Loading mechanism

A controller in `internal/health/` (and `internal/diff/`) watches `HealthAssessor` (resp. `DiffNormalizer`) CRs cluster-wide via controller-runtime informers. An in-memory `Registry` indexes by `(group, version, kind)` → `[]Assessor` (multiple assessors per kind because of overlapping scopes). The registry rebuilds on every CR change.

The lookup at health-assessment time:

```text
For a resource R of kind K in Application A in Project P:

  candidates = registry.find(K)
  candidates = candidates.filter(scope matches A or P)
  candidates = sort(candidates, by specificity DESC)
                where specificity =
                  Application-scope > Project-scope > cluster-scope

  if candidates is non-empty:
    return candidates[0].assess(R)
  else:
    return built-in default(R)
```

### 7.3 Conflict resolution

Within a single specificity tier (e.g., two cluster-scoped assessors both targeting `Certificate`), the assessor with `provenance.source == customer` wins over `cert-manager-io` (operator-shipped) wins over `skaphos-curated`. This lets customers always have the last word at any scope without needing to delete competing assessors.

The lookup also emits an audit event when two assessors of the same scope conflict — operators may be unintentionally shadowing each other; surfacing it lets the customer notice and clean up.

### 7.4 Validation at admission time

The validating webhook for `HealthAssessor` and `DiffNormalizer` does three things:

1. **CEL compile check.** The expression is compiled against the target's apiGroup/version/kind shape. Failures are rejected with the compiler error.
2. **Webhook reachability** (best-effort). If `spec.webhook.url` is set, the webhook performs a `HEAD` request to confirm DNS resolves; non-fatal warnings on connection failure (the webhook target may not be live yet at install time).
3. **Scope sanity.** `scope.projects` and `scope.applications` are checked to exist (warning, not rejection — the assessor may be installed before the Project/Application it references).

## 8. Audit and observability

### 8.1 Audit envelope verbs

New verbs added to the §13 registry of `docs/plans/2026-05-audit-event-schema.md`:

| Verb | Payload type | Intent required | Notes |
| --- | --- | --- | --- |
| `healthassessor-installed` | `healthassessor.installed.v1` | yes (customer); no (operator-shipped) | First-time install of a HealthAssessor |
| `healthassessor-evaluated` | `healthassessor.evaluated.v1` | no (system) | Optional — emitted only when verdict differs from prior; high-volume otherwise |
| `healthassessor-failed` | `healthassessor.failed.v1` | no (system) | CEL eval error, webhook 5xx, or panic — falls back to Unknown |
| `healthassessor-conflict` | `healthassessor.conflict.v1` | no (system) | Two assessors of equal specificity, same scope, different verdicts |
| `diffnormalizer-installed` | `diffnormalizer.installed.v1` | yes (customer); no (operator-shipped) | First-time install |
| `diffnormalizer-evaluated` | `diffnormalizer.evaluated.v1` | no | Off by default; opt-in for debug |
| `diffnormalizer-failed` | `diffnormalizer.failed.v1` | no | Same fallback as health |

The `evaluated` verbs are off by default because they fire on every reconcile — too much volume. Operators turn them on per-Application for debugging when drift detection is misbehaving.

### 8.2 Metrics

Per the observability-stack plan §3.1 label vocabulary, the health subsystem emits:

- `keleustes_healthassessor_evaluations_total{kind, source, result}` — counter, where `source` is provenance (`skaphos-curated`, `cert-manager-io`, `customer`, `built-in`) and `result` is the HealthVerdict.
- `keleustes_healthassessor_evaluation_seconds{kind, mode}` — histogram, where `mode` is `cel` or `webhook`. Histograms carry **only** bounded labels per the cardinality discipline.
- `keleustes_healthassessor_registry_size{kind}` — gauge of how many assessors are currently registered per kind (for catching unintended shadow installs).
- `keleustes_healthassessor_webhook_failures_total{kind, reason}` — counter; `reason` is closed enum (`timeout`, `5xx`, `tls`, `malformed-response`).

Mirror set for diff normalization.

## 9. Failure Modes

| Failure | Behavior |
| --- | --- |
| CEL expression has a compile error | Rejected at admission time — never reaches the registry |
| CEL expression panics at runtime (shouldn't happen if compile-time validation works, but defense in depth) | Verdict = Unknown; `healthassessor-failed` audit event; metric increment |
| Webhook DNS fails | Verdict = Unknown; backoff retry next reconcile; audit event |
| Webhook 5xx | Same — verdict = Unknown; audit event |
| Webhook returns malformed verdict JSON | Same |
| Webhook times out (`spec.webhook.timeout` exceeded) | Same |
| No assessor matches a kind | Built-in default rule (Ready-condition heuristic); no error, no audit event |
| Two assessors of equal specificity conflict on verdict | Customer-source > operator-source > skaphos-source; conflict audit event; metric increment |
| `HealthAssessor` references a non-existent Project | Admission-time warning, not rejection; the assessor is inert until the Project exists |

Diff normalization has the same fallback shape — failure means "no normalization," and the raw diff is what surfaces. Drift detection becomes noisier but doesn't break.

**The fail-open default is deliberate.** Health and diff are **operational** signals, not security gates. Per the rationale that distinguishes ADR 0001 plugin surfaces by failure mode (§3 of the extensibility plan): `SignatureVerifier` fails closed, `Notifier` fails open. Health and Diff are squarely in the fail-open camp — a broken Lua snippet in Argo CD breaks every Application that uses that resource type, which is exactly the operability disaster we're avoiding.

## 10. Operator Version Migration

When an operator ships a new version that changes its CRD schema or status semantics, the existing assessors may go stale. The lifecycle:

- Operator publishes `v1.19.0` with a new field in `.status`. The previously-correct CEL expression now either misses a state or returns wrong verdicts.
- CRD-owner-shipped assessor: the operator bundles a new `HealthAssessor` CR with the v1.19.0 install. The Helm/kustomize upgrade replaces the old assessor with the new one. Atomic with the operator upgrade.
- Skaphos-curated: the `skaphos/keleustes-curated` bundle ships an updated assessor when Skaphos validates the new operator version. Lag is the same as for any other Skaphos release.
- Customer override: the customer's assessor may need updating. The audit `healthassessor-failed` events surface this if the new CRD shape breaks the old CEL expression.

`spec.provenance.version` is informational metadata for traceability — it doesn't affect runtime selection. The CRD `target.version: ""` (any version) is the common case; specific versions are reserved for operators that ship breaking changes between minor versions and need version-specific assessors.

## 11. Compliance with Prior Decisions

| Decision | This plan honors it by |
| --- | --- |
| ADR 0001 (Plugin model — webhook-first, declarative CRDs) | `HealthAssessor` and `DiffNormalizer` follow the same CEL-or-webhook discriminator shape as `Notifier` |
| ADR 0003 (Git invariant) | Both CRDs live in Git like every other Keleustes resource; assessor changes are PR-reviewed, auditable, rollback-able |
| ADR 0006 (Engine boundaries) | `pkg/health` and `pkg/diff` from gitops-engine are wrapped in `internal/health/` and `internal/diff/`; the rest of the codebase consumes Keleustes-translated types (HealthVerdict, DiffSummary), never gitops-engine types |
| ADR 0007 (Friendly fork) | Assessor types and any CEL extensions land first on the Skaphos fork; valuable ones are PR'd back to argoproj |
| Audit-event-schema plan §13 (Verb registry) | New verbs registered per §8.1 above |
| Observability-stack plan §3.1 (Label vocabulary) | Histograms use only bounded labels; per-kind metrics use the canonical `kind` label |

## 12. Open questions

1. **CEL function-library extensibility.** Customers may want to register helper functions (e.g., `parseSemver`, `daysAgo`, `pingTCP`) into the CEL environment for use in expressions. Possible but adds a real extension surface (Keleustes binary loads customer-defined CEL functions at startup — how? Where do they live?). Defer to MVP 3 unless a real demand surfaces.

2. **Composite assessors.** A `Composite` HealthAssessor that AND/OR-combines verdicts from other assessors is appealing for cross-resource health ("an Argo Rollout is healthy only if its replicasets and analysisruns are healthy"). Schema doable; deferred to a future plan once we see real customer demand.

3. **Long-poll webhooks vs. always-synchronous.** Webhook assessors are synchronous today. For low-latency operators (gitops-engine reconcile budget is in milliseconds) a 5s webhook timeout could matter. Consider async / cached-result patterns at MVP 2+ when we know if it's actually a problem.

4. **DiffNormalizer interaction with server-side apply conflict detection.** SSA gives us field-ownership conflicts for free; the normalizer overlays additional ignore rules. There may be edge cases where the two disagree (the operator owns a field per managedFields, but the customer's normalizer also wants to ignore changes to it). Probably benign — but worth a concrete test case to confirm.

5. **Webhook envelope versioning.** ADR 0001's plugin envelope is at v1; HealthAssessor / DiffNormalizer webhooks will be the first new consumers of that envelope. Verify v1 is sufficient before locking in.

6. **Performance budget.** With N Applications × M resources per Application × P assessors per kind, the worst-case evaluation count per reconcile loop is bounded but not free. SKA-326 (scale benchmark harness) needs a "many-assessors" scenario for MVP 1's 1K-Application target. Concrete numbers will inform whether we need result caching keyed on `(resource.uid, resource.resourceVersion, assessor.name)`.

7. **`HealthAssessor` for the `Application` CRD itself.** Keleustes can ship an assessor for its own `Application` CR — meta but useful for "is this Application healthy" rollups. Symmetry suggests yes; just make sure the recursive case (Application's health depends on its child resources' health) doesn't bite.

## 13. Concrete Follow-ups

1. **SKA-### — Implement the `HealthAssessor` CRD scaffold** (MVP 1). New CRD in `api/v1alpha1/healthassessor_types.go`; reconciler in `internal/controller/healthassessor_controller.go`; admission webhook that compiles CEL expressions. Same shape as SKA-404 (Notifier scaffold).

2. **SKA-### — Implement the `DiffNormalizer` CRD scaffold** (MVP 1). Same pattern.

3. **SKA-### — In-process registry + lookup logic** (MVP 1, after SKA-339 land the gitops-engine integration). `internal/health/registry.go` + `internal/diff/registry.go`. Watches the CRs via controller-runtime informers, exposes a thread-safe lookup.

4. **SKA-### — Built-in default rule** (MVP 1). The Ready-condition heuristic for kinds without an assessor. Pure Go, no CR backing.

5. **SKA-### — Set up `skaphos/keleustes-curated` repository** (MVP 1). Separate repo for the curated assessor + normalizer bundle. Initial coverage: the 10 operator families listed in §4.4. Released as a Helm chart.

6. **SKA-### — Audit verb registration** (MVP 1, alongside SKA-432). Update the audit-event-schema plan §13 registry with the new verbs from §8.1 above.

7. **SKA-### — Observability metrics** (MVP 1). The metrics in §8.2 wired into the `internal/health/` and `internal/diff/` packages.

8. **SKA-### — Operator publisher contributor guide** (MVP 2). One-page doc explaining how operator publishers ship their own `HealthAssessor` and `DiffNormalizer` alongside their install bundles. Lives at `docs/contributors/operator-publishers.md`.

9. **SKA-### — Customer extensibility guide** (MVP 2). Mirror for customers writing rules for their own CRDs.

10. **`docs/plans/2026-05-value-change-promotion.md` is the companion plan** (separate Linear ticket, to be filed). Out of scope here.

11. **PROPOSAL.md cross-link.** PROPOSAL §12 (health model) currently describes "we'll do per-resource health" without naming the extensibility story. Add a `> See SKA-431` marker during the SKA-325-style refresh.

12. **DECISIONS.md row.** This plan added as an active interim contract once §12 open questions are resolved enough to lock the schemas. Likely after the SKA-### implementation tickets land their first reconcilers.

## 14. Phased Rollout

| MVP | Work in this plan's scope |
| --- | --- |
| **MVP 1** | `HealthAssessor` and `DiffNormalizer` CRDs land as scaffolds. In-process registry. Built-in default rule. CEL-mode assessors operational (no webhook mode yet). Skaphos-curated bundle covers cert-manager + Argo Rollouts + Prometheus Operator (the three operators MVP 1 customers are most likely to run). Audit verbs registered. Metrics wired. |
| **MVP 2** | Webhook mode for both surfaces. Operator publisher contributor guide. Customer extensibility guide. Curated bundle expands to cover the full operator list in §4.4. Conflict-detection audit events go live. |
| **MVP 3** | Composite assessors (if customer demand surfaces). CEL function-library extensibility (if customer demand surfaces). Per-Application result caching if the scale benchmark shows it's needed. |
| **MVP 4** | No specific work in this plan's scope. |

---

**When this plan stabilizes** (after the §12 open questions resolve and the first round of `HealthAssessor`s have shipped against real customer operators), §1–§11 promote into a new ADR — likely co-located with ADR 0001 or appended to ADR 0006 §4. The Skaphos-curated registry stays a living artifact; the schemas and the extensibility contract are the durable record.
