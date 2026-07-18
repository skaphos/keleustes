/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package observability

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

func TestRegister_IsIdempotent(t *testing.T) {
	// Two calls in one process must not panic on duplicate registration. The
	// suite_test bootstrap registers, then individual tests also call Register
	// from their own setup paths.
	_ = Register()
	_ = Register()
}

func TestObserveReconcileEvent_AccumulatesCounter(t *testing.T) {
	_ = Register()
	reconcileEvents.WithLabelValues(EngineSync, "Application", ResultSuccess).Add(0)
	before := testutil.ToFloat64(reconcileEvents.WithLabelValues(EngineSync, "Application", ResultSuccess))

	ObserveReconcileEvent(EngineSync, "Application", ResultSuccess)
	ObserveReconcileEvent(EngineSync, "Application", ResultSuccess)
	ObserveReconcileEvent(EngineSync, "Application", ResultError)

	gotSuccess := testutil.ToFloat64(reconcileEvents.WithLabelValues(EngineSync, "Application", ResultSuccess))
	gotError := testutil.ToFloat64(reconcileEvents.WithLabelValues(EngineSync, "Application", ResultError))
	if gotSuccess-before != 2 {
		t.Errorf("success counter delta: got %v, want 2", gotSuccess-before)
	}
	if gotError < 1 {
		t.Errorf("error counter: got %v, want >=1", gotError)
	}
}

func TestObserveReconcileGeneration_SetsGauge(t *testing.T) {
	_ = Register()
	ObserveReconcileGeneration(EngineManager, "Application", 42)
	got := testutil.ToFloat64(reconcileGeneration.WithLabelValues(EngineManager, "Application"))
	if got != 42 {
		t.Errorf("gauge: got %v, want 42", got)
	}
}

func TestRegister_AttachesToControllerRuntimeRegistry(t *testing.T) {
	_ = Register()
	// Gather and assert that at least one keleustes-namespaced metric is
	// present on the controller-runtime registry. We do a substring check
	// rather than parsing the full text exposition.
	got, err := ctrlmetrics.Registry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	var names []string
	for _, mf := range got {
		names = append(names, mf.GetName())
	}
	joined := strings.Join(names, ",")
	for _, want := range []string{
		"keleustes_reconcile_events_total",
		"keleustes_reconcile_observed_generation",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing metric %q in registry; have: %s", want, joined)
		}
	}
}
