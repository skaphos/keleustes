<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Extensibility and Plugin Surfaces Plan

**Status:** Draft
**Date:** 2026-05
**Related:** PROPOSAL §8 (CRDs), §13 (Security gates), §14 (Promotion Policy), §15 (Audit), §17 (CLI); engine-boundaries-and-technology-integration.md; rbac-audit-and-git-invariant.md
**Owner:** Platform Architecture (Skaphos)
**Purpose:** Define the extension surfaces of Keleustes — notifications, signature verification, security scanning, policy gates, audit destinations — and the mechanism by which third-party implementations plug into them. The goal is for Keleustes to be **opinionated about contracts and unopinionated about vendors**: no operator team should have to fork the control plane to swap a scanner, route an alert, or add a custom policy gate.

---

## 1. Why This Matters Now

Several existing tickets imply specific vendor integrations:

- SKA-354 — cosign verification at Source resolution
- SKA-370 — vulnerability scanning via Trivy and Grype
- SKA-381 — OPA / Gatekeeper integration in Policy Engine
- SKA-388 — reference audit exporter (Splunk / Sumo / Datadog / Elastic / syslog)

If each lands as a hard-coded integration, the result is a control plane that **knows the names of vendors**. That is the wrong abstraction. The right abstraction is: Keleustes knows that *something must verify image signatures at Source resolution*; cosign is a default implementation; Notation, Sigstore policy engine, GPG, and customer-built verifiers are equivalent citizens.

Doing this work *before* the engines land is materially cheaper than retrofitting it. The engine-boundaries plan already names the Source, Policy, and Health engines as the natural homes for these surfaces; defining the plugin contract now lets each engine grow with the contract baked in.

This is also the framing that aligns with the Git-source-of-truth invariant (rbac-audit-and-git-invariant.md §3). Plugin **configuration** lives in Git as declarative CRDs; plugin **implementations** are independent processes that the operator calls. The operator never embeds vendor code paths.

## 2. What Is and Is Not a Plugin

| Surface | Plugin? | Why |
|---|---|---|
| **Notifications** | ✅ Yes | Every team routes events to different sinks (Slack, PagerDuty, MS Teams, ServiceNow, webhooks); no defensible default |
| **Signature verification** | ✅ Yes | cosign / notation / Sigstore policy / GPG / vendor PKI; multiple valid choices |
| **Security / vulnerability scanning** | ✅ Yes | Trivy / Grype / Snyk / Aqua / Xray; high vendor lock-in risk if hardcoded |
| **Policy gates** | ✅ Yes | Native gates (image signed, SBOM present) are built-in; OPA / Kyverno / Gatekeeper / custom are plugins behind the same `PolicyGate` interface |
| **Audit destination** | ✅ Yes | Already plugin-shaped via JetStream consumer + `contrib/audit-exporter` |
| **Render** (Kustomize / Helm / raw) | ❌ No, but extensible later | Render is on the security-critical hot path. First-class Kustomize + Helm + raw; Helmfile in MVP 2 (SKA-362). Plugin-rendering is a future RFC — risks are larger (manifest tampering, supply-chain) |
| **Git provider** | ❌ No, interface-first | GitHub / GitLab / Azure DevOps are first-class via a `GitProvider` interface (engine-boundaries §4). Customers can implement the interface in-tree; we do not expose an external plugin model here |
| **CRD-defined integrations** (anything else) | ⚠️ Case by case | Default to "no" unless there is a real vendor diversity reason |

The rule is: **if the team will plausibly want to swap the vendor without forking, it is a plugin surface.** If the choice is structural (e.g., which Git provider to support at all), it is an in-tree interface, not a plugin.

## 3. The Five Extension Surfaces

### 3.1 Notifier

**Purpose:** Route lifecycle events to external sinks.

**Event sources:** Source revision observed, Source verification failed, Application sync drift detected, SyncRun failed, Promotion proposed / approved / blocked / merged / rolled back, FreezeWindow activated / deactivated, Approval recorded, break-glass invoked, Policy gate failed, Health degraded.

**CRD sketch (`Notifier`):**

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: Notifier
metadata:
  name: platform-slack
spec:
  endpoint:
    webhook:
      url: https://hooks.slack.com/services/T.../B.../...
      authSecretRef:
        name: slack-webhook
        key: token
  events:
    include:
      - Promotion.Blocked
      - SyncRun.Failed
      - FreezeWindow.Activated
      - BreakGlass.Invoked
    exclude:
      - Source.RevisionObserved
  filters:
    environments: [prod, qa]
    applications: ["marshaller-*"]
  delivery:
    mode: async             # async | sync
    timeout: 5s
    retries: 3
    backoff: exponential
```

**Failure semantics:** Notifiers are **always async, fail-open**. A broken Slack webhook never blocks a Promotion. Failed deliveries are visible in the Notifier's status and emit their own `NotifierDeliveryFailed` audit event.

### 3.2 SignatureVerifier

**Purpose:** Verify image, chart, or artifact signatures before a Source revision is considered eligible.

**Trigger points:** Source Engine on every new observed revision; Policy Engine when the `imageSigned` gate is evaluated.

**CRD sketch (`SignatureVerifier`):**

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: SignatureVerifier
metadata:
  name: cosign-default
spec:
  scope:
    artifactTypes: [containerImage, ociArtifact, helmChart]
    sources:
      registries: ["ghcr.io/marshaller-*"]
  endpoint:
    builtin: cosign         # builtin: cosign | notation
    # OR
    # webhook:
    #   url: https://verifier.platform.internal/verify
    #   authSecretRef: ...
  policy:
    keyRef:
      secretName: cosign-public-keys
      keyName: platform.pub
    requireSBOM: true
  delivery:
    mode: sync              # signature verification is on the critical path
    timeout: 10s
    onError: fail           # fail | warn (warn is for soak-testing only)
```

**Failure semantics:** Sync mode, **fail-closed** by default (`onError: fail`). `onError: warn` is allowed for explicit soak periods and surfaces a degraded-trust condition rather than blocking. Built-in `cosign` and `notation` engines satisfy SKA-354; external verifiers plug in via webhook.

### 3.3 Scanner

**Purpose:** Produce vulnerability findings for an artifact or rendered manifest set.

**Trigger points:** Source Engine on revision; Policy Engine when the `vulnThreshold` gate is evaluated; Promotion Engine as gate evidence.

**CRD sketch (`Scanner`):**

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: Scanner
metadata:
  name: trivy-prod
spec:
  scope:
    artifactTypes: [containerImage]
    environments: [prod]
  endpoint:
    builtin: trivy          # builtin: trivy | grype
    # OR
    # webhook:
    #   url: https://scanner.platform.internal/scan
    #   authSecretRef: ...
    timeout: 90s
  thresholds:
    fail:
      critical: 0
      high: 5
    warn:
      medium: 50
  cache:
    ttl: 24h                # cache findings keyed by digest
  delivery:
    mode: async             # findings attach to Source as conditions
    onError: warn
```

**Failure semantics:** Async by default (scanners are expensive). Findings attach as `Source.status.conditions` and are consumed by Policy gates. Scanner *unavailability* fails open (with a `ScannerUnavailable` condition); scanner *findings above threshold* fail the gate. SKA-370 (Trivy/Grype) is the first built-in implementation behind this interface.

### 3.4 PolicyGate

**Purpose:** Evaluate a named gate during Promotion. Gates produce **evidence** (typed structured data) that attaches to the Promotion and feeds the audit stream.

**Built-in gates:** `imageSigned`, `sbomPresent`, `vulnThreshold`, `sourceHealthy`, `targetUnlocked`, `changeApproved`, `ownerApproved` (already enumerated in `PromotionPolicy.required`).

**Plugin gates:** Anything registered via a `PolicyGate` CRD. OPA / Kyverno / Gatekeeper are the first external implementations (SKA-381).

**CRD sketch (`PolicyGate`):**

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: PolicyGate
metadata:
  name: opa-prod-baseline
spec:
  gateId: opaProdBaseline   # referenced from PromotionPolicy.required
  scope:
    environments: [prod]
  endpoint:
    builtin: opa
    config:
      bundleUrl: https://policy.platform.internal/bundles/prod.tar.gz
      decisionPath: keleustes/prod/allow
    # OR
    # webhook:
    #   url: https://policy.platform.internal/evaluate
    #   authSecretRef: ...
  delivery:
    mode: sync
    timeout: 15s
    onError: fail
  evidence:
    captureInput: true       # include the gate input in the audit envelope
    captureDecision: true
```

**Failure semantics:** Sync, fail-closed by default. A timed-out gate is treated as failure with explicit `Reason: GateTimeout`. Evidence is captured in the Promotion's status and the audit stream regardless of outcome.

`PromotionPolicy.required` references gates by `gateId`. The Promotion Engine resolves each gateId to either a built-in implementation or a `PolicyGate` CRD. Unknown gateIds are a hard validation error on the `PromotionPolicy` resource.

### 3.5 AuditDestination

**Purpose:** Forward audit events from the central JetStream stream to external SIEM and analytics tools. Already plugin-shaped via the `contrib/audit-exporter` consumer model (SKA-388, rbac-audit-and-git-invariant.md §11).

This plan formalizes it as a first-class `AuditDestination` CRD owned by the audit engine:

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: AuditDestination
metadata:
  name: splunk-prod
spec:
  endpoint:
    builtin: splunkHEC      # splunkHEC | sumoLogic | datadog | elastic | syslog
    config:
      url: https://hec.splunk.platform.internal/services/collector
      tokenSecretRef:
        name: splunk-hec
        key: token
      index: keleustes-audit
  filters:
    eventTypes: ["*"]
  delivery:
    mode: async
    onError: retry
    deadLetter:
      jetStream: true       # parked on a DLQ subject for replay
```

**Failure semantics:** Async, with persistent retry + dead-letter via JetStream. AuditDestinations cannot fail-closed by design: audit emission must never block a sync or promotion.

## 4. Mechanism: How Plugins Run

Three options are on the table. The plan recommends **option B (declarative CRD → external endpoint, default HTTPS webhook)** as the baseline, with **option C (gRPC sidecar)** kept open for performance-critical cases.

### Option A — In-process Go plugins (Go `plugin` package, or build-time registration)

**Pros:** Lowest latency; zero network overhead.

**Cons:** Build coupling (plugin and operator must match Go version, library versions, build tags); no isolation (plugin panic crashes the operator); supply-chain risk (plugin binary runs with operator's full privileges); poor multi-tenancy. **Rejected.**

### Option B — Declarative CRD pointing at an HTTPS endpoint (RECOMMENDED)

**Pros:** GitOps-native (configuration lives in Git as CRDs); language-agnostic; failure-isolated; well-trodden pattern (Knative Eventing, Tekton tasks, Kubernetes admission webhooks); customer-built plugins are trivial to write; plugins can be packaged as ordinary Kubernetes Deployments + Services.

**Cons:** Latency floor of a network round-trip per call; harder for very chatty interactions.

**Mitigations:** Async-by-default for high-frequency events; in-cluster endpoints (`<svc>.<ns>.svc.cluster.local`) keep latency low; caching by content digest for verifier/scanner results.

### Option C — gRPC sidecar with a defined `.proto`

**Pros:** Strong typing; streaming for high-throughput cases (e.g., bulk scan); lower latency than HTTPS+JSON when colocated.

**Cons:** Operationally heavier (per-pod sidecar lifecycle); more rigid versioning; less familiar to most platform teams.

**Verdict:** Available as a fallback if specific surfaces (likely Scanner with high-volume image streams) need it. Not the default.

### Option D — WebAssembly (Wasm) plugins

Considered. Mature for some control planes (Envoy, Falco's Wasm rules) but not yet for Kubernetes operators broadly. **Deferred to a future RFC**; not part of MVP.

## 5. Shared Plugin Envelope

All plugin surfaces share the same envelope shape so the Go interface, CRD shape, and webhook payload remain consistent.

### CRD shape (common fields)

```yaml
spec:
  scope: { ... }            # what this plugin applies to (artifact types, environments, scoping selectors)
  endpoint:                 # exactly one of:
    builtin: <name>         #   built-in implementation by name
    webhook:                #   external HTTPS endpoint
      url: <url>
      authSecretRef: { name, key }
      caBundleSecretRef: { name, key }   # optional, for self-signed CAs
  delivery:
    mode: sync | async
    timeout: <duration>
    retries: <int>          # async only
    onError: fail | warn | retry
status:
  conditions: [ ... ]       # Ready, Available, DeliveryDegraded
  lastInvocation: { at, latencyMs, outcome }
  failureCount: <int>
```

### Webhook request envelope (HTTPS POST, JSON)

```json
{
  "envelopeVersion": "keleustes.skaphos.io/v1alpha1",
  "kind": "Notifier|SignatureVerifier|Scanner|PolicyGate|AuditDestination",
  "invocationId": "uuid",
  "correlation": {
    "traceId": "otel-trace-id",
    "spanId": "otel-span-id"
  },
  "subject": { ... },        // surface-specific (e.g., the Source revision, the Promotion, the artifact ref)
  "context": { ... }         // surface-specific
}
```

### Webhook response envelope

```json
{
  "envelopeVersion": "keleustes.skaphos.io/v1alpha1",
  "outcome": "allow|deny|defer|emitted",
  "reason": "...",
  "evidence": { ... },       // typed structured data, attached to the originating CR
  "messages": ["..."]        // human-readable, surfaced in status
}
```

## 6. Authentication

Plugins must authenticate Keleustes; Keleustes must authenticate plugins.

- **Keleustes → plugin:** mTLS (in-cluster service mesh) or a signed JWT in `Authorization: Bearer ...`. JWT carries `aud: <pluginCRDname>`, `iss: keleustes.skaphos.io`, short TTL, signed by the operator's serving key.
- **Plugin → Keleustes (if a plugin calls back):** discouraged. Plugins return outcomes in their webhook response; they do not call back into the operator. If a plugin needs to fetch additional context, that context is included in the request envelope.
- **Secrets in plugin specs** (e.g., Slack token, Splunk HEC token) are referenced from Secrets in the same namespace, never inlined.

This is consistent with rbac-audit-and-git-invariant.md §6 (identity boundaries).

## 7. Failure Semantics, Summarized

| Surface | Default mode | Default onError | Rationale |
|---|---|---|---|
| Notifier | async | retry | Routing failures must never block sync or promotion |
| SignatureVerifier | sync | fail | Signature is a security gate; missing signature is a fail |
| Scanner | async | warn | Findings inform a gate, but scanner outages should not block; gate logic can still fail |
| PolicyGate | sync | fail | Gate failure is exactly what gates exist to express |
| AuditDestination | async | retry+DLQ | Audit must reach durable storage; central JetStream is the source of truth |

Operators of Keleustes can override these per CR (with the constraint that audit cannot be made sync, and policy gates cannot be made fire-and-forget).

## 8. Reuse of Existing OSS

| Plugin surface | Existing tooling we can lean on |
|---|---|
| Notifier | Notification controllers exist in Argo (Argo Notifications) and Flux (alertmanager-style). Pattern is well-trodden. Webhook receivers like [shoutrrr](https://containrrr.dev/shoutrrr/) cover most sinks out of the box; we should not reinvent. |
| SignatureVerifier | [Sigstore policy-controller](https://github.com/sigstore/policy-controller); [Notary v2 / notation](https://github.com/notaryproject/notation) verifier. |
| Scanner | [Trivy operator](https://github.com/aquasecurity/trivy-operator) already emits CRDs for findings — could be a Scanner implementation that just reads its CRs. [Grype](https://github.com/anchore/grype). |
| PolicyGate | [OPA Gatekeeper](https://github.com/open-policy-agent/gatekeeper); [Kyverno](https://kyverno.io); [Sigstore policy-controller] also overlaps. |
| AuditDestination | Existing SIEM agents (Splunk HEC, OTel collector exporters) — most can consume a webhook directly. |

We do **not** vendor these into the operator. We provide built-in implementations for the most common defaults (cosign, Trivy, OPA) so installations work out of the box; everything else is webhook.

## 9. Configuration vs. Wiring

There is a deliberate split:

- **Plugin configuration** is declarative, lives in Git, and is owned by the platform team (CRDs above).
- **Plugin invocation** is operator-internal: the Source Engine looks up applicable `SignatureVerifier`s for a given revision and invokes them; the Promotion Engine looks up applicable `PolicyGate`s and invokes them; the audit stream walks registered `AuditDestination`s.

Selection is by **scope match**: a `SignatureVerifier` with `scope.artifactTypes: [containerImage]` and `scope.sources.registries: ["ghcr.io/foo-*"]` applies to revisions matching both. Multiple matches are allowed; the engine invokes all and aggregates results (with explicit, documented aggregation semantics per surface).

## 10. Open Questions for the Eventual ADR

1. **Mechanism lock-in.** Do we commit to webhook-only for v1alpha1 and defer gRPC/Wasm to v1beta1, or accept gRPC as a v1alpha1 alternative? Lean: webhook-only for v1alpha1.
2. **Built-in implementations: vendored or out-of-tree?** Lean: built-in `cosign`, `trivy`, `opa` are implemented inside Keleustes (no plugin needed for the common case). External verifiers/scanners/gates are webhooks.
3. **Plugin lifecycle ownership.** Does the Keleustes operator manage plugin Deployments (sidecars, Helm-bundled), or does the customer? Lean: customer-managed; we ship reference Helm charts in `contrib/`.
4. **Multi-tenant plugin isolation.** A namespaced plugin should not be able to receive events from other tenants' resources. Scope filters must be enforced server-side, and authentication must include tenant identity.
5. **Schema versioning.** `envelopeVersion` is in the envelope, but how do we handle a plugin that supports v1alpha1 receiving a v1alpha2 envelope? Lean: backward-compatible additions only within a major; explicit version negotiation in v1beta1.
6. **Aggregation semantics when multiple plugins match.** AND (all must pass) for verifiers and gates; UNION (all findings combined) for scanners; FAN-OUT (all receive a copy) for notifiers and audit destinations. Document explicitly.
7. **Caching policy.** Scanners and verifiers benefit from digest-keyed caches. Where does the cache live — per-plugin (in its CR status?), or in JetStream KV (rbac-audit-and-git-invariant.md §10)? Lean: JetStream KV, surfaced read-only in CR status.
8. **`Render` plugin surface.** Reserved for a future RFC. Risks are higher because rendering produces the manifests that get applied; tampering would defeat every downstream control.

## 11. Phased Rollout

Aligned with the existing MVP roadmap:

| MVP | Plugin work |
|---|---|
| **MVP 0** | Define the shared envelope and CRD shape. Land empty `Notifier` CRD + reconciler stub. No invocation paths yet (the engines are stubs). |
| **MVP 1** | Notifier delivery operational against the first events (SyncRun.Failed, Source.RevisionObserved). Built-in webhook delivery only. |
| **MVP 2** | `SignatureVerifier` (cosign built-in, webhook for others) wired into Source Engine. `Scanner` (Trivy built-in) wired into Source/Policy. `PolicyGate` (OPA built-in, webhook for others) wired into Promotion Engine. `AuditDestination` wired into the audit stream. |
| **MVP 3** | Multi-tenant scoping enforcement, mTLS-only mode, gRPC alternative considered for Scanner if perf demands it. |
| **MVP 4** | Federated plugin registries; FIDO2/WebAuthn integration for plugin-driven approval flows. |

## 12. What This Plan Replaces

This plan supersedes the implicit assumption in several existing tickets that vendors are hard-coded:

- SKA-354 (cosign verification): becomes "implement `SignatureVerifier` interface + cosign built-in"
- SKA-370 (Trivy/Grype): becomes "implement `Scanner` interface + Trivy/Grype built-ins"
- SKA-381 (OPA/Gatekeeper): becomes "implement `PolicyGate` interface + OPA built-in"
- SKA-388 (audit exporter): becomes "implement `AuditDestination` interface; reference exporter is one implementation"

Those tickets should be updated to call out the interface, not just the vendor.

## 13. Next Steps

1. Create the four new tickets covering the plan items above (see Linear).
2. Open ADR-NNNN: *Plugin extension model (webhook-first, declarative CRDs)*.
3. Refine SKA-354 / 370 / 381 / 388 descriptions to reference this plan.
4. Once the engine packaging (engine-boundaries plan §3) lands, add a `pkg/plugins/` shared package with the envelope types, dispatcher, and webhook client.
