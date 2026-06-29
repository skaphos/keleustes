/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package readmodel

import (
	"context"
	"errors"
	"time"

	"github.com/skaphos/keleustes/internal/api/openapi"
)

// Sentinel errors returned by adapters. The API server's error handler
// (internal/api/server) maps these to the contract's Error codes:
// ErrNotFound -> 404/not_found, ErrForbidden -> 403/forbidden. Any other error
// maps to 500/degraded.
var (
	// ErrNotFound is returned when a named/identified resource does not exist.
	ErrNotFound = errors.New("readmodel: not found")
	// ErrForbidden is returned when the caller may not see a resource (ADR 0004).
	ErrForbidden = errors.New("readmodel: forbidden")
)

// ApplicationFilter narrows GET /applications. Empty fields are not applied.
type ApplicationFilter struct {
	Project string
	Env     string
	Region  string
	Q       string
}

// DiffQuery parameterizes GET /diff. Mode is one of the contract's diff modes
// (git-live, release-release, env-env, rendered, policy).
type DiffQuery struct {
	Mode  string
	Left  string
	Right string
}

// AuditQuery parameterizes GET /audit. Nil time bounds are open; Limit <= 0
// lets the adapter apply the contract default.
type AuditQuery struct {
	Resource string
	Actor    string
	Verb     string
	From     *time.Time
	To       *time.Time
	Limit    int
}

// ApplicationsPage is the GET /applications body: the application rollup plus a
// snapshot-freshness timestamp (the matrix is eventually consistent — ADR 0005).
type ApplicationsPage struct {
	Items []openapi.Application
	AsOf  time.Time
}

// AuditPage is the GET /audit body: newest-first events plus an opaque cursor
// for the next page ("" when there are no more).
type AuditPage struct {
	Items      []openapi.AuditEvent
	NextCursor string
}

// ReadModel is the interaction layer the API server handlers and keleustesctl
// depend on. It exposes the read surface of openapi/keleustes.v1.yaml in
// product-concept terms (not raw Kubernetes objects — PROPOSAL §18). Every
// method returns generated openapi types so the port and the wire share one
// shape.
//
// Adapters must be safe for concurrent use: the API server is stateless and
// freely replicated (ADR 0005 §10), so a single ReadModel serves many requests.
type ReadModel interface {
	// GET /applications
	ListApplications(ctx context.Context, f ApplicationFilter) (ApplicationsPage, error)
	// GET /applications/{name}
	GetApplication(ctx context.Context, name string) (openapi.Application, error)
	// GET /applications/{name}/matrix — name == "all" returns the whole fleet.
	GetMatrix(ctx context.Context, name string) (openapi.Matrix, error)
	// GET /applications/{name}/releases
	ListApplicationReleases(ctx context.Context, name string) ([]openapi.Release, error)
	// GET /applications/{name}/promotions
	ListApplicationPromotions(ctx context.Context, name string) ([]openapi.Promotion, error)

	// GET /promotions — state is one of active, blocked, mine, history ("" = all).
	ListPromotions(ctx context.Context, state string) ([]openapi.Promotion, error)
	// GET /promotions/{id} — id is a ULID, stable across renames.
	GetPromotion(ctx context.Context, id string) (openapi.Promotion, error)

	// GET /releases
	ListReleases(ctx context.Context, project string) ([]openapi.Release, error)

	// GET /targets
	ListTargets(ctx context.Context) ([]openapi.DeploymentTarget, error)
	// GET /targets/{name}/health
	GetTargetHealth(ctx context.Context, name string) ([]openapi.HealthCheck, error)
	// GET /targets/{name}/drift
	GetTargetDrift(ctx context.Context, name string) (openapi.Diff, error)

	// GET /environments
	ListEnvironments(ctx context.Context) ([]openapi.Environment, error)

	// GET /diff
	GetDiff(ctx context.Context, q DiffQuery) (openapi.Diff, error)

	// GET /audit
	QueryAudit(ctx context.Context, q AuditQuery) (AuditPage, error)
}
