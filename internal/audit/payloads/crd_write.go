/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

// Package payloads implements the registered audit payload types from
// audit-event-schema plan §13. Each type is a Go struct whose JSON form is
// the discriminated-union variant for one verb.
package payloads

import "errors"

// CRDWriteV1 is the payload for the four CRD-lifecycle verbs
// (create, edit, delete, view-on-sensitive). It carries an optional
// JSON-patch between before/after for compact rendering by the UI;
// the full before/after snapshots ride on the envelope's Result block.
type CRDWriteV1 struct {
	// Patch is a JSON-Patch (RFC 6902) document describing the diff
	// between Result.Before and Result.After. Optional — consumers can
	// always rebuild the diff from the snapshots if it's omitted.
	Patch []map[string]any `json:"patch,omitempty"`

	// SubresourceWrite is true when the operation targets a status or
	// scale subresource rather than the main spec. The controller's own
	// reconcile-loop writes set this to true; user-triggered writes via
	// kubectl/UI set it to false.
	SubresourceWrite bool `json:"subresourceWrite,omitempty"`

	// DryRun is true when kube-apiserver was asked to evaluate the change
	// without persisting it. Producers must still emit the event so the
	// audit trail records the intent.
	DryRun bool `json:"dryRun,omitempty"`
}

// AuditType satisfies the audit.Payload interface.
func (CRDWriteV1) AuditType() string { return "crd.write.v1" }

// Validate enforces the producer-side schema invariants from plan §11.4.
// Returns nil for CRDWriteV1 today; the type is intentionally permissive
// because every required envelope field is on Envelope itself, not on the
// payload.
func (CRDWriteV1) Validate() error { return nil }

// ErrUnregisteredVerb is returned by Registry.PayloadType for a verb that
// hasn't been registered. The audit Emitter wraps this so producers see a
// clear "your verb is not in §13" message.
var ErrUnregisteredVerb = errors.New("audit: verb has no registered payload type")
