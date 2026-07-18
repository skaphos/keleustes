/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"

	"github.com/skaphos/keleustes/internal/audit/payloads"
)

// Emitter publishes one audit event. Implementations:
//
//   - LogEmitter — MVP 0. Writes one canonical-JSON log line per event.
//   - InMemoryEmitter — tests. Buffers events in a slice.
//   - jetstream.Emitter (SKA-347) — MVP 1. Publishes to keleustes.audit.
//
// Engines take an Emitter via constructor injection, never via a
// package-level singleton (plan §11.2).
type Emitter interface {
	Emit(ctx context.Context, env Envelope, payload Payload) (eventID string, err error)
}

// Emit assigns EventID + OccurredAt when they are unset, validates the
// payload against the verb registry, and delegates to the supplied
// Emitter. Returns the assigned EventID on success.
//
// Producers may construct an Envelope without EventID/OccurredAt and let
// this helper fill them in; callers that need to thread the eventId into
// downstream logs can also set EventID themselves.
func Emit(ctx context.Context, e Emitter, env Envelope, p Payload) (string, error) {
	if env.SchemaVersion == "" {
		env.SchemaVersion = SchemaVersion
	}
	if env.EventID == "" {
		env.EventID = NewEventID()
	}
	if env.OccurredAt.IsZero() {
		env.OccurredAt = time.Now().UTC()
	}
	if err := validatePayloadForVerb(string(env.Action.Verb), p); err != nil {
		return "", err
	}
	if p != nil {
		if err := p.Validate(); err != nil {
			return "", fmt.Errorf("audit: payload validation: %w", err)
		}
		env.Payload = withType(p)
	}
	return e.Emit(ctx, env, p)
}

// validatePayloadForVerb asserts the (verb, payload) pairing matches the
// audit-event-schema plan §13 registry. A producer that ships the wrong
// pair gets a clear error at emit time rather than a partial publish on
// the wire (§11.4).
func validatePayloadForVerb(verb string, p Payload) error {
	want, ok := payloads.AllowedPayloadType(verb)
	if !ok {
		return fmt.Errorf("audit: verb %q is not in the §13 registry", verb)
	}
	if want == "" {
		// Envelope-only verb — payload must be nil.
		if p != nil {
			return fmt.Errorf("audit: verb %q takes no payload, got %T", verb, p)
		}
		return nil
	}
	if p == nil {
		return fmt.Errorf("audit: verb %q requires payload @type %q", verb, want)
	}
	if got := p.AuditType(); got != want {
		return fmt.Errorf("audit: verb %q wants payload @type %q, got %q", verb, want, got)
	}
	return nil
}

// withType wraps a Payload in the JSON shape consumers expect:
//
//	{"@type": "<verb>.vN", ...payload fields }
//
// Implemented via a small map[string]any rather than a generic wrapper to
// keep the wire output unambiguous when payload structs grow new fields.
func withType(p Payload) map[string]any {
	// Round-trip the payload through JSON to inherit the producer's tag
	// definitions, then prepend @type.
	raw, err := json.Marshal(p)
	if err != nil {
		return map[string]any{"@type": p.AuditType(), "@marshalError": err.Error()}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{"@type": p.AuditType(), "@marshalError": err.Error()}
	}
	if out == nil {
		out = map[string]any{}
	}
	out["@type"] = p.AuditType()
	return out
}

// LogEmitter writes each event as a single info-level log line. The log
// field is "audit"; consumers can grep for it without parsing the entire
// log stream.
type LogEmitter struct {
	log logr.Logger
}

// NewLogEmitter returns an Emitter that writes to the supplied logger.
// Each call to Emit produces one info-level entry on log with msg
// "audit.event" and a single "audit" field carrying the canonical JSON.
func NewLogEmitter(log logr.Logger) *LogEmitter {
	return &LogEmitter{log: log}
}

// Emit serializes env to canonical JSON and emits one log line. _ is the
// already-validated payload (Emit() above invoked it); LogEmitter consumes
// the envelope only.
func (e *LogEmitter) Emit(_ context.Context, env Envelope, _ Payload) (string, error) {
	if e == nil || e.log.GetSink() == nil {
		return "", fmt.Errorf("audit: log emitter not initialized")
	}
	body, err := json.Marshal(env)
	if err != nil {
		return "", fmt.Errorf("audit: marshal envelope: %w", err)
	}
	e.log.Info("audit.event", "audit", json.RawMessage(body))
	return env.EventID, nil
}

// InMemoryEmitter is a test double that captures every emitted envelope.
// Safe for concurrent use.
type InMemoryEmitter struct {
	mu     sync.Mutex
	events []Envelope
}

// NewInMemoryEmitter returns an empty InMemoryEmitter.
func NewInMemoryEmitter() *InMemoryEmitter { return &InMemoryEmitter{} }

// Emit records env. Returns env.EventID.
func (m *InMemoryEmitter) Emit(_ context.Context, env Envelope, _ Payload) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, env)
	return env.EventID, nil
}

// Events returns a snapshot of every event observed since construction.
// The slice is a copy; callers can iterate without holding the emitter's
// lock.
func (m *InMemoryEmitter) Events() []Envelope {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Envelope, len(m.events))
	copy(out, m.events)
	return out
}
