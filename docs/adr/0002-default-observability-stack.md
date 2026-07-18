<!--
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
-->

# ADR 0002 — Default observability stack: Prometheus Operator + OpenTelemetry, dual-export

- **Status:** Accepted
- **Date:** 2026-05-17
- **Deciders:** Platform Architecture (Skaphos)
- **Linear:** SKA-402
- **Supersedes:** open questions in `docs/plans/2026-05-observability-stack.md` §12

## Context

Keleustes orchestrates deploys across thousands of
`Application × DeploymentTarget` cells (PROPOSAL §19). A control plane at
that scale has more moving parts than the workloads it manages, and without
first-class observability it is unoperable: no SLOs, no regression detection,
no on-call narrative, no customer trust.

Two ecosystems matter and neither is going away — Prometheus + Grafana, and
OpenTelemetry + vendor APM. The operator needs to work in both without code
changes. The Prometheus Operator is the de-facto Kubernetes wiring for the
former, and the OpenTelemetry SDK is the cross-stack contract for the
latter.

`docs/plans/2026-05-observability-stack.md` proposes a bundled stack
combining both. This ADR promotes the plan into a binding decision and
resolves the §12 open questions.

## Decision

### 1. Dual export is the default

The operator emits the same metric registry through two paths:

- **Prometheus** — pull, via `/metrics` on a dedicated `metrics` port.
- **OpenTelemetry** — push, via OTLP, enabled when
  `OTEL_EXPORTER_OTLP_ENDPOINT` (and related standard env vars) is set.

Both reflect the same registry; cardinality controls apply to both. OTel is
disabled when the env var is unset — Prom-native metrics still work in that
mode. OTel-only is **not** the v1alpha1 default; the dual stance is what
lets Keleustes drop into either ecosystem without an integration phase.

### 2. Prometheus Operator is the supported wiring path

`config/observability/prometheus/` ships first-party Prometheus Operator
manifests:

- `ServiceMonitor` per controller component (`manager`, MVP 2 webhook
  receiver, future worker pools).
- `PodMonitor` for the regional agent (no front-end Service required).
- `PrometheusRule` bundles per engine, listed in plan §4.3.
- `kube-state-metrics` CustomResourceState configuration
  (`customresource-state.yaml`) that exposes Keleustes CRD status fields as
  metrics. This is the v1 mechanism for CRD state metrics; a dedicated
  exporter is reserved for if customer-tunable cardinality controls become
  necessary.

Clusters without the Prometheus Operator can scrape `/metrics` directly and
author their own scrape configuration. The bundled experience targets
Prom-Operator users.

### 3. OpenTelemetry SDK with conventional wiring

The operator initializes the OTel SDK at startup, gated on the standard
environment variables. When enabled it emits:

- **Traces** — one root span per reconcile loop, child spans for render,
  apply, plugin webhook calls, Git provider calls, JetStream
  publish/consume. Plugin invocations carry the trace context in the
  request envelope (correlation.traceId / spanId) so downstream plugin
  spans stitch into the customer's APM. Default sampling: head-based, 1%,
  configurable per engine.
- **Metrics** — dual-exported as in (1).
- **Logs** — opt-in only (see §4).

Span naming format: `keleustes.<engine>.<operation>` (e.g.,
`keleustes.sync.reconcile`, `keleustes.plugin.webhook.invoke`). Span
attributes use the same label vocabulary as metrics.

The bundle ships a reference `OpenTelemetryCollector` CR under
`config/observability/otel/` as documentation of the expected wiring —
customers replace it with their own collector.

### 4. Logging: structured stdout by default, OTel logs opt-in

The operator emits structured JSON to stdout via controller-runtime's `logr`
+ Zap. Standard Kubernetes log collectors pick it up. Required fields per
log line:

- `traceId`, `spanId` (when a trace is active)
- `engine`, `application`, `environment`, `target` (when applicable)
- `reconcileGeneration`
- `event` — short verb-noun string (e.g., `render.started`,
  `policy.gate.failed`)

OTLP log export is implemented but disabled by default. Customers enable it
when they want one pipeline for logs, metrics, and traces. The choice is
deliberate: stdout is universally supported, costs nothing, and makes the
operator usable on day one in any cluster.

### 5. Label and cardinality conventions

| Label         | Cardinality control                                                  |
|---------------|-----------------------------------------------------------------------|
| `engine`      | Bounded (8 values)                                                    |
| `application` | Unbounded — counters and gauges only; **never** on histograms        |
| `environment` | Bounded by customer's environment count                               |
| `target`      | Unbounded — same restriction as `application`                         |
| `result`      | Bounded                                                                |
| `phase`       | Bounded                                                                |
| `region`      | Bounded                                                                |

**Per-application histograms are prohibited.** Where per-application latency
visibility is needed, it is recorded as `(count, sum)` counters and aggregated
in a recording rule. This rule is enforced in CI (see §6).

Histograms use exponential buckets (`prometheus.ExponentialBuckets`). Each
engine's section in the engine-boundaries plan should enumerate the metrics
it owns when that engine's code lands.

### 6. Alerts: runbook URL mandatory, enforced in CI

Every shipped `PrometheusRule` alert must carry:

- `annotations.summary` — one sentence
- `annotations.description` — multi-line, names affected resources and
  likely cause
- `annotations.runbook` — URL into the published docs

Enforcement: `task lint:alerts` walks every YAML under
`config/observability/prometheus/` and fails if any rule lacks `runbook`,
or if any metric expression references a histogram with an `application` or
`target` label. The check runs in CI on every PR touching observability
manifests or operator code that defines metrics.

No admission webhook in v1alpha1. The repo-local lint is the contract; we do
not police customer-installed `PrometheusRule` objects at runtime. A
follow-up could promote this to a `keleustesctl lint alerts` subcommand or
an admission webhook if real-world drift demands it.

### 7. Severity and scope taxonomy

| Severity   | Meaning                                                | Routing       |
|------------|--------------------------------------------------------|---------------|
| `critical` | User-facing failure or imminent risk                   | Paging        |
| `warning`  | Degraded but operating                                 | Ticket / Slack|
| `info`     | Observability event (e.g., freeze window activated)    | Slack / log   |

Scope label values are bounded:
`manager`, `source-engine`, `sync-engine`, `promotion-engine`,
`git-mutation-engine`, `policy-engine`, `health-engine`, `agent`,
`jetstream`, `plugins`.

### 8. Suggested SLOs (recording rules ship enabled, alerts opt-in)

| SLO                                       | Target                            | Window     |
|-------------------------------------------|-----------------------------------|------------|
| Sync success rate                         | 99%                               | 30d rolling|
| Promotion phase residency p95 → terminal  | < 30 min non-prod, < 4h prod      | 7d         |
| Render duration p95                       | < 5s                              | 1h         |
| Source revision detection lag             | < 60s p95                         | 1h         |
| Git mutation duration p95                 | < 30s                             | 1h         |
| Plugin webhook latency p95                | < 1s                              | 1h         |

These are baselines bundled in a `keleustes-slos` `PrometheusRule`.
Customers' own SLOs are theirs to set; the bundle's role is to ship the
recording rules so customers don't have to author them.

### 9. Dashboards

Grafana dashboards ship as ConfigMaps with the
`grafana_dashboard: "1"` label (the convention recognized by
`grafana-operator` and the Grafana sidecar). The bundle does not require
`grafana-operator`; the label convention works for either.

Initial dashboards: Operator Overview, Sync Engine, Promotion Engine, Source
Engine, Health Engine, Plugin Surfaces, Agent Health. Each dashboard JSON is
validated in CI against a Grafana schema.

### 10. Multi-region: regional scrape, federate aggregates

Each region runs its own Prometheus (or equivalent), scrapes the local agent
and workloads locally, and exports **aggregates only** to the hub via
federation. Per-application series stay regional. The hub overview
dashboard shows per-region rollups (e.g., `keleustes:sync_success_rate_5m`
by `region`), not per-pod or per-application detail.

Tracing is hub-and-spoke via OTel: regional collectors forward spans to the
hub-side collector, which sends to the customer's APM. A
`keleustes.region` attribute is on every span.

Logs are local-region by default; the hub's UI links out to the regional
log search. This keeps the hub from becoming a federation chokepoint.

There is no hub-side push lane for agent metrics in v1alpha1. If sub-scrape
visibility into a specific region becomes operationally necessary, the
decision will be revisited in a follow-up ADR — adding a curated push set
later is a non-breaking change.

### 11. Kubernetes Events: narrow by default

The operator writes Kubernetes Events only for:

- Promotion phase transitions (one event per `phase` change on a
  `Promotion`).
- Break-glass invocation (begin and end).

All other state transitions are surfaced through metrics and structured logs
exclusively. At the PROPOSAL §19 scale target (10K Applications) this keeps
etcd write pressure bounded and `kubectl get events` useful, while
preserving narrative for the two user-facing flows where operators
genuinely read Events (`kubectl describe promotion`,
`kubectl describe application` during break-glass).

### 12. Audit ≠ telemetry

Audit events (per
`docs/plans/2026-05-rbac-audit-and-git-invariant.md` §11) and telemetry are
separate pipelines:

- **Audit** is durable, source of truth, lives on JetStream.
- **Telemetry** is sampled/aggregated, operational, lives in
  Prometheus / OTel.

They are correlated through `traceId` (every audit envelope carries the
active OTel `traceId`) and through shared CRD references (both name the
same `Application`, `SyncRun`, `Promotion`). Neither replaces the other.
The UI navigates between them; the storage backends do not collapse.

### 13. Bundle layout and gating

```
config/observability/
├── prometheus/
│   ├── servicemonitor-manager.yaml
│   ├── podmonitor-agent.yaml
│   ├── prometheusrule-*.yaml
│   └── customresource-state.yaml
├── otel/
│   ├── opentelemetrycollector-reference.yaml
│   └── README.md
└── dashboards/
    ├── overview.json
    ├── sync-engine.json
    └── ...
```

Each subdirectory is independently gateable via kustomize, so customers can
opt out of any of `prometheus/`, `otel/`, or `dashboards/` without forking.

## Consequences

**Positive**

- Drops into Prom + Grafana stacks and into OTel + vendor-APM stacks without
  code changes; one operator binary serves both ecosystems.
- Prometheus Operator manifests are the supported wiring path, which matches
  the majority of Kubernetes operations teams' existing investment.
- `kube-state-metrics` CustomResourceState removes the need for a dedicated
  CRD-state exporter Deployment for v1.
- The cardinality and runbook lints are enforced before the first engine
  ships real metrics, so contributors don't accidentally explode cardinality
  or write unactionable alerts.
- Regional-scrape + aggregate-federation bounds cross-region traffic and
  preserves regional autonomy under hub partition.
- Narrow Kubernetes Events policy keeps etcd write load manageable at
  10K-Application scale without losing the two user flows that actually
  depend on Events.

**Negative / accepted costs**

- Dual export pays one extra registry tick on emit. Trivial in practice;
  measured during MVP 1 engine work.
- No hub-side agent push lane in v1alpha1. The hub overview dashboard's
  "is this region alive right now?" widget has scrape-interval latency
  (30s default). If that proves operationally unacceptable, a follow-up ADR
  adds a curated push set; this is a non-breaking change.
- Stdout-only logs by default means trace ↔ log correlation requires the
  customer's collector to merge stdout JSON with OTLP traces by `traceId`.
  The fields are present; the wiring is the customer's. Opting into OTLP
  logs gets the unified pipeline.
- Runbook-mandatory is a CI lint, not a runtime check. Drift in customer
  forks is possible. If we see it in the wild we promote to a CLI lint or
  an admission webhook (§6).
- Prometheus Operator dependency for the bundled experience. Non-Prom-
  Operator clusters are supported only via raw `/metrics` and BYO config.

## Alternatives considered

- **OTel-only.** Drops the Prom registry, simplifies emission. Rejected for
  v1alpha1 because the operations teams Keleustes targets overwhelmingly
  have Prom-Operator already deployed; OTel push without Prom pull cuts off
  the dashboards and alerts those teams already use.
- **Dedicated stats exporter sidecar for CRD state metrics.** More flexible
  than `kube-state-metrics`, allows per-customer cardinality knobs. Costs
  another Deployment in the bundle and another reconciler in the operator.
  Reserved for if `kube-state-metrics` CustomResourceState proves too rigid
  in practice.
- **OTel logs as the default.** Higher-fidelity correlation with traces,
  single pipeline. Rejected because it soft-couples the operator's basic
  log emission to an OTel collector being present. Opt-in keeps the
  zero-config path working.
- **Push everything from regional agents to a hub OTel collector.** Simpler
  topology, centralized failure domain. Rejected because it sacrifices
  regional autonomy under hub partition — a multi-region control plane
  must keep working when the hub is unreachable.
- **Verbose Kubernetes Events** (phase transitions on all major CRDs).
  Rejected at PROPOSAL §19 scale; etcd write rate at 10K Applications is
  the binding constraint. The metrics + logs path covers the same
  information for tooling, and the narrow Event set covers the two flows
  where humans depend on Events.
- **ValidatingAdmissionWebhook for the runbook-mandatory rule.** Catches
  drift after install. Rejected: webhook adds operator attack surface and
  failure modes; CI lint covers shipped bundles. Promote later if needed.

## Compliance and follow-ups

- SKA-336 (Observability foundation) becomes the MVP 0 implementation
  ticket for this ADR; its description should be tightened to
  "Implement MVP 0 of the observability bundle per ADR 0002."
- SKA-403 (Observability bundle — Prom-Operator manifests + customresource-
  state + overview dashboard) is the concrete deliverable for the MVP 0
  surface of this ADR.
- New ticket required: establish the `task lint:alerts` check and the
  cardinality lint *before* the first engine lands real logic (plan §15
  step 4).
- New ticket required: OTel SDK + reference collector for MVP 1.
- This ADR will be revisited if a hub-side agent push lane becomes
  necessary, if `kube-state-metrics` CustomResourceState proves too rigid,
  or if customer drift forces enforcement of the runbook rule beyond CI.
