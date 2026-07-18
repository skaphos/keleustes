/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/skaphos/keleustes/internal/api/openapi"
	"github.com/skaphos/keleustes/internal/audit"
)

func TestPassthroughInjectsStubIdentity(t *testing.T) {
	var (
		got Identity
		ok  bool
	)
	h := Middleware(Config{Required: false})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/applications", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !ok {
		t.Fatal("passthrough should inject an identity")
	}
	if want := StubIdentity(); !reflect.DeepEqual(got, want) {
		t.Errorf("identity = %+v, want %+v", got, want)
	}
}

func TestRequiredWithoutHeaderReturns401(t *testing.T) {
	h := Middleware(Config{Required: true})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler must not run without authentication")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/applications", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	var body openapi.Problem
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	const want = "https://keleustes.skaphos.io/errors/unauthenticated"
	if body.Type != want {
		t.Errorf("type = %q, want %q", body.Type, want)
	}
}

func TestRequiredWithBearerInjectsIdentity(t *testing.T) {
	var ok bool
	h := Middleware(Config{Required: true})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/applications", nil)
	req.Header.Set("Authorization", "Bearer abc.def.ghi")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !ok {
		t.Fatal("authenticated request should carry an identity")
	}
}

func TestFromContextRoundTrip(t *testing.T) {
	if _, ok := FromContext(context.Background()); ok {
		t.Fatal("bare context must not carry an identity")
	}

	want := StubIdentity()
	got, ok := FromContext(withIdentity(context.Background(), want))
	if !ok {
		t.Fatal("expected identity after withIdentity")
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("identity = %+v, want %+v", got, want)
	}
}

func TestStubIdentityActorMapsHuman(t *testing.T) {
	a := StubIdentity().Actor()
	if a.Type != audit.ActorHuman {
		t.Errorf("actor type = %q, want %q", a.Type, audit.ActorHuman)
	}
	if a.Subject != "u_stub" {
		t.Errorf("actor subject = %q, want %q", a.Subject, "u_stub")
	}
	if a.IdentityProvider != "stub-oidc" {
		t.Errorf("actor identityProvider = %q, want %q", a.IdentityProvider, "stub-oidc")
	}
}

func TestAllowAllPermitsEverything(t *testing.T) {
	if !AllowAll().Can(context.Background(), "promote", "applications/foo", "prod") {
		t.Error("AllowAll().Can should always be true")
	}
}
