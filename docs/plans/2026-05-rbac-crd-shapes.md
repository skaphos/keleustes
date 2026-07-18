<!--
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
-->

# RBAC CRD Shapes

- **Status:** Draft — 2026-05-17
- **Linear:** SKA-323. Blocks SKA-330 (MVP 0 IdentityProvider scaffold + single-IdP OIDC) and SKA-345 (MVP 1 Role/RoleBinding/Project CRDs + reconcilers).
- **Promotes into:** a future ADR co-located with ADR 0004. Until then, this is the authoritative schema reference for any code or sample CR that touches the five RBAC types.
- **Refines:** [`docs/plans/2026-05-rbac-audit-and-git-invariant.md`](./2026-05-rbac-audit-and-git-invariant.md) §5 (CRD sketches) — turns ASCII bullets into concrete Go shapes with kubebuilder markers, validation webhook outlines, status conditions, and sample CRs.
- **Related:** [ADR 0004](../adr/0004-crd-based-rbac.md) (the decision this concretizes), [ADR 0001](../adr/0001-plugin-extension-model.md) (`PolicyGate` plugins layer above RBAC), [ADR 0003](../adr/0003-git-source-of-truth-invariant.md) (RBAC lives in Git like everything else), [ADR 0005](../adr/0005-distributed-runtime.md) §168 (`Agent` is a CR — §12 below explains why it does not join the RBAC alphabet), [SKA-322 Audit Schema](./2026-05-audit-event-schema.md) §13.2 (RBAC-related audit verbs).

## 1. Purpose and Scope

ADR 0004 accepted the five RBAC CRDs (`IdentityProvider`, `Role`,
`RoleBinding`, `Project`, `ApprovalPolicy`) as the model. This plan
is the layer below: concrete OpenAPI v3 schemas via kubebuilder
markers, sample CRs, validation webhook contracts, and status
conditions.

**In scope:**

- Go types under `api/v1alpha1/` for the five RBAC CRDs, with
  kubebuilder validation markers tight enough that `make manifests`
  produces an enforceable OpenAPI v3 schema.
- The `Scope` micro-shape used by `RoleBinding` and several plugin
  surfaces — declared once, reused everywhere.
- Validation webhook outlines (admission rules that cannot be
  expressed by CRD schema alone).
- Status condition taxonomy (which conditions every reconciler must
  publish).
- Sample CRs (one per CRD) demonstrating the common shape.
- The "Agent CR is its own thing, not an RBAC CRD" decision (§12).

**Out of scope:**

- Reconciler implementation (MVP 1 — SKA-345).
- The custom policy evaluator (ADR 0004 §11; MVP 1).
- The Promotion-state-machine enforcement of `ApprovalPolicy` (MVP
  2 — separate ticket).
- Native Kubernetes RBAC `+kubebuilder:rbac` markers on the
  operator itself (those live in `config/rbac/` and are unaffected
  by this ADR per ADR 0004 §3).

## 2. Shared Building Blocks

Several CRDs reference the same primitive shapes. Declared once in
`api/v1alpha1/rbac_common.go` and reused everywhere:

```go
// Scope identifies what a RoleBinding (or other scoped reference)
// applies to. Exactly one of the One-Of fields must be set; the
// validation webhook enforces this because kubebuilder CRD schemas
// cannot express "exactly one of."
type Scope struct {
    // Cluster, when true, scopes the binding cluster-wide. Reserved
    // for operator-internal bindings and rare admin paths.
    // +optional
    Cluster bool `json:"cluster,omitempty"`

    // Project names the Project this scope is contained by.
    // +optional
    // +kubebuilder:validation:MaxLength=253
    Project string `json:"project,omitempty"`

    // Application is a fully-qualified namespace/name reference.
    // +optional
    // +kubebuilder:validation:MaxLength=509
    // +kubebuilder:validation:Pattern=`^[a-z0-9.-]+/[a-z0-9-]+$`
    Application string `json:"application,omitempty"`

    // Environment is the name of an Environment CR; sub-scopes
    // (e.g., specific Cells within the Environment) are expressed
    // via Selector.
    // +optional
    // +kubebuilder:validation:MaxLength=253
    Environment string `json:"environment,omitempty"`

    // Selector is a Kubernetes label selector evaluated over the
    // resource kind named in the parent RoleBinding's Role. Useful
    // for "every Application labeled team=payments and tier=prod."
    // +optional
    Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// Subject identifies a principal that a RoleBinding grants
// permissions to.
type Subject struct {
    // Kind is the principal type. ServiceAccount and User refer to
    // Kubernetes-native concepts; Group refers to a normalized
    // Keleustes group name (per IdentityProvider claim mapping).
    // Agent refers to an NKey-authenticated agent identity.
    // +kubebuilder:validation:Enum=Group;User;ServiceAccount;Agent
    Kind string `json:"kind"`

    // Name is the principal identifier within its Kind:
    //   Group           -> canonical group name (e.g., "platform-engineering")
    //   User            -> normalized subject (typically email)
    //   ServiceAccount  -> "<namespace>/<name>"
    //   Agent           -> "<target>.<instance>"
    // +kubebuilder:validation:MinLength=1
    // +kubebuilder:validation:MaxLength=253
    Name string `json:"name"`

    // IdentityProviderRef restricts the subject to a specific
    // IdentityProvider — useful when two providers can both emit
    // a "platform-engineering" group and only one should be trusted
    // for this binding. Empty matches any.
    // +optional
    // +kubebuilder:validation:MaxLength=253
    IdentityProviderRef string `json:"identityProviderRef,omitempty"`
}

// VerbRef is a verb-on-resource permission entry inside a Role.
type VerbRef struct {
    // Resource is one of the registered Keleustes resource kinds
    // OR the wildcard "*". Resource names are case-sensitive; the
    // validation webhook rejects any value not in the registered
    // alphabet (ADR 0004 §4).
    // +kubebuilder:validation:MinLength=1
    // +kubebuilder:validation:MaxLength=128
    Resource string `json:"resource"`

    // Verbs is the set of action verbs granted on Resource. The
    // validation webhook rejects verbs not in the alphabet for the
    // given Resource. The wildcard "*" is allowed only when
    // Resource is also "*" — i.e., super-admin only.
    // +kubebuilder:validation:MinItems=1
    Verbs []string `json:"verbs"`
}
```

Adding a new resource or verb to the alphabet is a code change
(updating the validation webhook's registered list) **plus** an
ADR amendment per ADR 0004 §4. The wildcards `"*"` exist but
require the `query-state` cross-cutting verb plus an explicit
cluster scope — i.e., reserved for the operator's own super-admin
`ClusterRole`, not for application teams.

## 3. `IdentityProvider`

Cluster-scoped. Configures how a single identity source is consulted
and how its group claims normalize into the canonical Keleustes
vocabulary.

```go
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=idp
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Kind",type=string,JSONPath=`.spec.kind`
// +kubebuilder:printcolumn:name="Issuer",type=string,JSONPath=`.spec.oidc.issuerUrl`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type IdentityProvider struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   IdentityProviderSpec   `json:"spec,omitempty"`
    Status IdentityProviderStatus `json:"status,omitempty"`
}

type IdentityProviderSpec struct {
    // Kind is the identity source category. Determines which of
    // OIDC, MTLS, NATSNKey is consulted.
    // +kubebuilder:validation:Enum=OIDC;MTLS;NATSNKey
    Kind string `json:"kind"`

    // Audience is what this IdP is recommended for. Informational
    // metadata used by the UI's "auth methods" page; does not
    // affect token validation.
    // +kubebuilder:validation:Enum=Human;CI;Agent
    Audience string `json:"audience"`

    // OIDC configures an OIDC issuer. Required when kind=OIDC.
    // +optional
    OIDC *OIDCConfig `json:"oidc,omitempty"`

    // MTLS configures the trusted CA bundle for client-certificate
    // identity. Required when kind=MTLS.
    // +optional
    MTLS *MTLSConfig `json:"mtls,omitempty"`

    // Normalization controls how raw claim values map into the
    // canonical Keleustes group vocabulary RoleBindings reference.
    // Applied at authentication time, ONCE — per ADR 0004 §6.
    Normalization GroupNormalization `json:"normalization"`
}

type OIDCConfig struct {
    // IssuerUrl is the OIDC issuer URL (no trailing slash).
    // +kubebuilder:validation:Pattern=`^https://[^/]+(/[^/]+)*$`
    // +kubebuilder:validation:MaxLength=2048
    IssuerUrl string `json:"issuerUrl"`

    // ClientId is the OIDC client identifier issued by the provider.
    // +kubebuilder:validation:MinLength=1
    // +kubebuilder:validation:MaxLength=512
    ClientId string `json:"clientId"`

    // ClientSecretRef references a Secret in the operator namespace
    // holding the OIDC client secret. Empty for public clients
    // (UI PKCE-only flows).
    // +optional
    ClientSecretRef *corev1.SecretKeySelector `json:"clientSecretRef,omitempty"`

    // GroupsClaim is the JWT claim name that carries group
    // membership. Default "groups".
    // +optional
    // +kubebuilder:default=groups
    // +kubebuilder:validation:MaxLength=128
    GroupsClaim string `json:"groupsClaim,omitempty"`

    // UsernameClaim is the JWT claim used for actor.subject.
    // Default "email". When the chosen claim is missing, the IdP is
    // marked NotReady.
    // +optional
    // +kubebuilder:default=email
    // +kubebuilder:validation:MaxLength=128
    UsernameClaim string `json:"usernameClaim,omitempty"`

    // RequiredAudiences narrows acceptance to tokens whose `aud`
    // includes at least one of these strings. Empty accepts any.
    // +optional
    RequiredAudiences []string `json:"requiredAudiences,omitempty"`

    // ClockSkew is the tolerated clock drift on token timestamps
    // (`exp`, `nbf`). Default 30s, max 5m.
    // +optional
    // +kubebuilder:default="30s"
    ClockSkew metav1.Duration `json:"clockSkew,omitempty"`
}

type MTLSConfig struct {
    // CABundleRef references a ConfigMap holding the PEM-encoded
    // trusted CA bundle.
    CABundleRef corev1.ConfigMapKeySelector `json:"caBundleRef"`

    // SubjectMapping picks a CN, O, or SAN entry as the actor.subject.
    // +kubebuilder:validation:Enum=CN;O;EmailSAN;URISAN
    // +kubebuilder:default=CN
    SubjectMapping string `json:"subjectMapping"`
}

type GroupNormalization struct {
    // Case controls case folding on group values before further
    // processing.
    // +kubebuilder:validation:Enum=lower;upper;preserve
    // +kubebuilder:default=lower
    Case string `json:"case"`

    // Trim removes any of these prefixes/suffixes from group values
    // before mapping. Common values: "roles/", "/".
    // +optional
    Trim []string `json:"trim,omitempty"`

    // Map applies after case folding and trimming: any group value
    // equal to a key is replaced by the value. Keys are matched
    // exactly; values are the canonical Keleustes group name.
    // +optional
    Map map[string]string `json:"map,omitempty"`

    // Allow restricts the final group set to entries matching at
    // least one of these glob patterns. Empty allows all.
    // +optional
    Allow []string `json:"allow,omitempty"`
}

type IdentityProviderStatus struct {
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`

    // DiscoveredEndpoints captures the OIDC discovery document
    // results (jwks_uri, token endpoint) at last successful sync.
    // Empty for non-OIDC providers.
    // +optional
    DiscoveredEndpoints *OIDCDiscovered `json:"discoveredEndpoints,omitempty"`

    // LastSync is when this IdP's keys/CA were last refreshed.
    // +optional
    LastSync *metav1.Time `json:"lastSync,omitempty"`

    // Conditions: Ready, DiscoveryFailed, KeysExpired.
    // +optional
    // +listType=map
    // +listMapKey=type
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type OIDCDiscovered struct {
    JWKSUri       string `json:"jwksUri"`
    TokenEndpoint string `json:"tokenEndpoint"`
    AuthEndpoint  string `json:"authEndpoint"`
}
```

**Validation webhook rules (cannot be expressed by schema alone):**

- Exactly one of `spec.oidc`, `spec.mtls` is set, matching
  `spec.kind` (`OIDC` ↔ `oidc`, `MTLS` ↔ `mtls`, `NATSNKey` ↔
  neither — config lives in the leaf node's NKey directory).
- `spec.normalization.map` values must be valid Keleustes group
  names (`^[a-z0-9-]+$`, ≤63 chars).
- `spec.oidc.issuerUrl` must resolve to a parseable
  `/.well-known/openid-configuration` at admission time (deferred
  to MVP 1; MVP 0 webhook only checks URL syntax).

**Status conditions:**

- `Ready` — IdP is currently usable for authentication.
- `DiscoveryFailed` — most recent OIDC discovery attempt failed.
- `KeysExpired` — cached JWKS is past TTL and a refresh failed.

## 4. `Role`

Namespace-scoped. A named bundle of verb-on-resource permissions.

```go
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=krole
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Permissions",type=integer,JSONPath=`.status.permissionCount`
// +kubebuilder:printcolumn:name="BoundBy",type=integer,JSONPath=`.status.boundByCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type Role struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   RoleSpec   `json:"spec,omitempty"`
    Status RoleStatus `json:"status,omitempty"`
}

type RoleSpec struct {
    // DisplayName is the human-facing label rendered in the UI.
    // Defaults to metadata.name.
    // +optional
    // +kubebuilder:validation:MaxLength=128
    DisplayName string `json:"displayName,omitempty"`

    // Description is a free-text explanation surfaced in the UI's
    // role catalog.
    // +optional
    // +kubebuilder:validation:MaxLength=1024
    Description string `json:"description,omitempty"`

    // Permissions is the verb-on-resource matrix this Role grants.
    // +kubebuilder:validation:MinItems=1
    Permissions []VerbRef `json:"permissions"`
}

type RoleStatus struct {
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`

    // PermissionCount is the total number of (resource, verb) pairs
    // expanded from Spec.Permissions. Maintained for the printer
    // column; cheap to compute, useful at a glance.
    // +optional
    PermissionCount int32 `json:"permissionCount,omitempty"`

    // BoundByCount is the number of RoleBindings currently
    // referencing this Role.
    // +optional
    BoundByCount int32 `json:"boundByCount,omitempty"`

    // Conditions: Valid, ReferencesUnknownResource, ReferencesUnknownVerb.
    // +optional
    // +listType=map
    // +listMapKey=type
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

**`ClusterRole` companion** has identical shape with
`+kubebuilder:resource:scope=Cluster`. Used exclusively for
operator-internal bindings (sync-engine SA, webhook receivers).
Application teams use namespace-scoped `Role`.

**Validation webhook rules:**

- Every `permissions[*].resource` matches the registered resource
  alphabet (ADR 0004 §4).
- Every `permissions[*].verbs[*]` matches the alphabet for its
  resource.
- `"*"` resource is allowed only in a `ClusterRole`, and only when
  accompanied by `+keleustes.skaphos.io/admin-acknowledged=true`
  annotation (a guardrail against accidental super-admin grants).

**Status conditions:** `Valid` (true when both alphabet checks
pass), `ReferencesUnknownResource`, `ReferencesUnknownVerb`.

## 5. `RoleBinding`

Namespace-scoped (cluster-scoped equivalent: `ClusterRoleBinding`).
Binds subjects to a Role within a Scope, with optional
time-bounding and break-glass annotations.

```go
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=krb
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Role",type=string,JSONPath=`.spec.roleRef.name`
// +kubebuilder:printcolumn:name="Subjects",type=integer,JSONPath=`.status.subjectCount`
// +kubebuilder:printcolumn:name="Scope",type=string,JSONPath=`.status.scopeDescription`
// +kubebuilder:printcolumn:name="ValidUntil",type=string,JSONPath=`.spec.validUntil`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.conditions[?(@.type=="Active")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type RoleBinding struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   RoleBindingSpec   `json:"spec,omitempty"`
    Status RoleBindingStatus `json:"status,omitempty"`
}

type RoleBindingSpec struct {
    // Subjects is the principals receiving the Role. Empty list is
    // rejected at admission.
    // +kubebuilder:validation:MinItems=1
    Subjects []Subject `json:"subjects"`

    // RoleRef points at a Role or ClusterRole in the same namespace
    // as the RoleBinding (Role) or cluster-wide (ClusterRole). The
    // referenced object must exist at admission time; if it later
    // disappears, the binding is marked NotActive.
    RoleRef RoleRef `json:"roleRef"`

    // Scope narrows what the bound Role applies to. See Scope shape
    // (§2). Exactly one One-Of field is required; the validation
    // webhook enforces this.
    Scope Scope `json:"scope"`

    // ValidUntil is the auto-expiry time. Empty for permanent
    // bindings. When set, the binding is Active until this instant
    // and Expired after.
    // +optional
    ValidUntil *metav1.Time `json:"validUntil,omitempty"`

    // Reason is a human-readable explanation, required when
    // ValidUntil is within 24 hours of metadata.creationTimestamp
    // (the short-lived break-glass case).
    // +optional
    // +kubebuilder:validation:MaxLength=1024
    Reason string `json:"reason,omitempty"`

    // AuditTicket is an incident-tracking ID propagated to every
    // audit event generated under this binding. Required when the
    // bound Role contains the `break-glass` verb.
    // +optional
    // +kubebuilder:validation:MaxLength=128
    AuditTicket string `json:"auditTicket,omitempty"`

    // RequiresStepUp reserves the MVP 4 field for FIDO2 / hardware-
    // key step-up auth (ADR 0004 §9). Honored as a no-op until then.
    // +optional
    RequiresStepUp bool `json:"requiresStepUp,omitempty"`
}

type RoleRef struct {
    // Kind is "Role" or "ClusterRole".
    // +kubebuilder:validation:Enum=Role;ClusterRole
    Kind string `json:"kind"`

    // Name is the referenced Role's metadata.name.
    // +kubebuilder:validation:MinLength=1
    // +kubebuilder:validation:MaxLength=253
    Name string `json:"name"`
}

type RoleBindingStatus struct {
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`

    // SubjectCount mirrors len(spec.subjects); printer column.
    // +optional
    SubjectCount int32 `json:"subjectCount,omitempty"`

    // ScopeDescription is a one-line rendering of Spec.Scope for
    // the printer column ("project:payments", "selector: ...").
    // +optional
    // +kubebuilder:validation:MaxLength=253
    ScopeDescription string `json:"scopeDescription,omitempty"`

    // ResolvedSubjects is the post-normalization view of
    // spec.subjects after IdentityProvider claim mapping has been
    // applied. Populated for groups/users so operators can verify
    // "this binding targets the same group I think it does." Empty
    // for ServiceAccount / Agent subjects.
    // +optional
    ResolvedSubjects []Subject `json:"resolvedSubjects,omitempty"`

    // ExpiresAt mirrors spec.validUntil for the printer column.
    // +optional
    ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`

    // Conditions: Active, Expired, RoleMissing, ScopeInvalid,
    // SubjectsUnresolved, AuditTicketMissing.
    // +optional
    // +listType=map
    // +listMapKey=type
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

**Validation webhook rules:**

- Exactly one of `spec.scope.{cluster,project,application,
  environment,selector}` is set.
- When `spec.scope.cluster=true`, the request is admitted only for
  identities holding the operator-internal `cluster-admin` Role
  (defense against accidental cluster-wide grants).
- When `spec.scope.project` is set, the value must match an
  existing `Project` and the binding's namespace must be a member
  namespace of that Project — otherwise reject. This prevents a
  RoleBinding in `team-a` namespace from referencing
  `project:team-b`.
- When `spec.roleRef.kind=Role`, the Role must live in the same
  namespace as the binding. `ClusterRole` references may target
  any Project's binding.
- If `spec.validUntil` is set and earlier than `now + 24h`,
  `spec.reason` is required.
- If `spec.roleRef` resolves to a Role granting `break-glass`,
  `spec.auditTicket` is required.
- `spec.validUntil` cannot be more than 1 year in the future.
  (Long-lived "temporary" bindings are the failure mode — force
  re-grant.)

**Status conditions:** `Active`, `Expired`, `RoleMissing`,
`ScopeInvalid`, `SubjectsUnresolved` (no `IdentityProvider`
authenticates any subject's identity provider), `AuditTicketMissing`.

## 6. `Project`

Namespace-scoped. The delegation boundary. Lists membership of
Applications, Sources, Environments, and DeploymentTargets;
identifies project-admin groups; references a default
ApprovalPolicy.

```go
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=kproj
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Apps",type=integer,JSONPath=`.status.applicationCount`
// +kubebuilder:printcolumn:name="Targets",type=integer,JSONPath=`.status.deploymentTargetCount`
// +kubebuilder:printcolumn:name="Admins",type=integer,JSONPath=`.status.adminGroupCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type Project struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   ProjectSpec   `json:"spec,omitempty"`
    Status ProjectStatus `json:"status,omitempty"`
}

type ProjectSpec struct {
    // DisplayName is the human-facing label.
    // +optional
    // +kubebuilder:validation:MaxLength=128
    DisplayName string `json:"displayName,omitempty"`

    // Description is free-text for the UI's project catalog.
    // +optional
    // +kubebuilder:validation:MaxLength=4096
    Description string `json:"description,omitempty"`

    // DefaultNamespace is the Kubernetes namespace member resources
    // land in by default. Recommended (and the 1:1 default per
    // ADR 0004 §7). Leave empty to opt into N:1 / non-Kubernetes-
    // native groupings.
    // +optional
    // +kubebuilder:validation:MaxLength=63
    // +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
    DefaultNamespace string `json:"defaultNamespace,omitempty"`

    // MemberNamespaces explicitly enumerates the namespaces this
    // Project owns. When DefaultNamespace is set, it is implicitly
    // a member. RoleBindings whose namespace is not a member
    // namespace cannot reference this Project as a scope.
    // +optional
    MemberNamespaces []string `json:"memberNamespaces,omitempty"`

    // AllowedDeploymentTargets is the closed set of DeploymentTarget
    // resources Applications in this Project are allowed to deploy
    // to. Selector-based; empty rejects all targets (default-deny).
    // +kubebuilder:validation:Required
    AllowedDeploymentTargets MemberSelector `json:"allowedDeploymentTargets"`

    // Members declares ownership of Applications, Sources, and
    // Environments — by name list, by label selector, or both.
    Members ProjectMembers `json:"members"`

    // AdminGroups is the list of canonical Keleustes group names
    // that may create/edit/delete RoleBindings within this Project
    // without admin help. Members of these groups receive an
    // implicit `RoleBinding (within project): view/create/edit/
    // delete` grant for this Project's scope.
    // +optional
    AdminGroups []string `json:"adminGroups,omitempty"`

    // DefaultApprovalPolicyRef references an ApprovalPolicy applied
    // to every Promotion in this Project unless the Promotion
    // overrides explicitly.
    // +optional
    DefaultApprovalPolicyRef *corev1.LocalObjectReference `json:"defaultApprovalPolicyRef,omitempty"`

    // CrossProjectGrants enumerates other Projects whose
    // Applications may declare dependencies on Applications in this
    // Project (engine plan §2.6). Validated at admit; enforced at
    // SyncPlan time. Empty rejects all cross-Project dependencies.
    // +optional
    CrossProjectGrants []ProjectGrant `json:"crossProjectGrants,omitempty"`
}

type MemberSelector struct {
    // Names is an exact list of resource names. Combined with
    // Selector as a union.
    // +optional
    Names []string `json:"names,omitempty"`

    // Selector is a label selector evaluated against the member
    // resource kind.
    // +optional
    Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

type ProjectMembers struct {
    Applications MemberSelector `json:"applications,omitempty"`
    Sources      MemberSelector `json:"sources,omitempty"`
    Environments MemberSelector `json:"environments,omitempty"`
}

type ProjectGrant struct {
    // ProjectName is the consuming Project's name.
    // +kubebuilder:validation:MinLength=1
    // +kubebuilder:validation:MaxLength=253
    ProjectName string `json:"projectName"`

    // ApplicationNames optionally restricts the grant to specific
    // Applications in this Project. Empty allows all.
    // +optional
    ApplicationNames []string `json:"applicationNames,omitempty"`
}

type ProjectStatus struct {
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`

    ApplicationCount      int32 `json:"applicationCount,omitempty"`
    SourceCount           int32 `json:"sourceCount,omitempty"`
    EnvironmentCount      int32 `json:"environmentCount,omitempty"`
    DeploymentTargetCount int32 `json:"deploymentTargetCount,omitempty"`
    AdminGroupCount       int32 `json:"adminGroupCount,omitempty"`

    // ResolvedMembers carries the materialized list of resource
    // names per kind after selectors are evaluated. Updated each
    // reconcile; useful for the UI to render membership without
    // a second list operation.
    // +optional
    ResolvedMembers *ResolvedProjectMembers `json:"resolvedMembers,omitempty"`

    // Conditions: Ready, MembershipDrift, ApprovalPolicyMissing,
    // CrossGrantTarget Unknown.
    // +optional
    // +listType=map
    // +listMapKey=type
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type ResolvedProjectMembers struct {
    Applications      []string `json:"applications,omitempty"`
    Sources           []string `json:"sources,omitempty"`
    Environments      []string `json:"environments,omitempty"`
    DeploymentTargets []string `json:"deploymentTargets,omitempty"`
}
```

**Validation webhook rules:**

- `spec.defaultNamespace`, when set, must already exist as a
  Kubernetes Namespace.
- A Namespace cannot be the `defaultNamespace` of more than one
  Project simultaneously (rejected at admission).
- `spec.memberNamespaces` must be a superset of
  `spec.defaultNamespace` if both are set.
- `spec.allowedDeploymentTargets` cannot be unset (empty selector
  expressing default-deny is fine — but the field is required so
  the operator's intent is explicit).
- `spec.crossProjectGrants[*].projectName` must reference an
  existing Project (or pass `--allow-forward-refs` for bootstrap
  scenarios).

**Status conditions:** `Ready`, `MembershipDrift` (a member name
no longer exists), `ApprovalPolicyMissing`, `CrossGrantTargetUnknown`.

## 7. `ApprovalPolicy`

Namespace-scoped. Encodes separation-of-duties and N-of-M
constraints. Attached to a Project (as default), a PromotionPolicy
(as per-policy override), or a single Promotion (as one-off).

```go
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=kap
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="MinApprovers",type=integer,JSONPath=`.spec.minApprovers`
// +kubebuilder:printcolumn:name="Distinct",type=boolean,JSONPath=`.spec.requireDistinctActors`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type ApprovalPolicy struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   ApprovalPolicySpec   `json:"spec,omitempty"`
    Status ApprovalPolicyStatus `json:"status,omitempty"`
}

type ApprovalPolicySpec struct {
    // RequireDistinctActors enforces requester ≠ approver. Default
    // true.
    // +kubebuilder:default=true
    RequireDistinctActors bool `json:"requireDistinctActors"`

    // MinApprovers is N in N-of-M approval (M is the size of any
    // RequiredGroups intersection). Default 1.
    // +kubebuilder:default=1
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:validation:Maximum=10
    MinApprovers int32 `json:"minApprovers"`

    // RequiredGroups: at least one approver must be a member of
    // each listed group. AND semantics.
    // +optional
    RequiredGroups []string `json:"requiredGroups,omitempty"`

    // ExcludeGroups: members of these groups cannot approve. Used
    // for "the requester's direct reports cannot rubber-stamp."
    // +optional
    ExcludeGroups []string `json:"excludeGroups,omitempty"`

    // CoolingOffPeriod requires this duration to elapse between
    // approval and the Promotion advancing. Allows manual
    // intervention on rushed approvals. Default 0.
    // +optional
    // +kubebuilder:default="0s"
    CoolingOffPeriod metav1.Duration `json:"coolingOffPeriod,omitempty"`

    // MaxApproverAge rejects approvals older than this duration.
    // Default 7d. Prevents stale approvals from advancing
    // Promotions long after the human intent passed.
    // +optional
    // +kubebuilder:default="168h"
    MaxApproverAge metav1.Duration `json:"maxApproverAge,omitempty"`
}

type ApprovalPolicyStatus struct {
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`

    // ReferencedByCount is how many Project/PromotionPolicy/Promotion
    // CRs name this ApprovalPolicy.
    // +optional
    ReferencedByCount int32 `json:"referencedByCount,omitempty"`

    // Conditions: Valid, GroupsUnknown.
    // +optional
    // +listType=map
    // +listMapKey=type
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

**Validation webhook rules:**

- `spec.minApprovers ≤ len(RequiredGroups) + 5` — guard against
  policies that can never be satisfied.
- `RequiredGroups ∩ ExcludeGroups = ∅` (no group both required and
  excluded).
- `spec.coolingOffPeriod ≤ 24h`, `spec.maxApproverAge ≤ 30d`
  (sanity caps; raise via amendment if a real workflow needs more).
- Every group in `RequiredGroups` and `ExcludeGroups` must be a
  canonical name (`^[a-z0-9-]+$`). Resolution to actual member
  identities happens at evaluation time, not admission.

**Status conditions:** `Valid`, `GroupsUnknown` (a referenced group
is not produced by any `IdentityProvider`'s normalization rules —
soft warning; does not block).

## 8. Scope Expression Conventions

`Scope` (§2) is referenced from `RoleBinding`, plus future plugin
surfaces (`Notifier.spec.scope`, `PolicyGate.spec.scope`). One
shape, one validator.

The four mutually-exclusive scope forms in priority order
(narrowest first when more than one would apply):

1. **`application: <ns>/<name>`** — single Application
2. **`selector: <labelSelector>`** — set of resources matching
   labels (evaluated against the resource kind named in the
   parent's `RoleRef`)
3. **`environment: <name>`** — every resource in a named Environment
4. **`project: <name>`** — every resource owned by a Project
5. **`cluster: true`** — operator-internal only

The validation webhook enforces "exactly one set." When `selector`
is used, the selector's `matchLabels` must include at least one
label key (`team`, `tier`, etc.) — a bare empty selector is
rejected so binders cannot accidentally grant cluster-wide
via an "any resource" selector.

## 9. Status Conditions — Common Taxonomy

Every RBAC CRD publishes `status.conditions` with this discipline:

- **Type names are PascalCase**, e.g., `Ready`, `Active`, `Expired`,
  `Valid`, `Invalid`, `Missing`, `Drift`.
- **Status** is `True`, `False`, or `Unknown`.
- **Reason** is a one-word code (PascalCase, no spaces).
- **Message** is a single sentence, no trailing period.
- **LastTransitionTime** is mandatory.
- The reconciler must publish at least one condition per reconcile
  (so a stuck reconciler is visible as `LastTransitionTime` going
  stale).

Per-CRD condition catalogues are listed in each section above. A
condition type that appears on multiple CRDs (`Ready`, `Valid`,
`Expired`) has the same semantics everywhere.

## 10. Validation Webhook Outline

Three CRDs need a webhook beyond what kubebuilder validation can
express. The webhook is a single `webhook` binary registered for
all three (avoids three separate webhook configurations):

| CRD               | Webhook rules                                                                                                                       |
|-------------------|--------------------------------------------------------------------------------------------------------------------------------------|
| `IdentityProvider` | Exactly one of `oidc`/`mtls` matches `kind`; `normalization.map` values are canonical group names; OIDC issuer URL is well-formed.   |
| `Role`            | Verbs and resources are in the registered alphabet; `"*"` wildcard requires `admin-acknowledged=true` annotation on a `ClusterRole`. |
| `RoleBinding`     | Exactly one Scope One-Of; project-scoped binding must live in a member namespace; `validUntil < now+24h` requires `reason`; `break-glass` Role requires `auditTicket`; `validUntil ≤ 1y`. |
| `Project`         | `defaultNamespace` exists and is uniquely owned; `crossProjectGrants` targets exist; `allowedDeploymentTargets` is set.              |
| `ApprovalPolicy`  | `minApprovers ≤ groups + 5`; `requiredGroups ∩ excludeGroups = ∅`; cooling-off ≤ 24h; group names canonical.                         |

The webhook runs in failure-mode `Fail` for `Role` and
`RoleBinding` (a misconfigured permission must not silently slip
in) and `Ignore` for `IdentityProvider` (transient discovery
failures should not block IdP creation — the Ready condition
surfaces the issue).

## 11. RBAC of the RBAC CRDs Themselves

A circular question: who can edit a `Role`? The answer:

- **`Role` and `RoleBinding`:** require the `RoleBinding (within
  project)` verbs (per ADR 0004 §4 table). Project admins manage
  bindings inside their Project; cluster admins manage everywhere.
- **`Project`:** create/delete is cluster-admin-only (Projects are
  the delegation boundary; teams shouldn't create new boundaries
  without coordination). Edit (description, ApprovalPolicyRef,
  AdminGroups) is project-admin within that Project.
- **`IdentityProvider`:** cluster-admin-only. Adding an IdP changes
  who can authenticate; not a project concern.
- **`ApprovalPolicy`:** project-admin within the namespace.

These shapes are encoded in the initial seed `ClusterRole`s that
ship with the operator (`config/rbac/`) and the `cluster-admin`
ClusterRoleBinding installed by the chart.

## 12. Why `Agent` is Not in This Alphabet

ADR 0005 §168 locked in that **`Agent` is its own CR**, not an
annotation on `DeploymentTarget`. SKA-323 asks whether `Agent`
should join the RBAC alphabet of resources that `Role` can grant
verbs on.

**Answer: No, not at this layer.**

Reasoning:

1. An Agent is *operator infrastructure*, not application state.
   Application teams do not author or edit `Agent` CRs — the
   agent registers itself via NKey on first connect and the hub
   creates the CR. A Role verb like "edit Agent" has no audience.
2. The two actions a human takes on an Agent — registering a new
   one, decommissioning an existing one — are already covered:
   `DeploymentTarget.register-agent` (§4 table) and the standard
   `delete` verb on `Agent` interpreted via native Kubernetes
   RBAC. Adding Keleustes Role verbs would duplicate.
3. Agent-to-hub authentication is NKey-based (per ADR 0005 §5);
   audit events for agent actions carry `actor.type=agent`
   (SKA-322 §6.3). Neither path consults the Keleustes RBAC CRDs
   — they go through `IdentityProvider.kind=NATSNKey` (this plan
   §3) instead.

If a future requirement emerges (e.g., "developer wants to
self-bootstrap a dev-cluster agent without admin involvement"),
add `Agent` to the alphabet as a §4 amendment. Not before.

## 13. Sample CRs

One sample per CRD lands in `config/samples/` alongside the
existing CRD samples. Outlines:

- `samples/identityprovider_okta.yaml` — `kind=OIDC`,
  `audience=Human`, `oidc.issuerUrl=https://example.okta.com`,
  normalization `{case: lower, trim: ["roles/"], map: {…}}`.
- `samples/identityprovider_github_oidc.yaml` — `kind=OIDC`,
  `audience=CI`, GitHub Actions OIDC issuer, claim `sub` as
  username mapping.
- `samples/role_application_operator.yaml` — verbs on `Application`
  (`view`, `sync`, `pause`, `resume`), `Source` (`view`,
  `force-refresh`), `SyncRun` (`view`).
- `samples/role_promoter.yaml` — `Promotion.create`, `Approval.grant`,
  `Application.view`.
- `samples/role_break_glass.yaml` — `break-glass` cross-cutting
  verb + `view-audit`.
- `samples/rolebinding_team_payments_operators.yaml` — project-scoped
  binding of `application-operator` to group `payments-eng`.
- `samples/rolebinding_break_glass_2h.yaml` — short-lived binding
  to `break-glass` with `validUntil`, `reason`, `auditTicket`.
- `samples/project_payments.yaml` — `defaultNamespace=payments`,
  `members.applications.selector={team: payments}`,
  `allowedDeploymentTargets.selector={environment-class: prod}`,
  `adminGroups=[payments-platform]`,
  `defaultApprovalPolicyRef=payments-default`.
- `samples/approvalpolicy_payments_default.yaml` —
  `requireDistinctActors=true`, `minApprovers=2`,
  `requiredGroups=[sre]`, `excludeGroups=[contractors]`,
  `coolingOffPeriod=15m`.

Each sample carries the SPDX header and a top-of-file comment
explaining what scenario it illustrates.

## 14. Open Questions

1. **`Project.spec.crossProjectGrants` enforcement timing.** ADR
   0004 §13 defers full cross-project enforcement to MVP 3 because
   ADR 0001 §11 makes the same call for multi-tenant. The schema
   carries the grants from day one; v1alpha1 logs the crossing but
   does not block. Confirm at MVP 3 implementation that the
   reconciler hooks can be tightened without a schema change —
   this plan believes yes; if not, schema additions land via the
   additive §5.1 protocol from SKA-322.

2. **`RoleBinding.spec.subjects[].name` for User principals.** The
   ADR uses "normalized subject (typically email)" but in
   workload-identity flows there is no email. The current schema
   accepts anything matching MaxLength=253; the validation webhook
   does not enforce email format. Open question: should User-Kind
   names be a typed sum (email | did | provider-issued opaque)?
   Defer until a real workload-identity-only customer hits the
   awkwardness.

3. **Helper CRDs for the audit envelope.** SKA-322's audit envelope
   carries `actor.identityProvider`, `actor.groups`, etc. — those
   values originate from the `IdentityProvider` that authenticated
   the request. Do we expose a `ResolvedActor` shape via the API
   that the audit consumer can join against, or expect every
   consumer to maintain its own IdP-membership cache? Lean toward
   API-server-side projection (cheaper for consumers); decide when
   SKA-332 lands.

4. **`Role` selector-by-label.** Currently `RoleBinding.scope.selector`
   selects target resources by label. An alternative would let
   `Role.spec.permissions[*]` carry a selector too — "grant
   `sync` on Applications matching `tier=prod`". The latter is
   strictly more expressive but doubles the cardinality of the
   evaluation matrix. Rejected for v1alpha1; revisit when a real
   customer asks.

5. **Conditional permissions** ("approve a Promotion *only if*
   `auditTicket` is set"). Not in v1alpha1. The `ApprovalPolicy`
   surface absorbs this for Promotions; other resources do not
   currently need it.

## 15. Compliance with Prior Decisions

| Decision                  | This plan honors it by                                                                                                                |
|---------------------------|----------------------------------------------------------------------------------------------------------------------------------------|
| ADR 0001 §6 (Render is not pluggable) | The RBAC CRDs are first-party Keleustes types, not plugin surfaces. `PolicyGate` plugins (ADR 0001) layer above RBAC, never replace it. |
| ADR 0003 (Git invariant)  | Every RBAC CR is edited in Git; the operator reconciles them like any other Keleustes CR.                                              |
| ADR 0004 §1 (5 CRDs)      | This plan ships exactly those 5 (plus `ClusterRole`/`ClusterRoleBinding` companions). No additional RBAC CRD is introduced.            |
| ADR 0004 §2 (Project boundary) | `Project.spec.adminGroups` carries the delegation; `MemberSelector` / `AllowedDeploymentTargets` express the tenancy edges.            |
| ADR 0004 §3 (layered with k8s RBAC) | None of these CRDs override or duplicate native k8s RBAC; the validation webhook is the Keleustes-side enforcement only.               |
| ADR 0004 §4 (verb alphabet) | `VerbRef` rejects out-of-alphabet entries at admission. Wildcards require explicit `admin-acknowledged` annotation.                    |
| ADR 0004 §6 (group normalization at IdP) | `GroupNormalization` lives on `IdentityProvider.spec.normalization`, not on `RoleBinding`.                                            |
| ADR 0004 §9 (time-bound + step-up reserve) | `RoleBinding.spec.{validUntil,reason,auditTicket,requiresStepUp}` are all present; `requiresStepUp` is a reserved no-op until MVP 4.   |
| ADR 0004 §10 (default-deny) | The chart's seed RoleBindings grant `view` to nobody by default; tenants must opt in by creating their first `RoleBinding`.            |
| ADR 0005 §168 (Agent is a CR) | `Agent` is deliberately excluded from the RBAC alphabet (§12) — agent identity flows through `IdentityProvider.kind=NATSNKey` instead. |
| SKA-322 §6.3 (actor normalization) | `IdentityProvider.spec.normalization` is the single point that produces the canonical group names referenced in audit `actor.groups`. |

## 16. Concrete Follow-ups

1. **SKA-330 (MVP 0 IdentityProvider scaffold)** — implements the
   `IdentityProvider` CRD per §3 plus a single-IdP OIDC handshake.
   Validation webhook lives in the same binary; runs in `Ignore`
   mode at MVP 0.
2. **SKA-345 (MVP 1 Role/RoleBinding/Project + reconcilers)** —
   the remaining four CRDs plus the custom evaluator (ADR 0004
   §11). Webhook flips to `Fail` for `Role` / `RoleBinding`.
3. **New ticket: `ApprovalPolicy` enforcement in the Promotion
   state machine** — MVP 2 work; depends on this CRD being live
   and on the Promotion Engine landing.
4. **New ticket: `samples/` content per §13** — small, can land
   alongside SKA-330 and SKA-345.
5. **New ticket: the seed `ClusterRole` set** that ships in the
   operator chart per §11 — `cluster-admin`, `project-admin`,
   `view`. Bundled with SKA-330.
6. **Update `docs/DECISIONS.md`** — add this plan to the active
   interim contracts table; pointer from RBAC plan §5.2 to here.
7. **PROPOSAL.md cross-link** — add a `> Refined by` marker at
   §15 (Policy model) and §18 (API auth) pointing at this plan;
   handled by SKA-325 (PROPOSAL refresh).

---

**When this plan stabilizes** (after SKA-330 lands and the
validation webhook has been tested against real OIDC providers),
§1–§13 promote into a new ADR co-located with ADR 0004 — likely
ADR 0009 (Render Contract becomes ADR 0007; Audit Schema ADR 0008;
RBAC CRD shapes ADR 0009). §14 open questions remain in this plan
until resolved.
