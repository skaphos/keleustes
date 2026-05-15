/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplicationManifestType enumerates supported manifest renderers for an
// Application's deployment payload. The initial sync engine is intentionally
// constrained to these three patterns; see PROPOSAL §11.
// +kubebuilder:validation:Enum=kustomize;helm;raw
type ApplicationManifestType string

const (
	ApplicationManifestKustomize ApplicationManifestType = "kustomize"
	ApplicationManifestHelm      ApplicationManifestType = "helm"
	ApplicationManifestRaw       ApplicationManifestType = "raw"
)

// ApplicationManifest configures how desired-state manifests are rendered for
// an Application.
type ApplicationManifest struct {
	// Type is the manifest renderer.
	Type ApplicationManifestType `json:"type"`

	// Repo is the Git repository URL (e.g., github.com/example/platform-state)
	// from which manifests are read. Required when Type is kustomize or raw.
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	Repo string `json:"repo,omitempty"`

	// BasePath is the path inside Repo containing the manifest root. For
	// Helm this is the chart directory; for Kustomize the kustomization
	// directory; for raw the directory of manifests.
	// +optional
	// +kubebuilder:validation:MaxLength=1024
	BasePath string `json:"basePath,omitempty"`

	// Chart names an OCI or Helm chart reference when Type is helm.
	// +optional
	// +kubebuilder:validation:MaxLength=512
	Chart string `json:"chart,omitempty"`
}

// ApplicationDeploymentStrategy controls how Keleustes applies manifests for
// the Application.
// +kubebuilder:validation:Enum=gitops;directApply
type ApplicationDeploymentStrategy string

const (
	ApplicationDeploymentGitOps      ApplicationDeploymentStrategy = "gitops"
	ApplicationDeploymentDirectApply ApplicationDeploymentStrategy = "directApply"
)

// ApplicationDeployment wires manifest rendering and deployment strategy.
type ApplicationDeployment struct {
	// Strategy selects how manifests are reconciled. gitops is the default and
	// only fully supported strategy in MVP 0 and 1.
	// +kubebuilder:default=gitops
	Strategy ApplicationDeploymentStrategy `json:"strategy"`

	// Manifest configures the renderer that produces desired-state manifests.
	Manifest ApplicationManifest `json:"manifest"`
}

// ApplicationTopology describes where an Application is expected to be deployed.
// The Topology block is informational in MVP 0; it becomes load-bearing once
// `Promotion` is wired up in MVP 2.
type ApplicationTopology struct {
	// Environments lists the names of Environment resources this application
	// targets, in promotion order.
	// +optional
	Environments []string `json:"environments,omitempty"`
}

// ApplicationSpec defines the desired state of Application.
type ApplicationSpec struct {
	// Owner describes the responsible team. Used by the UI and audit trails.
	// +optional
	Owner OwnerInfo `json:"owner,omitempty"`

	// SourceRefs are Source resources this Application consumes.
	// +optional
	SourceRefs []LocalObjectReference `json:"sourceRefs,omitempty"`

	// Deployment configures how the application is reconciled into clusters.
	Deployment ApplicationDeployment `json:"deployment"`

	// Topology declares the application's intended environment scope.
	// +optional
	Topology ApplicationTopology `json:"topology,omitempty"`
}

// ApplicationStatus defines the observed state of Application.
type ApplicationStatus struct {
	// ObservedGeneration is the most recent metadata.generation reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions summarize the controller's view of the Application.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=`.spec.deployment.strategy`
// +kubebuilder:printcolumn:name="Manifest",type=string,JSONPath=`.spec.deployment.manifest.type`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Application is the central delivery abstraction in Keleustes. See PROPOSAL §8.1.
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationSpec   `json:"spec,omitempty"`
	Status ApplicationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationList contains a list of Application.
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Application{}, &ApplicationList{})
}
