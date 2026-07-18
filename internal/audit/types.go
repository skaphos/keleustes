/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package audit

// SchemaVersion is the wire identifier for the current envelope shape. Bumps
// follow the deprecation lane in audit-event-schema plan §5.3 and require a
// new ADR; producers running today must always emit this exact value.
const SchemaVersion = "audit/v1"

// ActorType enumerates the closed set of audit actor types from
// audit-event-schema plan §6.2.
type ActorType string

const (
	ActorHuman  ActorType = "human"
	ActorCI     ActorType = "ci"
	ActorAgent  ActorType = "agent"
	ActorSystem ActorType = "system"
)

// Outcome is the closed set of result.outcome values from §3.
type Outcome string

const (
	OutcomeSuccess Outcome = "success"
	OutcomeDenied  Outcome = "denied"
	OutcomeError   Outcome = "error"
	OutcomePartial Outcome = "partial"
)

// Verb is the registered enum of audit action verbs (§13). Only values
// listed here may appear on the wire; the registry in payloads/ enforces the
// pairing between verb and payload @type.
type Verb string

// CRD-lifecycle verbs (§13.1).
const (
	VerbCreate Verb = "create"
	VerbEdit   Verb = "edit"
	VerbDelete Verb = "delete"
	VerbView   Verb = "view"
)
