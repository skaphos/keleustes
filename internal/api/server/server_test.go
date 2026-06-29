/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package server_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"

	"github.com/skaphos/keleustes/internal/api/openapi"
	"github.com/skaphos/keleustes/internal/api/readmodel/fixtures"
	"github.com/skaphos/keleustes/internal/api/server"
)

// baseURL mirrors the contract mount point in server.Handler. The black-box
// test reconstructs it rather than reaching into the unexported const.
const baseURL = "/api/v1"

// newHandler wires the server over the embedded fixture corpus with the default
// (dev-passthrough auth, AllowAll authz) options, so requests need no token.
func newHandler() http.Handler {
	return server.New(fixtures.New(), server.Options{}).Handler()
}

// serve runs one request through the full middleware chain and returns the
// recorder.
func serve(t *testing.T, h http.Handler, method, path string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, body))
	return rec
}

// decodeJSON unmarshals a response body into T, failing the test on error.
func decodeJSON[T any](t *testing.T, b []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("decode %T: %v (body: %s)", v, err, b)
	}
	return v
}

// readCase is a GET endpoint plus an assertion over its decoded body. The same
// set drives both the value test and the spec-conformance test.
type readCase struct {
	name string
	path string
	// routePath, when set, resolves the spec operation for conformance. It is
	// used only for path/{name} targets whose natural key contains a "/": the
	// served path percent-encodes it, but the legacy router matches on the
	// decoded url.Path, so route resolution uses a slash-free name that hits the
	// identical operation schema.
	routePath string
	check     func(t *testing.T, body []byte)
}

// prodUSAPI / prodEUAPI are fixture target natural keys. The "/" is percent-
// encoded so the {name} path segment matches a single segment; ServeMux
// unescapes it back to the real key before binding.
const (
	prodUSAPI = "prod-us%2Fapi"
	prodEUAPI = "prod-eu%2Fapi"
)

func readCases() []readCase {
	return []readCase{
		{
			name: "applications",
			path: baseURL + "/applications",
			check: func(t *testing.T, b []byte) {
				resp := decodeJSON[openapi.GetApplications200JSONResponse](t, b)
				if resp.AsOf.IsZero() {
					t.Error("asOf should be a real snapshot stamp")
				}
				if len(resp.Items) != 3 {
					t.Fatalf("items = %d, want 3", len(resp.Items))
				}
				if resp.Items[0].Name != "api" || resp.Items[0].Status != "Degraded" {
					t.Errorf("items[0] = %+v, want api/Degraded", resp.Items[0])
				}
			},
		},
		{
			name: "application-by-name",
			path: baseURL + "/applications/api",
			check: func(t *testing.T, b []byte) {
				app := decodeJSON[openapi.Application](t, b)
				if app.Name != "api" || app.Ulid == "" || app.Status != "Degraded" {
					t.Errorf("app = %+v, want name=api, non-empty ulid, status=Degraded", app)
				}
			},
		},
		{
			name: "matrix",
			path: baseURL + "/applications/all/matrix",
			check: func(t *testing.T, b []byte) {
				m := decodeJSON[openapi.Matrix](t, b)
				if m.AsOf.IsZero() {
					t.Error("matrix asOf should be set")
				}
				if len(m.Columns) != 4 || len(m.Rows) != 3 {
					t.Fatalf("columns=%d rows=%d, want 4/3", len(m.Columns), len(m.Rows))
				}
				if m.Rows[0].Application != "api" {
					t.Errorf("rows[0].application = %q, want api", m.Rows[0].Application)
				}
			},
		},
		{
			name: "application-releases",
			path: baseURL + "/applications/api/releases",
			check: func(t *testing.T, b []byte) {
				rs := decodeJSON[[]openapi.Release](t, b)
				if len(rs) != 1 {
					t.Fatalf("releases = %d, want 1", len(rs))
				}
				if rs[0].App != "api" || rs[0].Version != "1.4.2" {
					t.Errorf("releases[0] = %+v, want api/1.4.2", rs[0])
				}
			},
		},
		{
			name: "application-promotions",
			path: baseURL + "/applications/api/promotions",
			check: func(t *testing.T, b []byte) {
				ps := decodeJSON[[]openapi.Promotion](t, b)
				if len(ps) != 1 {
					t.Fatalf("promotions = %d, want 1", len(ps))
				}
				if ps[0].Application != "api" || ps[0].Ulid == "" {
					t.Errorf("promotions[0] = %+v, want application=api, non-empty ulid", ps[0])
				}
			},
		},
		{
			name: "promotions",
			path: baseURL + "/promotions",
			check: func(t *testing.T, b []byte) {
				ps := decodeJSON[[]openapi.Promotion](t, b)
				if len(ps) != 2 {
					t.Fatalf("promotions = %d, want 2", len(ps))
				}
				if ps[0].Ulid == "" || ps[0].Application == "" {
					t.Errorf("promotions[0] = %+v, want non-empty ulid/application", ps[0])
				}
			},
		},
		{
			name: "promotion-by-id",
			path: baseURL + "/promotions/01J0PROM00000000000000001",
			check: func(t *testing.T, b []byte) {
				p := decodeJSON[openapi.Promotion](t, b)
				if p.Ulid != "01J0PROM00000000000000001" {
					t.Errorf("ulid = %q, want 01J0PROM00000000000000001", p.Ulid)
				}
				if p.Application != "api" || p.From != "staging" || p.To != "prod" || p.Status != "Blocked" {
					t.Errorf("promotion = %+v, want api staging->prod Blocked", p)
				}
			},
		},
		{
			name: "releases",
			path: baseURL + "/releases",
			check: func(t *testing.T, b []byte) {
				rs := decodeJSON[[]openapi.Release](t, b)
				if len(rs) != 2 {
					t.Fatalf("releases = %d, want 2", len(rs))
				}
				if rs[0].Version == "" || rs[0].App == "" {
					t.Errorf("releases[0] = %+v, want non-empty version/app", rs[0])
				}
			},
		},
		{
			name: "targets",
			path: baseURL + "/targets",
			check: func(t *testing.T, b []byte) {
				ts := decodeJSON[[]openapi.DeploymentTarget](t, b)
				if len(ts) != 2 {
					t.Fatalf("targets = %d, want 2", len(ts))
				}
				if ts[0].Name != "prod-us/api" || ts[0].Status == "" {
					t.Errorf("targets[0] = %+v, want name=prod-us/api, non-empty status", ts[0])
				}
			},
		},
		{
			name:      "target-health",
			path:      baseURL + "/targets/" + prodUSAPI + "/health",
			routePath: baseURL + "/targets/x/health",
			check: func(t *testing.T, b []byte) {
				hs := decodeJSON[[]openapi.HealthCheck](t, b)
				if len(hs) != 2 {
					t.Fatalf("health checks = %d, want 2", len(hs))
				}
				if hs[0].Resource != "Deployment/api" || hs[0].Status == "" {
					t.Errorf("health[0] = %+v, want resource=Deployment/api, non-empty status", hs[0])
				}
			},
		},
		{
			name:      "target-drift",
			path:      baseURL + "/targets/" + prodEUAPI + "/drift",
			routePath: baseURL + "/targets/x/drift",
			check: func(t *testing.T, b []byte) {
				d := decodeJSON[openapi.Diff](t, b)
				if d.Mode != "git-live" {
					t.Errorf("mode = %q, want git-live", d.Mode)
				}
				if d.Right == nil || *d.Right != "prod-eu/api" {
					t.Errorf("right = %v, want prod-eu/api", d.Right)
				}
				if len(d.Entries) < 1 {
					t.Errorf("entries = %d, want >= 1 (prod-eu/api is drifting)", len(d.Entries))
				}
			},
		},
		{
			name: "environments",
			path: baseURL + "/environments",
			check: func(t *testing.T, b []byte) {
				es := decodeJSON[[]openapi.Environment](t, b)
				if len(es) != 1 {
					t.Fatalf("environments = %d, want 1", len(es))
				}
				if es[0].Name != "prod" {
					t.Errorf("environments[0].name = %q, want prod", es[0].Name)
				}
			},
		},
		{
			name: "diff",
			path: baseURL + "/diff?mode=git-live",
			check: func(t *testing.T, b []byte) {
				d := decodeJSON[openapi.Diff](t, b)
				if d.Mode != "git-live" {
					t.Errorf("mode = %q, want git-live", d.Mode)
				}
				if len(d.Entries) != 1 {
					t.Fatalf("entries = %d, want 1", len(d.Entries))
				}
				if d.Entries[0].Object != "Deployment/api" {
					t.Errorf("entries[0].object = %q, want Deployment/api", d.Entries[0].Object)
				}
			},
		},
		{
			name: "audit",
			path: baseURL + "/audit",
			check: func(t *testing.T, b []byte) {
				resp := decodeJSON[openapi.GetAudit200JSONResponse](t, b)
				if len(resp.Items) != 3 {
					t.Fatalf("audit items = %d, want 3", len(resp.Items))
				}
				// QueryAudit returns newest-first; the latest fixture event is alice's approve.
				if resp.Items[0].Actor != "alice" || resp.Items[0].Verb != "approve" {
					t.Errorf("items[0] = %+v, want alice/approve", resp.Items[0])
				}
			},
		},
	}
}

// TestReadEndpoints exercises every GET on the contract: each returns 200 and a
// body that decodes into its openapi type with the expected fixture values.
func TestReadEndpoints(t *testing.T) {
	h := newHandler()
	for _, tc := range readCases() {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(t, h, http.MethodGet, tc.path, nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
			}
			tc.check(t, rec.Body.Bytes())
		})
	}
}

// TestPostPromotionsNotImplemented confirms the write surface is inert: a
// well-formed request still yields 501/not_implemented (ADR 0003 — desired
// state changes only via Git, never a live override).
func TestPostPromotionsNotImplemented(t *testing.T) {
	h := newHandler()
	body := strings.NewReader(`{"application":"api","from":"staging","to":"prod"}`)
	rec := serve(t, h, http.MethodPost, baseURL+"/promotions", body)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501 (body: %s)", rec.Code, rec.Body.String())
	}
	e := decodeJSON[openapi.Problem](t, rec.Body.Bytes())
	if e.Type != "https://keleustes.skaphos.io/errors/not_implemented" {
		t.Errorf("type = %q, want .../not_implemented", e.Type)
	}
}

// TestGetApplicationUnknownReturns404 confirms readmodel.ErrNotFound maps to
// 404/not_found at the wire.
func TestGetApplicationUnknownReturns404(t *testing.T) {
	h := newHandler()
	rec := serve(t, h, http.MethodGet, baseURL+"/applications/does-not-exist", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}
	e := decodeJSON[openapi.Problem](t, rec.Body.Bytes())
	if e.Type != "https://keleustes.skaphos.io/errors/not_found" {
		t.Errorf("type = %q, want .../not_found", e.Type)
	}
}

// TestReadResponsesConformToSpec validates each read response against the
// embedded contract. GetSwagger().Validate confirms the spec itself loads and
// is internally consistent; the legacy router resolves each tested path to its
// operation; openapi3filter.ValidateResponse then checks the body, status, and
// content type against that operation's response schema.
//
// The legacy router (not gorillamux) is used deliberately: it resolves routes
// with only the kin-openapi dependency already on the module graph, whereas
// gorillamux would pull in github.com/gorilla/mux as a new dependency.
func TestReadResponsesConformToSpec(t *testing.T) {
	ctx := context.Background()

	swagger, err := openapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}
	if err := swagger.Validate(ctx); err != nil {
		t.Fatalf("spec is not valid: %v", err)
	}
	router, err := legacy.NewRouter(swagger)
	if err != nil {
		t.Fatalf("build router: %v", err)
	}

	h := newHandler()
	for _, tc := range readCases() {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
			}

			routePath := tc.routePath
			if routePath == "" {
				routePath = tc.path
			}
			routeReq := httptest.NewRequest(http.MethodGet, routePath, nil)
			route, pathParams, err := router.FindRoute(routeReq)
			if err != nil {
				t.Fatalf("no spec route for %s: %v", routePath, err)
			}
			respInput := &openapi3filter.ResponseValidationInput{
				RequestValidationInput: &openapi3filter.RequestValidationInput{
					Request:    routeReq,
					PathParams: pathParams,
					Route:      route,
				},
				Status: rec.Code,
				Header: rec.Header(),
			}
			respInput.SetBodyBytes(rec.Body.Bytes())
			if err := openapi3filter.ValidateResponse(ctx, respInput); err != nil {
				t.Errorf("response does not conform to spec: %v", err)
			}
		})
	}
}
