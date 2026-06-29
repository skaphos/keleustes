/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package crdsource

import (
	"sort"
	"time"

	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	keleustesv1alpha1 "github.com/skaphos/keleustes/api/v1alpha1"
	"github.com/skaphos/keleustes/internal/api/openapi"
)

const (
	// conditionAccepted is the scaffold reconcilers' readiness condition; it is
	// the only signal available to derive a product status today.
	conditionAccepted = "Accepted"

	// labelPartOf groups resources into a product "project" for the read model.
	// The scaffold CRDs have no project field, so we lean on the standard
	// app.kubernetes.io/part-of label and fall back to the namespace.
	labelPartOf = "app.kubernetes.io/part-of"

	// annotationULID is where an engine-resolved ULID is surfaced to the read
	// model in the scaffold. ADR 0008 keeps the durable ULID in NATS KV / status
	// once the identity engine lands; until then we read it from this annotation
	// when an operator has set it, otherwise the ULID is "".
	annotationULID = "keleustes.skaphos.io/ulid"
)

// matrixColumn aliases the anonymous column struct generated for openapi.Matrix
// so columns can be built outside the openapi package. It must stay byte-for-
// byte identical (fields + json tags) to that anonymous struct.
type matrixColumn = struct {
	Env     *string `json:"env,omitempty"`
	Lagging *bool   `json:"lagging,omitempty"`
	Region  *string `json:"region,omitempty"`
}

// envCell aliases the anonymous cell struct on openapi.Environment.
type envCell = struct {
	Name    *string                     `json:"name,omitempty"`
	Region  *string                     `json:"region,omitempty"`
	Status  *openapi.Status             `json:"status,omitempty"`
	Targets *[]openapi.DeploymentTarget `json:"targets,omitempty"`
}

// provenanceFields aliases the anonymous provenance struct on openapi.Release.
type provenanceFields = struct {
	Attestation *bool `json:"attestation,omitempty"`
	Sbom        *bool `json:"sbom,omitempty"`
	Signed      *bool `json:"signed,omitempty"`
}

// statusFromConditions maps the scaffold Accepted condition onto the contract's
// canonical status. The scaffold only knows whether the spec was accepted, so:
//
//	no Accepted condition  -> Missing      (controller has not observed it yet)
//	Accepted == True       -> Progressing  (accepted; sync/health engines pending)
//	Accepted == False      -> Blocked      (rejected by admission/validation)
//	Accepted == Unknown    -> Progressing  (in flight)
//
// Richer states (Healthy, Drifted, Degraded, Frozen) arrive with the engines.
func statusFromConditions(conds []metav1.Condition) openapi.Status {
	c := apiMeta.FindStatusCondition(conds, conditionAccepted)
	if c == nil {
		return openapi.StatusMissing
	}
	switch c.Status {
	case metav1.ConditionTrue:
		return openapi.StatusProgressing
	case metav1.ConditionFalse:
		return openapi.StatusBlocked
	default:
		return openapi.StatusProgressing
	}
}

// applicationToAPI maps an Application CRD to the product concept. Owner is the
// spec owner team; project is best-effort (see labelPartOf); source is the
// deployment manifest's repo/path. ULID is "" until the identity engine lands.
func applicationToAPI(a keleustesv1alpha1.Application) openapi.Application {
	return openapi.Application{
		Name:    a.Name,
		Status:  statusFromConditions(a.Status.Conditions),
		Ulid:    ulidOf(&a),
		Owner:   strPtrOrNil(a.Spec.Owner.Team),
		Project: strPtrOrNil(projectOf(&a)),
		Source:  sourceRefFromManifest(a.Spec.Deployment.Manifest),
	}
}

// sourceRefFromManifest projects the Application's manifest repo/path onto a
// SourceRef. Commit/ref are resolved by the Source engine, so they stay nil.
func sourceRefFromManifest(m keleustesv1alpha1.ApplicationManifest) *openapi.SourceRef {
	if m.Repo == "" && m.BasePath == "" {
		return nil
	}
	return &openapi.SourceRef{
		Repo: strPtrOrNil(m.Repo),
		Path: strPtrOrNil(m.BasePath),
	}
}

// releaseToAPI maps a Release CRD. Version prefers the first artifact's
// human-readable version, falling back to the object name. DeployedOn and full
// provenance verification fill in when the Source/Sync engines land.
func releaseToAPI(r keleustesv1alpha1.Release) openapi.Release {
	out := openapi.Release{
		App:     r.Spec.Application,
		Version: releaseVersion(r),
		Ulid:    strPtrOrNil(ulidOf(&r)),
	}
	if !r.CreationTimestamp.IsZero() {
		out.Created = timePtr(r.CreationTimestamp.Time)
	}
	if c := r.Spec.Provenance.Commit; c != "" {
		out.Source = &openapi.SourceRef{Commit: strPtrOrNil(c)}
	}
	if r.Spec.Provenance.SBOMRef != "" {
		out.Provenance = &provenanceFields{Sbom: boolPtr(true)}
	}
	return out
}

func releaseVersion(r keleustesv1alpha1.Release) string {
	if len(r.Spec.Artifacts) > 0 && r.Spec.Artifacts[0].Version != "" {
		return r.Spec.Artifacts[0].Version
	}
	return r.Name
}

// targetToAPI maps a DeploymentTarget CRD. Drift, Frozen, and Version are not
// derivable from the scaffold status; they fill in with the Diff/Sync/Freeze
// engines. LastSync uses the last successful contact when recorded.
func targetToAPI(t keleustesv1alpha1.DeploymentTarget) openapi.DeploymentTarget {
	out := openapi.DeploymentTarget{
		Name:    t.Name,
		Status:  statusFromConditions(t.Status.Conditions),
		Cell:    strPtrOrNil(t.Spec.Cell),
		Cluster: strPtrOrNil(t.Spec.Cluster.Name),
		Env:     strPtrOrNil(t.Spec.Environment),
		Region:  strPtrOrNil(t.Spec.Region),
		Ulid:    strPtrOrNil(ulidOf(&t)),
	}
	if t.Status.LastContactedAt != nil {
		out.LastSync = timePtr(t.Status.LastContactedAt.Time)
	}
	return out
}

// promotionToAPI maps a Promotion CRD. Gates, approvals, the PR URL, and sync
// phase are populated by the Promotion/Policy engines; they stay nil here.
func promotionToAPI(p keleustesv1alpha1.Promotion) openapi.Promotion {
	out := openapi.Promotion{
		Application: p.Spec.Application,
		From:        p.Spec.From.Environment,
		To:          p.Spec.To.Environment,
		Status:      promotionStatus(p.Status.Phase),
		Ulid:        ulidOf(&p),
		Release:     strPtrOrNil(p.Spec.Release),
		Mode:        strPtrOrNil(string(p.Spec.Mode)),
	}
	if !p.CreationTimestamp.IsZero() {
		out.RequestedAt = timePtr(p.CreationTimestamp.Time)
	}
	return out
}

// promotionStatus collapses the Promotion lifecycle phase onto the canonical
// status vocabulary. The many in-flight phases all read as Progressing.
func promotionStatus(phase keleustesv1alpha1.PromotionPhase) openapi.Status {
	switch phase {
	case keleustesv1alpha1.PromotionPhaseBlocked:
		return openapi.StatusBlocked
	case keleustesv1alpha1.PromotionPhaseSucceeded:
		return openapi.StatusHealthy
	case keleustesv1alpha1.PromotionPhaseFailed:
		return openapi.StatusFailed
	case keleustesv1alpha1.PromotionPhaseRolledBack, keleustesv1alpha1.PromotionPhaseCanceled:
		return openapi.StatusDegraded
	default:
		// Proposed/Evaluating/Approved/MutatingGit/WaitingForMerge/
		// WaitingForSync/Verifying and the empty (pre-engine) phase are in flight.
		return openapi.StatusProgressing
	}
}

// promotionMatchesState filters by the inbox state. "mine" cannot be scoped to
// the caller without an auth context in the scaffold, so it behaves as "all".
func promotionMatchesState(p keleustesv1alpha1.Promotion, state string) bool {
	switch state {
	case "", "mine":
		return true
	case "blocked":
		return p.Status.Phase == keleustesv1alpha1.PromotionPhaseBlocked
	case "history":
		return isTerminalPhase(p.Status.Phase)
	case "active":
		return !isTerminalPhase(p.Status.Phase) && p.Status.Phase != keleustesv1alpha1.PromotionPhaseBlocked
	default:
		return true
	}
}

func isTerminalPhase(phase keleustesv1alpha1.PromotionPhase) bool {
	switch phase {
	case keleustesv1alpha1.PromotionPhaseSucceeded,
		keleustesv1alpha1.PromotionPhaseFailed,
		keleustesv1alpha1.PromotionPhaseRolledBack,
		keleustesv1alpha1.PromotionPhaseCanceled:
		return true
	default:
		return false
	}
}

// healthCheckToAPI maps a HealthCheck CRD. Resource prefers the checked
// application, falling back to the check's own name. LastProbe fills in when the
// Health engine records probe timestamps.
func healthCheckToAPI(h keleustesv1alpha1.HealthCheck) openapi.HealthCheck {
	resource := h.Spec.Application
	if resource == "" {
		resource = h.Name
	}
	return openapi.HealthCheck{
		Resource: resource,
		Status:   healthStatus(h.Status.State),
		Message:  strPtrOrNil(h.Status.Summary),
	}
}

// healthStatus maps the CRD HealthState onto the canonical status. Suspended is
// reported as Frozen; Unknown and the empty (pre-engine) state read as in flight.
func healthStatus(state keleustesv1alpha1.HealthState) openapi.Status {
	switch state {
	case keleustesv1alpha1.HealthStateHealthy:
		return openapi.StatusHealthy
	case keleustesv1alpha1.HealthStateDegraded:
		return openapi.StatusDegraded
	case keleustesv1alpha1.HealthStateMissing:
		return openapi.StatusMissing
	case keleustesv1alpha1.HealthStateSuspended:
		return openapi.StatusFrozen
	case keleustesv1alpha1.HealthStateProgressing:
		return openapi.StatusProgressing
	default:
		return openapi.StatusProgressing
	}
}

// environmentToAPI maps an Environment CRD, enriched with the regions and cells
// of the targets that name it (targets is the pre-filtered slice for this env).
func environmentToAPI(e keleustesv1alpha1.Environment, targets []keleustesv1alpha1.DeploymentTarget) openapi.Environment {
	out := openapi.Environment{Name: e.Name}
	if regions := uniqueRegions(targets); len(regions) > 0 {
		out.Regions = &regions
	}
	if cells := envCells(targets); len(cells) > 0 {
		out.Cells = &cells
	}
	return out
}

// matrixColumns derives the deduplicated, sorted (env,region) column headers
// from the deployment targets. Returns a non-nil slice so the matrix is valid
// even when there are no targets.
func matrixColumns(targets []keleustesv1alpha1.DeploymentTarget) []matrixColumn {
	sorted := append([]keleustesv1alpha1.DeploymentTarget(nil), targets...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Spec.Environment != sorted[j].Spec.Environment {
			return sorted[i].Spec.Environment < sorted[j].Spec.Environment
		}
		return sorted[i].Spec.Region < sorted[j].Spec.Region
	})
	seen := map[string]bool{}
	cols := make([]matrixColumn, 0)
	for i := range sorted {
		env := sorted[i].Spec.Environment
		region := sorted[i].Spec.Region
		key := env + "\x00" + region
		if seen[key] {
			continue
		}
		seen[key] = true
		cols = append(cols, matrixColumn{Env: strPtrOrNil(env), Region: strPtrOrNil(region)})
	}
	return cols
}

// matrixRow builds one application's row. Every cell carries the application's
// overall status because the scaffold has no per-target sync state yet.
func matrixRow(a keleustesv1alpha1.Application, cols []matrixColumn) openapi.MatrixRow {
	status := statusFromConditions(a.Status.Conditions)
	cells := make([]openapi.MatrixCell, 0, len(cols))
	for _, c := range cols {
		cells = append(cells, openapi.MatrixCell{Env: c.Env, Region: c.Region, Status: status})
	}
	return openapi.MatrixRow{
		Application: a.Name,
		Ulid:        strPtrOrNil(ulidOf(&a)),
		Cells:       cells,
	}
}

// envCells groups the env's targets by cell name (deterministically ordered).
// Per-cell aggregate status is left nil until the Sync engine supplies it.
func envCells(targets []keleustesv1alpha1.DeploymentTarget) []envCell {
	groups := map[string][]keleustesv1alpha1.DeploymentTarget{}
	order := make([]string, 0)
	for i := range targets {
		cell := targets[i].Spec.Cell
		if _, ok := groups[cell]; !ok {
			order = append(order, cell)
		}
		groups[cell] = append(groups[cell], targets[i])
	}
	sort.Strings(order)
	cells := make([]envCell, 0, len(order))
	for _, cell := range order {
		members := groups[cell]
		mapped := make([]openapi.DeploymentTarget, 0, len(members))
		region := ""
		for i := range members {
			mapped = append(mapped, targetToAPI(members[i]))
			if region == "" {
				region = members[i].Spec.Region
			}
		}
		cells = append(cells, envCell{
			Name:    strPtrOrNil(cell),
			Region:  strPtrOrNil(region),
			Targets: &mapped,
		})
	}
	return cells
}

// sortedEnvironments orders environments by spec.Order then name.
func sortedEnvironments(in []keleustesv1alpha1.Environment) []keleustesv1alpha1.Environment {
	out := append([]keleustesv1alpha1.Environment(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Spec.Order != out[j].Spec.Order {
			return out[i].Spec.Order < out[j].Spec.Order
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func uniqueRegions(targets []keleustesv1alpha1.DeploymentTarget) []string {
	seen := map[string]bool{}
	out := make([]string, 0)
	for i := range targets {
		r := targets[i].Spec.Region
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// projectOf derives a product project: the part-of label when set, else the
// object's namespace. The scaffold CRDs carry no first-class project field.
func projectOf(o metav1.Object) string {
	if v := o.GetLabels()[labelPartOf]; v != "" {
		return v
	}
	return o.GetNamespace()
}

// ulidOf returns the engine-surfaced ULID annotation, or "" when absent.
func ulidOf(o metav1.Object) string {
	return o.GetAnnotations()[annotationULID]
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func boolPtr(b bool) *bool { return &b }

func timePtr(t time.Time) *time.Time { return &t }
