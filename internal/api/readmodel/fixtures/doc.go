/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

// Package fixtures provides an in-memory readmodel.ReadModel backed by a static
// JSON corpus. It serves the exact dataset the UI mock backend ships
// (ui/src/mocks/fixtures.ts), so the API server can run with no cluster and the
// same fixtures double as contract-test golden data shared by both stacks.
//
// The corpus lives under testdata/contract/*.json, shaped to the generated
// openapi JSON field names, and is embedded at build time. It is read-only and
// deterministic: every method returns the same data for the same input, which
// makes the package safe for concurrent use by the stateless, freely-replicated
// API server (ADR 0005 §10).
//
// Endpoints the UI mock does not model (diff, per-target drift and health) are
// synthesized from the data that is present, so every array-typed field on the
// contract is returned non-nil and valid against the Status vocabulary.
package fixtures
