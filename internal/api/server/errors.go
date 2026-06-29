/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/skaphos/keleustes/internal/api/openapi"
	"github.com/skaphos/keleustes/internal/api/readmodel"
)

// errorBaseURI is the stable namespace for the RFC 9457 problem `type` slugs
// (ADR 0009 §1). Clients branch on the full URI, never on the `detail` prose.
const errorBaseURI = "https://keleustes.skaphos.io/errors/"

// errNotImplemented marks the promotion write surface inert until the
// Git-mutation engine lands (ADR 0003). onError maps it to 501/not_implemented.
var errNotImplemented = errors.New("write path not implemented: Git-mutation engine pending (ADR 0003)")

// forbiddenError is a 403 that names the verb+resource the caller lacks so the
// problem body can carry them (ADR 0004 / ADR 0009 §1). The authz checkpoint
// raises it; a bare readmodel.ErrForbidden still maps to 403, just without the
// verb/resource detail.
type forbiddenError struct {
	verb     string
	resource string
}

func (e *forbiddenError) Error() string {
	return "forbidden: missing " + e.verb + " on " + e.resource
}

// onError is the single error sink for both the strict handler (request decode
// and handler-returned errors) and the generated router (param binding). It
// classifies the error onto an RFC 9457 problem and writes
// application/problem+json (ADR 0009 §1).
func (s *Server) onError(w http.ResponseWriter, r *http.Request, err error) {
	p := classify(err)
	p.Instance = strPtr(r.URL.Path)
	if p.Status == http.StatusInternalServerError {
		// Degraded responses hide their cause on the wire; log it instead.
		s.log.Error(err, "request failed",
			"path", r.URL.Path,
			"requestId", requestIDFromContext(r.Context()),
		)
	}
	writeProblem(w, p)
}

// classify maps an error to its RFC 9457 problem (status + `type` slug + title).
// Sentinels match with errors.Is/As; the generated param-binding errors are
// client faults (400/invalid).
func classify(err error) openapi.Problem {
	switch {
	case errors.Is(err, errNotImplemented):
		return problem(http.StatusNotImplemented, "not_implemented", "Not implemented", err.Error())
	case errors.Is(err, readmodel.ErrNotFound):
		return problem(http.StatusNotFound, "not_found", "Not found", "the requested resource does not exist")
	case isParamError(err):
		return problem(http.StatusBadRequest, "invalid", "Invalid request", err.Error())
	}

	var fe *forbiddenError
	if errors.As(err, &fe) {
		p := problem(http.StatusForbidden, "forbidden", "Forbidden", "caller lacks the required verb on the resource")
		p.Verb = strPtrIfSet(fe.verb)
		p.Resource = strPtrIfSet(fe.resource)
		return p
	}
	if errors.Is(err, readmodel.ErrForbidden) {
		return problem(http.StatusForbidden, "forbidden", "Forbidden", "caller lacks the required verb on the resource")
	}

	// Default: degraded. The detail is fixed and the real cause is logged by
	// onError — it never reaches the wire.
	return problem(http.StatusInternalServerError, "degraded", "Internal error", "internal error")
}

// problem builds a base RFC 9457 Problem: the `type` URI plus the required
// title/status and a human-readable detail.
func problem(status int, slug, title, detail string) openapi.Problem {
	return openapi.Problem{
		Type:   errorBaseURI + slug,
		Title:  title,
		Status: status,
		Detail: strPtr(detail),
	}
}

// isParamError reports whether err is one of the generated request-binding
// failures, all of which are client faults (400/invalid).
func isParamError(err error) bool {
	var (
		required   *openapi.RequiredParamError
		badFormat  *openapi.InvalidParamFormatError
		tooMany    *openapi.TooManyValuesForParamError
		badUnmarsh *openapi.UnmarshalingParamError
		header     *openapi.RequiredHeaderError
	)
	return errors.As(err, &required) ||
		errors.As(err, &badFormat) ||
		errors.As(err, &tooMany) ||
		errors.As(err, &badUnmarsh) ||
		errors.As(err, &header)
}

// writeProblem emits an application/problem+json body with the problem's status.
func writeProblem(w http.ResponseWriter, p openapi.Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

// writeProblemStatus builds and writes a problem in one call, for error sites
// outside the classify path (spec-load failure, panic recovery).
func writeProblemStatus(w http.ResponseWriter, status int, slug, title, detail string) {
	writeProblem(w, problem(status, slug, title, detail))
}

func strPtr(s string) *string { return &s }

func strPtrIfSet(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
