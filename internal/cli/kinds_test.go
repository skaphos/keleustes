/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package cli

import (
	"strings"
	"testing"
)

func TestResolveKind_KnownAliases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in       string
		wantKind string
	}{
		{"Application", "Application"},
		{"application", "Application"},
		{"applications", "Application"},
		{"app", "Application"},
		{"SyncRun", "SyncRun"},
		{"syncrun", "SyncRun"},
		{"syncruns", "SyncRun"},
		{"sr", "SyncRun"},
		{"Promotion", "Promotion"},
		{"promo", "Promotion"},
		{"Notifier", "Notifier"},
		{"notif", "Notifier"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.in, func(t *testing.T) {
			t.Parallel()
			got, err := resolveKind(c.in)
			if err != nil {
				t.Fatalf("resolveKind(%q): %v", c.in, err)
			}
			if got.GVK.Kind != c.wantKind {
				t.Errorf("resolveKind(%q).GVK.Kind = %q, want %q", c.in, got.GVK.Kind, c.wantKind)
			}
		})
	}
}

func TestResolveKind_UnknownAliasErrors(t *testing.T) {
	t.Parallel()
	_, err := resolveKind("not-a-real-kind")
	if err == nil {
		t.Fatalf("expected error for unknown kind, got nil")
	}
	if !strings.Contains(err.Error(), "unknown kind") {
		t.Errorf("error should mention 'unknown kind': %v", err)
	}
}

func TestKeleustesKinds_AllFifteenCRDsCovered(t *testing.T) {
	t.Parallel()
	// Sanity check that every CRD in api/v1alpha1 has a row here. If a new
	// CRD ships, this test fails until the registry catches up.
	want := []string{
		"Application", "Source", "Release", "Deployment", "Environment",
		"Cell", "DeploymentTarget", "Promotion", "PromotionPolicy",
		"Approval", "FreezeWindow", "SyncPlan", "SyncRun", "HealthCheck",
		"Notifier",
	}
	have := map[string]bool{}
	for _, k := range keleustesKinds {
		have[k.GVK.Kind] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("keleustesKinds is missing %q", w)
		}
	}
	if len(have) != len(want) {
		t.Errorf("keleustesKinds has %d entries, want %d (likely a new CRD landed without registry update, or a duplicate)", len(have), len(want))
	}
}

func TestKnownAliases_ReturnsSortedPlurals(t *testing.T) {
	t.Parallel()
	got := knownAliases()
	if len(got) != len(keleustesKinds) {
		t.Fatalf("knownAliases len = %d, want %d", len(got), len(keleustesKinds))
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Errorf("knownAliases not sorted: %q >= %q", got[i-1], got[i])
		}
	}
}
