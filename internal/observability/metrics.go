/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Namespace is the Prometheus metric namespace for every Keleustes-specific
// metric. Engine-specific subsystems live under this namespace; controller-
// runtime's own metrics are emitted in their own namespace and are not affected.
const Namespace = "keleustes"

var (
	registerOnce sync.Once

	// reconcileEvents counts engine-driven status transitions. Engines call
	// ObserveReconcileEvent(engine, kind, result) once per Reconcile that
	// observed a generation change. Bounded labels only: see labels.go.
	reconcileEvents *prometheus.CounterVec

	// reconcileGeneration is the highest observed-generation per (engine, kind,
	// application, environment). Exposes a coarse freshness signal to
	// dashboards without per-resource cardinality on a histogram.
	reconcileGeneration *prometheus.GaugeVec
)

// Register wires the Keleustes-specific metrics into the controller-runtime
// metrics registry. Idempotent — safe to call from main and from suite_test
// without double-registration. Returns the registerer it used so tests can
// observe collected metric families.
func Register() prometheus.Registerer {
	registerOnce.Do(func() {
		reconcileEvents = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: Namespace,
				Subsystem: "reconcile",
				Name:      "events_total",
				Help:      "Count of reconcile transitions that materially changed status, by engine, kind, and result.",
			},
			[]string{LabelEngine, LabelKind, LabelResult},
		)
		reconcileGeneration = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: Namespace,
				Subsystem: "reconcile",
				Name:      "observed_generation",
				Help:      "Latest observedGeneration reflected into status, by engine and kind. Application/environment dimensions are added only where cardinality allows.",
			},
			[]string{LabelEngine, LabelKind},
		)
		ctrlmetrics.Registry.MustRegister(reconcileEvents, reconcileGeneration)
	})
	return ctrlmetrics.Registry
}

// ObserveReconcileEvent records one reconcile-level event. Skip on no-op
// reconciles (where status was already equal); only call on transitions.
// Unknown engine or result values still record — labels are validated at lint
// time, not at runtime, so a typo here does not crash the manager.
func ObserveReconcileEvent(engine, kind, result string) {
	if reconcileEvents == nil {
		return
	}
	reconcileEvents.WithLabelValues(engine, kind, result).Inc()
}

// ObserveReconcileGeneration sets the latest observedGeneration gauge for a
// given engine/kind pair. Call after a successful Status().Update with the
// generation that was just recorded.
func ObserveReconcileGeneration(engine, kind string, generation int64) {
	if reconcileGeneration == nil {
		return
	}
	reconcileGeneration.WithLabelValues(engine, kind).Set(float64(generation))
}
