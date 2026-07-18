/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
)

// newGetCommand returns the `keleustesctl get` subcommand.
//
// Usage:
//
//	keleustesctl get <kind> [name] [-n namespace] [-A] [-o table|yaml|json] [-w]
//
// Read-only. No mutating flags.
func newGetCommand() *cobra.Command {
	var (
		namespace     string
		allNamespaces bool
		output        string
		watchMode     bool
		kubeconfig    string
	)

	cmd := &cobra.Command{
		Use:   "get KIND [NAME]",
		Short: "List or fetch Keleustes resources",
		Long: "List the named Keleustes kind (Applications, SyncRuns, Promotions, ...) " +
			"or fetch a single resource by name. Output defaults to a kubectl-style table; " +
			"use -o yaml or -o json for round-trippable representation.",
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: completeKindArg,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(output)
			if err != nil {
				return err
			}
			kind, err := resolveKind(args[0])
			if err != nil {
				return err
			}
			kc, err := newKubeContext(kubeconfig)
			if err != nil {
				return err
			}

			ns := namespace
			if allNamespaces {
				ns = ""
			}
			ctx := cmd.Context()

			if len(args) == 2 {
				if watchMode {
					return fmt.Errorf("--watch is only supported on list operations")
				}
				return runGetOne(ctx, cmd, kc, kind, ns, args[1], format)
			}
			if watchMode {
				return runWatch(ctx, cmd, kc, kind, ns)
			}
			return runGetList(ctx, cmd, kc, kind, ns, format)
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Namespace to query")
	cmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "If present, list across all namespaces")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output format: table (default), yaml, json")
	cmd.Flags().BoolVarP(&watchMode, "watch", "w", false, "Watch for changes (table output only)")
	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Explicit kubeconfig path (defaults to $KUBECONFIG or ~/.kube/config)")

	_ = cmd.RegisterFlagCompletionFunc("output", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"table", "yaml", "json"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runGetList(ctx context.Context, cmd *cobra.Command, kc *kubeContext, kind resolvedKind, namespace string, format outputFormat) error {
	var (
		list *unstructured.UnstructuredList
		err  error
	)
	if namespace == "" {
		list, err = kc.Dynamic.Resource(kind.GVR).List(ctx, metav1.ListOptions{})
	} else {
		list, err = kc.Dynamic.Resource(kind.GVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return fmt.Errorf("list %s: %w", kind.GVR.Resource, err)
	}
	return renderList(cmd.OutOrStdout(), kind, list.Items, format)
}

func runGetOne(ctx context.Context, cmd *cobra.Command, kc *kubeContext, kind resolvedKind, namespace, name string, format outputFormat) error {
	obj, err := kc.Dynamic.Resource(kind.GVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("%s %q not found in namespace %q", kind.GVK.Kind, name, namespace)
		}
		return fmt.Errorf("get %s/%s: %w", kind.GVR.Resource, name, err)
	}
	return renderOne(cmd.OutOrStdout(), kind, obj, format)
}

func runWatch(ctx context.Context, cmd *cobra.Command, kc *kubeContext, kind resolvedKind, namespace string) error {
	var (
		w   watch.Interface
		err error
	)
	if namespace == "" {
		w, err = kc.Dynamic.Resource(kind.GVR).Watch(ctx, metav1.ListOptions{})
	} else {
		w, err = kc.Dynamic.Resource(kind.GVR).Namespace(namespace).Watch(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return fmt.Errorf("watch %s: %w", kind.GVR.Resource, err)
	}
	defer w.Stop()

	// Print one header line, then a row per event. Event-type marker
	// goes in front of NAME so the output stays kubectl-like.
	out := cmd.OutOrStdout()
	headers := []string{"EVENT", "NAME"}
	for _, c := range kind.Columns {
		headers = append(headers, strings.ToUpper(c.Header))
	}
	headers = append(headers, "AGE")
	if _, err := fmt.Fprintln(out, strings.Join(headers, "\t")); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-w.ResultChan():
			if !ok {
				return nil
			}
			obj, ok := ev.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			row := []string{string(ev.Type), obj.GetName()}
			for _, c := range kind.Columns {
				row = append(row, extractColumn(obj, c.JSONPath))
			}
			row = append(row, formatAge(obj.GetCreationTimestamp()))
			if _, err := fmt.Fprintln(out, strings.Join(row, "\t")); err != nil {
				return err
			}
		}
	}
}

// completeKindArg supplies bash/zsh/fish completion for the first
// argument of `get` / `describe`.
func completeKindArg(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return knownAliases(), cobra.ShellCompDirectiveNoFileComp
}
