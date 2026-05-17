# Graph Report - keleustes  (2026-05-16)

## Corpus Check
- 55 files · ~57,473 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 686 nodes · 879 edges · 69 communities (37 shown, 32 thin omitted)
- Extraction: 95% EXTRACTED · 5% INFERRED · 0% AMBIGUOUS · INFERRED: 48 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `ab72dda4`
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
- [[_COMMUNITY_Community 53|Community 53]]
- [[_COMMUNITY_Community 55|Community 55]]
- [[_COMMUNITY_Community 56|Community 56]]
- [[_COMMUNITY_Community 57|Community 57]]
- [[_COMMUNITY_Community 58|Community 58]]
- [[_COMMUNITY_Community 59|Community 59]]
- [[_COMMUNITY_Community 60|Community 60]]
- [[_COMMUNITY_Community 61|Community 61]]
- [[_COMMUNITY_Community 64|Community 64]]
- [[_COMMUNITY_Community 65|Community 65]]
- [[_COMMUNITY_Community 66|Community 66]]
- [[_COMMUNITY_Community 67|Community 67]]
- [[_COMMUNITY_Community 68|Community 68]]

## God Nodes (most connected - your core abstractions)
1. `Keleustes Controller Manager` - 18 edges
2. `Extensibility and Plugin Surfaces Plan` - 14 edges
3. `Application CRD` - 13 edges
4. `Decision` - 12 edges
5. `Promotion CRD` - 12 edges
6. `Repository Guidelines` - 10 edges
7. `NewRootCommand()` - 10 edges
8. `Promotion CustomResourceDefinition` - 10 edges
9. `Accepted Status Condition` - 10 edges
10. `keleustesctl Root Command` - 9 edges

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

## Communities (69 total, 32 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.06
Nodes (54): Application CustomResourceDefinition, Application Spec, Approval CustomResourceDefinition, Cell CustomResourceDefinition, Controller Generated CRD Bases Directory, CRD Kustomization, Default Kustomization Overlay, skaphos-keleustes-system Namespace (+46 more)

### Community 1 - "Community 1"
Cohesion: 0.05
Nodes (50): DuckDB over Parquet Derived Query Layer, Gateway API Tiered Exposure, Hub and Regional Agent Model, NATS JetStream Event Bus, NATS KV Hot Index Layer, NATS Leaf Transport, Content Addressed Object Storage, Regional Agent (+42 more)

### Community 2 - "Community 2"
Cohesion: 0.07
Nodes (46): Application CRD, Application Deployment Strategy, Application Manifest Rendering, Application Topology, Approval CRD, Approval Decision, Cell CRD, LocalObjectReference (+38 more)

### Community 3 - "Community 3"
Cohesion: 0.07
Nodes (41): Application Reconciler, Approval Reconciler, Cell Reconciler, App Inspection Command, Promotion Blockers Command, Application Environment Diff Command, Application Environment Matrix Command, Shared Not Implemented Command Handler (+33 more)

### Community 4 - "Community 4"
Cohesion: 0.06
Nodes (34): 10. Open Questions for the Eventual ADR, 11. Phased Rollout, 12. What This Plan Replaces, 13. Next Steps, 1. Why This Matters Now, 2. What Is and Is Not a Plugin, 3.1 Notifier, 3.2 SignatureVerifier (+26 more)

### Community 5 - "Community 5"
Cohesion: 0.1
Nodes (20): 10. Schema versioning, 11. Multi-tenant scope enforcement, 1. Five extension surfaces, governed by CRDs, 2. Mechanism: webhook-only in v1alpha1, 3. Built-in implementations live in-tree, 4. Shared envelope schema, 5. Authentication: mTLS preferred, JWT fallback, 6. Default failure semantics, per surface (+12 more)

### Community 6 - "Community 6"
Cohesion: 0.11
Nodes (7): PromotionChange, PromotionFrom, PromotionMode, PromotionPhase, PromotionSpec, PromotionStatus, PromotionTo

### Community 7 - "Community 7"
Cohesion: 0.14
Nodes (5): Application, ApplicationStatus, ApprovalStatus, DeploymentTargetSpec, PromotionList

### Community 8 - "Community 8"
Cohesion: 0.2
Nodes (17): marshaller-api Kustomize Manifest Deployment, marshaller-api Application, guest-prod Cell, aks-prod-guest-westus2-001 Cluster, prod-guest-westus2 DeploymentTarget, prod Environment, marshaller-api 1.8.2 to prod wave4 Promotion, ServiceNow CRQ123456 Change (+9 more)

### Community 9 - "Community 9"
Cohesion: 0.12
Nodes (4): PromotionPolicy, PromotionPolicyList, PromotionPolicySpec, PromotionPolicyStatus

### Community 10 - "Community 10"
Cohesion: 0.12
Nodes (4): Cell, CellList, CellSpec, CellStatus

### Community 11 - "Community 11"
Cohesion: 0.12
Nodes (4): DeploymentTarget, DeploymentTargetCluster, DeploymentTargetList, DeploymentTargetStatus

### Community 12 - "Community 12"
Cohesion: 0.12
Nodes (4): FreezeWindow, FreezeWindowList, FreezeWindowSpec, FreezeWindowStatus

### Community 13 - "Community 13"
Cohesion: 0.12
Nodes (6): ApplicationDeployment, ApplicationDeploymentStrategy, ApplicationManifest, ApplicationManifestType, ApplicationSpec, ApplicationTopology

### Community 14 - "Community 14"
Cohesion: 0.13
Nodes (5): SyncRun, SyncRunList, SyncRunPhase, SyncRunSpec, SyncRunStatus

### Community 15 - "Community 15"
Cohesion: 0.13
Nodes (4): EnvironmentChangeControl, EnvironmentList, EnvironmentSpec, EnvironmentStatus

### Community 16 - "Community 16"
Cohesion: 0.13
Nodes (5): ReleaseArtifact, ReleaseArtifactType, ReleaseProvenance, ReleaseSpec, ReleaseStatus

### Community 17 - "Community 17"
Cohesion: 0.14
Nodes (4): Approval, ApprovalDecision, ApprovalList, ApprovalSpec

### Community 18 - "Community 18"
Cohesion: 0.22
Nodes (10): newAppCommand(), newBlockersCommand(), newDiffCommand(), newMatrixCommand(), newPromoteCommand(), newReleaseCommand(), newRollbackCommand(), newVersionCommand() (+2 more)

### Community 19 - "Community 19"
Cohesion: 0.15
Nodes (4): HealthCheck, HealthCheckSpec, HealthCheckStatus, HealthState

### Community 20 - "Community 20"
Cohesion: 0.17
Nodes (3): Promotion, SourceList, SourceObservedRevision

### Community 21 - "Community 21"
Cohesion: 0.17
Nodes (4): SourceSpec, SourceStatus, SourceType, SourceVerify

### Community 22 - "Community 22"
Cohesion: 0.17
Nodes (3): Deployment, DeploymentSpec, DeploymentStatus

### Community 23 - "Community 23"
Cohesion: 0.17
Nodes (3): SyncPlan, SyncPlanSpec, SyncPlanStatus

### Community 24 - "Community 24"
Cohesion: 0.18
Nodes (10): Build, Test, and Development Commands, Coding Standards, Commit & Pull Request Guidelines, Engineering Guardrails, Knowledge Graph (`graphify`), Project Structure & Module Organization, Repository Guidelines, Testing Guidelines (+2 more)

### Community 25 - "Community 25"
Cohesion: 0.2
Nodes (3): LocalObjectReference, OwnerInfo, SecretKeyRef

### Community 26 - "Community 26"
Cohesion: 0.22
Nodes (9): kube-state-metrics CRD State Metrics, Grafana Dashboard ConfigMaps, Observability Stack Plan, OpenTelemetry Dual Export, Agent PodMonitor Bundle, PrometheusRule Alerts, Runbook-backed Alerts, ServiceMonitor Bundle (+1 more)

### Community 27 - "Community 27"
Cohesion: 0.29
Nodes (7): Keleustes Conceptual Components, Keleustes Core API Concepts, Keleustes Deployment Models, GitOps Delivery Control Plane Purpose, Keleustes Identity, Keleustes Collision Review, Keleustes Name

### Community 28 - "Community 28"
Cohesion: 0.29
Nodes (7): ApprovalPolicy CRD, CRD-based RBAC Model, Default-deny RBAC, IdentityProvider CRD, Project CRD, Role CRD, RoleBinding CRD

### Community 29 - "Community 29"
Cohesion: 0.33
Nodes (6): leader-election-role, leader-election-rolebinding, controller-manager ServiceAccount, manager-rolebinding, Keleustes CRD Permissions, manager-role

### Community 55 - "Community 55"
Cohesion: 0.67
Nodes (3): Internal CLI Root Command, keleustesctl Command Entrypoint, PROPOSAL Section 17 CLI Surface

### Community 56 - "Community 56"
Cohesion: 0.67
Nodes (3): Extension Surfaces, Policy Engine, Policy Model

### Community 57 - "Community 57"
Cohesion: 0.67
Nodes (3): Go Mod Download Bootstrap, Pinned Developer Tooling Module, Task Tool Invocation Workflow

## Knowledge Gaps
- **158 isolated node(s):** `Context`, `1. Five extension surfaces, governed by CRDs`, `2. Mechanism: webhook-only in v1alpha1`, `3. Built-in implementations live in-tree`, `code:yaml (spec:)` (+153 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **32 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Source Reconciler` connect `Community 3` to `Community 2`?**
  _High betweenness centrality (0.008) - this node is a cross-community bridge._
- **Why does `Source Engine` connect `Community 2` to `Community 3`?**
  _High betweenness centrality (0.008) - this node is a cross-community bridge._
- **What connects `Context`, `1. Five extension surfaces, governed by CRDs`, `2. Mechanism: webhook-only in v1alpha1` to the rest of the system?**
  _158 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.06 - nodes in this community are weakly interconnected._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.05 - nodes in this community are weakly interconnected._
- **Should `Community 2` be split into smaller, more focused modules?**
  _Cohesion score 0.07 - nodes in this community are weakly interconnected._
- **Should `Community 3` be split into smaller, more focused modules?**
  _Cohesion score 0.07 - nodes in this community are weakly interconnected._