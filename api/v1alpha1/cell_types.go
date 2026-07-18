/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CellSpec defines the desired state of Cell — a failure domain or operational
// grouping inside an Environment. See PROPOSAL §8.4.
type CellSpec struct {
	// Environment is the name of the Environment this Cell belongs to.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Environment string `json:"environment"`

	// Purpose is a short human-readable purpose tag (e.g., "guest-facing").
	// +optional
	// +kubebuilder:validation:MaxLength=253
	Purpose string `json:"purpose,omitempty"`

	// FailureBoundary names the failure-isolation boundary this Cell
	// represents (e.g., "guest", "internal").
	// +optional
	// +kubebuilder:validation:MaxLength=253
	FailureBoundary string `json:"failureBoundary,omitempty"`

	// Regions is the set of geographic or cloud-provider regions covered
	// by DeploymentTargets in this Cell.
	// +optional
	Regions []string `json:"regions,omitempty"`
}

// CellStatus defines the observed state of Cell.
type CellStatus struct {
	// ObservedGeneration is the most recent metadata.generation reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions summarize the controller's view of the Cell.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Environment",type=string,JSONPath=`.spec.environment`
// +kubebuilder:printcolumn:name="Boundary",type=string,JSONPath=`.spec.failureBoundary`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Cell is a failure domain or operational grouping inside an Environment.
type Cell struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CellSpec   `json:"spec,omitempty"`
	Status CellStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CellList contains a list of Cell.
type CellList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cell `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cell{}, &CellList{})
}
