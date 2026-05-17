<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Scale Benchmark Harness — Design

- **Status:** Draft — 2026-05-17
- **Linear:** SKA-326. Blocks SKA-337 (MVP 0 benchmark gate — 100 Applications) and SKA-351 (MVP 1 benchmark gate — 1,000 Applications). Every MVP-exit ticket downstream (MVP 2 / 3 / 4) consumes this harness.
- **Promotes into:** a future ADR co-located with ADR 0005. Until then, authoritative for any code that lives under `tools/benchmark/`.
- **Resolves:** [`docs/plans/2026-05-distributed-runtime-architecture.md`](./2026-05-distributed-runtime-architecture.md) §13 Q13 (benchmark harness funding). Also turns runtime plan §11.5 "Required benchmarks per MVP exit gate" from a bullet list into actual runnable artifacts.
- **Related:** [ADR 0005](../adr/0005-distributed-runtime.md) §11.5 (the scale targets per MVP this harness validates), [SKA-322 Audit Schema](./2026-05-audit-event-schema.md) §12 (the consumers whose latency we measure), [SKA-324 JetStream Layout](./2026-05-jetstream-subject-and-stream-layout.md) §4 (the partition counts whose adequacy this harness benchmarks), [SKA-328 Sharded Controllers](./2026-05-sharded-controller-pattern.md) §10 (the per-MVP shard timeline whose exit gates we validate).

## 1. Purpose and Scope

Every MVP exit criterion in ADR 0005 §11.5 includes a benchmark
gate ("MVP 1 handles 1 K Applications", "MVP 2 handles 2.5 K
Applications", etc.). Without a runnable harness those numbers are
claims, not facts. This plan picks the harness shape, decides how
it integrates with CI, and pins the metrics + pass/fail mechanism
so MVP-exit reviewers have a single artifact to look at when
saying "ship" or "no."

**In scope:**

- The harness shape (two complementary parts — a Go binary plus K6
  scripts — and how they coordinate).
- Where the harness lives in the repo.
- Workload generation: synthetic Applications, Sources, Releases,
  DeploymentTargets at the MVP's target cardinality.
- Agent simulation: mock for CI; real agent binaries for full
  benchmark.
- Webhook burst simulation: provider HMAC, realistic payload
  sizes, ramp profiles.
- Promotion-wave simulation: many Promotions × many
  DeploymentTargets concurrent.
- Metrics emitted, dashboards consumed.
- Pass/fail mechanism (absolute thresholds per MVP gate + relative
  regression detection between consecutive runs).
- Cluster fixture profiles (kind for ephemeral CI; real cluster
  for the MVP exit gate).
- CI integration (pre-release-only) + recommended developer
  workflow.
- Per-MVP harness evolution.

**Out of scope:**

- The operator under test's implementation — the harness is
  product-agnostic about what's reconciling.
- Long-term performance regression tracking systems (`Conbench`,
  `bencher.dev`); we publish CSV/Prometheus snapshots and let
  whatever the team uses for trend analysis consume them.
- Per-cloud-provider cost characterization (separate exercise; the
  harness emits raw numbers, not dollars).

## 2. Decisions (Short Form)

The four decisions taken explicitly before drafting:

1. **Two complementary harnesses.** A Go binary under
   `tools/benchmark/` owns the CRD-aware operations (creating
   Applications/Sources/Releases, watching SyncRun phase
   transitions, mock-agent event publication). K6 scripts under
   `tools/benchmark/k6/` own the HTTP-shaped load (provider
   webhook bursts against the receiver, API server query patterns
   from a synthetic UI). The Go binary is the orchestrator; it
   launches K6 as a subprocess for the HTTP portion when needed.
   Single `task bench:*` namespace surfaces both.
2. **Cluster fixture: hybrid.** `task bench:ci` runs against
   kind; `task bench:full` runs against a real cluster (k3d local,
   or any cloud `KUBECONFIG`). Same harness binary, `--profile`
   flag switches.
3. **CI cadence: pre-release only.** No nightly run; the harness
   fires from a `workflow_dispatch` GitHub Actions workflow that
   the release manager triggers before cutting an MVP-exit tag.
   Developers may run `task bench:ci` locally on their workstation
   any time. Trade-off (§12.2): gives up continuous regression
   detection in exchange for zero standing CI cost; mitigated by
   making local runs easy and by the relative-regression check
   inside every pre-release run (§10.2).
4. **Agent simulation: mock for CI; real for full.** Mock agent
   publishes synthetic `Deployment` / `HealthCheck` events
   directly to JetStream via the real `internal/events` package
   (subject grammar from SKA-324). The full-benchmark profile
   spins up N real `internal/agent/` binaries on the cluster.

## 3. Why Two Harnesses

A single harness in one language can be made to do everything
this design needs. The case for splitting:

- **K6 is the right tool for HTTP load.** Webhook bursts (provider
  HMAC, realistic JSON payloads of 50–500 KiB, ramp + steady-state
  + spike profiles) are exactly K6's home turf. The dashboards
  (Grafana + InfluxDB / Prometheus output) and ramp grammar are
  proven; reimplementing them in Go would consume engineering for
  no gain.
- **Go is the right tool for CRD-aware operations.** Creating
  500 `Application` CRs with valid `OwnerReferences` + Source/
  Release links, watching for `SyncRun.status.phase=Succeeded`,
  reading `Deployment.status.conditions` to compute reconcile
  latency — all of this needs a typed `client-go` informer. K6
  with a custom `xk6` extension can do it, but the extension is
  more Go code than the equivalent stand-alone binary, with worse
  ergonomics.
- **Coordination is cheap.** The Go binary owns the lifecycle
  (create workload → launch K6 → collect K6 output → tear down →
  emit report). K6 runs as a `os/exec` subprocess with
  `--out experimental-prometheus-rw` pointed at the same
  Prometheus the Go side is publishing to. One report at the end.

The split is by job, not by language preference: Go for k8s,
K6 for HTTP, one orchestrator (the Go binary).

## 4. Repo Layout

```
tools/benchmark/
├── cmd/
│   └── keleustes-bench/
│       └── main.go              # the orchestrator binary
├── internal/
│   ├── workload/                # synthetic CR generator
│   │   ├── apps.go              # Application + SourceRef + Release wiring
│   │   ├── targets.go           # DeploymentTarget population
│   │   ├── promotions.go        # promotion-wave generator
│   │   ├── charts/              # synthetic Helm chart fixture library
│   │   │   ├── tiny/            # 1 Deployment + 1 Service
│   │   │   ├── small/           # ~10 templates, simple conditionals
│   │   │   ├── medium/          # ~30 templates, subcharts, hooks
│   │   │   └── large/           # ~60 templates, the pathological case
│   │   └── seed.go              # deterministic RNG; same seed → same workload
│   ├── agentsim/                # mock agent (CI profile)
│   │   ├── publisher.go         # writes Deployment/HealthCheck events via internal/events
│   │   └── topology.go          # N mock agents claiming N×targets pattern
│   ├── observe/                 # measurement-side glue
│   │   ├── informers.go         # watch SyncRun/Deployment for phase transitions
│   │   ├── promclient.go        # scrape the operator's /metrics
│   │   └── jetstream.go         # measure publish throughput & consumer lag
│   ├── report/                  # per-run report generation
│   │   ├── csv.go               # raw CSV output for trend analysis
│   │   ├── markdown.go          # human-readable summary
│   │   └── thresholds.go        # absolute thresholds per MVP, regression detection
│   └── profile/                 # the --profile flag's behavior
│       ├── ci.go                # 100 Apps, kind, mock agent
│       ├── mvp1.go              # 1,000 Apps, real cluster, mock agent
│       ├── mvp2.go              # 2,500 Apps, real cluster, real agents
│       ├── mvp3.go              # 10,000 Apps, real cluster, real agents
│       └── mvp4.go              # 10,000 Apps + full policy eval, real cluster
├── k6/
│   ├── webhook-burst.js         # provider HMAC + payload generation
│   ├── api-server-queries.js    # synthetic UI query patterns
│   └── shared/
│       ├── hmac.js              # GitHub/GitLab/Azure DevOps HMAC helpers
│       └── payloads/            # sample webhook bodies per provider
├── dashboards/
│   ├── benchmark-overview.json  # Grafana dashboard for the MVP-exit reviewer
│   ├── reconcile-latency.json   # SyncRun p50/p95/p99 by phase
│   ├── jetstream-throughput.json
│   └── memory-and-cpu.json
├── thresholds/
│   ├── mvp0.yaml                # absolute pass/fail per MVP
│   ├── mvp1.yaml
│   ├── mvp2.yaml
│   ├── mvp3.yaml
│   └── mvp4.yaml
└── README.md
```

A new `tools/benchmark/` directory at the operator-repo root,
peer to the existing `tools/` toolchain. Reuses the operator's
`go.mod` so the synthetic CR generator imports `api/v1alpha1`
directly (no schema drift). K6 scripts live alongside.

Why not a separate repo (`skaphos/keleustes-benchmark`)? Schema
drift. The benchmark's synthetic CR generator must track every
`api/v1alpha1` change; co-locating in the operator repo means a
PR that breaks the schema also breaks the benchmark build, caught
in code review. The cost — a slightly fatter operator repo — is
worth it.

## 5. Workload Generation

### 5.1 The synthetic-resource matrix

A single profile-driven generator. The user passes `--profile mvp1`
(or `mvp2`, etc.); the generator reads `internal/profile/mvp1.go`
for cardinalities and emits the corresponding CR set.

| Resource           | MVP 0   | MVP 1    | MVP 2    | MVP 3    | MVP 4   |
|--------------------|--------:|---------:|---------:|---------:|--------:|
| `Application`       | 100     | 1 000    | 2 500    | 10 000   | 10 000  |
| `Source`            | 100     | 1 000    | 2 500    | 10 000   | 10 000  |
| `Release` (active)  | 100     | 1 000    | 2 500    | 10 000   | 10 000  |
| `Release` (history) | 0       | 5 × 1 K  | 5 × 2.5 K | 5 × 10 K | 5 × 10 K |
| `DeploymentTarget`  | 10      | 50       | 200      | 500      | 500     |
| `Project`           | 5       | 25       | 100      | 250      | 250     |
| `Promotion` (steady)| 0       | 10/min   | 50/min   | 200/min  | 200/min |
| `RoleBinding`       | 50      | 500      | 1 500    | 5 000    | 5 000   |

Numbers above MVP 1 mirror ADR 0005 §11.5's scale targets exactly;
MVP 0 is a 1/10 downscale for the CI profile.

### 5.2 Realism dial

`internal/workload/charts/` contains a small library of synthetic
chart fixtures across complexity tiers:

- `tiny/`: 1 Deployment, 1 Service. Single template; no
  conditionals. Used to measure best-case render latency.
- `small/`: ~10 templates, simple `if .Values.x` conditionals.
  Representative of a typical microservice chart.
- `medium/`: ~30 templates, subcharts, named templates, a Pre-Sync
  hook. Representative of a typical Skaphos production chart.
- `large/`: ~60 templates, many subcharts, multiple hook waves,
  Helm dependencies on external repos. The pathological case
  that exposes render OOM (SKA-320 §9.1).

Per profile, a *mix* of chart shapes is generated. Default mix
matches observed Skaphos production distribution (50 % small,
35 % medium, 10 % tiny, 5 % large); the mix is configurable per
run with a JSON manifest if a specific incident reproduction
demands it.

### 5.3 Determinism

The workload generator is seeded by a flag (`--seed`, default
`0x5ca1ab1e`). Same seed + same profile = identical CR set across
runs. Lets a regression be re-run to confirm reproducibility.
Different seeds at the same profile probe robustness to specific
workload distributions.

### 5.4 Apply pace

Workload application is *not* instantaneous. The generator
emits CRs at a controllable rate (default 50/sec) so the
operator's admission webhook and apiserver write path are
exercised at a realistic ingestion rate — bulk-loading 10 K
Applications in one second doesn't measure anything except the
apiserver's QPS limit.

## 6. Agent Simulation

### 6.1 CI profile — mock agent

```go
// tools/benchmark/internal/agentsim/publisher.go (sketch)
type MockAgent struct {
    targetName  string
    eventBus    *events.Publisher  // the production internal/events client
    syncRunSeed *workload.SyncRunSeed
}

func (a *MockAgent) Run(ctx context.Context) {
    for range time.Tick(a.heartbeatInterval) {
        // 1) Synthesize Deployment events per assigned target.
        // 2) Publish Deployment status events on
        //    keleustes.events.<shard>.deployment.<ulid>.status
        // 3) Publish HealthCheck reports on
        //    keleustes.events.<shard>.healthcheck.<ulid>.report
        // 4) On Sync request, publish a 'sync.applied' event after a
        //    realistic delay drawn from a configurable distribution.
    }
}
```

The mock agent uses the real `internal/events` package — subject
grammar, partition function, NATS-Msg-Id dedup all come from
production code. The only fakery is *behind* the publish call:
no real `gitops-engine` `pkg/sync` runs, no real cluster cache,
no real `kubectl apply`. From the operator's perspective, the
mock agent is indistinguishable from a real agent except for the
timing distribution being statistically controlled.

The mock topology generator (`agentsim/topology.go`) creates one
mock agent per DeploymentTarget by default; a `--agents-per-target`
flag enables N>1 for testing concurrent-claim contention scenarios.

### 6.2 Full-benchmark profile — real agent

The MVP 2+ exit-gate runs spin up real `internal/agent/` binaries
on the cluster under test. The harness uses a pre-built agent
container image (from the operator repo's CI artifacts) and a
NATS leaf-node topology defined in `tools/benchmark/manifests/
agents.yaml`.

Trade-off explicit: real agents catch end-to-end issues (NATS
auth, leaf-node connectivity, cluster cache initialization) that
the mock cannot. Cost is full-benchmark runtime — minutes to spin
up the agent fleet, minutes more to settle into steady state.
Acceptable because MVP exit gates are infrequent.

## 7. Webhook Burst Simulation (K6)

### 7.1 The script shape

```js
// tools/benchmark/k6/webhook-burst.js
import http from 'k6/http';
import { check } from 'k6';
import { sign as githubSign } from './shared/hmac.js';
import { github as samplePayload } from './shared/payloads/github.js';

export const options = {
  scenarios: {
    ramp: {
      executor: 'ramping-arrival-rate',
      startRate: 1,
      timeUnit: '1s',
      preAllocatedVUs: 100,
      stages: [
        { duration: '1m', target: 50 },    // ramp to 50 webhooks/sec
        { duration: '5m', target: 50 },    // hold for 5 min steady state
        { duration: '30s', target: 200 },  // spike
        { duration: '2m', target: 200 },   // hold the spike
        { duration: '30s', target: 50 },   // back to baseline
        { duration: '5m', target: 50 },    // recovery observation
      ],
    },
  },
  thresholds: {
    http_req_duration:  ['p(95)<1000', 'p(99)<2000'],
    http_req_failed:    ['rate<0.01'],
  },
};

export default function () {
  const body = samplePayload();
  const headers = {
    'Content-Type':      'application/json',
    'X-GitHub-Event':    'push',
    'X-Hub-Signature-256': githubSign(body, __ENV.WEBHOOK_SECRET),
  };
  const res = http.post(`${__ENV.RECEIVER_URL}/webhook/github`, body, { headers });
  check(res, { 'status is 200': r => r.status === 200 });
}
```

Three K6 scripts ship at MVP 1: `webhook-burst.js` (above),
`api-server-queries.js` (synthetic UI query patterns —
`/api/applications`, `/api/promotions/<id>/audit`), and a
placeholder `agent-bus-load.js` for the MVP 2 agent-bus benchmark
when that becomes interesting.

### 7.2 Provider HMAC fixtures

`k6/shared/hmac.js` ships HMAC helpers for the three Git providers
we plan to support (GitHub SHA-256, GitLab, Azure DevOps). The
shared secret is injected via `K6_WEBHOOK_SECRET` env var; the
harness pre-creates the matching `Source.spec.webhook.secret`
references on the cluster.

### 7.3 Sample payloads

Real provider webhook bodies (PR opened, push, tag created)
captured from public providers and stored under
`k6/shared/payloads/`. Bodies are ~50–500 KiB to exercise the
receiver's body-parsing path realistically.

## 8. Promotion-Wave Simulation

Owned by the Go binary, not K6 (Promotion creation requires
typed CR writes, not HTTP).

```go
// tools/benchmark/internal/workload/promotions.go (sketch)
func RunPromotionWaves(ctx context.Context, cli client.Client, cfg WaveConfig) {
    // Every cfg.WaveInterval, create cfg.PromotionsPerWave Promotion
    // CRs across the workload's Applications. Each Promotion targets
    // a uniform sample of cfg.TargetsPerPromotion DeploymentTargets.
    // The harness watches each Promotion's status.phase, recording
    // wall-clock latency from creation to (Succeeded | Failed).
}
```

Profile defaults (per MVP):

| Profile | Wave interval | Promotions/wave | Targets/Promotion |
|---------|---------------|----------------:|-------------------:|
| MVP 0   | 30 s          | 5               | 2                  |
| MVP 1   | 30 s          | 10              | 5                  |
| MVP 2   | 30 s          | 25              | 20                 |
| MVP 3   | 30 s          | 100             | 50                 |
| MVP 4   | 30 s          | 100             | 50                 |

Steady-state per-MVP throughput maps directly to the §5.1 matrix's
"Promotion (steady)" row.

## 9. Metrics

### 9.1 What's measured

Categorized by who needs them:

**Latency (the headline numbers MVP reviewers look at):**

- `keleustes_syncrun_phase_seconds{phase=...}` — time-in-phase
  for every SyncRun, percentile aggregation. p50/p95/p99 per
  phase.
- `keleustes_promotion_wall_clock_seconds` — Promotion creation
  → terminal phase.
- `keleustes_render_seconds` — RenderRequest → RenderResult.
- `keleustes_webhook_to_source_revision_seconds` — webhook
  receipt → `Source.status.latestRevision` update.

**Throughput:**

- `keleustes_apiserver_writes_total` — CRD writes/sec by kind.
- `keleustes_jetstream_published_total{stream=...}` —
  events/sec per stream.
- `keleustes_jetstream_consumer_lag_messages{stream,consumer=...}`
  — backlog depth.

**Resource consumption per pod:**

- `process_resident_memory_bytes` by `pod` label.
- `container_cpu_usage_seconds_total` by `pod`.
- `go_gc_duration_seconds` percentiles (GC pause time).

**Errors:**

- `keleustes_reconcile_errors_total{controller,reason}`.
- `keleustes_audit_emit_failures_total` — SKA-322 §11.1 produces
  these; benchmark should see zero.

### 9.2 Where it goes

The operator already emits a Prometheus `/metrics` endpoint per
ADR 0002. The harness scrapes that endpoint plus the JetStream
`monitor:8222/jsz` endpoint at 10-second intervals throughout the
run. Output:

1. **Live to Prometheus** during the run (operators with the
   observability stack deployed see the run in their Grafana).
2. **CSV dump** at run end (`tools/benchmark/out/<timestamp>/
   metrics.csv`) for offline analysis and long-term trend storage.
3. **Markdown report** at run end (`report.md`) — the one
   document an MVP-exit reviewer reads.

### 9.3 Dashboards

Four Grafana dashboards under `dashboards/` ship with the harness:

- `benchmark-overview.json` — the single dashboard an MVP-exit
  reviewer opens. Latency percentiles, throughput, errors, memory
  envelope, pass/fail callouts at the top.
- `reconcile-latency.json` — per-controller, per-phase percentile
  breakdowns for debugging regressions.
- `jetstream-throughput.json` — per-stream publish/consume rates,
  partition skew (per SKA-324 §13 Q1), durable-consumer lag.
- `memory-and-cpu.json` — per-pod resource consumption, GC pause
  time.

Dashboards are JSON ConfigMaps the harness applies before a run;
they remove themselves after teardown unless `--keep` is passed.

## 10. Pass/Fail Mechanism

### 10.1 Absolute thresholds per MVP

Each MVP carries a `tools/benchmark/thresholds/mvp<N>.yaml` file
of absolute pass criteria. Example for MVP 1:

```yaml
# tools/benchmark/thresholds/mvp1.yaml
profile: mvp1
metrics:
  syncrun_phase_seconds_p95:
    succeeded: 5.0    # SyncRuns reach Succeeded within 5s p95
    failed:    10.0
  promotion_wall_clock_seconds_p95:
    completed: 30.0
  webhook_to_source_revision_seconds_p95: 2.0
  render_seconds_p95:
    small:  1.0
    medium: 3.0
    large:  10.0
errors:
  audit_emit_failures_total: 0           # MUST be zero
  reconcile_errors_total_per_minute: 1.0 # acceptable steady-state error rate
throughput:
  jetstream_published_total_per_sec_min:
    keleustes_audit: 100
    keleustes_events: 50
memory:
  manager_pod_max_mib: 1024
  per_agent_pod_max_mib: 512
```

The thresholds files are checked into the repo; bumping them is a
PR and a conversation. Source of truth for "MVP 1 passes."

### 10.2 Relative regression detection

Every pre-release run also compares against the previous run's
CSV (stored under `tools/benchmark/baselines/<profile>/last.csv`
in the repo). Two specific checks:

- **Latency regression:** any p95 metric exceeding 110 % of the
  baseline AND exceeding the absolute threshold's "warn" band
  fails the run.
- **Memory regression:** any pod's `process_resident_memory_bytes`
  exceeding 120 % of the baseline fails.

After a successful run, the orchestrator offers to update the
baseline (`--accept-baseline` flag, requires explicit operator
confirmation; never automatic).

The relative check exists because absolute thresholds drift over
time as the operator's feature set grows. Catching "this run is
10 % slower than last week" early prevents the death-by-a-thousand-
cuts pattern.

### 10.3 The report

```markdown
# Benchmark Report — MVP 1 — 2026-05-17T14:32Z

**Verdict:** ✅ PASS (3 warns, 0 fails)

## Headline metrics

| Metric                            | Threshold | Actual  | Δ vs baseline | Status |
|-----------------------------------|----------:|--------:|--------------:|--------|
| SyncRun→Succeeded p95             |   5.00 s  |  3.41 s |        +7 %   | PASS   |
| Promotion wall-clock p95          |  30.00 s  | 21.85 s |        -2 %   | PASS   |
| Render seconds p95 (medium)       |   3.00 s  |  2.78 s |       +12 %   | WARN   |
| JetStream events publish rate     |    ≥50/s  |    71/s |        -3 %   | PASS   |
| Audit emit failures               |       0   |     0   |          0    | PASS   |

(detail tables follow, organized by §9.1 categories)
```

Generated by `tools/benchmark/internal/report/markdown.go`. Lives
alongside the CSV under `tools/benchmark/out/<timestamp>/`.

## 11. Cluster Fixture Profiles

### 11.1 `--profile ci` — kind

Spins up a fresh kind cluster (`kind create cluster --config
tools/benchmark/manifests/kind-config.yaml`), installs the
operator + NATS + minimal observability stack, runs the workload,
tears down. Total runtime target: ≤ 30 minutes on a 16-core
laptop.

Profile cardinality is fixed at MVP-0 scale (100 Applications)
regardless of which target MVP a developer is preparing for —
kind is not big enough to honestly represent higher tiers, and
running them anyway produces noise that misleads regression
detection. For real MVP-exit scale, use `--profile mvpN` with a
real cluster.

### 11.2 `--profile mvp1` (and higher) — real cluster

Requires `KUBECONFIG` pointing at a real cluster. The harness
performs *no* cluster provisioning — bring-your-own. Reusable
fixture manifests under `tools/benchmark/manifests/` install the
operator + observability + NATS at the cardinality the profile
expects.

Recommended target shapes:

- **MVP 1:** 3-node k3d / 3-node managed (4 vCPU, 16 GiB per node).
- **MVP 2:** 6-node managed (8 vCPU, 32 GiB per node).
- **MVP 3:** 12-node managed (8 vCPU, 64 GiB per node) plus
  multi-region NATS supercluster.
- **MVP 4:** Same as MVP 3 plus the full policy evaluator running
  in the hot path.

These shapes are recommended, not enforced. The harness honors
whatever the cluster offers; reviewer judgement decides whether
the cluster shape was representative.

## 12. CI Integration

### 12.1 Pre-release workflow

```yaml
# .github/workflows/benchmark.yml
name: benchmark
on:
  workflow_dispatch:
    inputs:
      profile:
        description: 'mvp profile (mvp0..mvp4)'
        required: true
        default: 'mvp1'
jobs:
  bench:
    runs-on: ubuntu-latest
    timeout-minutes: 240
    steps:
      - uses: actions/checkout@v4
      - run: go -C tools/benchmark task bench:full -- --profile=${{ inputs.profile }}
      - uses: actions/upload-artifact@v4
        with:
          name: benchmark-report-${{ inputs.profile }}
          path: tools/benchmark/out/
```

Triggered manually by the release manager from the GitHub UI
before cutting any MVP-exit tag. The full benchmark requires a
`KUBECONFIG` available to the runner — either a self-hosted
runner with cluster credentials, or a workflow that first
provisions an ephemeral cloud cluster (out of scope for this
plan).

No nightly scheduled run. (The trade-off below.)

### 12.2 The "no nightly" trade-off

Pre-release-only means weeks of accumulated changes hit the
benchmark in one batch. A regression that landed three weeks ago
is harder to bisect than one caught the night it landed. We
accept this for two reasons:

- **CI cost.** A nightly real-cluster run costs cloud money;
  multiplied across 365 nights/year, the cost is non-trivial. The
  operator-the-team-uses costs and CI cost trade more clearly in
  favor of pre-release-only.
- **Local-run ergonomics compensate.** `task bench:ci` against
  kind is fast enough that any developer making a perf-sensitive
  change can run it on their workstation. The harness is built to
  encourage this; the kind profile finishes in ≤ 30 minutes on
  reasonable hardware. Developers self-monitor; the pre-release
  run is the final gate.

If post-MVP-2 we observe that regressions are slipping through
in batches, the trade-off can be revisited — nightly is one
workflow change away.

### 12.3 Developer workflow

```sh
# quick smoke against kind, 100 Apps, ~30 min
task bench:ci

# specific MVP profile against a real cluster
KUBECONFIG=~/.kube/staging task bench:full -- --profile=mvp1

# regenerate baseline after a deliberate perf improvement
task bench:full -- --profile=mvp1 --accept-baseline
```

The `task` namespace mirrors what's needed; nothing in the
harness assumes CI is the primary caller.

## 13. Per-MVP Harness Evolution

| MVP | What lands in the harness                                                                                                                        |
|-----|---------------------------------------------------------------------------------------------------------------------------------------------------|
| 0   | Initial scaffold: Go binary + K6 webhook script + kind profile + MVP 0 thresholds + Grafana dashboards + report generator. Mock agent only.        |
| 1   | MVP 1 profile + thresholds. Real-cluster fixture manifests. The mvp1 GitHub Actions workflow_dispatch wiring. First production-shaped benchmark.   |
| 2   | Real-agent simulation (real `internal/agent/` binaries on the cluster). Agent-bus K6 script. MVP 2 profile + thresholds.                          |
| 3   | Multi-region cluster fixture (NATS supercluster across two clouds / two regions). MVP 3 profile + thresholds. Cross-region latency metrics added. |
| 4   | Full policy evaluator in the hot path (Trivy, Grype, OPA). MVP 4 profile + thresholds. Per-policy evaluation latency metrics added.                |

Each MVP's "land the benchmark gate" ticket (SKA-337, SKA-351,
and equivalents for 2/3/4) implements the per-MVP slice of this
harness, not a from-scratch rewrite.

## 14. Open Questions

1. **Threshold drift cadence.** The thresholds files in
   `tools/benchmark/thresholds/` are checked-in YAML. A
   deliberate perf-degrading change (e.g., enabling
   cryptographic audit-chain hashing in MVP 4) needs an explicit
   PR to relax the threshold. Open: who reviews the relaxation,
   what's required as evidence. Probably: the architecture lead
   plus the MVP owner. Confirm once the team shape settles.

2. **Cloud-cluster fixture automation.** The MVP-exit workflow
   currently assumes someone provisions the cluster and points
   `KUBECONFIG` at it. A future refinement: a GitHub workflow
   that spins up the cluster via Terraform/Crossplane, runs the
   benchmark, tears down. Cost: maintaining the IaC. Deferred
   until the team finds the manual step painful.

3. **Long-term trend storage.** CSVs accumulate under
   `tools/benchmark/out/` and `baselines/`. A regression
   pattern across N MVPs requires querying the historical set
   — currently a shell script. If trend analysis becomes a
   recurring activity, plug into a real benchmarking service
   (`bencher.dev`, `Conbench`); this plan does not commit to
   one because the volume doesn't yet justify it.

4. **Synthetic Helm chart fixture realism.** The four-tier
   library (§5.2) is a first-cut. The "medium" tier ought to be
   regenerated periodically against a sample of real Skaphos
   production charts so it stays representative. Process for
   doing so without leaking proprietary chart content: open.

5. **K6 output ingestion into the same Prometheus.** K6's
   `experimental-prometheus-rw` is currently the recommended
   path; this is the area of K6 that changes most often. If the
   `experimental` flag flips before MVP 1, the harness picks up
   whatever K6's then-current stable path is — but the dashboards
   may need a label-name update.

## 15. Compliance with Prior Decisions

| Decision                                        | This plan honors it by                                                                                                    |
|-------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------|
| ADR 0005 §11.5 (scale targets per MVP)           | The five profiles match the §11.5 cardinalities exactly; thresholds files turn those targets into machine-checkable pass criteria. |
| ADR 0002 (default observability)                | The harness scrapes the operator's existing `/metrics` endpoint and reuses Grafana — no parallel observability stack.       |
| ADR 0006 §4 (containment rule)                  | The mock agent uses `internal/events` (Keleustes-owned) — it does not import `gitops-engine` packages.                     |
| SKA-322 §11.1 (write-then-act, audit emit must not fail) | `audit_emit_failures_total: 0` is a mandatory threshold in every MVP's `thresholds/*.yaml`.                          |
| SKA-324 §4 (xxhash64 partition function)        | The mock agent uses the production partition function via `internal/events/subject.For()` — it does not reinvent partition selection. |
| SKA-324 §13 Q1 (per-shard skew detection)       | The benchmark dashboards include a per-shard publish-rate panel that surfaces skew alongside the live metric the §13 Q1 monitor will eventually publish. |
| SKA-328 §10 (per-MVP shard timeline)            | MVP 1 profile runs at `partitionCount=1`; MVP 2+ profiles run at `=16` (or higher). The two-fleet transition (§8.5 of that plan) gets its own benchmark scenario at MVP 2. |
| CLAUDE.md (reconcilers must be idempotent)      | The harness deliberately produces duplicate events under contention scenarios; reconcilers that crash on duplicate input fail the benchmark immediately. |

## 16. Concrete Follow-ups

1. **SKA-337 (MVP 0 benchmark gate)** — implements the harness
   scaffold through §13's MVP 0 row: Go binary + K6 webhook
   script + kind profile + 100-Application thresholds +
   dashboards. ~2 weeks engineering. **Must land before MVP 0
   is closed.**
2. **SKA-351 (MVP 1 benchmark gate)** — adds the MVP 1 profile,
   real-cluster fixture manifests, the GitHub workflow_dispatch
   wiring, and the first production-shaped run. Builds on
   SKA-337.
3. **New ticket: cloud-cluster IaC for the bench workflow** —
   open question §14.2. Funded only if manual cluster
   provisioning becomes painful.
4. **New ticket: chart fixture refresh from real charts** — open
   question §14.4. Periodic; not on the MVP critical path.
5. **Update `docs/DECISIONS.md`** — add this plan to the active
   interim contracts table (handled in the same commit as this
   plan).

---

**When this plan stabilizes** (after SKA-337 + SKA-351 land and
at least one MVP-exit gate has actually been run from a real
release-manager workflow), §1–§13 promote into a new ADR
co-located with ADR 0005 — likely ADR 0012 (Render → 0007,
Audit → 0008, RBAC shapes → 0009, JetStream layout → 0010,
Sharded controllers → 0011, Benchmark harness → 0012). §14
open questions remain in this plan until resolved.
