<!--
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
-->

# ADR 0001 — Plugin extension model: webhook-first, declarative CRDs

- **Status:** Accepted
- **Date:** 2026-05-17
- **Deciders:** Platform Architecture (Skaphos)
- **Linear:** SKA-401
- **Supersedes:** open questions in `docs/plans/2026-05-extensibility-plugin-surfaces.md` §10

## Context

Keleustes must integrate with notification sinks, signature verifiers,
vulnerability scanners, policy engines, and audit destinations. Several
existing tickets — SKA-354 (cosign), SKA-370 (Trivy/Grype), SKA-381
(OPA/Gatekeeper), SKA-388 (audit exporter) — name specific vendors. If each
lands as a hard-coded integration, the operator ends up "knowing the names of
vendors," which is the wrong abstraction. We want Keleustes to be opinionated
about contracts and unopinionated about vendors.

`docs/plans/2026-05-extensibility-plugin-surfaces.md` proposes five extension
surfaces governed by declarative CRDs, with built-in implementations for the
common defaults and HTTPS webhooks for everything else. This ADR promotes that
plan into a durable decision and resolves the open questions in plan §10.

## Decision

### 1. Five extension surfaces, governed by CRDs

The `keleustes.skaphos.io/v1alpha1` API group ships these plugin-surface CRDs:

| Surface              | Purpose                                                                |
|----------------------|------------------------------------------------------------------------|
| `Notifier`           | Route lifecycle events to external sinks                               |
| `SignatureVerifier`  | Verify image/chart/artifact signatures at Source resolution            |
| `Scanner`            | Produce vulnerability findings for an artifact or rendered manifest set|
| `PolicyGate`         | Evaluate a named gate during Promotion; produces typed evidence        |
| `AuditDestination`   | Forward audit events from the central JetStream stream to SIEM/analytics|

Other extension points (Render, Git provider) are explicitly **not** plugin
surfaces in v1alpha1 — see "Alternatives considered."

### 2. Mechanism: webhook-only in v1alpha1

Plugins are invoked over **HTTPS** against an endpoint declared in the plugin
CR. gRPC sidecars and Wasm plugins are reserved for future RFCs (no earlier
than v1beta1). In-process Go plugins are rejected outright (build coupling,
no isolation, supply-chain risk).

### 3. Built-in implementations live in-tree

For the common defaults the operator ships first-party implementations:

- `SignatureVerifier`: `builtin: cosign`, `builtin: notation`
- `Scanner`: `builtin: trivy`, `builtin: grype`
- `PolicyGate`: `builtin: opa`
- `AuditDestination`: `builtin: splunkHEC | sumoLogic | datadog | elastic | syslog`

A CR with `endpoint.builtin: <name>` invokes the in-tree implementation. A CR
with `endpoint.webhook: { url, authSecretRef, … }` invokes an external
process. The two are mutually exclusive.

### 4. Shared envelope schema

All plugin surfaces share a common envelope. The CRD shape, Go interface, and
HTTPS payload remain consistent.

**CRD common fields** (each surface adds its own typed `spec` extensions):

```yaml
spec:
  scope:        { … }    # surface-specific selectors (artifactTypes, environments, etc.)
  endpoint:              # exactly one of:
    builtin: <name>
    webhook:
      url: <url>
      authSecretRef:    { name, key }
      caBundleSecretRef:{ name, key }   # optional
  delivery:
    mode:    sync | async
    timeout: <duration>
    retries: <int>       # async only
    onError: fail | warn | retry
status:
  conditions:    [ … ]   # Ready, Available, DeliveryDegraded
  lastInvocation:{ at, latencyMs, outcome }
  failureCount:  <int>
```

**Webhook request envelope** (HTTPS `POST`, JSON):

```json
{
  "envelopeVersion": "keleustes.skaphos.io/v1alpha1",
  "kind":            "Notifier|SignatureVerifier|Scanner|PolicyGate|AuditDestination",
  "invocationId":    "<uuid>",
  "correlation":     { "traceId": "<otel>", "spanId": "<otel>" },
  "subject":         { … },
  "context":         { … }
}
```

**Webhook response envelope**:

```json
{
  "envelopeVersion": "keleustes.skaphos.io/v1alpha1",
  "outcome":         "allow|deny|defer|emitted",
  "reason":          "…",
  "evidence":        { … },
  "messages":        ["…"]
}
```

The shared envelope types and dispatcher live in `pkg/plugins/` once the
engine packaging from `docs/plans/2026-05-engine-boundaries-and-technology-integration.md`
§3 lands.

### 5. Authentication: mTLS preferred, JWT fallback

The plugin CR's `spec.endpoint.webhook.auth` field selects the mode:

- **Default (and required for in-cluster endpoints):** mTLS using the
  operator's serving certificate. The dispatcher trusts the cluster CA;
  plugins present a server cert and the dispatcher presents a client cert
  derived from its ServiceAccount.
- **Fallback (out-of-cluster endpoints):** signed JWT in
  `Authorization: Bearer …`. Claims:
  - `iss: keleustes.skaphos.io`
  - `aud: <pluginCRD-namespace>/<pluginCRD-name>`
  - short TTL (≤ 5 min)
  - signed by the operator's serving key

Plugins never call back into the operator; if a plugin needs additional
context, it must be included in the request envelope. Secret material in
plugin specs (Slack tokens, Splunk HEC tokens, etc.) is referenced from
namespace-scoped `Secret`s and never inlined.

### 6. Default failure semantics, per surface

| Surface             | Default mode | Default `onError` | Hard constraint                            |
|---------------------|--------------|-------------------|--------------------------------------------|
| `Notifier`          | async        | retry             | Cannot be made sync (would block sync/promotion) |
| `SignatureVerifier` | sync         | fail              | —                                          |
| `Scanner`           | async        | warn              | —                                          |
| `PolicyGate`        | sync         | fail              | Cannot be made fire-and-forget             |
| `AuditDestination`  | async        | retry+DLQ         | Cannot be made sync (audit must never block) |

Operators may override per CR within those constraints. A timed-out sync gate
is treated as failure with `Reason: GateTimeout`. Scanner *unavailability*
fails open (with a `ScannerUnavailable` condition); scanner *findings above
threshold* fail the gate.

### 7. Aggregation when multiple plugins match

Plugin selection is by scope match (e.g., a `SignatureVerifier` with
`scope.artifactTypes: [containerImage]` and `scope.sources.registries:
["ghcr.io/foo-*"]` applies to revisions matching both). When multiple plugins
match, the dispatcher aggregates results as follows:

| Surface             | Aggregation |
|---------------------|-------------|
| `SignatureVerifier` | AND — all matching verifiers must allow |
| `PolicyGate`        | AND — all matching gates must allow     |
| `Scanner`           | UNION — all findings are merged, deduped by `(cve, package)` |
| `Notifier`          | FAN-OUT — every matching notifier receives the event |
| `AuditDestination`  | FAN-OUT — every destination receives the event |

Aggregation semantics are documented in CRD field doc-comments and in the
`pkg/plugins/` dispatcher.

### 8. Plugin lifecycle ownership: mixed

- **Built-in implementations** (`endpoint.builtin: …`) run inside the operator
  process. The operator owns their lifecycle.
- **Webhook plugins** (`endpoint.webhook: …`) are deployed by the customer.
  Keleustes does not reconcile plugin `Deployment`s or `Service`s. Reference
  Helm charts for common plugins ship under `contrib/`.

This split keeps the operator's RBAC narrow (no need for cluster-wide
`Deployment` write permissions just to host plugins) while still giving
customers a working out-of-the-box experience for the common defaults.

### 9. Caching: JetStream KV

Digest-keyed caches for `SignatureVerifier` and `Scanner` results live in the
NATS JetStream KV store already required by
`docs/plans/2026-05-rbac-audit-and-git-invariant.md` §10. Cache keys are
`<surface>/<digest>/<verifierOrScannerName>`. The most recent invocation
outcome is surfaced read-only in the plugin CR's `status.lastInvocation`; the
authoritative cache state is in JetStream KV.

This keeps cache state out of CR status (no unbounded growth, no controller
write-rate pressure on status), survives operator restarts, and is safe across
multiple operator replicas.

### 10. Schema versioning

`envelopeVersion` is part of every request and response payload. Within a
major envelope version (`v1alpha1`):

- Additions are backward-compatible (new optional fields only).
- Required-field changes or semantic changes bump the major
  (`v1alpha1` → `v1beta1`), with explicit version negotiation on first request.
- A plugin that supports `v1alpha1` receiving a `v1alpha2`-flavoured envelope
  must continue to function — additions are non-breaking by construction in
  this window.

### 11. Multi-tenant scope enforcement

v1alpha1 ships **single-tenant-by-default**:

- Plugin CRs are namespaced; scope filters are honored by the dispatcher on a
  best-effort basis.
- The dispatcher does *not* yet cryptographically prove tenant identity to the
  plugin.

Hard cross-tenant enforcement (tenant identity in JWT claims, server-side
rejection of cross-tenant subjects, mTLS-only mode) lands in **MVP 3**
alongside the rest of the multi-tenant work named in plan §11.

This is acceptable for MVP 0–2 because:

1. Plugin invocation paths do not exist until MVP 1 (notifications) / MVP 2
   (verifier, scanner, gate, audit).
2. The first customers are single-tenant. The plan's MVP 3 milestone is the
   first one that ships multi-tenant in production.
3. Tightening enforcement is a non-breaking change — adding a claim and
   rejecting unrecognized subjects strengthens guarantees without breaking
   conforming plugins.

## Consequences

**Positive**

- One CRD shape and one webhook envelope across five surfaces means a single
  Go package (`pkg/plugins/`) implements dispatch, retry, caching, and
  observability once.
- Customer-built plugins are trivial: an HTTPS server that decodes JSON, emits
  JSON, and authenticates by JWT or mTLS.
- The operator never embeds vendor code paths beyond the documented
  built-ins; supply chain stays narrow.
- Plugin configuration is in Git as CRDs, satisfying the GitOps invariant
  (`docs/plans/2026-05-rbac-audit-and-git-invariant.md` §3).
- Built-in defaults (cosign, Trivy, OPA) mean installations work out of the
  box without deploying any sidecar.

**Negative / accepted costs**

- Webhook latency floor: one network round-trip per call. Mitigated by
  async-by-default for high-frequency events, in-cluster endpoints, and the
  JetStream KV cache.
- gRPC and Wasm escape hatches are explicitly deferred. If a perf-sensitive
  surface (likely `Scanner`) needs them, a follow-up ADR is required.
- The dispatcher is now in the security-critical path for verifier and gate
  results — bugs there have the same blast radius as a misconfigured gate.
  Mitigated by aggressive unit and integration tests in `pkg/plugins/`.
- Multi-tenant enforcement is deferred to MVP 3. Documented and acceptable
  because no plugin invocation paths exist before MVP 1.

## Alternatives considered

- **A: In-process Go plugins** (Go `plugin` package or build-time
  registration). Rejected: build coupling, no isolation (plugin panic crashes
  the operator), supply-chain risk, poor multi-tenancy.
- **C: gRPC sidecar with a defined `.proto`.** Operationally heavier (per-pod
  sidecar lifecycle), more rigid versioning, less familiar to platform teams.
  Kept open as a future option for `Scanner`-style high-throughput surfaces.
- **D: WebAssembly plugins.** Mature in some control planes (Envoy, Falco) but
  not broadly in Kubernetes operators. Deferred to a future RFC.
- **Render as a plugin surface.** Rendering produces the manifests that get
  applied; tampering would defeat every downstream control. Kustomize, Helm,
  and raw remain first-class in-tree. Reserved for a future RFC.
- **Git provider as a plugin surface.** GitHub, GitLab, and Azure DevOps are
  first-class via an in-tree `GitProvider` interface
  (`docs/plans/2026-05-engine-boundaries-and-technology-integration.md` §4).
  External Git providers can implement that interface but are not exposed as a
  plugin model in v1alpha1.

## Compliance and follow-ups

- Tickets that imply vendor lock-in (SKA-354, SKA-370, SKA-381, SKA-388)
  should be refined to call out the interface, not just the vendor.
- A new ticket should track `pkg/plugins/` (shared envelope types, dispatcher,
  webhook client, JetStream KV cache adapter) once engine packaging lands.
- MVP 0 ticket SKA-404 (`Notifier` CRD scaffold + reconciler stub) implements
  the first CRD under this ADR.
- This ADR will be revisited if `Scanner` latency demands gRPC, if a plugin
  needs to call back into the operator, or when multi-tenant enforcement
  lands in MVP 3.
