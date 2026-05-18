/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package cli

import (
	"fmt"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// kubeContext bundles the rest.Config and dynamic client a command uses.
// Kept separate from the cobra command tree so tests can construct one
// against envtest or a fake client.
type kubeContext struct {
	Config  *rest.Config
	Dynamic dynamic.Interface
}

// newKubeContext resolves kubeconfig the same way kubectl does:
//   - explicit --kubeconfig flag value when non-empty,
//   - $KUBECONFIG env var,
//   - $HOME/.kube/config fallback (set by clientcmd).
//
// The dynamic client is GVR-keyed so the CLI's read commands can talk to
// every Keleustes CRD without registering the typed scheme.
func newKubeContext(kubeconfig string) (*kubeContext, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build dynamic client: %w", err)
	}
	return &kubeContext{Config: cfg, Dynamic: dyn}, nil
}
