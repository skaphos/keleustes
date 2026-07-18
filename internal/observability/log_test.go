/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package observability

import (
	"encoding/json"
	"testing"

	"github.com/go-logr/logr/funcr"
	"k8s.io/apimachinery/pkg/types"
)

func newJSONLogger() (sink func(string), entries func() []map[string]any) {
	var lines []map[string]any
	push := func(obj string) {
		var entry map[string]any
		if err := json.Unmarshal([]byte(obj), &entry); err == nil {
			lines = append(lines, entry)
		}
	}
	return push, func() []map[string]any { return lines }
}

func TestWithFields_AppendsBoundedKeys(t *testing.T) {
	t.Parallel()
	push, entries := newJSONLogger()
	log := funcr.NewJSON(push, funcr.Options{Verbosity: 1})

	WithFields(log, LogFields{
		Engine:              EngineSync,
		Kind:                "Application",
		Application:         "checkout-api",
		Environment:         "prod",
		Target:              "prod-guest-westus2",
		ReconcileGeneration: 17,
		TraceID:             "abc123",
	}).Info("reconcile.done")

	captured := entries()
	if len(captured) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(captured))
	}
	entry := captured[0]
	for k, want := range map[string]any{
		LabelEngine:                 EngineSync,
		LabelKind:                   "Application",
		LabelApplication:            "checkout-api",
		LabelEnvironment:            "prod",
		LabelTarget:                 "prod-guest-westus2",
		LogFieldReconcileGeneration: float64(17),
		LogFieldTraceID:             "abc123",
	} {
		got, ok := entry[k]
		if !ok {
			t.Errorf("missing field %q in log entry: %v", k, entry)
			continue
		}
		if got != want {
			t.Errorf("field %q: got %v (%T), want %v (%T)", k, got, got, want, want)
		}
	}
	if _, present := entry[LabelRegion]; present {
		t.Errorf("region label should be omitted when unset, got %v", entry[LabelRegion])
	}
}

func TestWithResource_EmitsEngineKindAndName(t *testing.T) {
	t.Parallel()
	push, entries := newJSONLogger()
	log := funcr.NewJSON(push, funcr.Options{Verbosity: 1})

	WithResource(log, EngineManager, "Application", types.NamespacedName{Namespace: "payments", Name: "checkout"}).
		Info("setup")

	captured := entries()
	if len(captured) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(captured))
	}
	entry := captured[0]
	if entry[LabelEngine] != EngineManager {
		t.Errorf("engine label: got %v, want %s", entry[LabelEngine], EngineManager)
	}
	if entry[LabelKind] != "Application" {
		t.Errorf("kind label: got %v, want Application", entry[LabelKind])
	}
	if entry["namespacedName"] != "payments/checkout" {
		t.Errorf("namespacedName: got %v, want payments/checkout", entry["namespacedName"])
	}
}
