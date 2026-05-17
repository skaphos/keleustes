/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

// Package audit implements the Keleustes audit-event envelope and emitter
// per docs/plans/2026-05-audit-event-schema.md (SKA-322).
//
// The package owns:
//
//   - The Envelope and Action/Actor/Result types that match the §3 wire shape.
//   - A small Payload interface, plus the verb→payload type registry from §13.
//   - The redaction rule list and recursive redactor from §8.2.
//   - ULID minting for the eventId field.
//   - The Emitter interface and a LogEmitter that serializes events as a
//     single canonical-JSON log line. The JetStream emitter lands in SKA-347.
//
// The admission-webhook handler that builds envelopes from kube-apiserver
// AdmissionRequest objects lives in the sibling internal/audit/webhook
// package.
//
// All audit emission is producer-side validated: an engine that ships a
// payload with the wrong @type for its declared verb gets a compile-time or
// emit-time error, never a partial publish (§11.4).
package audit
