/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package sync

import (
	"testing"

	syncCommon "github.com/skaphos/gitops-engine/pkg/sync/common"
)

func TestPhaseFromOperation(t *testing.T) {
	cases := []struct {
		name string
		in   syncCommon.OperationPhase
		want SyncRunPhase
	}{
		{"running", syncCommon.OperationRunning, PhaseRunning},
		{"terminating", syncCommon.OperationTerminating, PhaseRunning},
		{"succeeded", syncCommon.OperationSucceeded, PhaseSucceeded},
		{"failed", syncCommon.OperationFailed, PhaseFailed},
		{"error", syncCommon.OperationError, PhaseError},
		{"empty", "", PhasePending},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PhaseFromOperation(tc.in); got != tc.want {
				t.Errorf("PhaseFromOperation(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
