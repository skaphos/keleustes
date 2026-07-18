/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package cli

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// resolvedKind ties a user-supplied kind alias to its canonical GVR + GVK.
type resolvedKind struct {
	GVK     schema.GroupVersionKind
	GVR     schema.GroupVersionResource
	Aliases []string
	// columns is the kubectl-style table column spec for `keleustesctl get`.
	// One entry per displayed column; the Name column is implicit and
	// always first. Age column is implicit and always last (rendered from
	// metadata.creationTimestamp).
	Columns []column
}

// column describes one printer column on the get output.
type column struct {
	Header   string
	JSONPath string // dot-path into the unstructured object
}

// keleustesGroup is the API group every Keleustes CRD lives under.
const keleustesGroup = "keleustes.skaphos.io"

const keleustesVersion = "v1alpha1"

// resolveKind maps a user-supplied alias (case-insensitive) to its
// canonical resolvedKind. Aliases include the kind name (Application),
// the lowercase singular (application), the plural (applications), and
// short aliases (app). Returns an error when the alias is unknown.
func resolveKind(s string) (resolvedKind, error) {
	key := strings.ToLower(s)
	for _, k := range keleustesKinds {
		if slices.Contains(k.Aliases, key) {
			return k, nil
		}
	}
	return resolvedKind{}, fmt.Errorf("unknown kind %q; valid kinds: %s",
		s, strings.Join(knownAliases(), ", "))
}

// knownAliases returns the canonical alias (singular lowercase plural) for
// every kind, sorted, for error messages and shell completion.
func knownAliases() []string {
	out := make([]string, 0, len(keleustesKinds))
	for _, k := range keleustesKinds {
		// Pick the plural alias as the canonical user-facing name.
		// Aliases are stored in the order: short, singular, plural.
		out = append(out, k.Aliases[len(k.Aliases)-1])
	}
	sort.Strings(out)
	return out
}

func gvk(kind string) schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: keleustesGroup, Version: keleustesVersion, Kind: kind}
}

func gvr(resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: keleustesGroup, Version: keleustesVersion, Resource: resource}
}

// keleustesKinds is the registry the CLI uses to resolve aliases, look up
// GVR for dynamic-client calls, and pick table columns. Column choices
// mirror the kubebuilder `+kubebuilder:printcolumn` markers in
// api/v1alpha1/*. When you add a new CRD, add a row here.
var keleustesKinds = []resolvedKind{
	{
		GVK:     gvk("Application"),
		GVR:     gvr("applications"),
		Aliases: []string{"app", "application", "applications"},
		Columns: []column{
			{"Strategy", ".spec.deployment.strategy"},
			{"Manifest", ".spec.deployment.manifest.type"},
			{"Status", ".status.conditions[?(@.type=='Accepted')].status"},
		},
	},
	{
		GVK:     gvk("Source"),
		GVR:     gvr("sources"),
		Aliases: []string{"src", "source", "sources"},
		Columns: []column{
			{"Type", ".spec.type"},
			{"Status", ".status.conditions[?(@.type=='Accepted')].status"},
		},
	},
	{
		GVK:     gvk("Release"),
		GVR:     gvr("releases"),
		Aliases: []string{"rel", "release", "releases"},
		Columns: []column{
			{"Application", ".spec.application"},
			{"Artifacts", ".spec.artifacts[*].type"},
		},
	},
	{
		GVK:     gvk("Deployment"),
		GVR:     gvr("deployments"),
		Aliases: []string{"deploy", "deployment", "deployments"},
		Columns: []column{
			{"Application", ".spec.application"},
			{"Target", ".spec.targetRef.name"},
			{"Status", ".status.conditions[?(@.type=='Accepted')].status"},
		},
	},
	{
		GVK:     gvk("Environment"),
		GVR:     gvr("environments"),
		Aliases: []string{"env", "environment", "environments"},
		Columns: []column{
			{"Order", ".spec.order"},
			{"Status", ".status.conditions[?(@.type=='Accepted')].status"},
		},
	},
	{
		GVK:     gvk("Cell"),
		GVR:     gvr("cells"),
		Aliases: []string{"cell", "cells"},
		Columns: []column{
			{"Environment", ".spec.environment"},
			{"Status", ".status.conditions[?(@.type=='Accepted')].status"},
		},
	},
	{
		GVK:     gvk("DeploymentTarget"),
		GVR:     gvr("deploymenttargets"),
		Aliases: []string{"dt", "deploymenttarget", "deploymenttargets"},
		Columns: []column{
			{"Environment", ".spec.environment"},
			{"Cluster", ".spec.cluster.name"},
			{"Status", ".status.conditions[?(@.type=='Accepted')].status"},
		},
	},
	{
		GVK:     gvk("Promotion"),
		GVR:     gvr("promotions"),
		Aliases: []string{"promo", "promotion", "promotions"},
		Columns: []column{
			{"Application", ".spec.application"},
			{"Release", ".spec.release"},
			{"To", ".spec.to.environment"},
			{"Phase", ".status.phase"},
		},
	},
	{
		GVK:     gvk("PromotionPolicy"),
		GVR:     gvr("promotionpolicies"),
		Aliases: []string{"pp", "promotionpolicy", "promotionpolicies"},
		Columns: []column{
			{"Required", ".spec.required[*]"},
		},
	},
	{
		GVK:     gvk("Approval"),
		GVR:     gvr("approvals"),
		Aliases: []string{"appr", "approval", "approvals"},
		Columns: []column{
			{"Promotion", ".spec.promotionRef.name"},
			{"Decision", ".spec.decision"},
			{"Reviewer", ".spec.reviewer"},
		},
	},
	{
		GVK:     gvk("FreezeWindow"),
		GVR:     gvr("freezewindows"),
		Aliases: []string{"fw", "freezewindow", "freezewindows"},
		Columns: []column{
			{"Reason", ".spec.reason"},
			{"Start", ".spec.start"},
			{"End", ".spec.end"},
		},
	},
	{
		GVK:     gvk("SyncPlan"),
		GVR:     gvr("syncplans"),
		Aliases: []string{"sp", "syncplan", "syncplans"},
		Columns: []column{
			{"Application", ".spec.application"},
			{"Targets", ".spec.targetRefs[*].name"},
		},
	},
	{
		GVK:     gvk("SyncRun"),
		GVR:     gvr("syncruns"),
		Aliases: []string{"sr", "syncrun", "syncruns"},
		Columns: []column{
			{"Plan", ".spec.planRef.name"},
			{"Target", ".spec.targetRef.name"},
			{"Phase", ".status.phase"},
		},
	},
	{
		GVK:     gvk("HealthCheck"),
		GVR:     gvr("healthchecks"),
		Aliases: []string{"hc", "healthcheck", "healthchecks"},
		Columns: []column{
			{"Application", ".spec.application"},
			{"State", ".status.state"},
		},
	},
	{
		GVK:     gvk("Notifier"),
		GVR:     gvr("notifiers"),
		Aliases: []string{"notif", "notifier", "notifiers"},
		Columns: []column{
			{"Builtin", ".spec.endpoint.builtin"},
			{"Mode", ".spec.delivery.mode"},
		},
	},
}
