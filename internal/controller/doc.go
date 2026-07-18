/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

// Package controller implements reconcilers for the keleustes.skaphos.io CRDs.
//
// The scaffold ships idempotent stubs that set ObservedGeneration and an
// Accepted condition for each owned kind. Real reconciliation logic (Source
// Engine, Sync Engine, Promotion Engine, Git Mutation Engine, Policy Engine,
// Health Engine, Diff Engine) lands as Keleustes MVPs progress; see PROPOSAL
// §9 and §20.
package controller
