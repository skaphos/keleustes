/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package audit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-logr/logr/funcr"

	"github.com/skaphos/keleustes/internal/audit/payloads"
)

func newCRDWriteEnv(verb Verb) Envelope {
	return Envelope{
		Actor:  Actor{Type: ActorHuman, Subject: "alice@example.com"},
		Action: Action{Verb: verb, Subject: ActionSubject{Kind: "Application", Name: "checkout"}},
		Result: Result{Outcome: OutcomeSuccess},
	}
}

func TestEmit_FillsEventIDAndOccurredAt(t *testing.T) {
	t.Parallel()
	em := NewInMemoryEmitter()
	id, err := Emit(context.Background(), em, newCRDWriteEnv(VerbCreate), payloads.CRDWriteV1{})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if err := validULID(id); err != nil {
		t.Errorf("returned eventId invalid: %v (%q)", err, id)
	}
	events := em.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	got := events[0]
	if got.SchemaVersion != SchemaVersion {
		t.Errorf("schemaVersion: got %q, want %q", got.SchemaVersion, SchemaVersion)
	}
	if got.OccurredAt.IsZero() {
		t.Errorf("occurredAt should have been set by Emit")
	}
}

func TestEmit_RejectsWrongPayloadType(t *testing.T) {
	t.Parallel()
	em := NewInMemoryEmitter()
	// "view" is envelope-only — passing any payload must error.
	_, err := Emit(context.Background(), em, newCRDWriteEnv(VerbView), payloads.CRDWriteV1{})
	if err == nil {
		t.Fatalf("expected error for view verb with payload, got nil")
	}
}

func TestEmit_RejectsUnregisteredVerb(t *testing.T) {
	t.Parallel()
	em := NewInMemoryEmitter()
	env := newCRDWriteEnv("not-a-verb")
	_, err := Emit(context.Background(), em, env, payloads.CRDWriteV1{})
	if err == nil {
		t.Fatalf("expected error for unregistered verb, got nil")
	}
	if !strings.Contains(err.Error(), "registry") {
		t.Errorf("error should mention the registry: %v", err)
	}
}

func TestEmit_PayloadWrappedWithAtType(t *testing.T) {
	t.Parallel()
	em := NewInMemoryEmitter()
	_, err := Emit(context.Background(), em, newCRDWriteEnv(VerbCreate), payloads.CRDWriteV1{SubresourceWrite: true})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	got := em.Events()[0]
	asMap, ok := got.Payload.(map[string]any)
	if !ok {
		t.Fatalf("payload should be map[string]any after wrapping, got %T", got.Payload)
	}
	if asMap["@type"] != "crd.write.v1" {
		t.Errorf("payload @type: got %v, want crd.write.v1", asMap["@type"])
	}
	if asMap["subresourceWrite"] != true {
		t.Errorf("payload subresourceWrite: got %v, want true", asMap["subresourceWrite"])
	}
}

func TestLogEmitter_WritesOneCanonicalJSONLine(t *testing.T) {
	t.Parallel()
	var lines []string
	log := funcr.NewJSON(func(obj string) { lines = append(lines, obj) }, funcr.Options{Verbosity: 1})
	le := NewLogEmitter(log)
	_, err := Emit(context.Background(), le, newCRDWriteEnv(VerbCreate), payloads.CRDWriteV1{})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(lines))
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("log line is not JSON: %v: %s", err, lines[0])
	}
	if entry["msg"] != "audit.event" {
		t.Errorf("log msg: got %v, want audit.event", entry["msg"])
	}
	audit, ok := entry["audit"].(map[string]any)
	if !ok {
		t.Fatalf("audit field missing or wrong type: %T", entry["audit"])
	}
	if audit["schemaVersion"] != SchemaVersion {
		t.Errorf("audit.schemaVersion: got %v, want %s", audit["schemaVersion"], SchemaVersion)
	}
}
