/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

// Package redaction implements the audit-event-schema plan §8.2 redaction
// rules. The same package is consumed by the audit emitter and by the UI's
// "view object" surface so users and audit logs see the same redaction.
package redaction

import "strings"

// Marker is the in-place value that replaces a redacted field. Format:
// `{"@redacted":"<class>"}`. Consumers detect redaction by the "@redacted"
// key, never by the placeholder string itself.
type Marker struct {
	Redacted string `json:"@redacted"`
}

// Class names map 1:1 to the redaction table in plan §8.2.
const (
	ClassSecretBytes     = "secret-bytes"
	ClassBearerToken     = "bearer-token"
	ClassTLSMaterial     = "tls-material"
	ClassWebhookSecret   = "webhook-secret"
	ClassPluginSensitive = "plugin-sensitive"
)

// Rule describes one field-redaction policy. Match returns true when the
// rule applies to the field at the given parent type + field name.
//
// Rules are evaluated in order; the first match wins. Add new rules to
// DefaultRules below; never extend this struct without amending the plan.
type Rule struct {
	// ParentKind is the Kubernetes Kind of the enclosing object
	// (e.g. "Secret"). Empty matches any parent kind.
	ParentKind string
	// FieldPath is the dotted path inside the object relative to the root
	// (e.g. "data", "stringData", "spec.webhook.secret"). The matcher
	// treats each segment as exact, with a single "*" allowed to mean
	// "any map key here."
	FieldPath string
	// Class is the redaction class name written to the @redacted marker.
	Class string
}

// DefaultRules is the canonical rule list pinned by audit-event-schema plan
// §8.2. Adding to this list is a producer-side schema decision; consumers
// already understand the @redacted marker and need no update.
var DefaultRules = []Rule{
	// Blanket-redact every child of Secret.data / Secret.stringData. The
	// "*" segment matches any single child key without descending further.
	{ParentKind: "Secret", FieldPath: "data.*", Class: ClassSecretBytes},
	{ParentKind: "Secret", FieldPath: "stringData.*", Class: ClassSecretBytes},
	// Field-specific rules — same blanket-redact semantics via the walker's
	// "container ends in .*" branch when applicable.
	{FieldPath: "spec.webhook.secret", Class: ClassWebhookSecret},
	{FieldPath: "spec.webhook.secretRef", Class: ClassWebhookSecret},
	{FieldPath: "data.tls\\.crt", Class: ClassTLSMaterial},
	{FieldPath: "data.tls\\.key", Class: ClassTLSMaterial},
	{FieldPath: "*.token", Class: ClassBearerToken},
	{FieldPath: "*.bearerToken", Class: ClassBearerToken},
}

// Apply walks an arbitrary JSON-shaped value (map[string]any /
// []any / scalar) and replaces fields that match any DefaultRule.
// The walk is destructive: callers pass a deep copy when retention of the
// original is required. Returns the same root value for fluent use.
//
// kind is the Kubernetes Kind of the root object (e.g. "Secret"); pass ""
// when the kind is unknown or doesn't matter.
func Apply(kind string, root any) any {
	walk(kind, "", root)
	return root
}

func walk(kind, path string, value any) {
	switch v := value.(type) {
	case map[string]any:
		for k, child := range v {
			childPath := joinPath(path, k)
			if cls, ok := matchRule(kind, childPath); ok {
				v[k] = Marker{Redacted: cls}
				continue
			}
			// Special case: a top-level map field that matches a rule on
			// its container (e.g. Secret.data) blanket-redacts all child
			// keys without descending.
			if cls, ok := matchRule(kind, childPath+".*"); ok {
				if asMap, isMap := child.(map[string]any); isMap {
					for ck := range asMap {
						asMap[ck] = Marker{Redacted: cls}
					}
					continue
				}
			}
			walk(kind, childPath, child)
		}
	case []any:
		for i, child := range v {
			walk(kind, joinPath(path, indexSegment(i)), child)
		}
	default:
		// scalar — nothing to redact
	}
}

func joinPath(parent, segment string) string {
	if parent == "" {
		return segment
	}
	return parent + "." + segment
}

func indexSegment(_ int) string { return "[]" }

// matchRule returns the redaction class for the first DefaultRule that
// matches the (kind, path) pair, or "", false when no rule matches.
func matchRule(kind, path string) (string, bool) {
	for _, r := range DefaultRules {
		if r.ParentKind != "" && r.ParentKind != kind {
			continue
		}
		if pathMatches(r.FieldPath, path) {
			return r.Class, true
		}
	}
	return "", false
}

// pathMatches compares a rule pattern against an observed path. Pattern
// segments separated by "." are matched literally except for "*", which
// matches any single segment. The pattern "spec.webhook.secret" matches
// the path "spec.webhook.secret"; the pattern "*.token" matches both
// "spec.token" and "auth.token".
func pathMatches(pattern, path string) bool {
	pp := strings.Split(pattern, ".")
	tp := strings.Split(path, ".")
	if len(pp) != len(tp) {
		return false
	}
	for i := range pp {
		// Unescape "\\." -> "." for paths that contain dots in field names
		// (e.g. ConfigMap.data["tls.crt"] flattens to "data.tls\\.crt"
		// in the path string).
		seg := strings.ReplaceAll(pp[i], `\.`, ".")
		if seg == "*" {
			continue
		}
		if seg != tp[i] {
			return false
		}
	}
	return true
}
