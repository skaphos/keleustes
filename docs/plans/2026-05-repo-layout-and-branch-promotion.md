<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Repo Layout Golden Paths + the `Addon` CRD

- **Status:** Draft — 2026-05-18
- **Linear:** SKA-434 (this plan). Spawns SKA-435 (Addon CRD scaffold + reconciler stub) plus the follow-ups in §15. Consumed by SKA-352 (Promotion Engine), SKA-353 (Git Mutation Engine), SKA-356 (Source Engine real implementation), and SKA-432 (value-change Promotion — location resolution depends on layout).
- **Promotes into:** a future ADR co-located with ADR 0003 (Git invariant) once two of the three golden paths have shipped end-to-end against a real customer.
- **Related:** ADR 0003 (Git invariant), ADR 0006 (Engine boundaries — controller↔engine split this plan respects), [`2026-05-value-change-promotion.md`](./2026-05-value-change-promotion.md) §5 (Application.spec.values.schema[] — *will be amended* by this plan), [`2026-05-git-mutation-attribution.md`](./2026-05-git-mutation-attribution.md) (orthogonal — same auth modes apply per layout), [`2026-05-extensibility-plugin-surfaces.md`](./2026-05-extensibility-plugin-surfaces.md) §3 (`Notifier` etc. plugin patterns Addon's lifecycle hooks mirror).
- **Out of scope:** per-engine *implementation* tickets (each Source Engine / Promotion Engine / Mutation Engine per-layout implementation gets its own SKA-### later); SAML / RBAC binding between platform-team-Project and Addon resources (SKA-323 RBAC CRD shapes plan covers this).

## 1. Purpose and Scope

Keleustes customers run wildly different Git repository shapes. A Promotion in one shop is a PR that merges `staging` into `prod`; in another it's a value edit in `env/prod/replicas.yaml`; in a third it's a library-version bump in an integration repo. The Promotion Engine, the Source Engine, the Git Mutation Engine, and the value-change Promotion machinery (SKA-432) all need to know *which* shape a given Application uses — otherwise every customer's Keleustes setup looks like a custom implementation.

Supporting every conceivable repository shape is a maintenance disaster (per-customer doc surfaces; per-customer support paths; per-customer test matrix). Supporting only one shape is operationally hostile (customers with valid alternative setups can't adopt Keleustes without restructuring repos).

**The compromise**: ship two or three *golden paths* — first-class, documented, opinionated repository layouts that match the shapes Keleustes customers most commonly run — and one *custom escape hatch* for anyone outside those layouts. Customers on a golden path get full default behavior with minimal config; customers on `custom` carry their own setup burden but still get the Promotion Engine / Source Engine / Mutation Engine primitives.

This plan also pins **`Addon` as a first-class CRD** distinct from `Application`. Platform-operator content (cert-manager, Argo Rollouts, Crossplane providers, Prometheus Operator, internal addons) has different lifecycle, criticality, and rollback semantics from application content. Trying to model addons as Applications with different annotations produces a CRD where every operator concern lives in undocumented metadata and every engine has to special-case it. Promoting it to its own CRD is the cleaner answer.

**In scope:**

- Three golden paths — branch-per-env (default for `Application`), flat-with-env-dirs + waves (Application opt-in), library+integration two-repo (default for `Addon`).
- The `custom` escape hatch and the primitive contract it commits to.
- The `repoLayout` field — `Application.spec.repoLayout` and `Addon.spec.integration.layout`.
- Per-layout `MutatingGit`-phase semantics (what the Git Mutation Engine actually does for each).
- Per-layout Source Engine watch semantics.
- Per-layout value-change Promotion location resolution (addendum to SKA-432 §5).
- Multi-environment scope semantics per layout (atomicity, ordering).
- Drift and merge-back semantics for branch-tracking layouts.
- Migration paths between layouts.
- The `Addon` CRD shape, reconciler responsibilities, upgrade gates, compatibility matrix, consumer enumeration.
- Audit verb routing per layout.
- Phased rollout across MVP 2 / MVP 3.

**Out of scope:**

- Implementation tickets per engine — filed separately.
- Multi-region branch federation (golden-path-1 across geographically split clusters with their own forks of the env branches). Future.
- Cross-repository ACL / Project boundary enforcement (SKA-323 RBAC plan).
- Promotion across non-Git Source types (OCI registries, Helm repositories). The same shape would adapt — promotion = tag bump rather than branch merge — but the wire mechanics are different enough to warrant their own plan.

## 2. Why Three (Maximum) Golden Paths

Customer repository shapes cluster into three real profiles:

| Profile | Repo shape | Why they want this |
| --- | --- | --- |
| Enterprise GitOps shop | Branch-per-env, single repo | PR-driven change management; auditors expect "approved by reviewer X, merged at time Y"; clean per-env git log |
| Argo CD migrant | Flat repo, env directories, ApplicationSet-style wave ordering | Familiar from prior tooling; scales well past ~50 environments where one branch per env becomes awkward; per-tenant cells map naturally to per-tenant directories |
| Platform team shipping addons | Library repo (tag-tracked) + integration repo (branch-tracked) | Library team's release cadence and integration team's deployment cadence are different concerns; tagging the library decouples the two |

A fourth pattern shows up occasionally — per-release branches, where every Release gets its own `release/1.8.2` branch — but it's rare enough that Skaphos doesn't carry the support burden. Customers who want it land on `custom`.

**Why three and not five.** Each golden path is roughly one page of recipe (manifest examples) + one substantial section of engine semantics + one dashboard slice + one alert bundle. Three is the upper bound where the team can ship coherent docs and the operators can confidently pick one without analysis paralysis. Five is where the docs become "here are five things that sort of look the same and you have to read the differences carefully to know which one to pick" — that's the failure mode this plan is structured to avoid.

**Why `Addon` gets its own primitive rather than being an Application variant.** The library+integration layout *could* be modeled as an Application with two Source refs. But that hides the criticality difference: an Addon failure cascades to N consumer Applications; an Addon upgrade needs explicit gating against consumer state; the platform team wants its own RBAC boundary; consumers expect explicit version compatibility ranges; rollback semantics differ (rolling back an addon may break consumers). All of these are first-class concerns the schema needs to surface — exactly the case where a separate CRD beats overloaded annotations. See §11 for the full Addon design.

## 3. Golden Path 1: Branch-per-env with Merge Promotion

**Default for `Application`.** Single repo, one branch per environment, promotion = PR that merges the upstream env's branch into the downstream env's branch.

### 3.1 Repository structure

```text
my-app/                              # one repo
├── README.md
├── kustomization.yaml               # base (or values.yaml for Helm)
├── manifests/
│   ├── deployment.yaml
│   ├── service.yaml
│   └── ...
└── env/                             # env-specific overlays IF NEEDED
    └── prod-only-overrides.yaml     # rare; most env diffs live in branch diffs
```

Branches:

```text
main      ← dev environment lives here (or a `dev` branch alternate)
staging   ← merges from main, sits between dev and prod
prod      ← merges from staging
```

Each environment's currently-deployed manifest set is the head of its branch. The git log on the `prod` branch is the literal deploy history.

### 3.2 Source CRD configuration

One `Source` per environment, watching that env's branch:

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: Source
metadata: { name: checkout-api-prod }
spec:
  type: git
  git:
    url: https://github.com/customer/checkout-api.git
    ref: prod                        # branch name
  verify: { ... }                    # SignatureVerifier per ADR 0001 plugin model
```

Repeat for `staging`, `main`/`dev`. The Application then references the appropriate Source per environment via the `DeploymentTarget` → `Environment` chain.

(The alternative — one Source watching multiple branches — keeps the CRD count down but complicates revision tracking. Skaphos recommends one Source per env at MVP 2; multi-branch Source is a future option.)

### 3.3 Promotion semantics

A Promotion CR in this layout has its `MutatingGit` phase produce a PR with one of two mechanics:

**Merge promotion (default):**

```text
PR head: upstream-env branch (e.g., `staging`)
PR base: downstream-env branch (e.g., `prod`)
Title:   keleustes: promote checkout-api staging → prod (release 1.8.2)
Body:    contains the Promotion CR ref, requesting actor, audit-event link
```

The PR includes every commit on staging since the last successful prod promotion. Operators reviewing the PR see the actual diff that's about to apply to prod.

**Cherry-pick promotion (opt-in, for trunk-based shops):**

```yaml
spec:
  repoLayout:
    pattern: branch-per-env-merge
    config:
      promotionStrategy: cherry-pick    # default is: merge
```

Cherry-pick takes the Release's tagged commit on `main` (or wherever the Release was cut) and cherry-picks it onto the target env's branch as a new commit. Useful when the dev branch contains many in-flight changes and the Promotion is supposed to promote only one Release.

### 3.4 Drift and merge-back

The classic branch-per-env failure mode: a prod hotfix lands on the `prod` branch via break-glass (SKA-360), but the same fix never propagates back to `staging` and `main`. The next planned promotion accidentally reverts the hotfix.

**Mitigation, MVP 2:** the Source Engine watches every env's branch independently and compares heads. When `prod`'s head contains commits that `staging`'s head doesn't, the Application reconciler surfaces a `MergeBackPending` status condition with the list of drifted commits.

**Mitigation, MVP 3:** an opt-in `mergeBackPolicy` field automates a merge-back PR — when prod gets new commits not in staging, Keleustes opens a PR that merges prod back into staging.

### 3.5 Multi-environment scope

A Promotion targeting `scope.environments: [staging, prod]` opens **two PRs in sequence**, not one PR touching both branches. The Promotion's phase machine waits for the staging PR to merge before opening the prod PR. Atomicity is "all or nothing across the whole sequence" — if the prod PR fails to merge after staging's already landed, the operator either rolls back via counter-Promotion (SKA-432 §11.3) or cancels.

This is materially different from Golden Path 2's atomicity (one PR per Promotion regardless of env count). Document explicitly in the operator-facing docs so customers don't expect Golden Path 2's atomicity here.

## 4. Golden Path 2: Flat Repo + Directory-per-Env with Waves

**Application opt-in.** Single repo, single branch (`main`), one directory per environment, promotion = file edits in the next env's directory.

### 4.1 Repository structure

```text
my-config/                           # one repo, one branch
├── README.md
├── base/                            # shared manifests
│   ├── deployment.yaml
│   └── ...
└── env/
    ├── dev/
    │   ├── kustomization.yaml       # references base/
    │   ├── replicas.yaml            # dev-specific values
    │   └── image.yaml
    ├── staging/
    │   ├── kustomization.yaml
    │   ├── replicas.yaml
    │   └── image.yaml
    └── prod/
        ├── kustomization.yaml
        ├── replicas.yaml
        └── image.yaml
```

Branches: just `main` (and feature branches when developers prepare changes; those merge to main via standard PR flow).

### 4.2 Source CRD configuration

One `Source` for the whole repo:

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: Source
metadata: { name: my-config-main }
spec:
  type: git
  git:
    url: https://github.com/customer/my-config.git
    ref: main
```

The Application references this single Source and uses `spec.repoLayout.config.envPathPattern: env/${env}` to resolve which subdirectory matches which environment.

### 4.3 Promotion semantics

`MutatingGit` produces a PR that copies the relevant files from the upstream env's directory into the downstream env's directory. For Argo-CD-style "we just want the same kustomization" the entire upstream env directory replaces the downstream env directory. For "promote this specific Release" it's a more surgical diff.

```yaml
spec:
  repoLayout:
    pattern: flat-with-env-dirs
    config:
      envPathPattern: env/${env}
      promotionStrategy: replace-directory
      # OR
      # promotionStrategy: selective-fields     # for value-change Promotions only
```

The PR's diff is the explicit set of files that change — easy to review per env.

### 4.4 Wave ordering

Argo CD's ApplicationSet pattern uses sync waves to order Application reconciliation. Keleustes adopts the same idea but exposes it explicitly on the Environment CR:

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: Environment
metadata: { name: prod }
spec:
  order: 30                          # wave 30: prod
  # wave 10: dev, wave 20: staging
```

A Promotion with `scope.environments: [staging, prod]` evaluates the envs in wave order; the resulting PR touches `env/staging/` then `env/prod/` in one commit, but the *sync* (downstream apply) happens in wave order.

### 4.5 Multi-environment scope

Unlike Golden Path 1, one Promotion produces **one PR** that touches every target env's directory. Atomicity is at the PR level — merge means all env directories land at once; the downstream Sync Engine then applies them in wave order.

This is the right model for Argo-CD-migrant shops where the existing mental model is "one PR, multiple env overlays change together."

### 4.6 Value-change Promotion (SKA-432) under this layout

`Application.spec.values.schema[].location.file` resolves via the `envPathPattern`:

```yaml
spec:
  repoLayout:
    pattern: flat-with-env-dirs
    config:
      envPathPattern: env/${env}
  values:
    schema:
      - logicalPath: spec.replicas
        location:
          file: ${envPath}/replicas.yaml      # ${envPath} expands per envPathPattern
          jsonPointer: /replicas
        ...
```

The Promotion Engine resolves `${envPath}` per target environment when computing the JSON Pointer location. For a Promotion targeting `[staging, prod]`, the engine produces one PR with diffs at both `env/staging/replicas.yaml` and `env/prod/replicas.yaml`.

## 5. Golden Path 3: Library + Integration Two-Repo

**Default for `Addon`.** Two repos: a **library** repo (the addon's canonical content; tag-tracked) + an **integration** repo (the customer's per-env application of the library; branch-per-env).

### 5.1 Repository structure

**Library repo** (`cert-manager-platform/`):

```text
cert-manager-platform/               # one repo
├── README.md
├── version.yaml                     # canonical version, semver
├── deploy/
│   ├── cert-manager.yaml            # the rendered or templated manifests
│   ├── crds.yaml
│   └── ...
└── examples/
    └── basic-issuer.yaml
```

Tagged: `v1.18.0`, `v1.18.1`, ..., `v1.19.0`. The tag is the unit of release. The library team controls when a tag exists.

**Integration repo** (`platform-integration/`):

```text
platform-integration/                # one repo, branch-per-env (Golden Path 1)
├── README.md
└── addons/
    └── cert-manager/
        ├── version.yaml             # references library tag: { ref: "v1.18.2" }
        ├── patches/
        │   ├── issuer-customer-ca.yaml
        │   └── ...
        └── kustomization.yaml       # base = library@tag, patches applied
```

Branches: `staging`, `prod`, etc. (Golden Path 1's branch model applied to the integration repo).

### 5.2 Source CRDs

Two — one for the library, one for the integration:

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: Source
metadata: { name: cert-manager-upstream }
spec:
  type: git
  git:
    url: https://github.com/customer/cert-manager-platform.git
    ref: v1.18.2                     # specific tag — fixed
    refType: tag
---
apiVersion: keleustes.skaphos.io/v1alpha1
kind: Source
metadata: { name: platform-integration-prod }
spec:
  type: git
  git:
    url: https://github.com/customer/platform-integration.git
    ref: prod                        # branch
    refType: branch
```

The library Source is *fixed* at a tag — the Addon's `spec.library.versionConstraint` is what drives the tag selection. The integration Source is *branch-tracked* per Golden Path 1.

### 5.3 The `Addon` CRD (preview — full spec in §11)

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: Addon
metadata: { name: cert-manager }
spec:
  library:
    sourceRef: { name: cert-manager-upstream }
    versionConstraint: ">=1.18, <2.0"
    artifactSubpath: deploy/

  integration:
    sourceRef: { name: platform-integration-prod }
    layout:
      pattern: branch-per-env-merge
      branchPattern: ${env}
      addonPathPattern: addons/${addon}/

  upgradeGates:
    - noOpenPromotionsForApplications: ["*"]
    - noUnhealthySyncRunsForApplications: ["*"]
    - kubernetesVersionRange: "1.31-1.36"

  compatibility:
    kubernetesMinor: ["1.31", "1.32", "1.33", "1.34", "1.35", "1.36"]
```

### 5.4 Promotion semantics for Addons

Two distinct Promotion content types:

**Library version bump** (most common):

A Promotion's content is "use library tag `v1.18.3` instead of `v1.18.2`." `MutatingGit` produces a PR against the integration repo's target branch that updates `addons/cert-manager/version.yaml` to the new tag. The library repo is *not* touched (it's tag-tracked; customer doesn't write to it).

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: Promotion
spec:
  application: cert-manager           # Addon name in spec.application
  addonVersion:                       # NEW field for Addon Promotions
    from: v1.18.2
    to: v1.18.3
  scope:
    environments: [prod]
```

**Integration-side patch** (occasional):

A Promotion's content is "edit the customer-side patches in `addons/cert-manager/patches/`." This is a value-change Promotion (SKA-432) targeting paths inside the integration repo's addon directory. Mechanics identical to Golden Path 1's value-change semantics, scoped to the addon's path prefix.

### 5.5 Upgrade gates (default-on)

Before the `MutatingGit` phase opens the version-bump PR, the Promotion Engine evaluates the Addon's `spec.upgradeGates`. Default-on gates:

- `noOpenPromotionsForApplications`: no consumer Application has an in-flight Promotion (`Proposed | Evaluating | Blocked | Approved | MutatingGit | WaitingForMerge | WaitingForSync | Verifying` phase). Wildcard `"*"` means all consumers; can be narrowed to specific Applications.
- `noUnhealthySyncRunsForApplications`: no consumer Application's most recent SyncRun is in `Failed` phase. Forces operators to stabilize consumers before changing the addon they depend on.
- `kubernetesVersionRange`: the cluster's K8s minor is within the new addon version's compatibility range. Computed against `spec.compatibility.kubernetesMinor`.

Gates that block transition the Promotion to `Blocked: AddonUpgradeGated` with the failing gate names recorded in conditions. Operators clear the gates (wait for consumer Promotions to finish, fix failing SyncRuns, etc.) and the Promotion proceeds automatically.

**Opt-out per Addon:**

```yaml
spec:
  upgradeGates: []                  # explicit empty — no gates
```

The empty-list opt-out is deliberate: a default-empty list would let customers accidentally ship Addons without gates. Empty list requires explicit action and documents the choice.

### 5.6 Per-K8s-version compatibility

`spec.compatibility.kubernetesMinor` lists supported Kubernetes minors. The Addon's reconciler compares against the cluster's discovered server version (via the Discovery API). Promotions are gated against incompatible upgrades:

- Cluster is `1.33`, Addon `v1.18.2` declares `["1.31","1.32","1.33"]`, customer attempts Promotion to `v1.19.0` declaring `["1.32","1.33","1.34","1.35"]` → allowed (cluster's `1.33` is in both).
- Same scenario but new version declares `["1.34","1.35","1.36"]` → gated. Customer must upgrade the cluster first.

This catches a real failure mode: customer upgrades the Addon to a version that doesn't support their K8s minor, sync succeeds in dry-run, real apply fails with subtle CRD-schema-incompatibility errors, recovery requires rolling back the version bump.

### 5.7 Consumer enumeration via annotations (scalable)

The Application's reconciler writes per-Addon annotations on every reconcile:

```yaml
metadata:
  annotations:
    keleustes.skaphos.io/depends-on-addon: cert-manager,prometheus-operator
```

The Addon reconciler watches Applications through a *namespaced* informer and filters or field-indexes them in controller logic based on the `keleustes.skaphos.io/depends-on-addon` annotation, NOT via a Kubernetes label selector and NOT via a cluster-wide "scan every Application" pass. The Application controller emits an event (or writes to a small `addon-consumer-index` NATS KV bucket — SKA-365) that the Addon reconciler subscribes to.

This avoids the O(N) cluster-wide enumeration cost an "Addon enumerates all Applications" naive design would incur at 10K-Application scale while keeping the mechanism consistent with the annotation projection.

```yaml
status:
  consumers:
    - { application: checkout-api, namespace: payments, dependsOn: cert-manager }
    - { application: billing-api, namespace: payments, dependsOn: cert-manager }
```

The annotation key is fixed (`keleustes.skaphos.io/depends-on-addon`); the value is a comma-separated list of Addon names the Application declares. Application's `spec.addonRefs[]` is the source-of-truth field; the annotation is the projection.

## 6. The `repoLayout` Configuration Field

Same shape applies to `Application.spec.repoLayout` and `Addon.spec.integration.layout`:

```yaml
repoLayout:
  pattern: branch-per-env-merge      # | flat-with-env-dirs | custom
  config:
    # Pattern-specific. The pattern's reconciler validates that
    # required keys are present and rejects unknown keys.
    branchPattern: ${env}            # branch-per-env-merge
    promotionStrategy: merge         # branch-per-env-merge: merge | cherry-pick
    envPathPattern: env/${env}       # flat-with-env-dirs
    addonPathPattern: addons/${addon}/   # used by library+integration's integration side
    waveOrdering: by-environment-order   # flat-with-env-dirs: by-environment-order | by-application-label
```

Validation: a CEL `XValidation` rule on the field enforces `pattern` is in the closed enum and that pattern-specific required keys are set (e.g., `branch-per-env-merge` requires `branchPattern`; `flat-with-env-dirs` requires `envPathPattern`).

### 6.1 Defaults

When `repoLayout` is unset on `Application`, defaults to:

```yaml
repoLayout:
  pattern: branch-per-env-merge
  config:
    branchPattern: ${env}
    promotionStrategy: merge
```

When `Addon.spec.integration.layout` is unset, same default. Addons whose integration repos use flat-with-env-dirs explicitly set the pattern.

## 7. The `custom` Escape Hatch

`pattern: custom` accepts an opaque `config` blob — the Promotion Engine, Source Engine, and Mutation Engine consult per-engine plugin code that customers register for their setup. **Skaphos ships no built-in `custom` handler.** Customers who pick `custom` are explicitly opting out of golden-path support; they implement the per-engine plugins themselves (probably via `Notifier`-style webhook handlers — TBD when the first `custom` customer materializes).

This is deliberate: shipping a generic `custom` mode would create a fourth implicit pattern. Forcing customers to register plugins keeps the support burden clear.

## 8. Per-Layout Value-Change Promotion Resolution (Amends SKA-432 §5)

The `Application.spec.values.schema[].location` shape from SKA-432 §5.2 gets a `${envPath}` token plus an optional `branch` field:

```yaml
spec:
  repoLayout: { pattern: branch-per-env-merge, config: { branchPattern: ${env} } }
  values:
    schema:
      - logicalPath: spec.replicas
        location:
          branch: ${env}                   # set automatically from repoLayout for golden paths
          file: replicas.yaml              # repo-relative; ${envPath} would be empty here
          jsonPointer: /replicas

# Versus flat-with-env-dirs:
spec:
  repoLayout: { pattern: flat-with-env-dirs, config: { envPathPattern: env/${env} } }
  values:
    schema:
      - logicalPath: spec.replicas
        location:
          # branch defaults to main when pattern is flat-with-env-dirs
          file: ${envPath}/replicas.yaml
          jsonPointer: /replicas
```

The Promotion Engine resolves `branch` (defaulting to layout-derived value) and `${envPath}` (substituted per target environment) when building the `MutationRequest`. The Git Mutation Engine sees fully-resolved values; it doesn't need to know about layouts.

For Addon value changes (the integration-side patches):

```yaml
spec:
  integration:
    layout:
      pattern: branch-per-env-merge
      branchPattern: ${env}
      addonPathPattern: addons/${addon}/
  patches:
    schema:
      - logicalPath: spec.duration
        location:
          branch: ${env}
          file: ${addonPath}patches/duration.yaml
          jsonPointer: /duration
```

`${addonPath}` substitutes the Addon's path within the integration repo (per the layout's `addonPathPattern`).

The Promotion CR doesn't know about layouts; the Promotion Engine resolves them at evaluation time. Customers see one consistent value-change interface regardless of repo shape.

## 9. Multi-Environment Scope per Layout

| Layout | One Promotion targeting `[staging, prod]` produces | Atomicity |
| --- | --- | --- |
| `branch-per-env-merge` | **Two PRs in sequence** (staging first, then prod once staging merges) | All-or-nothing across the sequence; prod PR not opened until staging merges |
| `flat-with-env-dirs` | **One PR** touching both env directories | All-or-nothing at the PR level |
| `library+integration` (Addon) | Depends on integration layout (typically branch-per-env-merge); same as Golden Path 1 | Same as integration layout |
| `custom` | Customer-defined | Customer-defined |

This is documented in the operator-facing UI/CLI — when a Promotion is queued against `[staging, prod]` on a branch-per-env Application, the UI shows "Promotion will open 2 PRs in sequence." Operators making the mental shift between Argo-CD-style (one PR) and Keleustes branch-per-env need this visible.

## 10. Migration Paths Between Layouts

Customers do change layouts. Real cases:

- Argo-CD-migrant moves from `flat-with-env-dirs` to `branch-per-env-merge` once they're comfortable with the branch-driven mental model.
- Single-team shop adopts addons and adds an `Addon` for cert-manager alongside the existing Application setup — the Application layout doesn't change; new resource type.
- Platform team that started with `branch-per-env-merge` for everything realizes addons need the two-repo split and migrates the addon-side to `library+integration`.

For each migration, this plan documents a recipe (filed as follow-ups under §15). The recipes describe:

1. The end-state target repo layout.
2. Application/Addon CRD edits required.
3. A sequence of customer-side Git operations (which branches/directories to create, when to flip the `repoLayout.pattern` field).
4. The expected Source revision behavior during the cutover (often: a single SyncRun applies the new layout's render output, with the old layout's resources pruned per inventory diff).

No live-state migration (per ADR 0003 Git invariant — the customer makes a Git PR that captures the layout shift; Keleustes follows).

## 11. The `Addon` CRD (Full Spec)

### 11.1 Schema

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: Addon
metadata:
  name: cert-manager
spec:
  # Library half: source of the addon's canonical content.
  library:
    sourceRef:
      name: cert-manager-upstream                # cluster-scoped Source CRD
    versionConstraint: ">=1.18, <2.0"            # semver range
    artifactSubpath: deploy/                     # path inside the library repo
    # Optional: override what the library exposes as the addon's name in
    # the integration repo. Defaults to metadata.name.
    integrationName: cert-manager

  # Integration half: where customer-side patches + env config live.
  integration:
    sourceRef:
      name: platform-integration                 # branch-per-env Source
    layout:
      pattern: branch-per-env-merge              # | flat-with-env-dirs | custom
      branchPattern: ${env}
      addonPathPattern: addons/${addon}/         # ${addon} = spec.library.integrationName
      promotionStrategy: merge

  # Default-on; explicit `[]` to opt out.
  upgradeGates:
    - noOpenPromotionsForApplications: ["*"]
    - noUnhealthySyncRunsForApplications: ["*"]
    - kubernetesVersionRange: "1.31-1.36"

  # Kubernetes minor versions this addon's currently-allowed library
  # versions support. Populated by the operator publisher OR by a
  # Skaphos-curated registry per SKA-431. Engine refuses Promotions
  # whose target version's compatibility list doesn't include the
  # cluster's minor.
  compatibility:
    kubernetesMinor: ["1.31", "1.32", "1.33", "1.34", "1.35", "1.36"]

  # Optional: per-environment overrides for the addon's behavior.
  # Mirrors Application's notion of per-environment overlays but
  # scoped to the addon's content.
  environments:
    - name: staging
      libraryVersion: "1.19.0"                   # canary in staging only
    # prod inherits spec.library.versionConstraint

status:
  installedVersion: "1.18.2"                     # what's actually live per env
  availableUpdate: "1.19.0"                      # surfaced from Source revisions
  consumers:                                     # populated from Application annotations
    - { application: checkout-api, namespace: payments, dependsOn: cert-manager }
    - { application: billing-api, namespace: payments, dependsOn: cert-manager }
  conditions:
    - type: Accepted
      status: "True"
      reason: ScaffoldReconciler
    - type: UpgradeGated                         # set when a Promotion is pending but gates block
      status: "False"
      reason: ConsumersBusy
      message: "1 consumer Application has an open Promotion"
```

### 11.2 Reconciler responsibilities

`internal/controller/addon_controller.go` does:

1. **Source revision watch.** Resolves `spec.library.sourceRef` and listens for new revisions. Populates `status.availableUpdate` when the library Source reports a new tag matching the version constraint.

2. **Consumer enumeration.** Watches Applications matching the annotation `keleustes.skaphos.io/depends-on-addon: <name>` via a namespace-scoped informer. Populates `status.consumers[]` on event.

3. **Upgrade gate evaluation.** When a Promotion targeting this Addon enters `Evaluating`, the Promotion Engine consults the Addon's `spec.upgradeGates` and `status.consumers[]` to decide whether to allow the transition to `Approved`. The Addon reconciler doesn't drive the Promotion machine — it provides the state.

4. **K8s compatibility check.** Compares the cluster's Discovery API server version against the target library version's `compatibility.kubernetesMinor` list. Failures surface as Promotion-blocking conditions.

5. **Status condition maintenance.** Sets `Accepted` (schema valid) + `UpgradeGated` (gates block currently-pending Promotion) + `Healthy` (last successful Promotion's resulting Application states are all `Healthy`).

The reconciler is a coordinator — the actual Git mutation is the Git Mutation Engine's job; the actual sync is the Sync Engine's job; this reconciler tracks state and gates.

### 11.3 RBAC

Cluster-scoped CRD (an Addon spans environments by design). RBAC:

- **`platform-admin` Role** (provided by SKA-323 RBAC scaffold or customer-defined): can create/update/delete Addons, can manage Promotions targeting Addons.
- **`platform-viewer` Role**: read-only on Addons.
- **`application-developer` Role**: cannot create Promotions targeting Addons. Can read Addons (to see what they depend on). Cannot edit Addon CRs.

`+kubebuilder:rbac` markers on the reconciler grant standard operator permissions; the customer-side Roles above are customer-managed.

### 11.4 `Application.spec.addonRefs[]`

New field on `Application`:

```yaml
spec:
  addonRefs:
    - name: cert-manager
    - name: prometheus-operator
```

When the Application reconciler reconciles, it writes the annotation `keleustes.skaphos.io/depends-on-addon: cert-manager,prometheus-operator` to the Application's metadata. The Addon reconciler watches that annotation. This is the scalable enumeration mechanism from §5.7.

Validation: each `addonRefs[].name` must resolve to an existing Addon CR. Validation webhook rejects creates/updates with dangling refs.

### 11.5 keleustesctl coverage

`keleustesctl get addons` and `keleustesctl describe addon/<name>` follow the same shape as the existing read commands (SKA-334). Columns: Name, InstalledVersion, AvailableUpdate, ConsumerCount, KubernetesCompatibility, Status.

## 12. Audit Verb Routing per Layout

Audit events emitted by Promotions inherit the layout via a payload field:

```jsonc
{
  "action": { "verb": "promote", ... },
  "payload": {
    "@type": "promote.v1",
    "repoLayout": "branch-per-env-merge",    // NEW field — payload addition per audit-event-schema §5.1
    "promotionScope": "application",         // | "addon"
    ...
  }
}
```

`repoLayout` and `promotionScope` are additive per audit-event-schema §5.1; no `schemaVersion` bump. SIEM consumers can route Addon-scoped Promotion events to a separate alert queue from Application-scoped ones — the "more critical" framing manifests in the audit pipeline.

Addon-specific verbs (new):

| Verb | Payload | Notes |
| --- | --- | --- |
| `addon-installed` | `addon.installed.v1` | First successful Promotion of an Addon to an environment |
| `addon-upgraded` | `addon.upgraded.v1` | Subsequent successful Promotion changing the library version |
| `addon-upgrade-gated` | `addon.upgrade.gated.v1` | Promotion blocked on upgrade gates; carries the failing gate names |
| `addon-rolled-back` | `addon.rolled-back.v1` | Counter-Promotion applied |
| `addon-incompatible` | `addon.incompatible.v1` | K8s compatibility check failed during Promotion evaluation |

These join the audit-event-schema §13 registry alongside the existing Promotion verbs (§13.3) and Git-mutation verbs (§13.6).

## 13. Compliance with Prior Decisions

| Decision | This plan honors it by |
| --- | --- |
| ADR 0003 (Git invariant) | Every layout's Promotion still produces a Git PR; no live-cluster mutation path. Layouts choose *how* Git is shaped, never bypass it. |
| ADR 0004 (CRD-based RBAC) | Addon RBAC follows the same Role/RoleBinding pattern; `application-developer` cannot manipulate Addons by default. |
| ADR 0006 (Engine boundaries) | No new engine. The existing Source/Promotion/Mutation/Sync engines handle layouts via configuration; `internal/controller/addon_controller.go` is a new reconciler in `internal/controller/` (the thin-coordinator package) per ADR 0006 §2. |
| ADR 0007 (Hard-fork gitops-engine) | Unchanged. Per-layout Render still happens through gitops-engine's render primitives. |
| Audit-event-schema plan §13 | Five new verbs added under §13 + two payload-field amendments (`repoLayout`, `promotionScope`). All additive per §5.1; no `schemaVersion` bump. |
| Value-change Promotion (SKA-432) §5 | This plan amends §5.2's `location` shape with `branch` + `${envPath}` / `${addonPath}` token resolution; the Promotion CR's user-facing surface is unchanged (still `valueChange.path / from / to / reason`). |
| Operator CRD integration (SKA-431) | Addons consume `HealthAssessor` + `DiffNormalizer` from SKA-431 verbatim; the addon's per-K8s-version compatibility matrix in §11.1 is the data the Skaphos-curated bundle ships per SKA-431 §4.4. |
| Git mutation attribution (SKA-433) | Orthogonal. Every layout's `MutatingGit` phase honors the IdentityProvider's `gitMutationAttribution.mode` — branch merges and file edits both go through the same auth contract. |
| Engine-boundaries plan §2.6 (cross-Application dependency) | Addon's consumer enumeration (§5.7) is a higher-fidelity expression of the same dependency model — Applications declare `addonRefs[]`; Addons enumerate consumers. |

## 14. Open Questions

1. **Source CRD per-branch vs. multi-branch.** Golden Path 1 specifies one Source per environment branch. The alternative (one Source watching multiple branches) reduces CRD count but complicates revision tracking — each `Source.status.observedRevision` becomes per-branch. Decision deferred to SKA-356 (Source Engine real implementation); document the chosen shape in this plan's §3.2 when the engine ships.

2. **Auto-merge-back PRs.** §3.4 leaves the merge-back automation as MVP 3 opt-in. Worth a customer-input round before MVP 3 commits the design — small shops may want it on by default; large shops with strict release cadence may never want it.

3. **Library-side patches.** Some customers want to patch the library repo (e.g., a security fix backported before the upstream library team releases). The plan's library-tag-tracked model says no — patches go in the integration repo. Confirm this is acceptable; if not, the library Source needs a per-customer fork model (similar to ADR 0007's friendly-fork pattern for gitops-engine itself).

4. **`addonRefs[]` deletion semantics.** When an Application removes an Addon from its `spec.addonRefs[]`, what happens? Options:
   - Annotation is rewritten; Addon's `status.consumers[]` shrinks; nothing else changes.
   - As above, plus emit a `consumer-detached` audit event so the platform team notices.
   - As above, plus refuse the Application update if the Application's manifests still reference Addon-owned CRs (would prevent silent breakage).
   Probably option 2 for MVP 2; option 3 is more correct but needs render-output introspection.

5. **Promotion targeting `[*]` environments.** A "deploy this addon to every environment" Promotion is operationally common. Golden Path 1 produces N sequential PRs; Golden Path 2 produces 1 PR with N directory diffs; Library+Integration produces N PRs per env on the integration repo. Document the per-layout expansion of `[*]` clearly in the operator docs.

6. **Wave override at Promotion time.** Currently wave ordering comes from `Environment.spec.order`. A customer's release-runbook might want "promote to prod and DR-prod in parallel even though they're at the same wave." Probably needs a `Promotion.spec.parallelEnvironments` field — defer to MVP 3.

7. **Addon Promotions with mixed library+integration changes.** Customer wants to bump the library version AND change a customer-side patch in one Promotion. Currently `addonVersion` and `valueChanges` are separate top-level fields on Promotion; the Mutation Engine handles them as one PR. Worth confirming the per-Engine implementation can compose these cleanly — open until SKA-352 lands.

8. **Project-level layout defaults.** Customers running many Applications of the same shape will want to pin the layout once at the Project level rather than per-Application. Probably a `Project.spec.defaultRepoLayout` field — defer to MVP 2 follow-up; mention in DECISIONS.md when added.

## 15. Concrete Follow-ups

1. **SKA-### — Add `Application.spec.repoLayout`** (MVP 2). Schema-only ticket. CEL XValidation rule enforcing the closed `pattern` enum + pattern-specific required keys. Defaults documented in §6.1.

2. **SKA-435 — Addon CRD scaffold + reconciler stub** (MVP 2, filed). See ticket description for the full schema + reconciler shape per this plan's §11.

3. **SKA-### — Application.spec.addonRefs[] field** (MVP 2). Schema extension + validation webhook for dangling refs.

4. **SKA-### — Application reconciler writes the consumer annotation** (MVP 2). On every reconcile, materialize `metadata.annotations["keleustes.skaphos.io/depends-on-addon"]` from `spec.addonRefs[]`. Idempotent.

5. **SKA-### — Per-layout Source Engine watch logic** (MVP 2; depends on SKA-356). Source Engine handles single-branch, per-env-branch, and tag-tracked Sources uniformly via a `RefType` enum.

6. **SKA-### — Per-layout Promotion Engine `MutatingGit` content** (MVP 2; depends on SKA-352 + SKA-353). The Promotion Engine dispatches to per-pattern handlers when computing the `MutationRequest`.

7. **SKA-### — Drift detection for branch-per-env** (MVP 2). Source Engine emits a `MergeBackPending` event when prod branch contains commits staging doesn't.

8. **SKA-### — Skaphos-curated layout templates** (MVP 2). `skaphos/keleustes-curated` ships starter manifests for each Golden Path so customers can copy-paste-edit rather than writing CRDs from scratch.

9. **SKA-### — Addon-specific audit verbs registered** (MVP 2). Update audit-event-schema plan §13 with `addon-installed/upgraded/upgrade-gated/rolled-back/incompatible` + the `repoLayout`/`promotionScope` payload fields.

10. **SKA-### — Per-layout migration recipes** (MVP 3). The cookbook for moving an Application between Golden Paths 1 ↔ 2, and adopting Library+Integration alongside an existing Application setup.

11. **SKA-### — `mergeBackPolicy` automation** (MVP 3). Auto-PR'd merge-back when branch-per-env detects drift.

12. **`Project.spec.defaultRepoLayout`** (MVP 3 — see §14 open question 8). Project-level default that Applications inherit.

13. **PROPOSAL.md cross-link**. PROPOSAL §11 (Sync engine) and §14 (Git mutation) currently describe a single-layout model. Drop a `> See SKA-434` marker during the next refresh.

14. **DECISIONS.md row**. Add to "Plans that have not yet stabilized"; promotes to an active interim contract once SKA-435 lands the Addon scaffold and the first per-layout Source Engine + Promotion Engine implementation ships.

## 16. Phased Rollout

| MVP | Work in this plan's scope |
| --- | --- |
| **MVP 2** | `Application.spec.repoLayout` field lands (defaults to `branch-per-env-merge`). `Addon` CRD scaffold (SKA-435). `Application.spec.addonRefs[]` + consumer annotation. Source Engine handles all three Golden Paths' watch semantics. Promotion Engine + Git Mutation Engine handle Golden Path 1 + Golden Path 3's `MutatingGit` content (Golden Path 2 follows in MVP 3 to keep MVP 2 scope tractable). Default-on upgrade gates implemented. Audit verbs registered. keleustesctl `get/describe addons` works. |
| **MVP 3** | Golden Path 2 (flat-with-env-dirs + waves) fully implemented. Migration recipes between layouts. `mergeBackPolicy` automation. `Project.spec.defaultRepoLayout`. Skaphos-curated layout templates published. Auto-merge-back PRs. |
| **MVP 4** | `custom` plugin contract finalized (when first customer materializes). Per-environment Addon canary support (SKA-365 NATS KV + Promotion-time variant of `spec.environments`). |

---

**When this plan stabilizes** (after Golden Paths 1 and 3 have shipped end-to-end with at least one real customer running each, and the Addon CRD has been exercised by the first multi-Application platform setup), §1–§13 promote into a new ADR co-located with ADR 0003 (the Git invariant remains the umbrella under which all layouts live). The `custom` escape and the per-layout migration recipes stay in working material indefinitely.
