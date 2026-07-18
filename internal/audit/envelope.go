/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package audit

import "time"

// Envelope is the per-event audit record. Field ordering and tag spelling
// follow audit-event-schema plan §3 verbatim — consumers (UI, DuckDB rebuild
// job, SIEM exporter) deserialize against this exact shape.
//
// `omitempty` is used for optional fields only; required fields are always
// emitted, even when empty, so consumers can detect drift. See plan §5.2.
type Envelope struct {
	SchemaVersion string `json:"schemaVersion"`

	// Correlation (§9). RecordedAt is the broker (JetStream) ack time and
	// is therefore set by the consumer/broker side, not the producer. The
	// MVP 0 log emitter leaves it nil; the MVP 1 JetStream emitter fills
	// it in once the broker acks.
	EventID     string     `json:"eventId"`
	OccurredAt  time.Time  `json:"occurredAt"`
	RecordedAt  *time.Time `json:"recordedAt,omitempty"`
	RequestID   string     `json:"requestId,omitempty"`
	SessionID   string     `json:"sessionId,omitempty"`
	TraceParent string     `json:"traceparent,omitempty"`

	Actor   Actor   `json:"actor"`
	Action  Action  `json:"action"`
	Intent  string  `json:"intent,omitempty"`
	Context Context `json:"context"`
	Result  Result  `json:"result"`

	// Payload is rendered via the discriminated-union pattern in §7.1. The
	// envelope marshaller does not own its shape; producers populate it via
	// the typed Payload interface, and Emit serializes the registered @type.
	Payload  any        `json:"payload,omitempty"`
	Evidence []Evidence `json:"evidence,omitempty"`
}

// Actor records the principal that initiated the action. Subject is the
// IdP-normalized identifier per §6.3; SubjectID is the immutable IdP-issued
// id used when the display name can change.
type Actor struct {
	Type             ActorType `json:"type"`
	Subject          string    `json:"subject"`
	SubjectID        string    `json:"subjectId,omitempty"`
	IdentityProvider string    `json:"identityProvider,omitempty"`
	Groups           []string  `json:"groups,omitempty"`
	DelegatedFrom    *Actor    `json:"delegatedFrom,omitempty"`
}

// Action describes the verb and the subject resource the verb is being
// applied to. Verb must be a registered value (§13); Subject names the
// affected Kubernetes object.
type Action struct {
	Verb    Verb          `json:"verb"`
	Scope   string        `json:"scope,omitempty"`
	Subject ActionSubject `json:"subject"`
}

// ActionSubject identifies the affected resource. Group + Version + Kind
// match the kube-apiserver's GroupVersionResource semantics; ULID is the
// stable per-resource identifier from ADR 0004.
type ActionSubject struct {
	APIGroup  string `json:"apiGroup,omitempty"`
	Version   string `json:"version,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
	ULID      string `json:"ulid,omitempty"`
}

// Context carries the environmental metadata about how the event was
// produced. AuditTicket is required by §13.7 when the actor is invoking
// break-glass; the producer enforces that pairing, not this struct.
type Context struct {
	SourceIP    string `json:"sourceIp,omitempty"`
	UserAgent   string `json:"userAgent,omitempty"`
	AuditTicket string `json:"auditTicket,omitempty"`
	ClusterName string `json:"clusterName,omitempty"`
	Shard       string `json:"shard,omitempty"`
}

// Result is the outcome plus optional snapshots (§8.1). Before/After are
// already-redacted JSON payloads when present; the redaction package owns
// the rules that produced them.
type Result struct {
	Outcome Outcome `json:"outcome"`
	Reason  string  `json:"reason,omitempty"`
	Before  any     `json:"before,omitempty"`
	After   any     `json:"after,omitempty"`
}

// Evidence is a pointer to a large artifact stored outside the envelope
// (§8.3). Never an inlined blob.
type Evidence struct {
	Kind      string `json:"kind"`
	Hash      string `json:"hash,omitempty"`
	ObjectRef string `json:"objectRef,omitempty"`
	Store     string `json:"store,omitempty"`
	Label     string `json:"label,omitempty"`
}
