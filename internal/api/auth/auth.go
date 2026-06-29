/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/skaphos/keleustes/internal/api/openapi"
	"github.com/skaphos/keleustes/internal/audit"
)

// Identity is the authenticated principal behind a request. Optional in the
// scaffold sense: every field except the OIDC subject may be empty until real
// token claims (SKA-330) populate them.
type Identity struct {
	Type      string
	Subject   string
	SubjectID string
	Name      string
	Email     string
	IDP       string
	Groups    []string
}

// Actor projects the request identity onto the audit envelope's actor shape
// (audit-event-schema §6.3) so emitted events name the real principal.
func (i Identity) Actor() audit.Actor {
	return audit.Actor{
		Type:             actorType(i.Type),
		Subject:          i.Subject,
		SubjectID:        i.SubjectID,
		IdentityProvider: i.IDP,
		Groups:           i.Groups,
	}
}

// actorType maps the identity's free-form type onto the closed audit actor
// enum. "human" and any unrecognized value resolve to the human actor — the
// API server only mints human identities today.
func actorType(t string) audit.ActorType {
	switch t {
	case string(audit.ActorCI):
		return audit.ActorCI
	case string(audit.ActorAgent):
		return audit.ActorAgent
	case string(audit.ActorSystem):
		return audit.ActorSystem
	default:
		return audit.ActorHuman
	}
}

// StubIdentity is the offline dev operator. Values mirror the UI auth stub
// (ui/src/auth/auth.tsx) so the server and shell agree on who "nobody" is.
func StubIdentity() Identity {
	return Identity{
		Type:    "human",
		Subject: "u_stub",
		Name:    "Dev Operator",
		Email:   "operator@keleustes.local",
		IDP:     "stub-oidc",
	}
}

// Authorizer answers verb-scoped permission questions. The ADR 0004 §11
// policy evaluator implements this interface in production.
type Authorizer interface {
	Can(ctx context.Context, verb, resource, scope string) bool
}

// AllowAll permits every action. It keeps the shell navigable before the real
// evaluator exists; production wiring replaces it.
func AllowAll() Authorizer { return allowAll{} }

type allowAll struct{}

func (allowAll) Can(context.Context, string, string, string) bool { return true }

// Config tunes the auth middleware.
type Config struct {
	// Required gates whether a bearer token is mandatory. When false the
	// middleware is a dev passthrough that injects the stub operator.
	Required bool
}

// Middleware injects the request Identity into the context for downstream
// handlers.
//
// Required=false: dev passthrough — always inject the stub operator so
// handlers can resolve an identity offline.
//
// Required=true: a "Authorization: Bearer <token>" header is mandatory; an
// absent or empty one yields a 401 JSON Error. The token signature is NOT
// validated here — real OIDC/JWKS lands with SKA-330; for now a presented
// bearer synthesizes an identity.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Required {
				ctx := withIdentity(r.Context(), StubIdentity())
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			tok, ok := bearerToken(r)
			if !ok {
				writeUnauthenticated(w)
				return
			}

			ctx := withIdentity(r.Context(), identityFromToken(tok))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// identityFromToken synthesizes an identity from a presented bearer. The real
// implementation (SKA-330) decodes and verifies OIDC claims; the scaffold
// trusts the caller and returns the stub operator.
func identityFromToken(_ string) Identity { return StubIdentity() }

// bearerToken extracts a non-empty token from the Authorization header.
func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	tok := strings.TrimSpace(h[len(prefix):])
	if tok == "" {
		return "", false
	}
	return tok, true
}

func writeUnauthenticated(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(openapi.Error{
		Code:    openapi.ErrorCodeUnauthenticated,
		Message: "authentication required",
	})
}

// contextKey is unexported so only this package can stamp an identity onto a
// context, preventing collisions with other packages' context values.
type contextKey struct{}

func withIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// FromContext returns the Identity stamped by Middleware, if any.
func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(contextKey{}).(Identity)
	return id, ok
}
