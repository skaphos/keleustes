/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package observability

// Label names. The set is closed: contributors must not introduce new label
// keys without amending the observability-stack plan §3.1 and adding the
// constant here. Anything not listed here belongs in a log field, not a metric
// label, to keep cardinality bounded.
const (
	// LabelEngine identifies which Keleustes engine emitted the signal. Bounded
	// at exactly the values in the Engine* constants below.
	LabelEngine = "engine"

	// LabelKind is the kubebuilder Kind name of the CRD being reconciled
	// (Application, SyncRun, Promotion, ...). Bounded by the CRD set in
	// api/v1alpha1.
	LabelKind = "kind"

	// LabelResult is the terminal outcome of a unit of engine work.
	// Bounded by the Result* constants.
	LabelResult = "result"

	// LabelPhase is the relevant Phase enum value from the affected resource's
	// Status. Bounded by the CRD's own phase enum.
	LabelPhase = "phase"

	// LabelApplication is an Application.metadata.name. Unbounded by the
	// customer's application count; only allowed on counters and gauges, never
	// on histograms (see observability-stack plan §3.1).
	LabelApplication = "application"

	// LabelEnvironment is an Environment.metadata.name. Bounded by the
	// customer's environment count.
	LabelEnvironment = "environment"

	// LabelTarget is a DeploymentTarget.metadata.name. Same cardinality
	// discipline as LabelApplication.
	LabelTarget = "target"

	// LabelRegion is the regional-agent region identifier. Used only on
	// agent-emitted metrics; bounded by the customer's region count.
	LabelRegion = "region"
)

// Engine identifiers. These values feed the LabelEngine label and the
// `keleustes.skaphos.io/engine` pod label that ServiceMonitor relabel rules
// (observability-stack plan §4.1) project into scraped metrics.
const (
	EngineManager     = "manager"
	EngineSource      = "source"
	EngineSync        = "sync"
	EnginePromotion   = "promotion"
	EngineGitMutation = "git_mutation"
	EnginePolicy      = "policy"
	EngineHealth      = "health"
	EngineDiff        = "diff"
	EngineWorker      = "worker"
)

// Result values for the LabelResult label and the audit envelope's
// result.outcome field. Aligned with audit-event-schema plan §3.
const (
	ResultSuccess  = "success"
	ResultError    = "error"
	ResultBlocked  = "blocked"
	ResultCanceled = "canceled"
	ResultTimeout  = "timeout"
)

// LogFieldEvent is the canonical key for the short verb-noun describing what
// just happened in a log line (e.g. "sync.applied"). Required on every
// info-level log emitted by an engine (observability-stack plan §3.3).
const LogFieldEvent = "event"

// LogFieldReconcileGeneration is the canonical key for the CRD generation the
// reconcile loop observed. Lets a reader correlate a log line to the audit
// envelope's action.subject.
const LogFieldReconcileGeneration = "reconcileGeneration"

// LogFieldTraceID and LogFieldSpanID carry the active OpenTelemetry trace
// context into the log line so the customer's log pipeline can join logs to
// traces (observability-stack plan §3.3 and §10).
const (
	LogFieldTraceID = "traceId"
	LogFieldSpanID  = "spanId"
)
