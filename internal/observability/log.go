/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package observability

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
)

// LogFields carries the standard structured-log keys every Keleustes engine
// emits. Unset values are skipped so a logger is never burdened with empty
// fields. See observability-stack plan §3.3.
type LogFields struct {
	Engine              string
	Kind                string
	Application         string
	Environment         string
	Target              string
	Region              string
	ReconcileGeneration int64
	TraceID             string
	SpanID              string
}

// WithFields returns a logger that has the Keleustes standard fields baked in.
// Pass it the logger you already pulled from logf.FromContext(ctx); call this
// once per reconcile, then use the returned logger for the rest of the loop.
func WithFields(log logr.Logger, f LogFields) logr.Logger {
	kv := make([]any, 0, 18)
	if f.Engine != "" {
		kv = append(kv, LabelEngine, f.Engine)
	}
	if f.Kind != "" {
		kv = append(kv, LabelKind, f.Kind)
	}
	if f.Application != "" {
		kv = append(kv, LabelApplication, f.Application)
	}
	if f.Environment != "" {
		kv = append(kv, LabelEnvironment, f.Environment)
	}
	if f.Target != "" {
		kv = append(kv, LabelTarget, f.Target)
	}
	if f.Region != "" {
		kv = append(kv, LabelRegion, f.Region)
	}
	if f.ReconcileGeneration != 0 {
		kv = append(kv, LogFieldReconcileGeneration, f.ReconcileGeneration)
	}
	if f.TraceID != "" {
		kv = append(kv, LogFieldTraceID, f.TraceID)
	}
	if f.SpanID != "" {
		kv = append(kv, LogFieldSpanID, f.SpanID)
	}
	return log.WithValues(kv...)
}

// WithResource is a shorthand for the common case of decorating a reconcile
// logger with engine + kind + namespaced name. Returns the original logger
// when nn is empty so callers don't have to branch.
func WithResource(log logr.Logger, engine, kind string, nn types.NamespacedName) logr.Logger {
	out := log.WithValues(LabelEngine, engine, LabelKind, kind)
	if nn.Name == "" && nn.Namespace == "" {
		return out
	}
	return out.WithValues("namespacedName", nn.String())
}
