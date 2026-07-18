/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package fixtures

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/skaphos/keleustes/internal/api/openapi"
	"github.com/skaphos/keleustes/internal/api/readmodel"
)

//go:embed testdata/contract/*.json
var contractFS embed.FS

// appsAsOf is the fixed freshness stamp reported by ListApplications. It is a
// constant so the fixture backend stays deterministic across processes and runs.
var appsAsOf = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

// Fixtures is a read-only, in-memory ReadModel served from the embedded contract
// corpus. It is immutable after New and therefore safe for concurrent use.
type Fixtures struct {
	apps     []openapi.Application
	matrix   openapi.Matrix
	proms    []openapi.Promotion
	releases []openapi.Release
	envs     []openapi.Environment
	audit    []openapi.AuditEvent
	// targets is derived from envs so the matrix, environments, and target views
	// stay consistent off one source of truth.
	targets []openapi.DeploymentTarget
}

var _ readmodel.ReadModel = (*Fixtures)(nil)

// New loads the embedded JSON corpus once and returns a ready ReadModel. The
// data is compile-time embedded, so a decode failure is a build defect and
// panics rather than returning an error.
func New() *Fixtures {
	f := &Fixtures{
		apps:     load[[]openapi.Application]("applications.json"),
		matrix:   load[openapi.Matrix]("matrix.json"),
		proms:    load[[]openapi.Promotion]("promotions.json"),
		releases: load[[]openapi.Release]("releases.json"),
		envs:     load[[]openapi.Environment]("environments.json"),
		audit:    load[[]openapi.AuditEvent]("audit.json"),
	}
	f.targets = flattenTargets(f.envs)
	return f
}

func load[T any](name string) T {
	var out T
	data, err := contractFS.ReadFile("testdata/contract/" + name)
	if err != nil {
		panic(fmt.Sprintf("fixtures: read %s: %v", name, err))
	}
	if err := json.Unmarshal(data, &out); err != nil {
		panic(fmt.Sprintf("fixtures: parse %s: %v", name, err))
	}
	return out
}

// flattenTargets collects every DeploymentTarget embedded in the environment
// cells, preserving document order for determinism.
func flattenTargets(envs []openapi.Environment) []openapi.DeploymentTarget {
	out := []openapi.DeploymentTarget{}
	for _, env := range envs {
		if env.Cells == nil {
			continue
		}
		for _, c := range *env.Cells {
			if c.Targets == nil {
				continue
			}
			out = append(out, *c.Targets...)
		}
	}
	return out
}

// --- Applications -----------------------------------------------------------

func (f *Fixtures) ListApplications(ctx context.Context, flt readmodel.ApplicationFilter) (readmodel.ApplicationsPage, error) {
	items := []openapi.Application{}
	for _, a := range f.apps {
		if f.appMatches(a, flt) {
			items = append(items, a)
		}
	}
	return readmodel.ApplicationsPage{Items: items, AsOf: appsAsOf}, nil
}

func (f *Fixtures) GetApplication(ctx context.Context, name string) (openapi.Application, error) {
	for _, a := range f.apps {
		if a.Name == name {
			return a, nil
		}
	}
	return openapi.Application{}, readmodel.ErrNotFound
}

// GetMatrix returns the full fleet matrix for "all", otherwise the single row
// for the named application (with the shared column headers).
func (f *Fixtures) GetMatrix(ctx context.Context, name string) (openapi.Matrix, error) {
	if name == "all" {
		return f.matrix, nil
	}
	for _, row := range f.matrix.Rows {
		if row.Application == name {
			m := f.matrix
			m.Rows = []openapi.MatrixRow{row}
			return m, nil
		}
	}
	return openapi.Matrix{}, readmodel.ErrNotFound
}

func (f *Fixtures) ListApplicationReleases(ctx context.Context, name string) ([]openapi.Release, error) {
	out := []openapi.Release{}
	for _, r := range f.releases {
		if r.App == name {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *Fixtures) ListApplicationPromotions(ctx context.Context, name string) ([]openapi.Promotion, error) {
	out := []openapi.Promotion{}
	for _, p := range f.proms {
		if p.Application == name {
			out = append(out, p)
		}
	}
	return out, nil
}

// appMatches applies the (best-effort) ApplicationFilter. Project and free-text
// Q match the application record directly; Env/Region are resolved through the
// matrix, where per-cell placement actually lives.
func (f *Fixtures) appMatches(a openapi.Application, flt readmodel.ApplicationFilter) bool {
	if flt.Project != "" && deref(a.Project) != flt.Project {
		return false
	}
	if flt.Q != "" && !appMatchesQuery(a, flt.Q) {
		return false
	}
	if (flt.Env != "" || flt.Region != "") && !f.appHasCell(a.Name, flt.Env, flt.Region) {
		return false
	}
	return true
}

func appMatchesQuery(a openapi.Application, q string) bool {
	q = strings.ToLower(q)
	for _, v := range []string{a.Name, deref(a.Project), deref(a.Owner)} {
		if strings.Contains(strings.ToLower(v), q) {
			return true
		}
	}
	return false
}

func (f *Fixtures) appHasCell(app, env, region string) bool {
	for _, row := range f.matrix.Rows {
		if row.Application != app {
			continue
		}
		for _, c := range row.Cells {
			if env != "" && deref(c.Env) != env {
				continue
			}
			if region != "" && deref(c.Region) != region {
				continue
			}
			return true
		}
	}
	return false
}

// --- Promotions -------------------------------------------------------------

func (f *Fixtures) ListPromotions(ctx context.Context, state string) ([]openapi.Promotion, error) {
	out := []openapi.Promotion{}
	for _, p := range f.proms {
		if promotionInState(p, state) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *Fixtures) GetPromotion(ctx context.Context, id string) (openapi.Promotion, error) {
	for _, p := range f.proms {
		if p.Ulid == id {
			return p, nil
		}
	}
	return openapi.Promotion{}, readmodel.ErrNotFound
}

// promotionInState filters by the contract's promotion states. "mine" has no
// caller identity at this layer, so it degrades to the full list.
func promotionInState(p openapi.Promotion, state string) bool {
	switch state {
	case "blocked":
		return p.Status == openapi.StatusBlocked
	case "active":
		return !isTerminalPromotion(p.Status)
	case "history":
		return isTerminalPromotion(p.Status)
	default: // "", "mine", and any unknown state
		return true
	}
}

func isTerminalPromotion(s openapi.Status) bool {
	return s == openapi.StatusHealthy || s == openapi.StatusFailed
}

// --- Releases ---------------------------------------------------------------

// ListReleases returns every release, or those whose application belongs to the
// given project (releases carry the app, the app carries the project).
func (f *Fixtures) ListReleases(ctx context.Context, project string) ([]openapi.Release, error) {
	if project == "" {
		return f.releases, nil
	}
	out := []openapi.Release{}
	for _, r := range f.releases {
		if f.projectOfApp(r.App) == project {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *Fixtures) projectOfApp(app string) string {
	for _, a := range f.apps {
		if a.Name == app {
			return deref(a.Project)
		}
	}
	return ""
}

// --- Targets ----------------------------------------------------------------

func (f *Fixtures) ListTargets(ctx context.Context) ([]openapi.DeploymentTarget, error) {
	return f.targets, nil
}

// GetTargetHealth synthesizes a representative health rollup: the UI mock has no
// per-target health dataset, so checks are derived from the target's own status.
func (f *Fixtures) GetTargetHealth(ctx context.Context, name string) ([]openapi.HealthCheck, error) {
	t, ok := f.findTarget(name)
	if !ok {
		return nil, readmodel.ErrNotFound
	}
	app := appOf(name)
	return []openapi.HealthCheck{
		{Resource: "Deployment/" + app, Status: t.Status, Message: strPtr("rollout " + string(t.Status)), LastProbe: t.LastSync},
		{Resource: "Service/" + app, Status: openapi.StatusHealthy, Message: strPtr("endpoints ready"), LastProbe: t.LastSync},
	}, nil
}

// GetTargetDrift synthesizes a git-vs-live diff for the target. A drifting
// target reports a changed object; a clean one reports an empty (non-nil) diff.
func (f *Fixtures) GetTargetDrift(ctx context.Context, name string) (openapi.Diff, error) {
	t, ok := f.findTarget(name)
	if !ok {
		return openapi.Diff{}, readmodel.ErrNotFound
	}
	app := appOf(name)
	d := openapi.Diff{
		Mode:    string(openapi.GitLive),
		Left:    strPtr("git"),
		Right:   strPtr(t.Name),
		Entries: []openapi.DiffEntry{},
	}
	if t.Drift != nil && *t.Drift {
		d.Entries = append(d.Entries, openapi.DiffEntry{
			Change: openapi.Changed,
			Object: "Deployment/" + app,
			Drift:  boolPtr(true),
			Patch:  strPtr("@@ spec.replicas @@\n-replicas: 3\n+replicas: 2"),
		})
	}
	return d, nil
}

func (f *Fixtures) findTarget(name string) (openapi.DeploymentTarget, bool) {
	for _, t := range f.targets {
		if t.Name == name {
			return t, true
		}
	}
	return openapi.DeploymentTarget{}, false
}

// --- Environments -----------------------------------------------------------

func (f *Fixtures) ListEnvironments(ctx context.Context) ([]openapi.Environment, error) {
	return f.envs, nil
}

// --- Diff -------------------------------------------------------------------

// GetDiff returns a representative diff for the requested mode. The UI mock has
// no diff dataset, so a single changed object is synthesized; entries is always
// non-nil per the contract.
func (f *Fixtures) GetDiff(ctx context.Context, q readmodel.DiffQuery) (openapi.Diff, error) {
	mode := q.Mode
	if mode == "" {
		mode = string(openapi.GitLive)
	}
	d := openapi.Diff{
		Mode: mode,
		Entries: []openapi.DiffEntry{
			{
				Change: openapi.Changed,
				Object: "Deployment/api",
				Drift:  boolPtr(false),
				Patch:  strPtr("@@ image @@\n-image: api:1.4.1\n+image: api:1.4.2"),
			},
		},
	}
	if q.Left != "" {
		d.Left = strPtr(q.Left)
	}
	if q.Right != "" {
		d.Right = strPtr(q.Right)
	}
	return d, nil
}

// --- Audit ------------------------------------------------------------------

// QueryAudit returns matching events newest-first, truncated to Limit. The
// cursor is always empty: the corpus is small and fully returned in one page.
func (f *Fixtures) QueryAudit(ctx context.Context, q readmodel.AuditQuery) (readmodel.AuditPage, error) {
	items := []openapi.AuditEvent{}
	for _, e := range f.audit {
		if auditMatches(e, q) {
			items = append(items, e)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].At.After(items[j].At)
	})
	if q.Limit > 0 && len(items) > q.Limit {
		items = items[:q.Limit]
	}
	return readmodel.AuditPage{Items: items, NextCursor: ""}, nil
}

func auditMatches(e openapi.AuditEvent, q readmodel.AuditQuery) bool {
	if q.Actor != "" && e.Actor != q.Actor {
		return false
	}
	if q.Verb != "" && e.Verb != q.Verb {
		return false
	}
	if q.Resource != "" && !strings.Contains(e.Target, q.Resource) {
		return false
	}
	if q.From != nil && e.At.Before(*q.From) {
		return false
	}
	if q.To != nil && e.At.After(*q.To) {
		return false
	}
	return true
}

// --- helpers ----------------------------------------------------------------

// appOf extracts the application segment from an "<env>-<region>/<app>" target
// name, falling back to the whole name when there is no slash.
func appOf(target string) string {
	if i := strings.LastIndex(target, "/"); i >= 0 && i+1 < len(target) {
		return target[i+1:]
	}
	return target
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func strPtr(s string) *string { return &s }

func boolPtr(b bool) *bool { return &b }
