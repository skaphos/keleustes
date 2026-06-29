/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/skaphos/keleustes/internal/api/auth"
	"github.com/skaphos/keleustes/internal/api/openapi"
)

// authzMiddleware is the single authorization checkpoint. It runs as a strict
// handler middleware, so every operation flows through it before its handler:
// it resolves the caller's Identity (stamped by auth.Middleware), derives the
// (verb, resource) the operation requires from its id, and asks the Authorizer.
// A deny becomes a forbiddenError carrying the verb+resource, which onError maps
// to a 403 problem (type=forbidden) whose body names them (ADR 0009 §1).
//
// The default Authorizer is AllowAll, so this is permissive today and changes
// no behavior. The point is that the enforcement seam now exists: the real
// ADR 0004 §11 evaluator (SKA-330) drops in behind auth.Authorizer with no
// handler or contract change, because the call site is already here.
func (s *Server) authzMiddleware(f openapi.StrictHandlerFunc, operationID string) openapi.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		verb, resource := authzForOperation(operationID)
		// Identity is resolved here so the seam's inputs sit in one place;
		// SKA-330 will derive per-resource scope from its claims.
		_, _ = auth.FromContext(ctx)
		if !s.authz.Can(ctx, verb, resource, "") {
			// forbiddenError carries verb+resource so the problem body can name
			// them (ADR 0009 §1); onError maps it to 403/forbidden.
			return nil, &forbiddenError{verb: verb, resource: resource}
		}
		return f(ctx, w, r, request)
	}
}

// authzForOperation maps a generated operation id to the (verb, resource) the
// policy evaluator reasons about. Reads share the "read" verb; the three
// sanctioned writes (ADR 0003 §6) each carry their own.
func authzForOperation(operationID string) (verb, resource string) {
	switch operationID {
	case "PostPromotions":
		return "promote", "promotions"
	case "PostPromotionsIDApprove":
		return "approve", "promotions"
	case "PostPromotionsIDCancel":
		return "cancel", "promotions"
	case "PostPromotionsIDRetry":
		return "retry", "promotions"
	default:
		return "read", resourceOf(operationID)
	}
}

// resourceOf extracts the resource family from a read operation id, e.g.
// "GetApplicationsNameMatrix" -> "applications". The id is the operation's
// generated name: the HTTP verb followed by the path segments in PascalCase.
func resourceOf(operationID string) string {
	name := strings.TrimPrefix(operationID, "Get")
	switch {
	case strings.HasPrefix(name, "Applications"):
		return "applications"
	case strings.HasPrefix(name, "Promotions"):
		return "promotions"
	case strings.HasPrefix(name, "Releases"):
		return "releases"
	case strings.HasPrefix(name, "Targets"):
		return "targets"
	case strings.HasPrefix(name, "Environments"):
		return "environments"
	case strings.HasPrefix(name, "Diff"):
		return "diff"
	case strings.HasPrefix(name, "Audit"):
		return "audit"
	default:
		return strings.ToLower(name)
	}
}
