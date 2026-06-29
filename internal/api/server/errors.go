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

// errNotImplemented marks the promotion write surface as inert until the
// Git-mutation engine lands (ADR 0003). onError maps it to 501/not_implemented.
var errNotImplemented = errors.New("write path not implemented: Git-mutation engine pending (ADR 0003)")

// onError is the single error sink for both the strict handler (request decode
// and handler-returned errors) and the generated router (param binding). It
// classifies the error onto a contract Error code and writes the JSON body.
func (s *Server) onError(w http.ResponseWriter, r *http.Request, err error) {
	status, code := classify(err)
	if status == http.StatusInternalServerError {
		// Degraded responses hide their cause on the wire; log it instead.
		s.log.Error(err, "request failed",
			"path", r.URL.Path,
			"requestId", requestIDFromContext(r.Context()),
		)
	}
	writeError(w, status, code, message(err, code))
}

// classify maps an error to its HTTP status and contract code. Sentinels are
// matched with errors.Is; the generated param-binding errors with errors.As.
func classify(err error) (int, openapi.ErrorCode) {
	switch {
	case errors.Is(err, errNotImplemented):
		return http.StatusNotImplemented, openapi.ErrorCodeNotImplemented
	case errors.Is(err, readmodel.ErrNotFound):
		return http.StatusNotFound, openapi.ErrorCodeNotFound
	case errors.Is(err, readmodel.ErrForbidden):
		return http.StatusForbidden, openapi.ErrorCodeForbidden
	case isParamError(err):
		return http.StatusBadRequest, openapi.ErrorCodeInvalid
	default:
		return http.StatusInternalServerError, openapi.ErrorCodeDegraded
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

// message returns the body message for a coded error. Degraded responses use a
// fixed string so internal failure detail never leaks to clients; every other
// code is safe to surface verbatim.
func message(err error, code openapi.ErrorCode) string {
	if code == openapi.ErrorCodeDegraded {
		return "internal error"
	}
	return err.Error()
}

// writeError emits a contract Error body with the matching status.
func writeError(w http.ResponseWriter, status int, code openapi.ErrorCode, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(openapi.Error{Code: code, Message: msg})
}
