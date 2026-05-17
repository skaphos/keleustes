<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Observability Stack Plan

**Status:** Draft
**Date:** 2026-05
**Related:** PROPOSAL §16 (UI), §17 (CLI), §19 (Scale targets); distributed-runtime-architecture.md plan §11 (storage), engine-boundaries-and-technology-integration.md plan, extensibility-plugin-surfaces.md plan, rbac-audit-and-git-invariant.md plan (audit ≠ telemetry)
**Owner:** Platform Architecture (Skaphos)
**Purpose:** Define the observability surfaces of Keleustes — metrics, traces, logs, dashboards, and alerts — so the system is operable from day one. The goal is an out-of-the-box bundle that works on any cluster running the Prometheus Operator and an OpenTelemetry collector, while remaining open enough that customers can route signals to their own pipelines.

---

## 1. Why This Matters Now

A control plane that orchestrates deploys across thousands of `Application` × `DeploymentTarget` cells has more moving parts than the workloads it manages. Without first-class observability:

- **No SLOs.** Reconcile lag, queue depth, sync error rate, render time — none of these become signals you can alert on.
- **No regressions caught.** A change that makes the Render Engine 10× slower is invisible until promotions start timing out.
- **No runbook narrative.** When a Promotion stalls at 03:00, the on-call engineer needs to know within 30 seconds whether the bottleneck is the Sync Engine, JetStream, an agent, or a third-party API.
- **No customer trust.** Operations teams adopt control planes they can observe. Argo CD's Prom-metrics ecosystem is one of the reasons it survived as long as it has; Flux's `kustomize-controller` likewise.

This plan is upstream of SKA-336 (the existing "observability foundation" MVP 0 ticket) and gives it the concrete shape it currently lacks.

Doing this in MVP 0 also lets every engine added in MVP 1+ inherit the patterns instead of bolting them on later — the cost of retrofitting OTel into a finished reconciler is materially higher than emitting it as you write.

## 2. Scope

In scope:

- What signals the operator and agents **emit** (controller-runtime metrics, custom metrics, traces, structured logs).
- What manifests we **ship** in `config/` (`ServiceMonitor`, `PodMonitor`, `PrometheusRule`, recording rules, default dashboards, optional `kube-state-metrics` custom resource state).
- The default **alert taxonomy** and SLO suggestions.
- Where **OpenTelemetry** fits (traces, metrics dual-export, logs correlation).
- How signals flow from **regional agents** back to the hub (or are scraped locally).

Out of scope:

- Specific Grafana stack choice (LGTM, Mimir, Cortex, vendor-hosted) — customers pick their own.
- Logs storage (Loki, Elastic, Splunk) — emitted via OTel; routing is the customer's collector configuration.
- The audit stream. **Audit is not telemetry.** Audit goes to JetStream and is durable; telemetry is sampled/aggregated and is operational. They share `traceId` for correlation but live in separate pipelines (rbac-audit-and-git-invariant.md §11).

## 3. Signal Surfaces

### 3.1 Metrics

Three layers:

1. **controller-runtime built-ins** — `controller_runtime_reconcile_total`, `controller_runtime_reconcile_errors_total`, `controller_runtime_reconcile_time_seconds`, workqueue depth/latency/adds. Free for every reconciler.
2. **Keleustes engine metrics** — per-engine counters, gauges, histograms with consistent label conventions.
3. **CRD state metrics** — derived from `Application`, `SyncRun`, `Promotion`, etc. status fields. Emitted via either a dedicated `kube-state-metrics` custom resource state config (the cheap option) or a small "stats exporter" sidecar (more flexibility).

**Label conventions** (every Keleustes metric, where applicable):

| Label | Source | Cardinality control |
|---|---|---|
| `engine` | source / sync / promotion / git_mutation / policy / health / diff / worker | Bounded (8 values) |
| `application` | `Application.metadata.name` | Unbounded — only on counters and gauges where the application name is essential, never on histograms |
| `environment` | `Environment.metadata.name` | Bounded by customer's environment count |
| `target` | `DeploymentTarget.metadata.name` | Unbounded — same restriction as `application` |
| `result` | success / error / blocked / canceled / timeout | Bounded |
| `phase` | enum from the relevant Status type | Bounded |
| `region` | for agent-emitted metrics | Bounded |

**Histograms** use exponential buckets (`prometheus.ExponentialBuckets(...)`) and **never** carry unbounded labels. Per-application reconcile latency, if needed, is recorded as a counter+sum, not a histogram.

**Indicative engine metrics:**

- `keleustes_source_revision_resolved_total{source_type, result}` — counter
- `keleustes_source_verification_failures_total{verifier, reason}` — counter
- `keleustes_render_duration_seconds{engine="render", manifest_type}` — histogram
- `keleustes_sync_run_phase_total{phase, result}` — counter
- `keleustes_sync_apply_resources_total{result}` — counter
- `keleustes_promotion_phase_duration_seconds{from_phase, to_phase}` — histogram
- `keleustes_git_mutation_duration_seconds{provider, mode, result}` — histogram
- `keleustes_policy_gate_evaluation_seconds{gate_id, result}` — histogram
- `keleustes_health_state{application, target}` — gauge (0=Unknown, 1=Healthy, 2=Degraded, 3=Unhealthy) — **bounded if customer has bounded app×target product**; otherwise emit aggregates only
- `keleustes_jetstream_consumer_lag_messages{stream, consumer}` — gauge

Each engine's plan section in engine-boundaries.md should be updated to enumerate the metrics it owns.

### 3.2 Traces

OTel traces span:

- One root span per reconcile loop (`engine.reconcile`, attributes include the `req.NamespacedName` and the CRD UID).
- Child spans for: render, gitops-engine sync, policy gate evaluation, plugin webhook calls, Git provider calls, JetStream publish/consume operations.
- Plugin invocations carry the trace context in the request envelope (`correlation.traceId`, `correlation.spanId`) so plugin spans can be stitched into the customer's APM.

Sampling: head-based, 1% default, configurable per-engine. Tail-based sampling is left to the customer's collector.

Span attributes mirror metric labels (engine, application, environment, target, region) plus a `keleustes.crd.generation` so a span can be tied to a specific generation.

### 3.3 Logs

Structured JSON via controller-runtime's `logr` + Zap, with mandatory fields:

- `traceId`, `spanId` — for trace correlation
- `engine`, `application`, `environment`, `target` where applicable
- `reconcileGeneration`
- `event` — short verb-noun string (e.g., `render.started`, `sync.applied`, `policy.gate.failed`)

Log levels follow Kubernetes operator conventions:

- `error` — operator-visible failure (e.g., manager startup, leader-election lost)
- `warn` — degraded but recoverable (e.g., plugin webhook 5xx, JetStream reconnect)
- `info` — lifecycle and significant transitions (e.g., reconcile complete, gate evaluated)
- `debug` (`v=1`) — per-reconcile noise (e.g., status equality short-circuit hits)
- `trace` (`v=2`) — controller-runtime internal noise

Logs ship via stdout → OTel collector → customer pipeline.

## 4. Prometheus Operator Integration

We ship Prometheus Operator manifests under `config/observability/prometheus/`. The Prometheus Operator is **expected** (not optional) for the bundled experience; customers without it can scrape `/metrics` directly and write their own configuration.

### 4.1 ServiceMonitor

One per controller component:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: keleustes-manager
  namespace: keleustes-system
  labels:
    app.kubernetes.io/part-of: keleustes
    release: prometheus       # customers override via kustomize patch
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: keleustes
      app.kubernetes.io/component: manager
  endpoints:
    - port: metrics
      interval: 30s
      scrapeTimeout: 10s
      relabelings:
        - sourceLabels: [__meta_kubernetes_pod_node_name]
          targetLabel: node
        - sourceLabels: [__meta_kubernetes_pod_label_keleustes_skaphos_io_engine]
          targetLabel: engine
```

Equivalent objects for: webhook receiver Deployment (MVP 2, SKA-366); any worker/job-pool Deployment; the agent Deployment (MVP 2, via `PodMonitor`).

### 4.2 PodMonitor

For agents (regional, possibly no Service in front of them):

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: keleustes-agent
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: keleustes-agent
  podMetricsEndpoints:
    - port: metrics
      interval: 30s
```

Agent metrics are scraped *locally* in each region by the regional Prometheus, then federated up. Agents do not push metrics to the hub.

### 4.3 PrometheusRule

We ship per-engine alert bundles. Customers can disable rules via kustomize or by editing alertmanager routing; the rules themselves are opinionated defaults, not configurations.

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: keleustes-sync-engine
spec:
  groups:
    - name: keleustes.sync.recording
      interval: 30s
      rules:
        - record: keleustes:sync_run_success_rate_5m
          expr: |
            sum(rate(keleustes_sync_run_phase_total{phase="Succeeded"}[5m]))
            /
            sum(rate(keleustes_sync_run_phase_total{phase=~"Succeeded|Failed"}[5m]))
    - name: keleustes.sync.alerts
      rules:
        - alert: KeleustesSyncRunSuccessRateLow
          expr: keleustes:sync_run_success_rate_5m < 0.95
          for: 10m
          labels:
            severity: warning
            component: sync-engine
          annotations:
            summary: "Keleustes SyncRun success rate is below 95% over 10 minutes."
            runbook: "https://docs.keleustes.skaphos.io/runbooks/sync-success-rate-low"
        - alert: KeleustesSyncRunStalled
          expr: |
            count by (application, target) (
              kube_customresource_keleustes_syncrun_phase{phase=~"Applying|Verifying"} > 0
              and
              (time() - kube_customresource_keleustes_syncrun_started_at) > 1800
            ) > 0
          for: 5m
          labels:
            severity: critical
          annotations:
            summary: "SyncRun stalled in non-terminal phase for >30m."
```

Initial rule bundles, one per `PrometheusRule`:

| Bundle | Sample alerts |
|---|---|
| `keleustes-manager` | manager pod restarts, leader-election flapping, certificate expiry |
| `keleustes-source-engine` | revision-resolution latency p95, verifier failure rate |
| `keleustes-sync-engine` | success rate, stalled SyncRun, drift rate |
| `keleustes-promotion-engine` | promotion-phase stuck, blocked-promotion count |
| `keleustes-git-mutation-engine` | provider error rate, mutation latency p95 |
| `keleustes-policy-engine` | gate timeout rate, plugin webhook 5xx |
| `keleustes-health-engine` | aggregated-unhealthy fraction |
| `keleustes-jetstream` | consumer lag, stream storage usage |
| `keleustes-agent` | agent connectivity, regional partition |

### 4.4 CRD State Metrics

Keleustes CRDs (`SyncRun`, `Promotion`, `Application`, etc.) carry critical state in their `status`. We expose that state to Prometheus via a `kube-state-metrics` **CustomResourceStateMetrics** configuration shipped in `config/observability/prometheus/customresource-state.yaml`:

```yaml
spec:
  resources:
    - groupVersionKind:
        group: keleustes.skaphos.io
        version: v1alpha1
        kind: SyncRun
      metrics:
        - name: keleustes_syncrun_phase
          help: "Phase of the SyncRun"
          each:
            type: StateSet
            stateSet:
              labelName: phase
              path: [status, phase]
              list: [Pending, Rendering, Applying, Verifying, Succeeded, Failed]
        - name: keleustes_syncrun_started_at
          help: "Unix timestamp when the SyncRun started"
          each:
            type: Gauge
            gauge:
              path: [status, startedAt]
```

This is the cheap option: no extra Deployment, no custom exporter. The "stats sidecar" alternative is reserved for if customer-tunable cardinality controls become necessary.

## 5. OpenTelemetry

### 5.1 SDK Integration

The operator initializes OTel at startup via `OTEL_EXPORTER_OTLP_ENDPOINT` (standard env vars). If unset, OTel is **disabled** (Prom-native metrics still work). If set, OTel emits:

- **Traces** — every reconcile loop and significant sub-operation.
- **Metrics** — dual-exported. Prom is scrape (pull), OTel is push. Both reflect the same metric registry; cardinality controls apply to both.
- **Logs** — optional; default off (stdout is already captured by Kubernetes log collectors).

### 5.2 Collector Pattern

Customers run their own OTel collector. Recommended deployment:

```
Keleustes pods ──OTLP──► OTel Collector (DaemonSet or Deployment) ──► customer backends (Tempo/Jaeger, Mimir, Loki, vendor-hosted)
```

We ship a reference `OpenTelemetryCollector` CR (operator-managed) under `config/observability/otel/` showing a sensible default pipeline. Customers will replace it; the reference exists to demonstrate the expected wiring.

### 5.3 Why Dual-Export

Some teams have invested in Prom + Grafana; some in OTel + vendor APM; many in both. Dual export means Keleustes works in either environment without code changes. The cost (one extra registry tick on emit) is trivial.

### 5.4 Span Naming Conventions

Span name format: `keleustes.<engine>.<operation>`, e.g.:

- `keleustes.sync.reconcile`
- `keleustes.sync.apply`
- `keleustes.render.kustomize`
- `keleustes.policy.gate.evaluate`
- `keleustes.git.mutation.commit`
- `keleustes.plugin.webhook.invoke`

Span attributes use the same label vocabulary as metrics (engine, application, environment, target).

## 6. Dashboards

We ship a starter set of Grafana dashboards as ConfigMaps with the `grafana_dashboard: "1"` label (the convention recognized by `grafana-operator` and the Grafana sidecar). Customers' Grafana picks them up automatically.

Initial dashboards (one ConfigMap each):

| Dashboard | Audience | Key panels |
|---|---|---|
| Keleustes Operator Overview | On-call | Manager health, JetStream lag, reconcile error rate, SyncRun success rate, blocked promotions, freezewindow active count |
| Sync Engine | Sync owner | Per-target SyncRun rate, p95 apply latency, drift detection rate, stalled SyncRuns table |
| Promotion Engine | Promotion owner | Phase-residency time, gate-failure heatmap, blocked-by table |
| Source Engine | Platform | Revision detection latency, verifier failure rate, scanner cache hit ratio |
| Health Engine | Platform / app teams | Aggregated health by environment×application, transition events |
| Plugin Surfaces | Platform | Per-plugin latency p95, error rate, retry/DLQ depth |
| Agent Health | SRE | Per-region agent connectivity, claim rate, queue depth |

Dashboards live in `config/observability/dashboards/`. Each dashboard JSON is validated in CI by `jsonnet`-rendering against a Grafana schema.

## 7. Alert Taxonomy

Alerts are bucketed by **severity** and **scope**:

| Severity | Meaning | Routing |
|---|---|---|
| `critical` | User-facing failure or imminent risk (e.g., manager down, JetStream unreachable, prod sync stalled) | Paging |
| `warning` | Degraded but operating (e.g., success rate below SLO, plugin webhook flapping) | Ticket / Slack |
| `info` | Observability event (e.g., freeze window activated, break-glass invoked) | Slack / log only |

Scope label values: `manager`, `source-engine`, `sync-engine`, `promotion-engine`, `git-mutation-engine`, `policy-engine`, `health-engine`, `agent`, `jetstream`, `plugins`.

Every alert ships with:

- `summary` — one sentence, dashboard-friendly.
- `description` — multi-line, includes the affected resources and likely cause.
- `runbook` — URL into the published docs. **Every alert must have a runbook.**

Alerts without runbooks fail the `task lint` check (a small `kubeconform`-adjacent validator we run on `PrometheusRule` objects).

## 8. SLOs (Suggested, Not Enforced)

Customers' SLOs are theirs to set, but we ship defaults in a `keleustes-slos` `PrometheusRule` (recording rules + ALERT) so customers can adopt them by labeling the rule group "enabled":

| SLO | Target | Window |
|---|---|---|
| Sync success rate | 99% | 30d rolling |
| Promotion phase residency p95 (non-terminal phase → terminal) | < 30 min for non-prod, < 4h for prod | 7d |
| Render duration p95 | < 5s | 1h |
| Source revision detection lag | < 60s p95 | 1h |
| Git mutation duration p95 | < 30s | 1h |
| Plugin webhook latency p95 | < 1s | 1h |

These are baselines; PROPOSAL §19 sets the hard scale targets (100 → 1,000 → 2,500 → 10,000 Applications).

## 9. Regional Agents and Federation

(Aligned with distributed-runtime-architecture.md.)

- Each region runs its own Prometheus (or equivalent), scraping the local agent + workloads.
- The hub Prometheus federates **aggregates only** from regional Prometheus instances (e.g., per-region `keleustes:sync_run_success_rate_5m`, not raw application-level series). This bounds cross-region cardinality.
- Tracing is hub-and-spoke via OTel: agents send spans through the regional collector, which forwards to the hub-side collector, which sends to the customer's APM. A `keleustes.region` attribute is on every span.
- Logs are local-region by default; the hub's UI links out to the region's log search.

This keeps the hub from becoming a federation chokepoint.

## 10. Tying Telemetry to Audit

Audit events (rbac-audit-and-git-invariant.md §11) and telemetry are separate pipelines. They are correlated through:

- **`traceId`** — every audit envelope carries the active OTel `traceId`. Clicking an audit event in the UI navigates to the trace; clicking a span navigates to the audit envelope.
- **CRD references** — both audit events and telemetry attributes name the same `Application`, `Promotion`, `SyncRun`, etc.

This is the difference between "what happened (durable, source of truth)" and "how the system behaved (sampled, operational)". Neither replaces the other.

## 11. Bundling and Packaging

The observability bundle ships under `config/observability/`:

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
    ├── promotion-engine.json
    └── ...
```

Kustomization gates each subdirectory so customers can disable any of `prometheus/`, `otel/`, `dashboards/` independently.

## 12. Open Questions for the Eventual ADR

1. **OTel-only vs. dual export.** Recommendation here is dual. If the team standardizes on OTel everywhere, dropping the Prom registry simplifies. Lean: keep dual for v1.
2. **CRD state via `kube-state-metrics` vs. dedicated exporter.** `kube-state-metrics` Custom Resource State works but is awkward to template. A small exporter is more flexible. Lean: start with `kube-state-metrics`; revisit if cardinality controls become necessary.
3. **Logging via stdout vs. OTel logs.** Default stdout is universally supported; OTel logs are higher-fidelity but add a dependency. Lean: stdout default, OTel logs opt-in.
4. **`grafana-operator` vs. sidecar ConfigMaps.** ConfigMap-with-label works for both; we ship that.
5. **Cardinality budget.** Per-application labels on counters are fine; per-application *histograms* are not. Need explicit guidance in the engine policy doc so contributors don't accidentally explode cardinality. Lean: enforce via lint.
6. **Agent metrics — pull from hub or scrape regionally?** Recommendation: scrape regionally, federate aggregates. Discuss whether some critical agent metrics should be pushed to a hub OTel collector for very-low-latency visibility.
7. **What about kube-events?** Recording controller-runtime `EventRecorder` Events into the API server is noise for clusters at scale. Lean: emit Events only at Promotion phase transitions and break-glass; everything else is metrics + logs.

## 13. Phased Rollout

| MVP | Observability work |
|---|---|
| **MVP 0** | controller-runtime metrics + Prom `/metrics` endpoint. Ship `ServiceMonitor`, `PrometheusRule` (manager-level alerts only), and the operator-overview dashboard. OTel SDK wired but disabled by default. (Closes SKA-336 substantially.) |
| **MVP 1** | Per-engine metrics for Source, Sync, Render. Sync Engine alert bundle. Sync Engine dashboard. OTel traces operational (opt-in). |
| **MVP 2** | Promotion / Git Mutation / Policy / Plugin metrics + alerts + dashboards. Agent `PodMonitor`. CRD State via `kube-state-metrics`. |
| **MVP 3** | Multi-region federation pattern documented and validated. Agent dashboards. SLO recording rules. |
| **MVP 4** | Audit ↔ trace correlation in the UI. Long-term analytics over the audit + telemetry corpus (overlaps with DuckDB plan, SKA-374). |

## 14. What This Plan Replaces / Refines

- **SKA-336** (Observability foundation, metrics/logs/dashboards) — this plan gives that ticket concrete shape. Description should be tightened to "Implement MVP 0 of the observability bundle per `docs/plans/2026-05-observability-stack.md`."
- Implicit assumptions in PROPOSAL §17 / §19 that an observability story exists — this plan **is** the story.

## 15. Next Steps

1. File the two new tickets covering "Prom-Operator manifests + dashboards bundle" (MVP 0) and "OTel SDK + reference collector" (MVP 1).
2. ~~Open ADR-NNNN: *Default observability stack (Prometheus Operator + OpenTelemetry, dual-export)*.~~ — landed as [ADR 0002](../adr/0002-default-observability-stack.md); §12 open questions are resolved there.
3. Update SKA-336 to reference this plan and adjust scope.
4. Establish the metric-label-cardinality lint rule before the first engine lands real logic.
