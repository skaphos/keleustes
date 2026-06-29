/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

// Package auth carries request identity and authorization for the Keleustes
// API server. ADR 0004: identity is OIDC, authorization is verb-scoped and
// enforced server-side; the UI never enforces permissions itself.
//
// This is the scaffold for that contract. Real OIDC/JWKS token validation and
// the ADR 0004 §11 policy evaluator slot in behind these types (SKA-330)
// without changing callers: handlers read the Identity from the request
// context and ask an Authorizer what the principal may do.
package auth
