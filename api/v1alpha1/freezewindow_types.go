/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FreezeWindowSpec defines a window during which promotions targeting the
// declared scope are blocked. See PROPOSAL §20 (MVP 3 enterprise topology).
type FreezeWindowSpec struct {
	// Reason is a short human-readable explanation displayed in the UI.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	Reason string `json:"reason"`

	// Start is the freeze window start (inclusive).
	Start metav1.Time `json:"start"`

	// End is the freeze window end (exclusive).
	End metav1.Time `json:"end"`

	// Environments optionally restricts the freeze to specific environments.
	// +optional
	Environments []string `json:"environments,omitempty"`

	// Cells optionally restricts the freeze to specific cells.
	// +optional
	Cells []string `json:"cells,omitempty"`
}

// FreezeWindowStatus defines the observed state of FreezeWindow.
type FreezeWindowStatus struct {
	// ObservedGeneration is the most recent metadata.generation reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Active reports whether the freeze window is currently in effect.
	// +optional
	Active bool `json:"active,omitempty"`

	// Conditions summarize the controller's view of the window.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Active",type=boolean,JSONPath=`.status.active`
// +kubebuilder:printcolumn:name="Start",type=string,JSONPath=`.spec.start`
// +kubebuilder:printcolumn:name="End",type=string,JSONPath=`.spec.end`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// FreezeWindow declares a period during which promotions to a scope are blocked.
type FreezeWindow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FreezeWindowSpec   `json:"spec,omitempty"`
	Status FreezeWindowStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FreezeWindowList contains a list of FreezeWindow.
type FreezeWindowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FreezeWindow `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FreezeWindow{}, &FreezeWindowList{})
}
