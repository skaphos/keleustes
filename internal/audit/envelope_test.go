/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package audit

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEnvelope_RequiredFieldsAlwaysEmitted(t *testing.T) {
	t.Parallel()
	env := Envelope{
		SchemaVersion: SchemaVersion,
		EventID:       "01HQ8FRVHX2BJW2N4Z9KZ8P6XK",
		OccurredAt:    time.Date(2026, 5, 17, 14, 32, 11, 482_000_000, time.UTC),
		Actor: Actor{
			Type:    ActorHuman,
			Subject: "alice@example.com",
		},
		Action: Action{
			Verb: VerbCreate,
			Subject: ActionSubject{
				APIGroup: "keleustes.skaphos.io",
				Kind:     "Application",
				Name:     "checkout-api",
			},
		},
		Result: Result{Outcome: OutcomeSuccess},
	}

	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Plan §3 declares schemaVersion, eventId, occurredAt, actor, action,
	// result as always-present. Detect a regression where one of them goes
	// missing because a contributor added `omitempty` to the wrong field.
	for _, want := range []string{
		`"schemaVersion":"audit/v1"`,
		`"eventId":"01HQ8FRVHX2BJW2N4Z9KZ8P6XK"`,
		`"occurredAt":"2026-05-17T14:32:11.482Z"`,
		`"actor":{`,
		`"action":{`,
		`"result":{`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("required field %q missing from marshaled envelope: %s", want, raw)
		}
	}
}

func TestEnvelope_OptionalFieldsOmittedWhenZero(t *testing.T) {
	t.Parallel()
	env := Envelope{
		SchemaVersion: SchemaVersion,
		EventID:       "01HQ8FRVHX2BJW2N4Z9KZ8P6XK",
		OccurredAt:    time.Date(2026, 5, 17, 14, 32, 11, 0, time.UTC),
		Actor:         Actor{Type: ActorSystem, Subject: "keleustes-sync-engine"},
		Action:        Action{Verb: VerbView},
		Result:        Result{Outcome: OutcomeSuccess},
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, unwanted := range []string{
		`"recordedAt"`, `"requestId"`, `"sessionId"`, `"traceparent"`,
		`"intent"`, `"payload"`, `"evidence"`,
	} {
		if strings.Contains(string(raw), unwanted) {
			t.Errorf("optional field %q must be omitted when zero: %s", unwanted, raw)
		}
	}
}
