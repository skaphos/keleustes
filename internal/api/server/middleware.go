/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-logr/logr"

	"github.com/skaphos/keleustes/internal/api/openapi"
)

// headerRequestID is the correlation header read from the client or minted here
// and echoed back so a caller can trace a request across the audit log.
const headerRequestID = "X-Request-Id"

type ctxKeyRequestID struct{}

// requestIDMiddleware ensures every request carries a stable id: it honors a
// client-supplied X-Request-Id, otherwise mints one, stamps it on the context
// and echoes it on the response.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(headerRequestID)
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set(headerRequestID, id)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requestIDFromContext returns the id stamped by requestIDMiddleware, or "".
func requestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyRequestID{}).(string)
	return id
}

// newRequestID returns a random hex id, falling back to a fixed marker if the
// system entropy source fails (it effectively never does).
func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b[:])
}

// loggingMiddleware emits one structured access log per request after the
// handler returns, including the resolved status and latency.
func loggingMiddleware(log logr.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.V(1).Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"durationMs", time.Since(start).Milliseconds(),
				"requestId", requestIDFromContext(r.Context()),
			)
		})
	}
}

// recoverMiddleware converts a panic in any downstream handler into a logged
// 500 so a single bad request can never take the stateless server down.
func recoverMiddleware(log logr.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				log.Error(fmt.Errorf("panic: %v", rec), "recovered from panic",
					"path", r.URL.Path,
					"requestId", requestIDFromContext(r.Context()),
					"stack", string(debug.Stack()),
				)
				writeError(w, http.StatusInternalServerError, openapi.ErrorCodeDegraded, "internal error")
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// statusRecorder captures the response status code for the access log without
// otherwise altering the ResponseWriter behavior.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.status = code
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wroteHeader = true
	return r.ResponseWriter.Write(b)
}
