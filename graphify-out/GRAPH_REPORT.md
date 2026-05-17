# Graph Report - keleustes  (2026-05-17)

## Corpus Check
- 65 files · ~72,225 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1098 nodes · 1270 edges · 96 communities (62 shown, 34 thin omitted)
- Extraction: 96% EXTRACTED · 4% INFERRED · 0% AMBIGUOUS · INFERRED: 49 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `4ba2ccde`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]
- [[_COMMUNITY_Community 3|Community 3]]
- [[_COMMUNITY_Community 4|Community 4]]
- [[_COMMUNITY_Community 5|Community 5]]
- [[_COMMUNITY_Community 6|Community 6]]
- [[_COMMUNITY_Community 7|Community 7]]
- [[_COMMUNITY_Community 8|Community 8]]
- [[_COMMUNITY_Community 9|Community 9]]
- [[_COMMUNITY_Community 10|Community 10]]
- [[_COMMUNITY_Community 11|Community 11]]
- [[_COMMUNITY_Community 12|Community 12]]
- [[_COMMUNITY_Community 13|Community 13]]
- [[_COMMUNITY_Community 14|Community 14]]
- [[_COMMUNITY_Community 15|Community 15]]
- [[_COMMUNITY_Community 16|Community 16]]
- [[_COMMUNITY_Community 17|Community 17]]
- [[_COMMUNITY_Community 18|Community 18]]
- [[_COMMUNITY_Community 19|Community 19]]
- [[_COMMUNITY_Community 20|Community 20]]
- [[_COMMUNITY_Community 21|Community 21]]
- [[_COMMUNITY_Community 22|Community 22]]
- [[_COMMUNITY_Community 23|Community 23]]
- [[_COMMUNITY_Community 24|Community 24]]
- [[_COMMUNITY_Community 25|Community 25]]
- [[_COMMUNITY_Community 26|Community 26]]
- [[_COMMUNITY_Community 27|Community 27]]
- [[_COMMUNITY_Community 28|Community 28]]
- [[_COMMUNITY_Community 29|Community 29]]
- [[_COMMUNITY_Community 30|Community 30]]
- [[_COMMUNITY_Community 31|Community 31]]
- [[_COMMUNITY_Community 32|Community 32]]
- [[_COMMUNITY_Community 33|Community 33]]
- [[_COMMUNITY_Community 34|Community 34]]
- [[_COMMUNITY_Community 35|Community 35]]
- [[_COMMUNITY_Community 36|Community 36]]
- [[_COMMUNITY_Community 37|Community 37]]
- [[_COMMUNITY_Community 38|Community 38]]
- [[_COMMUNITY_Community 39|Community 39]]
- [[_COMMUNITY_Community 40|Community 40]]
- [[_COMMUNITY_Community 41|Community 41]]
- [[_COMMUNITY_Community 42|Community 42]]
- [[_COMMUNITY_Community 43|Community 43]]
- [[_COMMUNITY_Community 44|Community 44]]
- [[_COMMUNITY_Community 45|Community 45]]
- [[_COMMUNITY_Community 46|Community 46]]
- [[_COMMUNITY_Community 47|Community 47]]
- [[_COMMUNITY_Community 48|Community 48]]
- [[_COMMUNITY_Community 49|Community 49]]
- [[_COMMUNITY_Community 50|Community 50]]
- [[_COMMUNITY_Community 51|Community 51]]
- [[_COMMUNITY_Community 52|Community 52]]
- [[_COMMUNITY_Community 53|Community 53]]
- [[_COMMUNITY_Community 54|Community 54]]
- [[_COMMUNITY_Community 55|Community 55]]
- [[_COMMUNITY_Community 56|Community 56]]
- [[_COMMUNITY_Community 57|Community 57]]
- [[_COMMUNITY_Community 58|Community 58]]
- [[_COMMUNITY_Community 59|Community 59]]
- [[_COMMUNITY_Community 60|Community 60]]
- [[_COMMUNITY_Community 61|Community 61]]
- [[_COMMUNITY_Community 62|Community 62]]
- [[_COMMUNITY_Community 63|Community 63]]
- [[_COMMUNITY_Community 64|Community 64]]
- [[_COMMUNITY_Community 65|Community 65]]
- [[_COMMUNITY_Community 66|Community 66]]
- [[_COMMUNITY_Community 67|Community 67]]
- [[_COMMUNITY_Community 69|Community 69]]
- [[_COMMUNITY_Community 71|Community 71]]
- [[_COMMUNITY_Community 72|Community 72]]
- [[_COMMUNITY_Community 73|Community 73]]
- [[_COMMUNITY_Community 74|Community 74]]
- [[_COMMUNITY_Community 75|Community 75]]
- [[_COMMUNITY_Community 76|Community 76]]
- [[_COMMUNITY_Community 77|Community 77]]
- [[_COMMUNITY_Community 78|Community 78]]
- [[_COMMUNITY_Community 79|Community 79]]
- [[_COMMUNITY_Community 80|Community 80]]
- [[_COMMUNITY_Community 81|Community 81]]
- [[_COMMUNITY_Community 82|Community 82]]
- [[_COMMUNITY_Community 83|Community 83]]
- [[_COMMUNITY_Community 84|Community 84]]
- [[_COMMUNITY_Community 85|Community 85]]
- [[_COMMUNITY_Community 86|Community 86]]
- [[_COMMUNITY_Community 87|Community 87]]
- [[_COMMUNITY_Community 88|Community 88]]
- [[_COMMUNITY_Community 91|Community 91]]
- [[_COMMUNITY_Community 92|Community 92]]
- [[_COMMUNITY_Community 93|Community 93]]
- [[_COMMUNITY_Community 94|Community 94]]
- [[_COMMUNITY_Community 95|Community 95]]

## God Nodes (most connected - your core abstractions)
1. `Keleustes Proposal (Draft)` - 26 edges
2. `Keleustes Controller Manager` - 18 edges
3. `Observability Stack Plan` - 16 edges
4. `Distributed Runtime Architecture Plan` - 16 edges
5. `Extensibility and Plugin Surfaces Plan` - 15 edges
6. `Decision` - 15 edges
7. `Decision` - 14 edges
8. `Decision` - 14 edges
9. `RBAC, Audit, and the Git-Source-of-Truth Invariant` - 13 edges
10. `Application CRD` - 13 edges

## Surprising Connections (you probably didn't know these)
- `Application Reconciler Wiring` --implements--> `Application CustomResourceDefinition`  [INFERRED]
  cmd/manager/main.go → config/crd/bases/keleustes.skaphos.io_applications.yaml
- `Release Reconciler Wiring` --implements--> `Release CustomResourceDefinition`  [INFERRED]
  cmd/manager/main.go → config/crd/bases/keleustes.skaphos.io_releases.yaml
- `Environment Reconciler Wiring` --implements--> `Environment CustomResourceDefinition`  [INFERRED]
  cmd/manager/main.go → config/crd/bases/keleustes.skaphos.io_environments.yaml
- `Cell Reconciler Wiring` --implements--> `Cell CustomResourceDefinition`  [INFERRED]
  cmd/manager/main.go → config/crd/bases/keleustes.skaphos.io_cells.yaml
- `DeploymentTarget Reconciler Wiring` --implements--> `DeploymentTarget CustomResourceDefinition`  [INFERRED]
  cmd/manager/main.go → config/crd/bases/keleustes.skaphos.io_deploymenttargets.yaml

## Hyperedges (group relationships)
- **Core Delivery Abstractions** — application_application, source_source, release_release, deployment_deployment, promotion_promotion [EXTRACTED 1.00]
- **Topology Model** — environment_environment, cell_cell, deploymenttarget_deployment_target, application_topology [EXTRACTED 1.00]
- **Promotion Governance Flow** — promotion_promotion, promotionpolicy_promotion_policy, approval_approval, freezewindow_freeze_window, environment_change_control [INFERRED 0.86]
- **Keleustes API Group CRD Family** — applications_application_crd, sources_source_crd, releases_release_crd, environments_environment_crd, cells_cell_crd, deploymenttargets_deploymenttarget_crd, deployments_deployment_crd, promotions_promotion_crd, promotionpolicies_promotionpolicy_crd, approvals_approval_crd, freezewindows_freezewindow_crd, syncplans_syncplan_crd, syncruns_syncrun_crd, healthchecks_healthcheck_crd [EXTRACTED 1.00]
- **Controller Manager Reconciler Setup Set** — manager_controller_manager, manager_application_reconciler, manager_source_reconciler, manager_release_reconciler, manager_environment_reconciler, manager_cell_reconciler, manager_deploymenttarget_reconciler, manager_deployment_reconciler, manager_promotion_reconciler, manager_promotionpolicy_reconciler, manager_approval_reconciler, manager_freezewindow_reconciler, manager_syncplan_reconciler, manager_syncrun_reconciler, manager_healthcheck_reconciler [EXTRACTED 1.00]
- **Promotion Governance Model** — promotions_promotion_crd, promotionpolicies_promotionpolicy_crd, approvals_approval_crd, freezewindows_freezewindow_crd, environments_change_control, promotions_change_reference [INFERRED 0.86]
- **Sample Promotion Path** — keleustes_v1alpha1_application_marshaller_api_marshaller_api_application, keleustes_v1alpha1_release_marshaller_api_1_8_2_marshaller_api_release, keleustes_v1alpha1_promotion_marshaller_api_to_prod_marshaller_api_prod_promotion, keleustes_v1alpha1_environment_prod_prod_environment, keleustes_v1alpha1_cell_guest_prod_guest_prod_cell [EXTRACTED 1.00]
- **Proposal Engine Stack** — PROPOSAL_source_engine, PROPOSAL_sync_engine, PROPOSAL_promotion_engine, PROPOSAL_git_mutation_engine, PROPOSAL_policy_engine, PROPOSAL_health_engine, PROPOSAL_diff_engine [EXTRACTED 1.00]
- **Five Plugin Surfaces** — 2026_05_extensibility_plugin_surfaces_notifier, 2026_05_extensibility_plugin_surfaces_signatureverifier, 2026_05_extensibility_plugin_surfaces_scanner, 2026_05_extensibility_plugin_surfaces_policygate, 2026_05_extensibility_plugin_surfaces_auditdestination [EXTRACTED 1.00]
- **keleustesctl Operational Surface** — root_keleustesctl_root_command, commands_app_command, commands_matrix_command, commands_release_command, commands_promote_command, commands_diff_command, commands_blockers_command, commands_rollback_command, commands_version_command [EXTRACTED 1.00]
- **Scaffold Reconciler Status Contract** — doc_controller_scaffold_package, controller_observed_generation_status, controller_accepted_condition, controller_scaffold_reason [EXTRACTED 1.00]
- **Controller Scaffold Test Matrix** — controllers_scaffold_cases, controllers_shared_scaffold_assertion, suite_envtest_environment, suite_k8s_client [EXTRACTED 1.00]

## Communities (96 total, 34 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.06
Nodes (54): Application CustomResourceDefinition, Application Spec, Approval CustomResourceDefinition, Cell CustomResourceDefinition, Controller Generated CRD Bases Directory, CRD Kustomization, Default Kustomization Overlay, skaphos-keleustes-system Namespace (+46 more)

### Community 1 - "Community 1"
Cohesion: 0.05
Nodes (50): DuckDB over Parquet Derived Query Layer, Gateway API Tiered Exposure, Hub and Regional Agent Model, NATS JetStream Event Bus, NATS KV Hot Index Layer, NATS Leaf Transport, Content Addressed Object Storage, Regional Agent (+42 more)

### Community 2 - "Community 2"
Cohesion: 0.05
Nodes (42): 10. Key Interfaces & Contracts to Define Early, 11.1 Hard constraint: restorable from zero, no backups, 11.2 Storage tiers, 11.3 Consistency model, 11.4 Recovery from zero (target time budgets, indicative), 11.5 Horizontal Scaling and Sharding, 11. Data, Consistency, and Restorability, 12. Failure Modes & Partition Behavior (Critical) (+34 more)

### Community 3 - "Community 3"
Cohesion: 0.07
Nodes (43): Application CRD, Application Deployment Strategy, Application Manifest Rendering, Application Topology, Approval CRD, Approval Decision, Cell CRD, LocalObjectReference (+35 more)

### Community 4 - "Community 4"
Cohesion: 0.07
Nodes (41): Application Reconciler, Approval Reconciler, Cell Reconciler, App Inspection Command, Promotion Blockers Command, Application Environment Diff Command, Application Environment Matrix Command, Shared Not Implemented Command Handler (+33 more)

### Community 5 - "Community 5"
Cohesion: 0.06
Nodes (34): 10. Open Questions for the Eventual ADR, 11. Phased Rollout, 12. What This Plan Replaces, 13. Next Steps, 1. Why This Matters Now, 2. What Is and Is Not a Plugin, 3.1 Notifier, 3.2 SignatureVerifier (+26 more)

### Community 6 - "Community 6"
Cohesion: 0.06
Nodes (34): 1. Why This Matters Now, 2.5 Reuse: `gitops-engine`, 2.6 The Dependency Model (Cross-Application Ordering), 2. Named Engines (from PROPOSAL §9), 3. Proposed Internal Package Structure, 4. Engine Ownership & Dependency Rules, 5.1 Rendering & Manifest Technologies, 5.2 Git & Mutation Providers (+26 more)

### Community 7 - "Community 7"
Cohesion: 0.06
Nodes (33): 10. Tying Telemetry to Audit, 11. Bundling and Packaging, 12. Open Questions for the Eventual ADR, 13. Phased Rollout, 14. What This Plan Replaces / Refines, 15. Next Steps, 1. Why This Matters Now, 2. Scope (+25 more)

### Community 8 - "Community 8"
Cohesion: 0.06
Nodes (33): 10. Phased Rollout, 11. Open Decisions & Future ADRs, 12. References, 1. The Three Things This Plan Is About, 2. What Argo CD Got Right, and Where It Hurts at Scale, 3. The Git-Source-of-Truth Invariant (Hard Rule), 4.1 Identity sources, 4.2 Group claims (+25 more)

### Community 9 - "Community 9"
Cohesion: 0.09
Nodes (19): code:block1 (modules total    k8s.io modules), code:go (AnnotationSyncWave            = "argocd.argoproj.io/sync-wav), code:go (autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"), Concrete follow-ups, Decision: adoption strategy, F1. Repository archived; canonical import path changed, F2. k8s.io ≤ v0.34 ceiling (load-bearing), F3. Mandatory `replace` block (+11 more)

### Community 10 - "Community 10"
Cohesion: 0.09
Nodes (21): 10. Dependency pinning strategy, 11. Helm chart repository authentication in a distributed world, 1. Seven engines plus a shared `Render` package, 2026-05-17 — SKA-327 spike findings, 2. Package layout, 3. Engine ownership and dependency rules, 4. `gitops-engine` reuse — yes, with a containment rule, 5. Render technology stack — library-only (+13 more)

### Community 11 - "Community 11"
Cohesion: 0.09
Nodes (20): 10. Schema versioning, 11. Multi-tenant scope enforcement, 1. Five extension surfaces, governed by CRDs, 2. Mechanism: webhook-only in v1alpha1, 3. Built-in implementations live in-tree, 4. Shared envelope schema, 5. Authentication: mTLS preferred, JWT fallback, 6. Default failure semantics, per surface (+12 more)

### Community 12 - "Community 12"
Cohesion: 0.09
Nodes (21): 10. Horizontal scaling and sharding, 11. Render and Git mutation distribution, 12. Local autonomy for emergency operations, 13. DuckDB freshness model, 14. Reference SQL consumer, 1. Hub + regional-agent topology, 2. NATS JetStream is the canonical event/state bus, 3. No RDBMS on the critical path (+13 more)

### Community 13 - "Community 13"
Cohesion: 0.09
Nodes (21): 10. Default-deny, 11. Policy evaluator: custom-over-CRDs, not Casbin, 12. UI deep-linking with stable identifiers, 13. Multi-tenant alignment with ADR 0001, 1. Five RBAC CRDs in `keleustes.skaphos.io/v1alpha1`, 2. `Project` is the delegation boundary, 3. Layered with native Kubernetes RBAC, not replaced, 4. Action verbs (+13 more)

### Community 14 - "Community 14"
Cohesion: 0.1
Nodes (20): 10. Multi-region: regional scrape, federate aggregates, 11. Kubernetes Events: narrow by default, 12. Audit ≠ telemetry, 13. Bundle layout and gating, 1. Dual export is the default, 2. Prometheus Operator is the supported wiring path, 3. OpenTelemetry SDK with conventional wiring, 4. Logging: structured stdout by default, OTel logs opt-in (+12 more)

### Community 15 - "Community 15"
Cohesion: 0.13
Nodes (5): Application, ApplicationSpec, Promotion, PromotionList, ReleaseList

### Community 16 - "Community 16"
Cohesion: 0.1
Nodes (6): Source, SourceList, SourceSpec, SourceStatus, SourceType, SourceVerify

### Community 17 - "Community 17"
Cohesion: 0.11
Nodes (7): PromotionChange, PromotionFrom, PromotionMode, PromotionPhase, PromotionSpec, PromotionStatus, PromotionTo

### Community 18 - "Community 18"
Cohesion: 0.11
Nodes (5): DeploymentTarget, DeploymentTargetCluster, DeploymentTargetList, DeploymentTargetSpec, DeploymentTargetStatus

### Community 19 - "Community 19"
Cohesion: 0.12
Nodes (5): Approval, ApprovalDecision, ApprovalList, ApprovalSpec, ApprovalStatus

### Community 20 - "Community 20"
Cohesion: 0.12
Nodes (5): HealthCheck, HealthCheckList, HealthCheckSpec, HealthCheckStatus, HealthState

### Community 21 - "Community 21"
Cohesion: 0.12
Nodes (17): 8.1 `Application`, 8.2 `Source`, 8.3 `Environment`, 8.4 `Cell`, 8.5 `DeploymentTarget`, 8.6 `Release`, 8.7 `Promotion`, 8.8 `PromotionPolicy` (+9 more)

### Community 22 - "Community 22"
Cohesion: 0.2
Nodes (17): marshaller-api Kustomize Manifest Deployment, marshaller-api Application, guest-prod Cell, aks-prod-guest-westus2-001 Cluster, prod-guest-westus2 DeploymentTarget, prod Environment, marshaller-api 1.8.2 to prod wave4 Promotion, ServiceNow CRQ123456 Change (+9 more)

### Community 23 - "Community 23"
Cohesion: 0.12
Nodes (4): Cell, CellList, CellSpec, CellStatus

### Community 24 - "Community 24"
Cohesion: 0.12
Nodes (4): FreezeWindow, FreezeWindowList, FreezeWindowSpec, FreezeWindowStatus

### Community 25 - "Community 25"
Cohesion: 0.12
Nodes (4): PromotionPolicy, PromotionPolicyList, PromotionPolicySpec, PromotionPolicyStatus

### Community 26 - "Community 26"
Cohesion: 0.12
Nodes (4): Deployment, DeploymentList, DeploymentSpec, DeploymentStatus

### Community 27 - "Community 27"
Cohesion: 0.12
Nodes (4): ApplicationList, Environment, Release, SourceObservedRevision

### Community 28 - "Community 28"
Cohesion: 0.12
Nodes (4): SyncPlan, SyncPlanList, SyncPlanSpec, SyncPlanStatus

### Community 29 - "Community 29"
Cohesion: 0.12
Nodes (6): ApplicationDeployment, ApplicationDeploymentStrategy, ApplicationManifest, ApplicationManifestType, ApplicationStatus, ApplicationTopology

### Community 30 - "Community 30"
Cohesion: 0.13
Nodes (5): ReleaseArtifact, ReleaseArtifactType, ReleaseProvenance, ReleaseSpec, ReleaseStatus

### Community 31 - "Community 31"
Cohesion: 0.13
Nodes (5): SyncRun, SyncRunList, SyncRunPhase, SyncRunSpec, SyncRunStatus

### Community 32 - "Community 32"
Cohesion: 0.13
Nodes (4): EnvironmentChangeControl, EnvironmentList, EnvironmentSpec, EnvironmentStatus

### Community 33 - "Community 33"
Cohesion: 0.13
Nodes (14): Building, code:block1 (api/v1alpha1/          # CRD type definitions (Application, ), code:bash (cd tools && go mod tidy   # one-time), code:bash (go -C tools tool task manifests        # regenerate CRDs + R), Core CRDs (group `keleustes.skaphos.io`), Design principles, License, MVP roadmap (+6 more)

### Community 34 - "Community 34"
Cohesion: 0.22
Nodes (10): newAppCommand(), newBlockersCommand(), newDiffCommand(), newMatrixCommand(), newPromoteCommand(), newReleaseCommand(), newRollbackCommand(), newVersionCommand() (+2 more)

### Community 35 - "Community 35"
Cohesion: 0.14
Nodes (13): 11. Sync engine approach, 12. Health model, 13. Diff model, 14. Git mutation model, 15. Policy model, 21. Build strategy, 22. Design principles, 24. Relationship to other Skaphos tools (+5 more)

### Community 36 - "Community 36"
Cohesion: 0.14
Nodes (13): 1. The hard rule, 2. What the rule forbids, 3. What the rule guarantees, 4. Break-glass: the single sanctioned exception, 5. Enforcement is structural, not policy, 6. UI shape, 7. Render inputs and the invariant, ADR 0003 — Git-source-of-truth invariant (+5 more)

### Community 37 - "Community 37"
Cohesion: 0.17
Nodes (11): Boundaries, Collision review (2026-05-14), Conceptual components, Core API concepts, Deployment models, External references, Identity, Initial scope (+3 more)

### Community 38 - "Community 38"
Cohesion: 0.18
Nodes (10): Build, Test, and Development Commands, Coding Standards, Commit & Pull Request Guidelines, Engineering Guardrails, Knowledge Graph (`graphify`), Project Structure & Module Organization, Repository Guidelines, Testing Guidelines (+2 more)

### Community 39 - "Community 39"
Cohesion: 0.2
Nodes (3): LocalObjectReference, OwnerInfo, SecretKeyRef

### Community 40 - "Community 40"
Cohesion: 0.2
Nodes (9): Branching and Commits, code:bash (cd tools && go mod tidy), code:bash (go -C tools tool task --list), Coding Standards, Contributing to Keleustes, Development Setup, Pull Requests, Safety Expectations (+1 more)

### Community 41 - "Community 41"
Cohesion: 0.22
Nodes (9): kube-state-metrics CRD State Metrics, Grafana Dashboard ConfigMaps, Observability Stack Plan, OpenTelemetry Dual Export, Agent PodMonitor Bundle, PrometheusRule Alerts, Runbook-backed Alerts, ServiceMonitor Bundle (+1 more)

### Community 42 - "Community 42"
Cohesion: 0.25
Nodes (7): Current Plans, How to Use These Plans, Lifecycle, Naming Convention, Planning Documents, Purpose, Relationship to Other Documents

### Community 43 - "Community 43"
Cohesion: 0.33
Nodes (3): PhaseFromOperation(), TestPhaseFromOperation(), SyncRunPhase

### Community 44 - "Community 44"
Cohesion: 0.29
Nodes (7): Keleustes Conceptual Components, Keleustes Core API Concepts, Keleustes Deployment Models, GitOps Delivery Control Plane Purpose, Keleustes Identity, Keleustes Collision Review, Keleustes Name

### Community 45 - "Community 45"
Cohesion: 0.29
Nodes (7): ApprovalPolicy CRD, CRD-based RBAC Model, Default-deny RBAC, IdentityProvider CRD, Project CRD, Role CRD, RoleBinding CRD

### Community 46 - "Community 46"
Cohesion: 0.33
Nodes (5): Adopting an Apache-2.0 dep with its own NOTICE, Implementation, License attribution, Regenerate after dep changes, Why generate rather than scan at build time

### Community 47 - "Community 47"
Cohesion: 0.33
Nodes (6): 10.1 Management cluster mode, 10.2 Cluster-local mode, 10.3 Future federation, 10. Deployment model, code:text (skaphos-keleustes-system), code:text (Keleustes Hub)

### Community 48 - "Community 48"
Cohesion: 0.33
Nodes (6): 20. MVP roadmap, MVP 0: Read-only replacement UI, MVP 1: Keleustes-managed sync, MVP 2: Releases and promotions, MVP 3: Enterprise topology, MVP 4: Policy and audit

### Community 49 - "Community 49"
Cohesion: 0.33
Nodes (6): leader-election-role, leader-election-rolebinding, controller-manager ServiceAccount, manager-rolebinding, Keleustes CRD Permissions, manager-role

### Community 50 - "Community 50"
Cohesion: 0.4
Nodes (5): 6.1 Platform engineer, 6.2 Application developer, 6.3 Operations engineer, 6.4 Security and compliance, 6. Target users

### Community 51 - "Community 51"
Cohesion: 0.4
Nodes (4): Collision review (2026-05-14), Identity, Naming, Rationale

### Community 66 - "Community 66"
Cohesion: 0.5
Nodes (4): 9.1 Components, 9.2 Cross-cutting concerns: extension surfaces and observability, 9. Architecture, code:text (+-------------------------------+)

### Community 67 - "Community 67"
Cohesion: 0.5
Nodes (4): 19. Data model, code:text (Kubernetes CRDs = active desired/control state), code:text (keleustes.skaphos.io), code:text (skaphos-keleustes-system)

### Community 71 - "Community 71"
Cohesion: 0.67
Nodes (3): 3. Problem statement, code:text (Argo CD = sync + UI), code:text (What applications exist?)

### Community 72 - "Community 72"
Cohesion: 0.67
Nodes (3): Internal CLI Root Command, keleustesctl Command Entrypoint, PROPOSAL Section 17 CLI Surface

### Community 73 - "Community 73"
Cohesion: 0.67
Nodes (3): Extension Surfaces, Policy Engine, Policy Model

### Community 74 - "Community 74"
Cohesion: 0.67
Nodes (3): Go Mod Download Bootstrap, Pinned Developer Tooling Module, Task Tool Invocation Workflow

## Knowledge Gaps
- **447 isolated node(s):** `HealthState`, `PromotionMode`, `PromotionPhase`, `SourceType`, `ApprovalDecision` (+442 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **34 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Keleustes Proposal (Draft)` connect `Community 35` to `Community 66`, `Community 67`, `Community 71`, `Community 47`, `Community 80`, `Community 81`, `Community 82`, `Community 50`, `Community 83`, `Community 21`, `Community 84`, `Community 48`, `Community 79`?**
  _High betweenness centrality (0.006) - this node is a cross-community bridge._
- **Why does `8. Core concepts` connect `Community 21` to `Community 35`?**
  _High betweenness centrality (0.004) - this node is a cross-community bridge._
- **Why does `SyncRunStatus` connect `Community 31` to `Community 27`, `Community 15`?**
  _High betweenness centrality (0.003) - this node is a cross-community bridge._
- **What connects `HealthState`, `PromotionMode`, `PromotionPhase` to the rest of the system?**
  _447 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.06 - nodes in this community are weakly interconnected._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.05 - nodes in this community are weakly interconnected._
- **Should `Community 2` be split into smaller, more focused modules?**
  _Cohesion score 0.05 - nodes in this community are weakly interconnected._