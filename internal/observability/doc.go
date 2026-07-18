/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

// Package observability provides the cross-cutting metric, log, and trace
// conventions every Keleustes engine consumes. See
// docs/plans/2026-05-observability-stack.md for the design.
//
// The package owns:
//
//   - The closed label vocabulary (engine, application, environment, target,
//     result, phase, region) so every Keleustes metric and log field is named
//     identically across packages.
//   - A logr-compatible field helper that pins the mandatory log keys used by
//     dashboards and alerts.
//   - Constructors for the Keleustes-specific per-kind reconcile metrics.
//     Controller-runtime emits its own metrics on /metrics; these helpers add
//     the Keleustes-engine dimension without colliding with the upstream ones.
//
// Per-engine metrics, dashboards, and SLO recording rules land alongside the
// engines themselves in MVPs 1–3 (observability-stack plan §13).
package observability
