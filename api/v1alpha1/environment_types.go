/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnvironmentChangeControl declares whether promotions into the Environment
// require an external change record (e.g., ServiceNow CRQ).
type EnvironmentChangeControl struct {
	// Required indicates change control is required for promotions targeting
	// this Environment.
	// +optional
	Required bool `json:"required,omitempty"`
}

// EnvironmentSpec defines the desired state of Environment.
type EnvironmentSpec struct {
	// Order is the sort key used by the UI and promotion engine to render
	// environments in lifecycle order. Lower values come first.
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000
	Order int32 `json:"order"`

	// Protected marks the environment as one requiring elevated promotion gates.
	// +optional
	Protected bool `json:"protected,omitempty"`

	// ChangeControl declares whether promotions require an external change record.
	// +optional
	ChangeControl EnvironmentChangeControl `json:"changeControl,omitempty"`
}

// EnvironmentStatus defines the observed state of Environment.
type EnvironmentStatus struct {
	// ObservedGeneration is the most recent metadata.generation reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions summarize the controller's view of the Environment.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Order",type=integer,JSONPath=`.spec.order`
// +kubebuilder:printcolumn:name="Protected",type=boolean,JSONPath=`.spec.protected`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Environment is an ordered lifecycle boundary such as dev, test, qa, or prod.
// See PROPOSAL §8.3.
type Environment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvironmentSpec   `json:"spec,omitempty"`
	Status EnvironmentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EnvironmentList contains a list of Environment.
type EnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Environment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Environment{}, &EnvironmentList{})
}
