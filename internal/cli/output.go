/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// outputFormat enumerates the supported -o values.
type outputFormat string

const (
	outputTable outputFormat = ""     // default
	outputWide  outputFormat = "wide" // not yet meaningful; reserved
	outputJSON  outputFormat = "json"
	outputYAML  outputFormat = "yaml"
)

func parseOutputFormat(s string) (outputFormat, error) {
	switch outputFormat(s) {
	case outputTable, outputWide, outputJSON, outputYAML:
		return outputFormat(s), nil
	default:
		return outputTable, fmt.Errorf("unsupported -o format %q; valid: table (default), wide, yaml, json", s)
	}
}

// renderList prints a list of unstructured objects in the requested format.
// kind drives which columns appear for table output; items is the slice
// returned by dynamic.Interface.Resource(...).List().Items.
func renderList(w io.Writer, kind resolvedKind, items []unstructured.Unstructured, format outputFormat) error {
	switch format {
	case outputJSON:
		return renderJSON(w, items)
	case outputYAML:
		return renderYAML(w, items)
	default:
		return renderTable(w, kind, items)
	}
}

// renderOne prints a single unstructured object. Table format prints the
// same row a list would.
func renderOne(w io.Writer, kind resolvedKind, obj *unstructured.Unstructured, format outputFormat) error {
	return renderList(w, kind, []unstructured.Unstructured{*obj}, format)
}

func renderJSON(w io.Writer, items []unstructured.Unstructured) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if len(items) == 1 {
		return enc.Encode(items[0].Object)
	}
	list := map[string]any{
		"apiVersion": "v1",
		"kind":       "List",
		"items":      asObjects(items),
	}
	return enc.Encode(list)
}

func renderYAML(w io.Writer, items []unstructured.Unstructured) error {
	if len(items) == 1 {
		b, err := yaml.Marshal(items[0].Object)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	}
	list := map[string]any{
		"apiVersion": "v1",
		"kind":       "List",
		"items":      asObjects(items),
	}
	b, err := yaml.Marshal(list)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func asObjects(items []unstructured.Unstructured) []map[string]any {
	out := make([]map[string]any, len(items))
	for i := range items {
		out[i] = items[i].Object
	}
	return out
}

// renderTable prints a kubectl-style aligned table using tabwriter.
// Columns: NAME, <kind.Columns...>, AGE.
func renderTable(w io.Writer, kind resolvedKind, items []unstructured.Unstructured) error {
	if len(items) == 0 {
		_, err := fmt.Fprintln(w, "No resources found.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer func() { _ = tw.Flush() }()

	headers := []string{"NAME"}
	for _, c := range kind.Columns {
		headers = append(headers, strings.ToUpper(c.Header))
	}
	headers = append(headers, "AGE")
	if _, err := fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
		return err
	}

	for _, item := range items {
		row := []string{item.GetName()}
		for _, c := range kind.Columns {
			row = append(row, extractColumn(&item, c.JSONPath))
		}
		row = append(row, formatAge(item.GetCreationTimestamp()))
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return nil
}

// extractColumn resolves a column's dotted JSON-path against an
// unstructured object. Supports a tiny subset of jsonpath-style syntax:
//
//	.a.b.c                      — nested field access
//	.a.b[*].name                — flatten an array and join names with ","
//	.conds[?(@.type=='X')].status — conditions-style filter
//
// Anything more sophisticated falls back to "<unsupported>". The CLI's
// rendering does not need full jsonpath — kubebuilder printcolumn
// expressions hew to these patterns.
func extractColumn(obj *unstructured.Unstructured, path string) string {
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return ""
	}
	// Conditions filter pattern: `<path>[?(@.type=='X')].<field>`
	if idx := strings.Index(path, "[?(@.type=='"); idx >= 0 {
		prefix := path[:idx]
		rest := path[idx+len("[?(@.type=='"):]
		end := strings.Index(rest, "')]")
		if end < 0 {
			return "<unsupported>"
		}
		condType := rest[:end]
		fieldPath := strings.TrimPrefix(rest[end+len("')]"):], ".")
		return conditionField(obj, prefix, condType, fieldPath)
	}
	// Array-flatten pattern: `<path>[*].<field>` or `<path>[*]`
	if idx := strings.Index(path, "[*]"); idx >= 0 {
		prefix := path[:idx]
		suffix := strings.TrimPrefix(path[idx+len("[*]"):], ".")
		return arrayJoin(obj, prefix, suffix)
	}
	// Plain dotted path.
	v, found, err := unstructured.NestedFieldCopy(obj.Object, strings.Split(path, ".")...)
	if err != nil || !found {
		return ""
	}
	return scalarToString(v)
}

func conditionField(obj *unstructured.Unstructured, condPath, condType, field string) string {
	conds, found, err := unstructured.NestedSlice(obj.Object, strings.Split(condPath, ".")...)
	if err != nil || !found {
		return ""
	}
	for _, c := range conds {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if m["type"] == condType {
			if field == "" {
				return scalarToString(m)
			}
			return scalarToString(m[field])
		}
	}
	return ""
}

func arrayJoin(obj *unstructured.Unstructured, prefix, suffix string) string {
	arr, found, err := unstructured.NestedSlice(obj.Object, strings.Split(prefix, ".")...)
	if err != nil || !found || len(arr) == 0 {
		return ""
	}
	parts := make([]string, 0, len(arr))
	for _, elem := range arr {
		if suffix == "" {
			parts = append(parts, scalarToString(elem))
			continue
		}
		if m, ok := elem.(map[string]any); ok {
			if v, ok := nested(m, strings.Split(suffix, ".")); ok {
				parts = append(parts, scalarToString(v))
			}
		}
	}
	return strings.Join(parts, ",")
}

// nested walks dotted path keys through a map. Returns (value, ok).
func nested(m map[string]any, parts []string) (any, bool) {
	cur := any(m)
	for _, p := range parts {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := mm[p]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

func scalarToString(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int, int32, int64:
		return fmt.Sprintf("%d", x)
	case float64:
		// JSON numbers come through as float64 even when they're integral.
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%g", x)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// formatAge renders a creationTimestamp in the kubectl-style compact form
// (`5m`, `3h`, `2d`, `45s`). Returns "<unknown>" when the timestamp is the
// zero value.
func formatAge(t metav1.Time) string {
	if t.IsZero() {
		return "<unknown>"
	}
	d := time.Since(t.Time)
	switch {
	case d < 0:
		return "0s"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
