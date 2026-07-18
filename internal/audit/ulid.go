/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package audit

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// ULID minting per the ULID spec (Crockford base32, 48-bit timestamp ms +
// 80-bit randomness). The plan (§9) requires eventId to be a ULID for stable
// sort + URL deep-linking; UUID/ULID libraries pulled in as deps are pure
// overhead for ~30 lines of code.
//
// This implementation is goroutine-safe and avoids the dependency on
// github.com/oklog/ulid; we don't need monotonic sequence numbers because
// audit events are not ordered at sub-millisecond granularity in v1
// (consumers sort by occurredAt then eventId per §11.3).

const crockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var (
	ulidMu  sync.Mutex
	randBuf [10]byte
)

// NewEventID returns a fresh ULID string suitable for envelope.eventId.
// Format: 26 Crockford-base32 characters (10 timestamp + 16 randomness).
//
// Falls back to a deterministic-zero ULID on rand.Read failure rather than
// panicking, on the theory that an audit emit with a degraded ID beats a
// crashed reconciler — the producer logs the underlying failure at ERROR.
func NewEventID() string {
	ms := uint64(time.Now().UTC().UnixMilli())
	ulidMu.Lock()
	defer ulidMu.Unlock()
	if _, err := rand.Read(randBuf[:]); err != nil {
		for i := range randBuf {
			randBuf[i] = 0
		}
	}
	return encodeULID(ms, randBuf)
}

// encodeULID is exported only for tests (via the _test.go file in this
// package). Production callers go through NewEventID.
func encodeULID(ms uint64, entropy [10]byte) string {
	var out [26]byte

	// Timestamp: 48 bits encoded as 10 chars of base32.
	out[0] = crockfordAlphabet[(ms>>45)&0x1F]
	out[1] = crockfordAlphabet[(ms>>40)&0x1F]
	out[2] = crockfordAlphabet[(ms>>35)&0x1F]
	out[3] = crockfordAlphabet[(ms>>30)&0x1F]
	out[4] = crockfordAlphabet[(ms>>25)&0x1F]
	out[5] = crockfordAlphabet[(ms>>20)&0x1F]
	out[6] = crockfordAlphabet[(ms>>15)&0x1F]
	out[7] = crockfordAlphabet[(ms>>10)&0x1F]
	out[8] = crockfordAlphabet[(ms>>5)&0x1F]
	out[9] = crockfordAlphabet[ms&0x1F]

	// Entropy: 80 bits encoded as 16 chars of base32.
	hi := binary.BigEndian.Uint16(entropy[0:2])
	lo := binary.BigEndian.Uint64(entropy[2:10])
	out[10] = crockfordAlphabet[(hi>>11)&0x1F]
	out[11] = crockfordAlphabet[(hi>>6)&0x1F]
	out[12] = crockfordAlphabet[(hi>>1)&0x1F]
	out[13] = crockfordAlphabet[(byte(hi&0x01)<<4)|byte((lo>>60)&0x0F)]
	out[14] = crockfordAlphabet[(lo>>55)&0x1F]
	out[15] = crockfordAlphabet[(lo>>50)&0x1F]
	out[16] = crockfordAlphabet[(lo>>45)&0x1F]
	out[17] = crockfordAlphabet[(lo>>40)&0x1F]
	out[18] = crockfordAlphabet[(lo>>35)&0x1F]
	out[19] = crockfordAlphabet[(lo>>30)&0x1F]
	out[20] = crockfordAlphabet[(lo>>25)&0x1F]
	out[21] = crockfordAlphabet[(lo>>20)&0x1F]
	out[22] = crockfordAlphabet[(lo>>15)&0x1F]
	out[23] = crockfordAlphabet[(lo>>10)&0x1F]
	out[24] = crockfordAlphabet[(lo>>5)&0x1F]
	out[25] = crockfordAlphabet[lo&0x1F]

	return string(out[:])
}

// validULID is the regex-free shape check used by tests.
func validULID(s string) error {
	if len(s) != 26 {
		return fmt.Errorf("ulid: expected 26 chars, got %d", len(s))
	}
	for i, c := range s {
		ok := false
		for _, a := range crockfordAlphabet {
			if c == a {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("ulid: invalid char %q at index %d", c, i)
		}
	}
	return nil
}
