/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package payloads

// Registry maps an audit verb (audit-event-schema plan §13) to the
// discriminator string of its registered payload type. Used by the audit
// emitter to assert that a producer hasn't shipped a payload of the wrong
// shape for the verb it declared.
//
// Adding a new entry requires:
//  1. A new payload struct in this package implementing AuditType + Validate.
//  2. A row in the verb-registry tables of audit-event-schema plan §13.
//  3. A row in this map.
//
// Producers and consumers update independently per §7.3.
var Registry = map[string]string{
	"create": "crd.write.v1",
	"edit":   "crd.write.v1",
	"delete": "crd.write.v1",
	"view":   "", // envelope-only — no payload required
}

// AllowedPayloadType returns the registered @type for verb. An empty
// returned string means the verb is registered but takes no payload (e.g.
// "view"). ok is false when verb is not in the registry.
func AllowedPayloadType(verb string) (typ string, ok bool) {
	typ, ok = Registry[verb]
	return
}
