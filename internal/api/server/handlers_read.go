/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package server

import (
	"context"

	"github.com/skaphos/keleustes/internal/api/openapi"
	"github.com/skaphos/keleustes/internal/api/readmodel"
)

// GetApplications returns the fleet summary, applying the optional
// project/env/region/q narrowing onto the read-model filter.
func (s *Server) GetApplications(ctx context.Context, request openapi.GetApplicationsRequestObject) (openapi.GetApplicationsResponseObject, error) {
	f := readmodel.ApplicationFilter{}
	if p := request.Params.Project; p != nil {
		f.Project = *p
	}
	if e := request.Params.Env; e != nil {
		f.Env = *e
	}
	if r := request.Params.Region; r != nil {
		f.Region = *r
	}
	if q := request.Params.Q; q != nil {
		f.Q = *q
	}

	page, err := s.rm.ListApplications(ctx, f)
	if err != nil {
		return nil, err
	}
	return openapi.GetApplications200JSONResponse{AsOf: page.AsOf, Items: page.Items}, nil
}

// GetApplicationsName returns one application by name.
func (s *Server) GetApplicationsName(ctx context.Context, request openapi.GetApplicationsNameRequestObject) (openapi.GetApplicationsNameResponseObject, error) {
	app, err := s.rm.GetApplication(ctx, request.Name)
	if err != nil {
		return nil, err
	}
	return openapi.GetApplicationsName200JSONResponse(app), nil
}

// GetApplicationsNameMatrix returns the env × region deployment matrix. The
// sentinel name "all" yields the whole-fleet matrix.
func (s *Server) GetApplicationsNameMatrix(ctx context.Context, request openapi.GetApplicationsNameMatrixRequestObject) (openapi.GetApplicationsNameMatrixResponseObject, error) {
	m, err := s.rm.GetMatrix(ctx, request.Name)
	if err != nil {
		return nil, err
	}
	return openapi.GetApplicationsNameMatrix200JSONResponse(m), nil
}

// GetApplicationsNamePromotions lists promotions touching an application.
func (s *Server) GetApplicationsNamePromotions(ctx context.Context, request openapi.GetApplicationsNamePromotionsRequestObject) (openapi.GetApplicationsNamePromotionsResponseObject, error) {
	ps, err := s.rm.ListApplicationPromotions(ctx, request.Name)
	if err != nil {
		return nil, err
	}
	return openapi.GetApplicationsNamePromotions200JSONResponse(ps), nil
}

// GetApplicationsNameReleases lists releases for an application.
func (s *Server) GetApplicationsNameReleases(ctx context.Context, request openapi.GetApplicationsNameReleasesRequestObject) (openapi.GetApplicationsNameReleasesResponseObject, error) {
	rs, err := s.rm.ListApplicationReleases(ctx, request.Name)
	if err != nil {
		return nil, err
	}
	return openapi.GetApplicationsNameReleases200JSONResponse(rs), nil
}

// GetAudit searches the append-only activity log, carrying the optional
// resource/actor/verb/time-bound/limit filters through to the read model.
func (s *Server) GetAudit(ctx context.Context, request openapi.GetAuditRequestObject) (openapi.GetAuditResponseObject, error) {
	q := readmodel.AuditQuery{From: request.Params.From, To: request.Params.To}
	if v := request.Params.Resource; v != nil {
		q.Resource = *v
	}
	if v := request.Params.Actor; v != nil {
		q.Actor = *v
	}
	if v := request.Params.Verb; v != nil {
		q.Verb = *v
	}
	if v := request.Params.Limit; v != nil {
		q.Limit = *v
	}

	page, err := s.rm.QueryAudit(ctx, q)
	if err != nil {
		return nil, err
	}
	resp := openapi.GetAudit200JSONResponse{Items: page.Items}
	if page.NextCursor != "" {
		resp.NextCursor = &page.NextCursor
	}
	return resp, nil
}

// GetDiff renders a diff between two refs in the requested mode.
func (s *Server) GetDiff(ctx context.Context, request openapi.GetDiffRequestObject) (openapi.GetDiffResponseObject, error) {
	q := readmodel.DiffQuery{Mode: string(request.Params.Mode)}
	if v := request.Params.Left; v != nil {
		q.Left = *v
	}
	if v := request.Params.Right; v != nil {
		q.Right = *v
	}

	d, err := s.rm.GetDiff(ctx, q)
	if err != nil {
		return nil, err
	}
	return openapi.GetDiff200JSONResponse(d), nil
}

// GetEnvironments returns the env → cell → target topology.
func (s *Server) GetEnvironments(ctx context.Context, _ openapi.GetEnvironmentsRequestObject) (openapi.GetEnvironmentsResponseObject, error) {
	es, err := s.rm.ListEnvironments(ctx)
	if err != nil {
		return nil, err
	}
	return openapi.GetEnvironments200JSONResponse(es), nil
}

// GetPromotions lists promotions, optionally narrowed to a workflow state.
func (s *Server) GetPromotions(ctx context.Context, request openapi.GetPromotionsRequestObject) (openapi.GetPromotionsResponseObject, error) {
	state := ""
	if v := request.Params.State; v != nil {
		state = string(*v)
	}

	ps, err := s.rm.ListPromotions(ctx, state)
	if err != nil {
		return nil, err
	}
	return openapi.GetPromotions200JSONResponse(ps), nil
}

// GetPromotionsID returns one promotion by its ULID.
func (s *Server) GetPromotionsID(ctx context.Context, request openapi.GetPromotionsIDRequestObject) (openapi.GetPromotionsIDResponseObject, error) {
	p, err := s.rm.GetPromotion(ctx, request.ID)
	if err != nil {
		return nil, err
	}
	return openapi.GetPromotionsID200JSONResponse(p), nil
}

// GetReleases returns the release inventory, optionally scoped to a project.
func (s *Server) GetReleases(ctx context.Context, request openapi.GetReleasesRequestObject) (openapi.GetReleasesResponseObject, error) {
	project := ""
	if v := request.Params.Project; v != nil {
		project = *v
	}

	rs, err := s.rm.ListReleases(ctx, project)
	if err != nil {
		return nil, err
	}
	return openapi.GetReleases200JSONResponse(rs), nil
}

// GetTargets lists all deployment targets.
func (s *Server) GetTargets(ctx context.Context, _ openapi.GetTargetsRequestObject) (openapi.GetTargetsResponseObject, error) {
	ts, err := s.rm.ListTargets(ctx)
	if err != nil {
		return nil, err
	}
	return openapi.GetTargets200JSONResponse(ts), nil
}

// GetTargetsNameDrift returns the Git-vs-live drift for one target.
func (s *Server) GetTargetsNameDrift(ctx context.Context, request openapi.GetTargetsNameDriftRequestObject) (openapi.GetTargetsNameDriftResponseObject, error) {
	d, err := s.rm.GetTargetDrift(ctx, request.Name)
	if err != nil {
		return nil, err
	}
	return openapi.GetTargetsNameDrift200JSONResponse(d), nil
}

// GetTargetsNameHealth returns the health checks for one target.
func (s *Server) GetTargetsNameHealth(ctx context.Context, request openapi.GetTargetsNameHealthRequestObject) (openapi.GetTargetsNameHealthResponseObject, error) {
	hs, err := s.rm.GetTargetHealth(ctx, request.Name)
	if err != nil {
		return nil, err
	}
	return openapi.GetTargetsNameHealth200JSONResponse(hs), nil
}
