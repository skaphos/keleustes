/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// apiTestCmd returns a command whose --api-url points at base (a bare host is
// fine — it exercises normalizeBaseURL), plus any extra string flags, with its
// output captured for assertions.
func apiTestCmd(base string, flags map[string]string) (*cobra.Command, *bytes.Buffer) {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background()) // a bare command has no context until executed
	cmd.Flags().String("api-url", base, "")
	for k, v := range flags {
		cmd.Flags().String(k, v, "")
	}
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	return cmd, out
}

// stubServer records the request path and replies with the given status/body.
func stubServer(t *testing.T, gotPath *string, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gotPath != nil {
			*gotPath = r.URL.Path
		}
		if status >= 400 {
			w.Header().Set("Content-Type", "application/problem+json")
		} else {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestRunAppListRendersFromServer(t *testing.T) {
	var path string
	srv := stubServer(t, &path, http.StatusOK,
		`{"asOf":"2026-06-01T00:00:00Z","items":[{"ulid":"01","name":"payments-api","project":"payments","status":"Degraded"}]}`)
	defer srv.Close()

	cmd, out := apiTestCmd(srv.URL, nil) // bare URL -> normalized to /api/v1
	if err := runAppList(cmd, nil); err != nil {
		t.Fatalf("runAppList: %v", err)
	}
	if path != "/api/v1/applications" {
		t.Errorf("path = %q, want /api/v1/applications", path)
	}
	if !strings.Contains(out.String(), "payments-api") {
		t.Errorf("output missing application name:\n%s", out.String())
	}
}

// TestRunReleaseListUsesAppScopedEndpoint pins the fix that `release list APP`
// calls GET /applications/{name}/releases, not the fleet-wide GET /releases.
func TestRunReleaseListUsesAppScopedEndpoint(t *testing.T) {
	var path string
	srv := stubServer(t, &path, http.StatusOK, `[{"version":"1.8.2","app":"payments-api"}]`)
	defer srv.Close()

	cmd, out := apiTestCmd(srv.URL, nil)
	if err := runReleaseList(cmd, []string{"payments-api"}); err != nil {
		t.Fatalf("runReleaseList: %v", err)
	}
	if path != "/api/v1/applications/payments-api/releases" {
		t.Errorf("path = %q, want app-scoped /api/v1/applications/payments-api/releases", path)
	}
	if !strings.Contains(out.String(), "1.8.2") {
		t.Errorf("output missing release version:\n%s", out.String())
	}
}

// TestRunBlockersUsesAppScopedEndpointAndFilters pins the fix that `blockers
// APP` calls GET /applications/{name}/promotions and keeps only Blocked
// promotions, narrowed to --to.
func TestRunBlockersUsesAppScopedEndpointAndFilters(t *testing.T) {
	var path string
	srv := stubServer(t, &path, http.StatusOK, `[
		{"ulid":"01","application":"payments-api","from":"staging","to":"prod","status":"Blocked"},
		{"ulid":"02","application":"payments-api","from":"qa","to":"dev","status":"Blocked"},
		{"ulid":"03","application":"payments-api","from":"canary","to":"prod","status":"Healthy"}
	]`)
	defer srv.Close()

	cmd, out := apiTestCmd(srv.URL, map[string]string{"to": "prod"})
	if err := runBlockers(cmd, []string{"payments-api"}); err != nil {
		t.Fatalf("runBlockers: %v", err)
	}
	if path != "/api/v1/applications/payments-api/promotions" {
		t.Errorf("path = %q, want app-scoped /api/v1/applications/payments-api/promotions", path)
	}
	s := out.String()
	if !strings.Contains(s, "staging") {
		t.Errorf("kept blocked promotion (from=staging) missing:\n%s", s)
	}
	if strings.Contains(s, "qa") {
		t.Errorf("blocked promotion to a different env (--to=prod) should be filtered out:\n%s", s)
	}
	if strings.Contains(s, "canary") {
		t.Errorf("non-blocked promotion should be filtered out:\n%s", s)
	}
}

// TestRunAppGetSurfacesProblem confirms a non-2xx RFC 9457 body becomes a Go
// error carrying the problem type slug (ADR 0009).
func TestRunAppGetSurfacesProblem(t *testing.T) {
	srv := stubServer(t, nil, http.StatusNotFound,
		`{"type":"https://keleustes.skaphos.io/errors/not_found","title":"Not found","status":404,"detail":"the requested resource does not exist"}`)
	defer srv.Close()

	cmd, _ := apiTestCmd(srv.URL, nil)
	err := runAppGet(cmd, []string{"ghost"})
	if err == nil {
		t.Fatal("expected an error for a 404 response")
	}
	if !strings.Contains(err.Error(), "not_found") {
		t.Errorf("error should carry the problem type slug, got: %v", err)
	}
}
