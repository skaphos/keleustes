/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func mkApp(t *testing.T, name string, ageMinutes int) unstructured.Unstructured {
	t.Helper()
	return unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "keleustes.skaphos.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]any{
				"name":              name,
				"namespace":         "default",
				"creationTimestamp": metav1.NewTime(time.Now().Add(-time.Duration(ageMinutes) * time.Minute)).Format(time.RFC3339),
			},
			"spec": map[string]any{
				"deployment": map[string]any{
					"strategy": "gitops",
					"manifest": map[string]any{"type": "kustomize"},
				},
			},
			"status": map[string]any{
				"conditions": []any{
					map[string]any{"type": "Accepted", "status": "True", "reason": "ScaffoldReconciler"},
				},
			},
		},
	}
}

func TestParseOutputFormat(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"":     true,
		"yaml": true,
		"json": true,
		"wide": true,
		"nope": false,
		"YAML": false, // case-sensitive
	}
	for in, ok := range cases {
		in, ok := in, ok
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			_, err := parseOutputFormat(in)
			if ok && err != nil {
				t.Errorf("parseOutputFormat(%q) = err %v, want nil", in, err)
			}
			if !ok && err == nil {
				t.Errorf("parseOutputFormat(%q) = nil err, want error", in)
			}
		})
	}
}

func TestRenderList_JSON_RoundTrips(t *testing.T) {
	t.Parallel()
	kind, _ := resolveKind("application")
	app := mkApp(t, "checkout", 5)
	var buf bytes.Buffer
	if err := renderList(&buf, kind, []unstructured.Unstructured{app}, outputJSON); err != nil {
		t.Fatalf("renderList json: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("re-parse json: %v: %s", err, buf.String())
	}
	if back["kind"] != "Application" {
		t.Errorf("kind round-trip: got %v, want Application", back["kind"])
	}
}

func TestRenderList_YAML_StartsWithHeader(t *testing.T) {
	t.Parallel()
	kind, _ := resolveKind("application")
	app := mkApp(t, "checkout", 5)
	var buf bytes.Buffer
	if err := renderList(&buf, kind, []unstructured.Unstructured{app}, outputYAML); err != nil {
		t.Fatalf("renderList yaml: %v", err)
	}
	if !strings.Contains(buf.String(), "kind: Application") {
		t.Errorf("yaml should contain 'kind: Application': %s", buf.String())
	}
}

func TestRenderList_Table_HasHeadersAndAge(t *testing.T) {
	t.Parallel()
	kind, _ := resolveKind("application")
	app := mkApp(t, "checkout", 5)
	var buf bytes.Buffer
	if err := renderList(&buf, kind, []unstructured.Unstructured{app}, outputTable); err != nil {
		t.Fatalf("renderList table: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"NAME", "STRATEGY", "MANIFEST", "STATUS", "AGE", "checkout"} {
		if !strings.Contains(out, want) {
			t.Errorf("table should contain %q: %s", want, out)
		}
	}
}

func TestRenderList_EmptyListEmitsFriendlyMessage(t *testing.T) {
	t.Parallel()
	kind, _ := resolveKind("application")
	var buf bytes.Buffer
	if err := renderList(&buf, kind, nil, outputTable); err != nil {
		t.Fatalf("renderList: %v", err)
	}
	if !strings.Contains(buf.String(), "No resources found") {
		t.Errorf("empty table should say no resources: %q", buf.String())
	}
}

func TestExtractColumn_ConditionFilter(t *testing.T) {
	t.Parallel()
	app := mkApp(t, "x", 1)
	got := extractColumn(&app, ".status.conditions[?(@.type=='Accepted')].status")
	if got != "True" {
		t.Errorf("expected condition.status 'True', got %q", got)
	}
	got = extractColumn(&app, ".status.conditions[?(@.type=='NotPresent')].status")
	if got != "" {
		t.Errorf("missing condition should return empty, got %q", got)
	}
}

func TestExtractColumn_NestedDottedPath(t *testing.T) {
	t.Parallel()
	app := mkApp(t, "x", 1)
	got := extractColumn(&app, ".spec.deployment.strategy")
	if got != "gitops" {
		t.Errorf("nested path: got %q, want 'gitops'", got)
	}
}

func TestExtractColumn_ArrayFlatten(t *testing.T) {
	t.Parallel()
	obj := unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"targets": []any{
					map[string]any{"name": "a"},
					map[string]any{"name": "b"},
					map[string]any{"name": "c"},
				},
			},
		},
	}
	got := extractColumn(&obj, ".spec.targets[*].name")
	if got != "a,b,c" {
		t.Errorf("array flatten: got %q, want 'a,b,c'", got)
	}
}

func TestSplitDescribeArgs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args     []string
		wantKind string
		wantName string
		wantErr  bool
	}{
		{[]string{"app", "checkout"}, "app", "checkout", false},
		{[]string{"app/checkout"}, "app", "checkout", false},
		{[]string{"SyncRun/sr-1"}, "SyncRun", "sr-1", false},
		{[]string{"app"}, "", "", true},       // missing name
		{[]string{"app/"}, "", "", true},      // empty name
		{[]string{"/checkout"}, "", "", true}, // empty kind
		{[]string{}, "", "", true},            // no args
	}
	for _, c := range cases {
		c := c
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			t.Parallel()
			kind, name, err := splitDescribeArgs(c.args)
			if c.wantErr {
				if err == nil {
					t.Errorf("expected error, got kind=%q name=%q", kind, name)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if kind != c.wantKind || name != c.wantName {
				t.Errorf("got kind=%q name=%q, want %q/%q", kind, name, c.wantKind, c.wantName)
			}
		})
	}
}
