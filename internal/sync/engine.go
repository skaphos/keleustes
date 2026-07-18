/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

// Package sync hosts Keleustes' Sync Engine.
//
// Per ADR 0006 §4, this package — along with internal/diff,
// internal/health, and internal/kube — is one of four permitted
// gitops-engine import sites (the containment rule). The full engine
// wrapper lands with MVP 1 (SKA-339, SKA-341). For now this file
// translates upstream phase enums into the Keleustes SyncRun status
// alphabet, anchoring the dependency and isolating the boundary the
// rest of the codebase will eventually depend on.
package sync

import (
	syncCommon "github.com/skaphos/gitops-engine/pkg/sync/common"
)

// SyncRunPhase mirrors SyncRun.status.phase. Keeping this type local to
// internal/sync (rather than reading the upstream OperationPhase across
// the controller boundary) keeps the containment rule honest.
type SyncRunPhase string

const (
	PhasePending   SyncRunPhase = "Pending"
	PhaseRunning   SyncRunPhase = "Running"
	PhaseSucceeded SyncRunPhase = "Succeeded"
	PhaseFailed    SyncRunPhase = "Failed"
	PhaseError     SyncRunPhase = "Error"
)

// PhaseFromOperation translates gitops-engine's OperationPhase into the
// Keleustes SyncRunPhase emitted on SyncRun.status.phase.
//
// OperationTerminating maps to Running because a terminating SyncRun is
// still in flight from the controller's perspective; the terminal state
// arrives when GetState() returns Failed or Error.
func PhaseFromOperation(op syncCommon.OperationPhase) SyncRunPhase {
	switch op {
	case syncCommon.OperationRunning, syncCommon.OperationTerminating:
		return PhaseRunning
	case syncCommon.OperationSucceeded:
		return PhaseSucceeded
	case syncCommon.OperationFailed:
		return PhaseFailed
	case syncCommon.OperationError:
		return PhaseError
	default:
		return PhasePending
	}
}
