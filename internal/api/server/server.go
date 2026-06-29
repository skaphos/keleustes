/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package server

import (
	"net/http"

	"github.com/go-logr/logr"
	"sigs.k8s.io/yaml"

	"github.com/skaphos/keleustes/internal/api/auth"
	"github.com/skaphos/keleustes/internal/api/openapi"
	"github.com/skaphos/keleustes/internal/api/readmodel"
)

// baseURL is where the contract is mounted; the generated router prefixes every
// operation path with it.
const baseURL = "/api/v1"

// Options configures a Server.
type Options struct {
	// Auth tunes the bearer-token middleware (dev passthrough vs. required).
	Auth auth.Config
	// Authz answers verb-scoped permission questions. Defaults to AllowAll.
	Authz auth.Authorizer
	// Logger receives access logs and recovered-panic reports.
	Logger logr.Logger
}

// Server implements openapi.StrictServerInterface against a ReadModel.
type Server struct {
	rm      readmodel.ReadModel
	authz   auth.Authorizer
	authCfg auth.Config
	log     logr.Logger
}

var _ openapi.StrictServerInterface = (*Server)(nil)

// New builds a Server over rm. A nil Authz defaults to AllowAll so the shell is
// navigable before the real policy evaluator (ADR 0004 §11) is wired in.
func New(rm readmodel.ReadModel, opts Options) *Server {
	authz := opts.Authz
	if authz == nil {
		authz = auth.AllowAll()
	}
	return &Server{
		rm:      rm,
		authz:   authz,
		authCfg: opts.Auth,
		log:     opts.Logger,
	}
}

// Handler returns the fully wired http.Handler: the contract routes under
// /api/v1, the spec and liveness/readiness probes, all behind the outer
// middleware chain (auth -> requestID -> logging -> recover).
func (s *Server) Handler() http.Handler {
	// authzMiddleware is the in-process authorization checkpoint (ADR 0004 §11);
	// it runs for every operation before the handler. AllowAll keeps it open.
	strict := openapi.NewStrictHandlerWithOptions(s, []openapi.StrictMiddlewareFunc{s.authzMiddleware}, openapi.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  s.onError,
		ResponseErrorHandlerFunc: s.onError,
	})

	mux := http.NewServeMux()
	openapi.HandlerWithOptions(strict, openapi.StdHTTPServerOptions{
		BaseURL:          baseURL,
		BaseRouter:       mux,
		ErrorHandlerFunc: s.onError,
	})

	mux.HandleFunc("GET "+baseURL+"/openapi.yaml", s.handleSpec)
	mux.HandleFunc("GET /healthz", handleOK)
	mux.HandleFunc("GET /readyz", handleOK)

	// Build inside-out so requests flow auth -> requestID -> logging -> recover.
	var h http.Handler = mux
	h = recoverMiddleware(s.log)(h)
	h = loggingMiddleware(s.log)(h)
	h = requestIDMiddleware(h)
	h = auth.Middleware(s.authCfg)(h)
	return h
}

// handleSpec serves the OpenAPI contract as YAML so clients and codegen can
// fetch the live document the server was built from.
func (s *Server) handleSpec(w http.ResponseWriter, _ *http.Request) {
	swagger, err := openapi.GetSpec()
	if err != nil {
		writeProblemStatus(w, http.StatusInternalServerError, "degraded", "Internal error", "internal error")
		return
	}
	out, err := yaml.Marshal(swagger)
	if err != nil {
		writeProblemStatus(w, http.StatusInternalServerError, "degraded", "Internal error", "internal error")
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

// handleOK is the probe response for /healthz and /readyz.
func handleOK(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
