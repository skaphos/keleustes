/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package audit

// Payload is the discriminator-bearing interface every per-verb extension
// satisfies. AuditType returns the registered "@type" string from
// audit-event-schema plan §13 (e.g. "crd.write.v1", "promote.v1").
//
// Implementations live in internal/audit/payloads/. Validate returns nil
// when the payload's required fields are populated; Emit calls it
// before serialization (§11.4).
type Payload interface {
	AuditType() string
	Validate() error
}
