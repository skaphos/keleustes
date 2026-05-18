/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// newDescribeCommand returns the `keleustesctl describe` subcommand.
//
// Usage:
//
//	keleustesctl describe KIND NAME [-n namespace]
//	keleustesctl describe KIND/NAME [-n namespace]
//
// Output is a kubectl-style human-readable block: metadata, spec, status
// (with conditions rendered as a table). Read-only.
func newDescribeCommand() *cobra.Command {
	var (
		namespace  string
		kubeconfig string
	)

	cmd := &cobra.Command{
		Use:               "describe KIND/NAME",
		Short:             "Show detailed state of a Keleustes resource",
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: completeKindArg,
		RunE: func(cmd *cobra.Command, args []string) error {
			kindArg, name, err := splitDescribeArgs(args)
			if err != nil {
				return err
			}
			kind, err := resolveKind(kindArg)
			if err != nil {
				return err
			}
			kc, err := newKubeContext(kubeconfig)
			if err != nil {
				return err
			}
			obj, err := kc.Dynamic.Resource(kind.GVR).Namespace(namespace).Get(cmd.Context(), name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return fmt.Errorf("%s %q not found in namespace %q", kind.GVK.Kind, name, namespace)
				}
				return fmt.Errorf("get %s/%s: %w", kind.GVR.Resource, name, err)
			}
			return renderDescribe(cmd.OutOrStdout(), kind, obj)
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Namespace to query")
	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Explicit kubeconfig path (defaults to $KUBECONFIG or ~/.kube/config)")
	return cmd
}

// splitDescribeArgs accepts both kubectl-style argument shapes:
//
//	describe KIND NAME    (two args)
//	describe KIND/NAME    (one slash-joined arg)
func splitDescribeArgs(args []string) (kind, name string, err error) {
	switch len(args) {
	case 1:
		parts := strings.SplitN(args[0], "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", fmt.Errorf("describe requires KIND NAME or KIND/NAME, got %q", args[0])
		}
		return parts[0], parts[1], nil
	case 2:
		return args[0], args[1], nil
	default:
		return "", "", fmt.Errorf("describe requires KIND NAME or KIND/NAME")
	}
}

// errWriter latches the first write error and turns subsequent writes
// into no-ops. Lets the describe printer avoid threading 30 separate
// `if err := fmt.Fprintf(...); err != nil` checks while still surfacing
// failures (a broken pipe to a paged less, for example).
type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) printf(format string, args ...any) {
	if e.err != nil {
		return
	}
	_, e.err = fmt.Fprintf(e.w, format, args...)
}

// renderDescribe prints the object in a kubectl-describe-like layout.
// Metadata block first, then Spec (raw YAML), then Status (Conditions
// table when present).
func renderDescribe(w io.Writer, kind resolvedKind, obj *unstructured.Unstructured) error {
	ew := &errWriter{w: w}

	ew.printf("Kind:        %s\n", kind.GVK.Kind)
	ew.printf("Name:        %s\n", obj.GetName())
	ew.printf("Namespace:   %s\n", obj.GetNamespace())
	ew.printf("Created:     %s (%s ago)\n",
		obj.GetCreationTimestamp().Format("2006-01-02T15:04:05Z07:00"),
		formatAge(obj.GetCreationTimestamp()))
	ew.printf("Generation:  %d\n", obj.GetGeneration())

	if labels := obj.GetLabels(); len(labels) > 0 {
		ew.printf("Labels:\n")
		for _, k := range sortedKeys(labels) {
			ew.printf("  %s=%s\n", k, labels[k])
		}
	}
	if anns := obj.GetAnnotations(); len(anns) > 0 {
		ew.printf("Annotations:\n")
		for _, k := range sortedKeys(anns) {
			// Skip the kubectl/managed-fields noise so describe stays scannable.
			if strings.HasPrefix(k, "kubectl.kubernetes.io/") {
				continue
			}
			ew.printf("  %s=%s\n", k, anns[k])
		}
	}

	ew.printf("Spec:\n")
	writeIndentedYAML(ew, obj.Object["spec"], "  ")

	if status, ok := obj.Object["status"].(map[string]any); ok && len(status) > 0 {
		ew.printf("Status:\n")
		if og, ok := status["observedGeneration"]; ok {
			ew.printf("  ObservedGeneration: %v\n", og)
		}
		if phase, ok := status["phase"].(string); ok && phase != "" {
			ew.printf("  Phase:              %s\n", phase)
		}
		if state, ok := status["state"].(string); ok && state != "" {
			ew.printf("  State:              %s\n", state)
		}
		if conds, ok := status["conditions"].([]any); ok && len(conds) > 0 {
			renderConditions(ew, conds)
		}
		if blockers, ok := status["blockers"].([]any); ok && len(blockers) > 0 {
			ew.printf("  Blockers:\n")
			for _, b := range blockers {
				ew.printf("    - %v\n", b)
			}
		}
	}
	return ew.err
}

func renderConditions(ew *errWriter, conds []any) {
	ew.printf("  Conditions:\n")
	ew.printf("    %-15s %-7s %-25s %s\n", "TYPE", "STATUS", "REASON", "MESSAGE")
	for _, c := range conds {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}
		ew.printf("    %-15s %-7s %-25s %s\n",
			scalarToString(m["type"]),
			scalarToString(m["status"]),
			scalarToString(m["reason"]),
			scalarToString(m["message"]),
		)
	}
}

// writeIndentedYAML serializes value as YAML and indents every line
// with prefix. Used to inline-render the spec/status blocks under
// describe's flat key:value layout.
func writeIndentedYAML(ew *errWriter, value any, prefix string) {
	if value == nil {
		ew.printf("%s<empty>\n", prefix)
		return
	}
	b, err := yaml.Marshal(value)
	if err != nil {
		ew.printf("%s<marshal error: %v>\n", prefix, err)
		return
	}
	for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		ew.printf("%s%s\n", prefix, line)
	}
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
