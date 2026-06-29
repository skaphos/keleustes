/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

// Command apiserver is the Keleustes read/interaction API server. It is a
// stateless, freely-replicated component (ADR 0005 §9/§10), distinct from the
// controller-manager: it serves the OpenAPI contract that the UI and
// keleustesctl consume. The gateway terminates TLS and routes to the
// keleustes-api Service on :8443, so this process speaks plain HTTP.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	// Pull in the standard client-go auth providers (Azure, GCP, OIDC, ...)
	// so out-of-cluster runs of the CRD backend can authenticate.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	keleustesv1alpha1 "github.com/skaphos/keleustes/api/v1alpha1"
	"github.com/skaphos/keleustes/internal/api/auth"
	"github.com/skaphos/keleustes/internal/api/readmodel"
	"github.com/skaphos/keleustes/internal/api/readmodel/crdsource"
	"github.com/skaphos/keleustes/internal/api/readmodel/fixtures"
	"github.com/skaphos/keleustes/internal/api/server"
)

// shutdownTimeout bounds graceful drain of in-flight requests on signal.
const shutdownTimeout = 10 * time.Second

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(keleustesv1alpha1.AddToScheme(scheme))
}

func main() {
	var (
		listenAddr   string
		probeAddr    string
		readModel    string
		authRequired bool
	)

	flag.StringVar(&listenAddr, "listen-address", ":8443",
		"The address the API server binds to (plain HTTP; TLS terminates at the gateway).")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081",
		"The address the liveness/readiness probe endpoint binds to.")
	flag.StringVar(&readModel, "read-model", "fixtures",
		"Read-model backend: one of fixtures|crd.")
	flag.BoolVar(&authRequired, "auth-required", false,
		"Require a bearer token on every request. When false, requests run as the stub operator.")
	// --kubeconfig is registered by controller-runtime's config package; the crd
	// read-model honors it (plus KUBECONFIG and in-cluster) via ctrl.GetConfig().

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	rm, err := buildReadModel(readModel)
	if err != nil {
		setupLog.Error(err, "unable to build read model", "backend", readModel)
		os.Exit(1)
	}

	srv := server.New(rm, server.Options{
		Auth:   auth.Config{Required: authRequired},
		Authz:  auth.AllowAll(),
		Logger: ctrl.Log.WithName("apiserver"),
	})

	httpSrv := &http.Server{
		Addr:              listenAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	// Dedicated probe listener so kubelet can check liveness/readiness on a
	// stable port independent of the gateway-facing API listener.
	probeSrv := &http.Server{
		Addr:              probeAddr,
		Handler:           probeHandler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx := ctrl.SetupSignalHandler()

	go serve(probeSrv, "health probe server", "")
	go serve(httpSrv, "api server", "listening on "+listenAddr+" (backend: "+readModel+")")

	<-ctx.Done()
	setupLog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		setupLog.Error(err, "graceful shutdown failed")
		os.Exit(1)
	}
	_ = probeSrv.Shutdown(shutdownCtx)
}

// serve binds the listener and then runs srv.Serve, treating any non-shutdown
// error as fatal so a failed bind takes the process down instead of silently
// degrading. Binding before announce means the "listening" log can never claim
// a port that is actually in use; announce, when non-empty, is logged only once
// the socket is bound and about to accept.
func serve(srv *http.Server, name, announce string) {
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		setupLog.Error(err, "server failed to bind", "server", name, "address", srv.Addr)
		os.Exit(1)
	}
	if announce != "" {
		setupLog.Info(announce)
	}
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		setupLog.Error(err, "server failed", "server", name)
		os.Exit(1)
	}
}

// buildReadModel selects the interaction-layer backend. fixtures serves the
// in-memory golden dataset (offline UI/CLI development); crd reads live state
// through a controller-runtime client.
func buildReadModel(backend string) (readmodel.ReadModel, error) {
	switch backend {
	case "fixtures":
		return fixtures.New(), nil
	case "crd":
		c, err := newClusterClient()
		if err != nil {
			return nil, err
		}
		return crdsource.New(c), nil
	default:
		return nil, fmt.Errorf("unknown --read-model %q: want fixtures or crd", backend)
	}
}

// newClusterClient builds a direct (uncached) controller-runtime client with
// the keleustes.skaphos.io types registered. Config resolution follows
// controller-runtime's rules — the --kubeconfig flag, then KUBECONFIG, then
// in-cluster. Direct reads suit a stateless API server with no cache to prime.
func newClusterClient() (client.Client, error) {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("load rest config: %w", err)
	}
	return client.New(cfg, client.Options{Scheme: scheme})
}

// probeHandler answers liveness/readiness on the dedicated probe listener.
func probeHandler() http.Handler {
	ok := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", ok)
	mux.HandleFunc("GET /readyz", ok)
	return mux
}
