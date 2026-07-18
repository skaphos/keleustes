/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

// Package server implements the Keleustes read API. It adapts the
// oapi-codegen StrictServerInterface generated from openapi/keleustes.v1.yaml
// onto a readmodel.ReadModel and exposes an http.Handler mounted at /api/v1.
//
// The read surface is fully wired to the interaction-layer port; the write
// surface (promotion request, approve, cancel, retry) returns 501 until the
// Git-mutation engine lands, because every desired-state change must flow
// through a Git commit rather than a live mutation (ADR 0003).
//
// The handler is stateless and safe to replicate (ADR 0005 §10): a single
// ReadModel serves every request.
package server
