/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package audit

import (
	"testing"
	"time"
)

func TestNewEventID_ShapeIsValidULID(t *testing.T) {
	t.Parallel()
	id := NewEventID()
	if err := validULID(id); err != nil {
		t.Fatalf("NewEventID(): %v (got %q)", err, id)
	}
}

func TestEncodeULID_DeterministicForFixedInputs(t *testing.T) {
	t.Parallel()
	ms := uint64(time.Date(2026, 5, 17, 14, 32, 11, 482_000_000, time.UTC).UnixMilli())
	var entropy [10]byte
	for i := range entropy {
		entropy[i] = byte(i + 1)
	}
	first := encodeULID(ms, entropy)
	second := encodeULID(ms, entropy)
	if first != second {
		t.Errorf("encodeULID not deterministic: %q vs %q", first, second)
	}
	if err := validULID(first); err != nil {
		t.Errorf("encoded id is not a valid ULID: %v", err)
	}
}

func TestNewEventID_GeneratesUniqueIDs(t *testing.T) {
	t.Parallel()
	seen := make(map[string]struct{}, 1024)
	for i := range 1024 {
		id := NewEventID()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate eventId at iteration %d: %q", i, id)
		}
		seen[id] = struct{}{}
	}
}
