/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploymentTargetCluster identifies the Kubernetes cluster behind a target.
type DeploymentTargetCluster struct {
	// Name is the cluster identifier as seen by Keleustes.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`

	// KubeconfigSecretRef references a Secret in the same namespace holding
	// the kubeconfig used to reach the cluster. When empty, Keleustes uses
	// its own service-account credentials.
	// +optional
	KubeconfigSecretRef *LocalObjectReference `json:"kubeconfigSecretRef,omitempty"`
}

// DeploymentTargetSpec defines the desired state of DeploymentTarget.
// See PROPOSAL §8.5.
type DeploymentTargetSpec struct {
	// Environment names the Environment this target belongs to.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Environment string `json:"environment"`

	// Cell names the Cell this target belongs to.
	// +optional
	// +kubebuilder:validation:MaxLength=253
	Cell string `json:"cell,omitempty"`

	// Region is the geographic or cloud-provider region.
	// +optional
	// +kubebuilder:validation:MaxLength=253
	Region string `json:"region,omitempty"`

	// Cluster identifies the cluster Keleustes reconciles into.
	Cluster DeploymentTargetCluster `json:"cluster"`
}

// DeploymentTargetStatus defines the observed state of DeploymentTarget.
type DeploymentTargetStatus struct {
	// ObservedGeneration is the most recent metadata.generation reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions summarize the controller's view of the target.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Reachable indicates whether Keleustes last contacted the target cluster
	// successfully.
	// +optional
	Reachable bool `json:"reachable,omitempty"`

	// LastContactedAt is the last successful contact timestamp.
	// +optional
	LastContactedAt *metav1.Time `json:"lastContactedAt,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Environment",type=string,JSONPath=`.spec.environment`
// +kubebuilder:printcolumn:name="Cell",type=string,JSONPath=`.spec.cell`
// +kubebuilder:printcolumn:name="Region",type=string,JSONPath=`.spec.region`
// +kubebuilder:printcolumn:name="Reachable",type=boolean,JSONPath=`.status.reachable`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// DeploymentTarget is a concrete place where an Application can run.
type DeploymentTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeploymentTargetSpec   `json:"spec,omitempty"`
	Status DeploymentTargetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DeploymentTargetList contains a list of DeploymentTarget.
type DeploymentTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeploymentTarget `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DeploymentTarget{}, &DeploymentTargetList{})
}
