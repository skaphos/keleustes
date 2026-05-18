<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Keleustes observability bundle

This directory ships the **MVP 0 slice** of the observability stack defined
in [`docs/plans/2026-05-observability-stack.md`](../../docs/plans/2026-05-observability-stack.md)
§4 + §13. ADR 0002 commits to Prometheus Operator + OpenTelemetry dual-export
as the default; this bundle covers the Prometheus side. The OpenTelemetry SDK
wiring lands with MVP 1 (SKA-405).

The bundle is **opt-in** (not included in `config/default/`). Prometheus
Operator and `kube-state-metrics` CRDs are customer-installed; pulling them in
as required dependencies of the main install would break clusters that don't
run those operators. Apply this overlay separately so it fails fast when the
prerequisite controllers are missing.

## Layout

| Path | Purpose |
| --- | --- |
| `prometheus/servicemonitor-manager.yaml` | Scrapes the controller-manager's `/metrics` via the Service shipped at `config/manager/service.yaml`. |
| `prometheus/prometheusrule-keleustes-manager.yaml` | Manager-level alerts (manager-down, restart-rate, leader-election flapping, reconcile error rate, workqueue depth) plus the `keleustes:reconcile_success_rate_5m` recording rule. Every alert carries a `runbook` annotation. |
| `prometheus/customresource-state.yaml` | `kube-state-metrics` CustomResourceStateMetrics config exposing `Application`, `SyncRun`, `Promotion`, `Notifier` status fields. |
| `dashboards/overview.json` | Keleustes Operator Overview Grafana dashboard. |
| `dashboards/kustomization.yaml` | Generates the dashboard ConfigMap with `grafana_dashboard: "1"` label so the Grafana sidecar / `grafana-operator` auto-discovers it. |

Each subdirectory is independently kustomize-built — apply only `prometheus/`
or only `dashboards/` via direct `kubectl apply -k <subdir>` if you don't want
both.

## What you need installed first

1. **Prometheus Operator** (`monitoring.coreos.com` CRDs) — install via the
   [`kube-prometheus-stack`](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack)
   chart or your platform team's preferred path. The `release: prometheus`
   label on the `ServiceMonitor` and `PrometheusRule` matches the default
   chart's `prometheusSpec.serviceMonitorSelector` / `ruleSelector`; override
   via kustomize patch if your Prometheus instance selects on a different
   label.

2. **`kube-state-metrics`** — required, not optional. Install via the
   [`kube-state-metrics` Helm chart](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-state-metrics).
   Two things in this bundle depend on it:
   - The `KeleustesManagerHighRestartRate` alert reads
     `kube_pod_container_status_restarts_total`, which is emitted by
     kube-state-metrics. Without it, the alert is permanently inert.
   - The CRD-state metrics (`keleustes_application_*`,
     `keleustes_syncrun_*`, `keleustes_promotion_*`,
     `keleustes_notifier_*`) come from the
     `keleustes-customresource-state` ConfigMap, fed to kube-state-metrics
     via the `--custom-resource-state-config-file` flag (the Helm chart's
     `customResourceState.config` value takes the inline body).
   Without kube-state-metrics, the dashboard renders empty CRD-status
   panels and one alert never evaluates with data.

3. **Grafana** with either the dashboard-discovery sidecar
   (`kiwigrid/k8s-sidecar`, default in `kube-prometheus-stack`) or
   [`grafana-operator`](https://github.com/grafana/grafana-operator) — both
   pick up ConfigMaps with the `grafana_dashboard: "1"` label and load the
   contained JSON.

## Apply

```bash
kubectl apply -k config/observability/
```

Apply the underlying Keleustes install (or at least `config/default/`) first
so the `keleustes-controller-manager-metrics` Service exists (the
`controller-manager-metrics` Service defined in `config/manager/service.yaml`
is renamed by `config/default/`'s `namePrefix: keleustes-`). The `job=`
label on scraped metrics is `controller-manager-metrics` regardless of the
kustomize prefix — that value comes from the Service's
`keleustes.skaphos.io/scrape-job` label and is what every PrometheusRule /
dashboard query selects on.

## Override examples

Different Prometheus instance label selector:

```yaml
# kustomization.yaml in your downstream overlay
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../keleustes/config/observability

patches:
  - target:
      kind: ServiceMonitor
      name: keleustes-manager
    patch: |-
      - op: replace
        path: /metadata/labels/release
        value: my-prom-instance
  - target:
      kind: PrometheusRule
      name: keleustes-manager
    patch: |-
      - op: replace
        path: /metadata/labels/release
        value: my-prom-instance
```

Disable an alert without forking the bundle:

```yaml
patches:
  - target:
      kind: PrometheusRule
      name: keleustes-manager
    patch: |-
      - op: remove
        path: /spec/groups/1/rules/4   # drops KeleustesManagerHighWorkqueueDepth
```

## Smoke test on `kind`

```bash
# 1. kind cluster + Prom-Operator
kind create cluster --name keleustes-obs-smoke
helm install kps oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack \
  --namespace monitoring --create-namespace --wait

# 2. Keleustes install
kubectl apply -k config/default/
kubectl apply -k config/observability/

# 3. Wait for the ServiceMonitor to discover the manager
kubectl -n skaphos-keleustes-system get servicemonitor keleustes-manager
sleep 60

# 4. Port-forward Prometheus and verify the manager is up
kubectl -n monitoring port-forward svc/kps-kube-prometheus-stack-prometheus 9090:9090 &
curl -s 'http://localhost:9090/api/v1/query?query=up{job="controller-manager-metrics"}' \
  | jq '.data.result[].value[1]'
# Expected: "1"

# 5. Verify the recording rule + alerts are loaded
curl -s 'http://localhost:9090/api/v1/rules' | jq '.data.groups[].name' | grep keleustes

# 6. Open Grafana to view the dashboard (default sidecar setup):
kubectl -n monitoring port-forward svc/kps-grafana 3000:80 &
# Login admin / prom-operator; "Keleustes Operator Overview" is in the
# auto-loaded dashboards list.

# 7. Teardown
kind delete cluster --name keleustes-obs-smoke
```

## What's deliberately not here yet

- **OpenTelemetry SDK + reference collector** (SKA-405, MVP 1). The plan's §5
  dual-export model lands when the SDK is wired into the manager binary.
- **Per-engine alert bundles** (Sync Engine, Promotion Engine, etc.) — each
  engine ships its own `PrometheusRule` and dashboard alongside its first
  reconciler. See plan §13.
- **SLO recording rules** (MVP 3 per plan §13).
- **Audit ↔ trace correlation in the UI** (MVP 4 per plan §13 — joins the
  audit envelope's `traceparent` field to the rendered dashboard via the
  UI's data-source bridging).
- **Multi-region federation** — the regional Prometheus + aggregate
  federation pattern from plan §9 lands with the agent work in MVP 2+.

These gaps are tracked by the tickets above; the bundle is intentionally
narrow at MVP 0 so customers can adopt it without first installing the rest
of the Keleustes engine surface.
