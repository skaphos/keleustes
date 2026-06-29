<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Keleustes UI — Design Specification

> **Status:** Draft v1 (scaffold-stage). This document is the design brief for
> the Keleustes web UI. It is written to be handed to a design tool (e.g. a
> Claude design pass) to add high-fidelity layout, visual, and interaction
> detail on top of an architecturally-correct skeleton.
>
> **How to use this doc:** Every screen below lists its *purpose*, *primary
> user*, *data* (mapped to API endpoints in [`openapi/keleustes.v1.yaml`](../../openapi/keleustes.v1.yaml)),
> *layout*, *states*, and *actions*. The **Hard Constraints** section is
> non-negotiable — designs that violate it contradict accepted ADRs and will be
> rejected. Everything in **Open for design** is yours to detail.

---

## 1. Product context

**Keleustes** is a Kubernetes-native GitOps delivery control plane. Think
"Argo CD, but Git is the only source of truth and the control plane is built for
fleets of clusters across regions." The UI is the primary operator surface; it
is **first-class, not optional** (PROPOSAL §16: *"Replacing Argo CD with
CLI-only purity will fail"*).

The UI and the `keleustesctl` CLI are **equal citizens** that consume the **same
REST API** (PROPOSAL §17/§18). The UI must never be a single point of
operational failure: anything an operator can do in the UI, they can also do in
the CLI. This means the UI is a *view + bounded actions* layer over the API — it
holds no privileged logic of its own.

### Personas

| Persona | Who they are | Primary jobs |
|---|---|---|
| **Platform operator (SRE)** | Owns the fleet. Lives in the matrix. | Spot unhealthy/drifted apps across env×region; diagnose; break-glass in incidents. |
| **Application team / release engineer** | Owns one or a few apps. | Track their app's rollout across environments; trigger and watch promotions. |
| **Approver / change manager** | Gates production. | Review promotion requests, policy gates, and evidence; approve or reject. |
| **Auditor / compliance** | Read-only. | Answer "what changed, when, by whom, with what evidence." |
| **Admin** | Configures the control plane. | Topology, policy, RBAC, identity providers. |

---

## 2. Hard constraints (ADR-derived — non-negotiable)

These come from accepted ADRs. When in doubt, the ADR wins over any visual idea.

1. **Read + exactly three write actions** (ADR 0003 §6). Every state-changing
   button does exactly one of:
   - **Approve** — a state-machine transition on a CRD (e.g. approve/reject a
     Promotion, cancel, retry).
   - **Promote** — *opens a Git pull request*. The UI never writes desired state
     directly; it proposes a Git change.
   - **Break-glass** — an explicitly audited, elevated cluster mutation, gated by
     step-up auth.
2. **No inline desired-state editing.** There is **no** "edit live manifest," no
   "override parameter," no "sync with overridden values," no free-form field
   that mutates cluster or desired state. Argo CD's edit-live and param-override
   patterns are *forbidden*. (ADR 0003.)
3. **Git is the source of truth.** Where the UI shows desired state, it shows it
   as "what Git says," with a link to the commit. Drift is *Git vs. live*, always
   framed as "live has diverged from Git," never "edit live to match."
4. **Identity is OIDC; authorization is verb-scoped and server-enforced**
   (ADR 0004). The UI obtains an OIDC token and sends it on every request. The
   **API server** decides what's allowed. The UI must **not** cache or infer
   roles client-side; it asks the server what the user may do and renders
   actions accordingly (disabled/hidden when not permitted), but never *enforces*
   on its own.
5. **Deep links prefer ULIDs where a resource has one** (audit/event model).
   Resources can be renamed; ULIDs are stable, so `/promotions/{ulid}` and audit
   deep-links survive renames. Applications and targets are addressed by **name**
   today — that is the API identifier in the REST contract (PROPOSAL §18,
   `/applications/{name}`). Giving them stable ULID deep-links is a forward
   enhancement that depends on the API gaining ULID lookup; until then the
   router params for those resources are names, not ULIDs.
6. **The matrix is eventually consistent.** At fleet scale (10k+ Applications)
   the app×env×region matrix is served from a **pre-computed snapshot**
   (DuckDB-on-parquet materialized from the event log), *not* live aggregation.
   Designs must accommodate a freshness indicator ("as of HH:MM:SS") and
   sub-minute staleness. (ADR 0005.)
7. **No RDBMS framing.** Audit/history comes from an event log (NATS JetStream);
   there is no "database" the UI talks to. This matters only in that history
   views are *event streams* (append-only, replayable), not editable records.

---

## 3. Visual language

> **Baseline:** Tailwind CSS + **shadcn/ui** (Radix primitives). Components are
> owned in-repo (copied, not a dependency), so the design can be fully bespoke.
> This section sets defaults; **Open for design** to refine.

### 3.1 Status vocabulary (load-bearing — keep consistent everywhere)

Keleustes surfaces a small, fixed set of states. Use one consistent color +
icon + label for each, across the matrix, detail views, badges, and timelines.

| State | Meaning | Suggested semantic | Applies to |
|---|---|---|---|
| **Healthy** | Live matches Git, all health checks pass | green | Application, target, release |
| **Progressing** | Sync/rollout in flight | blue (animated) | SyncRun, Promotion, Deployment |
| **Degraded** | Health checks failing | red | Application, target, HealthCheck |
| **Drifted** | Live has diverged from Git | amber | Application, target |
| **Blocked** | A policy gate or approval is preventing progress | purple | Promotion, gate |
| **Frozen** | A FreezeWindow is active for this target | slate + lock icon | Environment, target |
| **Missing / Unknown** | Expected resource absent or state not yet reported | gray | any |
| **Failed / Error** | Terminal failure of a run | red (solid) | SyncRun, Promotion |

Sync phases (from the engine): `Pending → Running → Succeeded | Failed | Error`.
Map `Running` → Progressing, `Succeeded` → Healthy, `Failed`/`Error` → Failed.

### 3.2 Density, type, layout

- **Dense, dashboard-native.** Operators scan large tables; favor compact rows,
  tabular numerals, monospace for versions/SHAs/ULIDs.
- **Dark and light themes** both required (operators run dark in NOCs). Status
  colors must pass WCAG AA contrast in both.
- **Primary layout:** persistent left **nav rail**, top **context bar**
  (environment/region/project scope selectors + identity), main content region.
- **Keyboard-first affordances** (operators live on keyboards): command palette
  (⌘K) for jump-to-app/promotion, `/` to focus search.

### 3.3 Open for design

Exact palette, typographic scale, spacing system, iconography set, the matrix
cell visual (chip vs. heatmap vs. sparkline), empty-state illustrations,
motion/transitions, command-palette design, responsive breakpoints.

---

## 4. Information architecture & navigation

Persistent **left nav rail** (top→bottom), grouped:

```
KELEUSTES
─ Overview          (fleet health summary; landing)
─ Applications      (the matrix — default view)
─ Promotions        (active + history, approvals queue)
─ Releases          (release/artifact inventory)
─ Environments      (topology: env → cell → target)
─ Audit             (activity log / event search)
─ Admin             (topology · policy · RBAC · identity)   [admins only]
```

Top **context bar** (global): Project scope selector · Environment filter ·
Region filter · global search (⌘K) · identity menu (user, IdP, sign-out) ·
theme toggle.

**Routing (deep-linkable; ULIDs for promotions/audit, names for apps/targets):**

| Route | Screen |
|---|---|
| `/` | Overview |
| `/applications` | Application matrix |
| `/applications/:appName` | Application detail |
| `/applications/:appName/diff` | Diff view (Git vs live / release vs release) |
| `/promotions` | Promotions list + approvals queue |
| `/promotions/:promotionUlid` | Promotion timeline detail |
| `/releases` | Release inventory |
| `/environments` | Environment topology |
| `/environments/:envName` | Environment detail (cells, targets, freeze) |
| `/audit` | Audit / activity search |
| `/admin/*` | Admin (topology, policy, rbac, identity) |

---

## 5. Global UI patterns

- **Loading:** skeleton rows/cards (not spinners) for tabular data; inline
  spinners only for in-place refresh.
- **Empty:** purposeful empty states with the one action that resolves them
  (e.g. "No applications yet — connect a Source").
- **Error:** non-destructive inline error banners with a retry; never swallow.
  Distinguish *auth error* (re-auth), *permission denied* (you lack verb X),
  *not found*, and *backend degraded*.
- **Degraded backend:** when the snapshot/matrix source is stale or a regional
  agent is unreachable, show a persistent, dismissible banner with the
  "as of" timestamp and which region is lagging — never silently show stale data
  as fresh.
- **Permission-aware actions:** action buttons render **disabled with a tooltip**
  ("requires `promote` on this Application") when the server says the user lacks
  the verb. Destructive/elevated actions (break-glass) require a confirm dialog
  and, per policy, step-up auth.
- **Toasts** for async action acknowledgement ("Promotion PR opened →
  github.com/...", "Approval recorded").
- **Every action surfaces its audit consequence**: a small "this will be
  recorded" affordance, with a link to the resulting audit event after the fact.

---

## 6. Screens

Each screen: **Purpose · User · Data (→ endpoint) · Layout · States · Actions.**
Wireframes are ASCII sketches — structure, not final visuals.

### 6.1 Overview (`/`)

- **Purpose:** at-a-glance fleet health; the landing surface.
- **User:** platform operator.
- **Data:** rollup counts of apps by status; active promotions; open approvals
  assigned to me; recent audit highlights; degraded targets.
  (`GET /applications` summary, `GET /promotions?state=active`, `GET /audit?limit=n`)
- **Layout:**
  ```
  ┌──────────────────────────────────────────────────────────┐
  │ Fleet health   [▇▇▇▇▇ 412 Healthy ·  7 Degraded · 3 Drift]│
  ├───────────────┬──────────────────┬───────────────────────┤
  │ Needs me      │ Active promotions │ Recent activity       │
  │ (approvals)   │ (progressing)     │ (audit highlights)    │
  │ • prom-…  ▸   │ • app-api → prod  │ • alice approved …    │
  └───────────────┴──────────────────┴───────────────────────┘
  ```
- **States:** all-healthy (celebratory-calm), incidents present (degraded tiles
  rise to top), nothing-assigned.
- **Actions:** navigational only (drill into a promotion/app/approval).

### 6.2 Application matrix (`/applications`) — the centerpiece

- **Purpose:** see every application across every environment and region, with
  deployed version + health per cell. The iconic Keleustes view.
- **User:** platform operator; app teams (filtered to their apps).
- **Data:** `GET /applications/{name}/matrix` (per app) / a fleet matrix
  endpoint; each cell = `{version, status, drift, lastSync, targetRef}`. Served
  from the **pre-computed snapshot** — show "as of" freshness.
- **Layout:** rows = applications, columns = environments (× regions, grouped).
  Cells are status chips with the deployed version.
  ```
  as of 14:32:07 ·  [filter: project ▾] [env ▾] [region ▾] [search /]
  ┌────────────┬──────────┬──────────┬──────────┬──────────┐
  │ Application │ dev      │ staging  │ prod-us  │ prod-eu  │
  ├────────────┼──────────┼──────────┼──────────┼──────────┤
  │ api        │ ●1.4.2   │ ●1.4.2   │ ◐1.4.1↻  │ ⚠1.3.9 ⤳│  ⚠=degraded ⤳=drift ↻=progressing
  │ web        │ ●2.0.0   │ ●2.0.0   │ ●1.9.4   │ ●1.9.4   │
  │ worker     │ ●0.7.1   │ ◌        │ ◌        │ ◌        │  ◌=not deployed
  └────────────┴──────────┴──────────┴──────────┴──────────┘
  ```
- **Interactions:** cell hover = mini-popover (version, last sync, health, drift
  summary); cell click = jump to Application detail scoped to that target; column
  header = environment detail; sticky first column + header; virtualized for
  thousands of rows.
- **States:** loading (skeleton grid), stale-snapshot (banner), filtered-empty,
  partial (region lagging → that column shows "lagging").
- **Actions:** none destructive from the grid; drill-in only. (Bulk actions are
  explicitly out of MVP scope.)

### 6.3 Application detail (`/applications/:appName`)

- **Purpose:** everything about one app: where it's deployed, at what version,
  health, drift, promotion history, the Git commit behind desired state,
  rendered resources, and the resource tree.
- **User:** app team, operator.
- **Data:** `GET /applications/{name}`, `/releases`, `/promotions`, target
  health/drift; rendered manifests (object-store cache); live resource tree.
- **Layout:** header (name, owner/project, overall status, source repo link) +
  tabs:
  ```
  api   ● Healthy   project: payments   source: git@…/api ↗
  [ Overview | Targets | Promotions | Resources | Diff | Audit ]
  ─ Overview: per-(env,region) cards: version · health · drift · last sync ·
              "desired = commit abc123 ↗"
  ─ Targets:  table of DeploymentTargets with health + drift + freeze state
  ─ Resources: live resource tree (drill into k8s objects, status, events)
  ─ Diff:     → 6.5
  ─ Audit:    → per-resource audit timeline (6.7 scoped to this app)
  ```
- **States:** healthy, degraded (surface failing HealthChecks), drifted (drift
  banner with "view diff"), frozen (freeze badge), mid-promotion (progress).
- **Actions:** **Promote** (opens PR — primary action, verb-gated); links to
  diff and audit. No inline edits.

### 6.4 Promotions — list (`/promotions`) + timeline (`/promotions/:ulid`)

- **Purpose:** track the lifecycle of a promotion (a proposed move of a release
  from one env to the next), including policy gates, approvals, and the audit
  trail. The approver's home.
- **User:** approver, release engineer, operator.
- **Data:** `GET /promotions` (filter active/blocked/mine/history),
  `GET /promotions/{id}`; gate + approval state; linked PR.
- **List layout:** queue with tabs `[ Needs my approval | Active | Blocked |
  History ]`; rows: app, from→to, mode, status, blockers, age.
- **Timeline layout:**
  ```
  Promotion 01J…  api: staging → prod-us   ● Blocked (policy gate)
  ┌ Source release: 1.4.2 (commit abc123 ↗)
  ● Requested      by alice · 14:01
  ● Policy gates   [✓ image-signed] [✓ vuln-scan] [✗ approvals: 1/2]
  ○ Approvals      alice ✓ · bob ⧖ pending
  ○ Git mutation   (will open PR on approval)
  ○ Sync           —
  ```
- **States:** requested, blocked (which gate, with evidence), awaiting-approval,
  approved/applying (PR link, sync progress), succeeded, failed/cancelled.
- **Actions:** **Approve / Reject** (state transition, verb-gated, with optional
  comment); **Cancel**; **Retry** (on failure). Each shows the evidence it acts
  on and records an audit event. Approve may require step-up per policy.

### 6.5 Diff view (`/applications/:appName/diff`)

- **Purpose:** explain *why* things differ. Git-vs-live drift, release-vs-release,
  env-vs-env, rendered-manifest, and policy diffs.
- **User:** operator diagnosing drift; approver reviewing a promotion's change.
- **Data:** `GET /diff?...` with a mode + two refs (commit/release/target).
- **Layout:** two-pane (or unified) diff with a mode selector and a resource
  navigator (list of changed objects → jump to hunk). Summary header: N added /
  M changed / K removed.
  ```
  mode: [ Git ↔ Live ▾ ]   left: commit abc123   right: live (prod-us)
  Changed objects (3)         │  Deployment/api
  • Deployment/api  ~         │  - replicas: 3
  • ConfigMap/api   +         │  + replicas: 5   ⤳ drift (live changed)
  • Service/api     =         │
  ```
- **States:** in-sync (empty, "no differences"), drift present, render-error
  (couldn't render one side), large-diff (collapse/expand).
- **Actions:** none that mutate. From a drift, the resolving action is
  **Promote** (reconcile via Git) — link back to detail. Never "make live match"
  inline.

### 6.6 Releases (`/releases`)

- **Purpose:** inventory of deployable artifacts (releases) and their provenance
  (signatures, SBOM, source commit), and where each is deployed.
- **User:** release engineer, auditor.
- **Data:** `GET /releases`; per release: version, source, provenance/attestation
  links, deployment footprint (which targets run it).
- **Layout:** table (version, app, created, provenance ✓/✗, deployed-on count) →
  detail drawer with provenance evidence and "deployed on" target list.
- **States:** signed/verified, unsigned/unverified (flag), orphaned (deployed
  nowhere).
- **Actions:** navigational; **Promote this release** (→ opens promotion flow).

### 6.7 Audit (`/audit`)

- **Purpose:** answer "what changed, when, by whom, with what evidence."
  Append-only activity stream from the event log. (SKA-348 wires the MVP-0
  placeholder; SKA-322 defines the event envelope.)
- **User:** auditor, operator, approver.
- **Data:** `GET /audit?resource=&actor=&verb=&from=&to=` — events carry
  `{ulid, actor, action.verb, target, result.before, result.after, evidence}`.
- **Layout:** filter bar (resource, actor, verb, time range) + virtualized
  timeline; row = who · verb · target · time; expand = before/after + evidence
  + deep link to the resource (by ULID).
  ```
  [resource ▾][actor ▾][verb ▾][last 24h ▾]                 1,204 events
  14:03  alice   approve   Promotion 01J… (api s→prod)   ▸
  14:01  alice   promote   Application api               ▸
  13:58  system  sync      DeploymentTarget prod-us/api  ▸
  ```
- **States:** results, filtered-empty, loading-more (infinite scroll), export.
- **Actions:** read-only; copy deep link; export (CSV/NDJSON) if permitted.

### 6.8 Environments / topology (`/environments`)

- **Purpose:** the shape of the fleet — environments → cells → deployment
  targets → clusters, with health and active freeze windows.
- **User:** operator, admin.
- **Data:** `GET /environments`, `/targets`, `/targets/{name}/health`,
  freeze-window state.
- **Layout:** hierarchical (tree or nested cards): Environment → Cell → Target
  (cluster), each with health + freeze badge. Detail shows target health and
  active/scheduled FreezeWindows.
- **States:** healthy, target-unreachable, freeze-active (lock + window), freeze-
  scheduled.
- **Actions:** navigational. (Declaring a freeze is a Git/CRD change → Admin or
  Promote-style PR flow, not an inline toggle.)

### 6.9 Admin (`/admin/*`)

- **Purpose:** configure topology, policy (PromotionPolicy / ApprovalPolicy),
  RBAC (Roles/RoleBindings/Projects), and identity (IdentityProvider / OIDC).
- **User:** admin only (verb-gated; hidden otherwise).
- **Data:** the corresponding CRDs (read), with changes flowing through Git/CRD
  PRs where they represent desired state.
- **Layout:** sub-nav (Topology · Policy · RBAC · Identity), each a read view
  with "propose change" (PR) affordances. IdP config shows configured providers
  and verb→role mappings.
- **States:** configured, misconfigured (validation surfaced), single-IdP (MVP 0)
  vs multi-IdP (later).
- **Actions:** changes are **proposed via Git PR**; break-glass and RBAC edits
  are audited. No silent live edits.

---

## 7. Key user flows

1. **Approve a promotion.** Overview "Needs me" → Promotion timeline → review
   gates + evidence + diff → **Approve** (step-up if required) → toast with audit
   link → status advances to applying (PR opened) → sync progresses to Healthy.
2. **Diagnose drift.** Matrix spots `⤳` on `api/prod-us` → cell → Application
   detail (drift banner) → **Diff** (Git ↔ Live) → identify the diverged object →
   resolve by **Promote** (reconcile through Git). Never edit live.
3. **Break-glass during an incident.** Degraded target → detail → **Break-glass**
   (confirm + step-up) → elevated action executes and is loudly audited → drift
   surfaced afterward so the temporary change is reconciled back into Git.
4. **Answer "who changed X."** Audit → filter by resource/actor/verb/time →
   expand event → before/after + evidence → deep-link to the resource.
5. **Track my app's rollout.** App team filters matrix to their project → watches
   version propagate dev→staging→prod → opens promotions as gates pass.

---

## 8. Data dictionary (domain objects the UI renders)

Keleustes CRDs (`keleustes.skaphos.io`). The UI reads these via the API; field
names below are what designs should label.

| Object | Key fields the UI shows |
|---|---|
| **Application** | name, project, source(repo,path,ref), overall status, owner, addonRefs |
| **Source** | repo URL, ref/branch, last-resolved commit, verification status |
| **Release** | version, app, source commit, provenance (signature, SBOM, attestation), created |
| **Deployment / DeploymentTarget** | target name, cluster/cell, env, region, deployed version, health, drift, lastSync |
| **Environment** | name, cells, regions, change-control, freeze state |
| **Cell** | name, region, targets, cluster health |
| **Promotion** | from→to, mode, source release, gates[], approvals[], status, linked PR, ulid |
| **PromotionPolicy / ApprovalPolicy** | required gates, required approvals, scope |
| **Approval** | approver, decision, comment, timestamp |
| **FreezeWindow** | scope (env/target), window (start/end/recurrence), reason |
| **SyncPlan / SyncRun** | phase (Pending/Running/Succeeded/Failed/Error), objects, message |
| **HealthCheck** | resource, status, message, lastProbe |
| **Audit event** | ulid, actor, action.verb, target, result.before/after, evidence, time |
| **IdentityProvider** | type (OIDC), issuer, verb→role mapping (admin) |

---

## 9. Out of scope (for this spec / MVP 0–1)

- Inline desired-state editing of any kind (forbidden, not deferred).
- Bulk fleet mutations from the matrix.
- Multi-region UI matrix specifics (SKA-373/379) — single-environment scoping
  first; multi-region columns are grouped but the cross-region *experience* is
  later.
- Notifications center beyond toasts (Notifier CRD, SKA-404, later).
- Cost/analytics dashboards (DuckDB BI consumer is `contrib/`, customer-owned).

---

## 10. For the design pass — what to add

Concrete, high-value detail we want back:

1. **The matrix cell** — the single most important visual. Explore chip vs.
   heatmap vs. version+status composite; hover popover; how drift/progressing/
   freeze read at a glance across hundreds of rows.
2. **Status system** — finalize the palette + iconography for §3.1 in both
   themes, WCAG AA.
3. **Promotion timeline** — make gates/approvals/evidence legible and scannable.
4. **Diff view** — the most information-dense screen; make large diffs navigable.
5. **App shell** — nav rail, context bar, command palette, identity menu,
   theme toggle; responsive behavior.
6. **Empty / loading / error / stale-snapshot** states for every screen.
7. **Overall design language** — typography, spacing, density, motion, dark/light.

> Pair this document with [`openapi/keleustes.v1.yaml`](../../openapi/keleustes.v1.yaml)
> (field names, shapes, enums) and the live scaffold under `ui/` (the stubbed
> screens these specs map onto).
