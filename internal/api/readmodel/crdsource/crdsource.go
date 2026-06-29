/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package crdsource

import (
	"context"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	keleustesv1alpha1 "github.com/skaphos/keleustes/api/v1alpha1"
	"github.com/skaphos/keleustes/internal/api/openapi"
	"github.com/skaphos/keleustes/internal/api/readmodel"
)

// Source is the CRD-backed [readmodel.ReadModel]. It is safe for concurrent use:
// it holds only a controller-runtime client, which is itself concurrency-safe,
// so a single Source serves the stateless, freely-replicated API server.
type Source struct {
	c client.Client
}

// New returns a Source reading through c. c must have the keleustes.skaphos.io
// types registered in its scheme (cmd/manager wires them via AddToScheme).
func New(c client.Client) *Source {
	return &Source{c: c}
}

var _ readmodel.ReadModel = (*Source)(nil)

// ListApplications lists the Application fleet and applies the best-effort
// filters the scaffold supports (project, env, free text). AsOf is the read
// time: the matrix is eventually consistent, so callers see a snapshot stamp.
func (s *Source) ListApplications(ctx context.Context, f readmodel.ApplicationFilter) (readmodel.ApplicationsPage, error) {
	var list keleustesv1alpha1.ApplicationList
	if err := s.c.List(ctx, &list); err != nil {
		return readmodel.ApplicationsPage{}, wrapErr(err)
	}
	// Items is initialized non-nil so an empty/zero-match fleet serializes as
	// "items":[] (the contract makes items a required, non-nullable array),
	// not "items":null.
	page := readmodel.ApplicationsPage{
		AsOf:  time.Now().UTC(),
		Items: make([]openapi.Application, 0, len(list.Items)),
	}
	for i := range list.Items {
		app := list.Items[i]
		if !matchesApplicationFilter(app, f) {
			continue
		}
		page.Items = append(page.Items, applicationToAPI(app))
	}
	return page, nil
}

// GetApplication resolves an Application by natural key. The port carries no
// namespace (ADR 0008 addresses by name), so we list cluster-wide and match.
func (s *Source) GetApplication(ctx context.Context, name string) (openapi.Application, error) {
	var list keleustesv1alpha1.ApplicationList
	if err := s.c.List(ctx, &list); err != nil {
		return openapi.Application{}, wrapErr(err)
	}
	for i := range list.Items {
		if list.Items[i].Name == name {
			return applicationToAPI(list.Items[i]), nil
		}
	}
	return openapi.Application{}, readmodel.ErrNotFound
}

// GetMatrix derives the application×(env,region) rollup best-effort. The
// scaffold has no per-cell sync state, so every cell carries the application's
// overall (Accepted-derived) status; real per-target state arrives with the
// Sync engine. name == "all" returns the whole fleet.
func (s *Source) GetMatrix(ctx context.Context, name string) (openapi.Matrix, error) {
	var apps keleustesv1alpha1.ApplicationList
	if err := s.c.List(ctx, &apps); err != nil {
		return openapi.Matrix{}, wrapErr(err)
	}
	selected := apps.Items
	if name != "all" {
		selected = nil
		for i := range apps.Items {
			if apps.Items[i].Name == name {
				selected = []keleustesv1alpha1.Application{apps.Items[i]}
				break
			}
		}
		if len(selected) == 0 {
			return openapi.Matrix{}, readmodel.ErrNotFound
		}
	}

	var targets keleustesv1alpha1.DeploymentTargetList
	if err := s.c.List(ctx, &targets); err != nil {
		return openapi.Matrix{}, wrapErr(err)
	}
	cols := matrixColumns(targets.Items)

	m := openapi.Matrix{
		AsOf:    time.Now().UTC(),
		Columns: cols,
		Rows:    make([]openapi.MatrixRow, 0, len(selected)),
	}
	for i := range selected {
		m.Rows = append(m.Rows, matrixRow(selected[i], cols))
	}
	return m, nil
}

// ListApplicationReleases returns the releases pinned to one application. An
// unknown parent yields an empty (200) collection, not 404: the contract
// declares only 200 here, and the fixtures adapter behaves the same, so both
// ReadModel backends stay wire-compatible.
func (s *Source) ListApplicationReleases(ctx context.Context, name string) ([]openapi.Release, error) {
	var list keleustesv1alpha1.ReleaseList
	if err := s.c.List(ctx, &list); err != nil {
		return nil, wrapErr(err)
	}
	out := make([]openapi.Release, 0)
	for i := range list.Items {
		if list.Items[i].Spec.Application != name {
			continue
		}
		out = append(out, releaseToAPI(list.Items[i]))
	}
	return out, nil
}

// ListApplicationPromotions returns the promotions for one application. As with
// releases, an unknown parent yields an empty (200) collection rather than 404,
// matching both the contract and the fixtures adapter.
func (s *Source) ListApplicationPromotions(ctx context.Context, name string) ([]openapi.Promotion, error) {
	var list keleustesv1alpha1.PromotionList
	if err := s.c.List(ctx, &list); err != nil {
		return nil, wrapErr(err)
	}
	out := make([]openapi.Promotion, 0)
	for i := range list.Items {
		if list.Items[i].Spec.Application != name {
			continue
		}
		out = append(out, promotionToAPI(list.Items[i]))
	}
	return out, nil
}

// ListPromotions lists promotions narrowed by the inbox state (active, blocked,
// mine, history; "" = all).
func (s *Source) ListPromotions(ctx context.Context, state string) ([]openapi.Promotion, error) {
	var list keleustesv1alpha1.PromotionList
	if err := s.c.List(ctx, &list); err != nil {
		return nil, wrapErr(err)
	}
	out := make([]openapi.Promotion, 0)
	for i := range list.Items {
		if !promotionMatchesState(list.Items[i], state) {
			continue
		}
		out = append(out, promotionToAPI(list.Items[i]))
	}
	return out, nil
}

// GetPromotion resolves a promotion by its durable ULID (ADR 0008). Until the
// identity engine mints and caches ULIDs, it also matches the natural key so
// the endpoint is usable against scaffold objects.
func (s *Source) GetPromotion(ctx context.Context, id string) (openapi.Promotion, error) {
	var list keleustesv1alpha1.PromotionList
	if err := s.c.List(ctx, &list); err != nil {
		return openapi.Promotion{}, wrapErr(err)
	}
	for i := range list.Items {
		p := list.Items[i]
		if u := ulidOf(&p); (u != "" && u == id) || p.Name == id {
			return promotionToAPI(p), nil
		}
	}
	return openapi.Promotion{}, readmodel.ErrNotFound
}

// ListReleases lists every release, optionally narrowed to a project.
func (s *Source) ListReleases(ctx context.Context, project string) ([]openapi.Release, error) {
	var list keleustesv1alpha1.ReleaseList
	if err := s.c.List(ctx, &list); err != nil {
		return nil, wrapErr(err)
	}
	out := make([]openapi.Release, 0)
	for i := range list.Items {
		if project != "" && projectOf(&list.Items[i]) != project {
			continue
		}
		out = append(out, releaseToAPI(list.Items[i]))
	}
	return out, nil
}

// ListTargets lists every deployment target.
func (s *Source) ListTargets(ctx context.Context) ([]openapi.DeploymentTarget, error) {
	var list keleustesv1alpha1.DeploymentTargetList
	if err := s.c.List(ctx, &list); err != nil {
		return nil, wrapErr(err)
	}
	out := make([]openapi.DeploymentTarget, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, targetToAPI(list.Items[i]))
	}
	return out, nil
}

// GetTargetHealth returns the HealthChecks bound to a target. The target must
// exist; with no checks it returns an empty (but valid) slice. Detailed probe
// timestamps fill in when the Health engine lands.
func (s *Source) GetTargetHealth(ctx context.Context, name string) ([]openapi.HealthCheck, error) {
	if _, err := s.findTarget(ctx, name); err != nil {
		return nil, err
	}
	var list keleustesv1alpha1.HealthCheckList
	if err := s.c.List(ctx, &list); err != nil {
		return nil, wrapErr(err)
	}
	out := make([]openapi.HealthCheck, 0)
	for i := range list.Items {
		h := list.Items[i]
		if h.Spec.TargetRef == nil || h.Spec.TargetRef.Name != name {
			continue
		}
		out = append(out, healthCheckToAPI(h))
	}
	return out, nil
}

// GetTargetDrift returns the live drift for a target. The target must exist.
// The Diff engine (MVP 3) computes real drift; until it lands we return a
// valid, empty git-live diff rather than fabricating entries.
func (s *Source) GetTargetDrift(ctx context.Context, name string) (openapi.Diff, error) {
	if _, err := s.findTarget(ctx, name); err != nil {
		return openapi.Diff{}, err
	}
	return openapi.Diff{
		Mode:    string(openapi.GitLive),
		Entries: []openapi.DiffEntry{},
	}, nil
}

// ListEnvironments lists environments in lifecycle order, each enriched
// best-effort with the regions and cells of the targets that name it.
func (s *Source) ListEnvironments(ctx context.Context) ([]openapi.Environment, error) {
	var envs keleustesv1alpha1.EnvironmentList
	if err := s.c.List(ctx, &envs); err != nil {
		return nil, wrapErr(err)
	}
	var targets keleustesv1alpha1.DeploymentTargetList
	if err := s.c.List(ctx, &targets); err != nil {
		return nil, wrapErr(err)
	}
	byEnv := map[string][]keleustesv1alpha1.DeploymentTarget{}
	for i := range targets.Items {
		env := targets.Items[i].Spec.Environment
		byEnv[env] = append(byEnv[env], targets.Items[i])
	}
	sorted := sortedEnvironments(envs.Items)
	out := make([]openapi.Environment, 0, len(sorted))
	for i := range sorted {
		out = append(out, environmentToAPI(sorted[i], byEnv[sorted[i].Name]))
	}
	return out, nil
}

// GetDiff echoes the query into a valid, empty diff envelope. Diff computation
// is the Diff engine's job (MVP 3); the CRD adapter only wires the endpoint.
func (s *Source) GetDiff(_ context.Context, q readmodel.DiffQuery) (openapi.Diff, error) {
	return openapi.Diff{
		Mode:    q.Mode,
		Left:    strPtrOrNil(q.Left),
		Right:   strPtrOrNil(q.Right),
		Entries: []openapi.DiffEntry{},
	}, nil
}

// QueryAudit returns an empty page. Audit/event history lives in NATS JetStream,
// not CRDs (ADR 0005 §3 — no RDBMS, audit in JetStream), so the CRD adapter has
// nothing to read; the JetStream-backed adapter serves /audit.
func (s *Source) QueryAudit(_ context.Context, _ readmodel.AuditQuery) (readmodel.AuditPage, error) {
	return readmodel.AuditPage{Items: []openapi.AuditEvent{}}, nil
}

// findTarget resolves a target by natural key, ErrNotFound when absent.
func (s *Source) findTarget(ctx context.Context, name string) (keleustesv1alpha1.DeploymentTarget, error) {
	var list keleustesv1alpha1.DeploymentTargetList
	if err := s.c.List(ctx, &list); err != nil {
		return keleustesv1alpha1.DeploymentTarget{}, wrapErr(err)
	}
	for i := range list.Items {
		if list.Items[i].Name == name {
			return list.Items[i], nil
		}
	}
	return keleustesv1alpha1.DeploymentTarget{}, readmodel.ErrNotFound
}

// matchesApplicationFilter applies the filters derivable from the scaffold
// Application. Region is intentionally not applied: targets carry region,
// applications do not, so a region filter would have no scaffold-stage meaning.
func matchesApplicationFilter(app keleustesv1alpha1.Application, f readmodel.ApplicationFilter) bool {
	if f.Project != "" && projectOf(&app) != f.Project {
		return false
	}
	if f.Env != "" && !contains(app.Spec.Topology.Environments, f.Env) {
		return false
	}
	if f.Q != "" {
		q := strings.ToLower(f.Q)
		if !strings.Contains(strings.ToLower(app.Name), q) &&
			!strings.Contains(strings.ToLower(app.Spec.Owner.Team), q) {
			return false
		}
	}
	return true
}

// wrapErr translates a not-found from the client into the port's sentinel so
// the server maps it to 404; everything else propagates as a 500-class error.
func wrapErr(err error) error {
	if apierrors.IsNotFound(err) {
		return readmodel.ErrNotFound
	}
	return err
}
