/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package redaction

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestApply_RedactsSecretDataBlanket(t *testing.T) {
	t.Parallel()
	obj := map[string]any{
		"kind": "Secret",
		"metadata": map[string]any{
			"name": "creds",
		},
		"data": map[string]any{
			"username": "YWxpY2U=",
			"password": "c2VjcmV0",
		},
	}
	Apply("Secret", obj)
	data := obj["data"].(map[string]any)
	for k, v := range data {
		m, ok := v.(Marker)
		if !ok {
			t.Errorf("data.%s should be redacted, got %T %v", k, v, v)
			continue
		}
		if m.Redacted != ClassSecretBytes {
			t.Errorf("data.%s class: got %q, want %q", k, m.Redacted, ClassSecretBytes)
		}
	}
	// metadata.name must NOT be redacted — identifiers are part of the
	// audit trail by design (plan §8.2).
	if name, _ := obj["metadata"].(map[string]any)["name"].(string); name != "creds" {
		t.Errorf("metadata.name should not be redacted, got %v", obj["metadata"].(map[string]any)["name"])
	}
}

func TestApply_PreservesNonSensitiveValues(t *testing.T) {
	t.Parallel()
	obj := map[string]any{
		"kind": "Application",
		"spec": map[string]any{
			"replicas": float64(3),
			"image":    "ghcr.io/skaphos/keleustes:1.0.0",
		},
	}
	Apply("Application", obj)
	spec := obj["spec"].(map[string]any)
	if spec["replicas"] != float64(3) {
		t.Errorf("spec.replicas changed: %v", spec["replicas"])
	}
	if spec["image"] != "ghcr.io/skaphos/keleustes:1.0.0" {
		t.Errorf("spec.image changed: %v", spec["image"])
	}
}

func TestApply_WebhookSecretRedacted(t *testing.T) {
	t.Parallel()
	obj := map[string]any{
		"kind": "Source",
		"spec": map[string]any{
			"webhook": map[string]any{
				"secret": "super-secret-hmac-key",
				"url":    "https://example.com/hooks",
			},
		},
	}
	Apply("Source", obj)
	wh := obj["spec"].(map[string]any)["webhook"].(map[string]any)
	m, ok := wh["secret"].(Marker)
	if !ok {
		t.Fatalf("spec.webhook.secret should be redacted, got %T", wh["secret"])
	}
	if m.Redacted != ClassWebhookSecret {
		t.Errorf("redaction class: got %q, want %q", m.Redacted, ClassWebhookSecret)
	}
	if wh["url"] != "https://example.com/hooks" {
		t.Errorf("spec.webhook.url should be preserved, got %v", wh["url"])
	}
}

func TestMarker_SerializesWithAtRedactedKey(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(Marker{Redacted: ClassSecretBytes})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"@redacted":"secret-bytes"`) {
		t.Errorf("marker wire shape wrong: %s", raw)
	}
}
