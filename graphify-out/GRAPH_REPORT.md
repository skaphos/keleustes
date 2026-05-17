# Graph Report - .  (2026-05-16)

## Corpus Check
- 88 files · ~55,481 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 638 nodes · 841 edges · 66 communities (34 shown, 32 thin omitted)
- Extraction: 94% EXTRACTED · 6% INFERRED · 0% AMBIGUOUS · INFERRED: 52 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_Generated CRD Manifests|Generated CRD Manifests]]
- [[_COMMUNITY_Distributed Runtime Architecture|Distributed Runtime Architecture]]
- [[_COMMUNITY_Core API Model|Core API Model]]
- [[_COMMUNITY_Controller CLI Surface|Controller CLI Surface]]
- [[_COMMUNITY_Plugin Extension Surfaces|Plugin Extension Surfaces]]
- [[_COMMUNITY_Promotion Type DeepCopy|Promotion Type DeepCopy]]
- [[_COMMUNITY_Application Type DeepCopy|Application Type DeepCopy]]
- [[_COMMUNITY_Approval Type DeepCopy|Approval Type DeepCopy]]
- [[_COMMUNITY_FreezeWindow Type DeepCopy|FreezeWindow Type DeepCopy]]
- [[_COMMUNITY_Cell Type DeepCopy|Cell Type DeepCopy]]
- [[_COMMUNITY_DeploymentTarget Type DeepCopy|DeploymentTarget Type DeepCopy]]
- [[_COMMUNITY_Repository Engineering Guardrails|Repository Engineering Guardrails]]
- [[_COMMUNITY_Release Type DeepCopy|Release Type DeepCopy]]
- [[_COMMUNITY_SyncRun Type DeepCopy|SyncRun Type DeepCopy]]
- [[_COMMUNITY_Core Resource DeepCopy|Core Resource DeepCopy]]
- [[_COMMUNITY_CLI Command Constructors|CLI Command Constructors]]
- [[_COMMUNITY_HealthCheck Type DeepCopy|HealthCheck Type DeepCopy]]
- [[_COMMUNITY_SyncPlan Type DeepCopy|SyncPlan Type DeepCopy]]
- [[_COMMUNITY_Source Type DeepCopy|Source Type DeepCopy]]
- [[_COMMUNITY_PromotionPolicy Type DeepCopy|PromotionPolicy Type DeepCopy]]
- [[_COMMUNITY_Deployment Type DeepCopy|Deployment Type DeepCopy]]
- [[_COMMUNITY_Source Resource DeepCopy|Source Resource DeepCopy]]
- [[_COMMUNITY_Environment Type DeepCopy|Environment Type DeepCopy]]
- [[_COMMUNITY_Shared API References|Shared API References]]
- [[_COMMUNITY_Observability Stack Plan|Observability Stack Plan]]
- [[_COMMUNITY_Naming and Concepts|Naming and Concepts]]
- [[_COMMUNITY_RBAC Audit Model|RBAC Audit Model]]
- [[_COMMUNITY_Manager RBAC Resources|Manager RBAC Resources]]
- [[_COMMUNITY_ReleaseList DeepCopy|ReleaseList DeepCopy]]
- [[_COMMUNITY_SyncPlan DeepCopy|SyncPlan DeepCopy]]
- [[_COMMUNITY_Release DeepCopy|Release DeepCopy]]
- [[_COMMUNITY_PromotionPolicyList DeepCopy|PromotionPolicyList DeepCopy]]
- [[_COMMUNITY_ApplicationList DeepCopy|ApplicationList DeepCopy]]
- [[_COMMUNITY_HealthCheckList DeepCopy|HealthCheckList DeepCopy]]
- [[_COMMUNITY_PromotionList DeepCopy|PromotionList DeepCopy]]
- [[_COMMUNITY_Environment DeepCopy|Environment DeepCopy]]
- [[_COMMUNITY_Promotion DeepCopy|Promotion DeepCopy]]
- [[_COMMUNITY_Application Reconciler|Application Reconciler]]
- [[_COMMUNITY_Deployment Reconciler|Deployment Reconciler]]
- [[_COMMUNITY_Approval Reconciler|Approval Reconciler]]
- [[_COMMUNITY_Source Reconciler|Source Reconciler]]
- [[_COMMUNITY_Promotion Reconciler|Promotion Reconciler]]
- [[_COMMUNITY_Cell Reconciler|Cell Reconciler]]
- [[_COMMUNITY_SyncPlan Reconciler|SyncPlan Reconciler]]
- [[_COMMUNITY_SyncRun Reconciler|SyncRun Reconciler]]
- [[_COMMUNITY_Release Reconciler|Release Reconciler]]
- [[_COMMUNITY_DeploymentTarget Reconciler|DeploymentTarget Reconciler]]
- [[_COMMUNITY_PromotionPolicy Reconciler|PromotionPolicy Reconciler]]
- [[_COMMUNITY_Environment Reconciler|Environment Reconciler]]
- [[_COMMUNITY_FreezeWindow Reconciler|FreezeWindow Reconciler]]
- [[_COMMUNITY_HealthCheck Reconciler|HealthCheck Reconciler]]
- [[_COMMUNITY_API Group Registration|API Group Registration]]
- [[_COMMUNITY_CLI Entrypoint|CLI Entrypoint]]
- [[_COMMUNITY_Pinned Tooling Workflow|Pinned Tooling Workflow]]
- [[_COMMUNITY_Controller Test Cases|Controller Test Cases]]
- [[_COMMUNITY_Contribution Rules|Contribution Rules]]
- [[_COMMUNITY_Initial MVP Scope|Initial MVP Scope]]
- [[_COMMUNITY_External GitOps References|External GitOps References]]
- [[_COMMUNITY_Webhook Plugin Model|Webhook Plugin Model]]
- [[_COMMUNITY_Keleustes Worker|Keleustes Worker]]
- [[_COMMUNITY_Skaphos Tool Relationship|Skaphos Tool Relationship]]
- [[_COMMUNITY_SPDX Header Template|SPDX Header Template]]

## God Nodes (most connected - your core abstractions)
1. `Keleustes Controller Manager` - 18 edges
2. `Application CRD` - 13 edges
3. `Promotion CRD` - 12 edges
4. `NewRootCommand()` - 10 edges
5. `Repository Guidelines` - 10 edges
6. `Promotion CustomResourceDefinition` - 10 edges
7. `Accepted Status Condition` - 10 edges
8. `keleustesctl Root Command` - 9 edges
9. `Release CRD` - 8 edges
10. `DeploymentTarget CRD` - 8 edges

## Surprising Connections (you probably didn't know these)
- `Promotion Git Mutation Mode` --semantically_similar_to--> `Application Deploys Mutate Git`  [INFERRED] [semantically similar]
  api/v1alpha1/promotion_types.go → AGENTS.md
- `Application Reconciler Wiring` --implements--> `Application CustomResourceDefinition`  [INFERRED]
  cmd/manager/main.go → config/crd/bases/keleustes.skaphos.io_applications.yaml
- `Release Reconciler Wiring` --implements--> `Release CustomResourceDefinition`  [INFERRED]
  cmd/manager/main.go → config/crd/bases/keleustes.skaphos.io_releases.yaml
- `Environment Reconciler Wiring` --implements--> `Environment CustomResourceDefinition`  [INFERRED]
  cmd/manager/main.go → config/crd/bases/keleustes.skaphos.io_environments.yaml
- `Cell Reconciler Wiring` --implements--> `Cell CustomResourceDefinition`  [INFERRED]
  cmd/manager/main.go → config/crd/bases/keleustes.skaphos.io_cells.yaml

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

## Communities (66 total, 32 thin omitted)

### Community 0 - "Generated CRD Manifests"
Cohesion: 0.06
Nodes (54): Application CustomResourceDefinition, Application Spec, Approval CustomResourceDefinition, Cell CustomResourceDefinition, Controller Generated CRD Bases Directory, CRD Kustomization, Default Kustomization Overlay, skaphos-keleustes-system Namespace (+46 more)

### Community 1 - "Distributed Runtime Architecture"
Cohesion: 0.05
Nodes (50): DuckDB over Parquet Derived Query Layer, Gateway API Tiered Exposure, Hub and Regional Agent Model, NATS JetStream Event Bus, NATS KV Hot Index Layer, NATS Leaf Transport, Content Addressed Object Storage, Regional Agent (+42 more)

### Community 2 - "Core API Model"
Cohesion: 0.08
Nodes (42): Application CRD, Application Deployment Strategy, Application Manifest Rendering, Application Topology, Approval CRD, Approval Decision, Cell CRD, LocalObjectReference (+34 more)

### Community 3 - "Controller CLI Surface"
Cohesion: 0.07
Nodes (41): Application Reconciler, Approval Reconciler, Cell Reconciler, App Inspection Command, Promotion Blockers Command, Application Environment Diff Command, Application Environment Matrix Command, Shared Not Implemented Command Handler (+33 more)

### Community 4 - "Plugin Extension Surfaces"
Cohesion: 0.11
Nodes (27): AuditDestination Plugin Surface, Plugin Failure Semantics, Notifier Plugin Surface, Extensibility Plugin Surfaces, PolicyGate Plugin Surface, Scanner Plugin Surface, SignatureVerifier Plugin Surface, Extension Surfaces (+19 more)

### Community 5 - "Promotion Type DeepCopy"
Cohesion: 0.11
Nodes (7): PromotionChange, PromotionFrom, PromotionMode, PromotionPhase, PromotionSpec, PromotionStatus, PromotionTo

### Community 6 - "Application Type DeepCopy"
Cohesion: 0.11
Nodes (7): ApplicationDeployment, ApplicationDeploymentStrategy, ApplicationManifest, ApplicationManifestType, ApplicationSpec, ApplicationStatus, ApplicationTopology

### Community 7 - "Approval Type DeepCopy"
Cohesion: 0.12
Nodes (5): Approval, ApprovalDecision, ApprovalList, ApprovalSpec, ApprovalStatus

### Community 8 - "FreezeWindow Type DeepCopy"
Cohesion: 0.12
Nodes (4): FreezeWindow, FreezeWindowList, FreezeWindowSpec, FreezeWindowStatus

### Community 9 - "Cell Type DeepCopy"
Cohesion: 0.12
Nodes (4): Cell, CellList, CellSpec, CellStatus

### Community 10 - "DeploymentTarget Type DeepCopy"
Cohesion: 0.12
Nodes (4): DeploymentTarget, DeploymentTargetCluster, DeploymentTargetList, DeploymentTargetStatus

### Community 11 - "Repository Engineering Guardrails"
Cohesion: 0.13
Nodes (16): Application Deploys Mutate Git, Idempotent Bounded Reconcilers, keleustesctl Operational Fallback, Kubebuilder, Operator SDK, Pinned Task Tooling, docs/PROPOSAL.md, Reconciler Stub Pattern (+8 more)

### Community 12 - "Release Type DeepCopy"
Cohesion: 0.13
Nodes (5): ReleaseArtifact, ReleaseArtifactType, ReleaseProvenance, ReleaseSpec, ReleaseStatus

### Community 13 - "SyncRun Type DeepCopy"
Cohesion: 0.13
Nodes (5): SyncRun, SyncRunList, SyncRunPhase, SyncRunSpec, SyncRunStatus

### Community 14 - "Core Resource DeepCopy"
Cohesion: 0.17
Nodes (4): Application, Deployment, DeploymentTargetSpec, EnvironmentStatus

### Community 15 - "CLI Command Constructors"
Cohesion: 0.22
Nodes (10): newAppCommand(), newBlockersCommand(), newDiffCommand(), newMatrixCommand(), newPromoteCommand(), newReleaseCommand(), newRollbackCommand(), newVersionCommand() (+2 more)

### Community 16 - "HealthCheck Type DeepCopy"
Cohesion: 0.15
Nodes (4): HealthCheck, HealthCheckSpec, HealthCheckStatus, HealthState

### Community 17 - "SyncPlan Type DeepCopy"
Cohesion: 0.17
Nodes (3): SyncPlanList, SyncPlanSpec, SyncPlanStatus

### Community 18 - "Source Type DeepCopy"
Cohesion: 0.17
Nodes (4): SourceSpec, SourceStatus, SourceType, SourceVerify

### Community 19 - "PromotionPolicy Type DeepCopy"
Cohesion: 0.17
Nodes (3): PromotionPolicy, PromotionPolicySpec, PromotionPolicyStatus

### Community 20 - "Deployment Type DeepCopy"
Cohesion: 0.17
Nodes (3): DeploymentList, DeploymentSpec, DeploymentStatus

### Community 21 - "Source Resource DeepCopy"
Cohesion: 0.17
Nodes (3): Source, SourceList, SourceObservedRevision

### Community 22 - "Environment Type DeepCopy"
Cohesion: 0.17
Nodes (3): EnvironmentChangeControl, EnvironmentList, EnvironmentSpec

### Community 23 - "Shared API References"
Cohesion: 0.2
Nodes (3): LocalObjectReference, OwnerInfo, SecretKeyRef

### Community 24 - "Observability Stack Plan"
Cohesion: 0.22
Nodes (9): kube-state-metrics CRD State Metrics, Grafana Dashboard ConfigMaps, Observability Stack Plan, OpenTelemetry Dual Export, Agent PodMonitor Bundle, PrometheusRule Alerts, Runbook-backed Alerts, ServiceMonitor Bundle (+1 more)

### Community 25 - "Naming and Concepts"
Cohesion: 0.29
Nodes (7): Keleustes Conceptual Components, Keleustes Core API Concepts, Keleustes Deployment Models, GitOps Delivery Control Plane Purpose, Keleustes Identity, Keleustes Collision Review, Keleustes Name

### Community 26 - "RBAC Audit Model"
Cohesion: 0.29
Nodes (7): ApprovalPolicy CRD, CRD-based RBAC Model, Default-deny RBAC, IdentityProvider CRD, Project CRD, Role CRD, RoleBinding CRD

### Community 27 - "Manager RBAC Resources"
Cohesion: 0.33
Nodes (6): leader-election-role, leader-election-rolebinding, controller-manager ServiceAccount, manager-rolebinding, Keleustes CRD Permissions, manager-role

### Community 54 - "CLI Entrypoint"
Cohesion: 0.67
Nodes (3): Internal CLI Root Command, keleustesctl Command Entrypoint, PROPOSAL Section 17 CLI Surface

### Community 55 - "Pinned Tooling Workflow"
Cohesion: 0.67
Nodes (3): Go Mod Download Bootstrap, Pinned Developer Tooling Module, Task Tool Invocation Workflow

## Knowledge Gaps
- **115 isolated node(s):** `HealthState`, `PromotionMode`, `PromotionPhase`, `SourceType`, `ApprovalDecision` (+110 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **32 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Source Engine` connect `Core API Model` to `Controller CLI Surface`?**
  _High betweenness centrality (0.012) - this node is a cross-community bridge._
- **Why does `Source Reconciler` connect `Controller CLI Surface` to `Core API Model`?**
  _High betweenness centrality (0.012) - this node is a cross-community bridge._
- **Are the 9 inferred relationships involving `NewRootCommand()` (e.g. with `main()` and `newAppCommand()`) actually correct?**
  _`NewRootCommand()` has 9 INFERRED edges - model-reasoned connections that need verification._
- **What connects `HealthState`, `PromotionMode`, `PromotionPhase` to the rest of the system?**
  _115 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Generated CRD Manifests` be split into smaller, more focused modules?**
  _Cohesion score 0.06 - nodes in this community are weakly interconnected._
- **Should `Distributed Runtime Architecture` be split into smaller, more focused modules?**
  _Cohesion score 0.05 - nodes in this community are weakly interconnected._
- **Should `Core API Model` be split into smaller, more focused modules?**
  _Cohesion score 0.08 - nodes in this community are weakly interconnected._