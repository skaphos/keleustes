/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SyncPlanSpec defines the desired state of SyncPlan — the binding from an
// Application to one or more DeploymentTargets. SyncPlans drive the Sync Engine.
type SyncPlanSpec struct {
	// Application names the Application to reconcile.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Application string `json:"application"`

	// TargetRefs lists the DeploymentTarget resources reconciled by this plan.
	// +kubebuilder:validation:MinItems=1
	TargetRefs []LocalObjectReference `json:"targetRefs"`

	// AutoSync enables automatic reconciliation when desired state changes.
	// +optional
	AutoSync bool `json:"autoSync,omitempty"`

	// Suspended pauses reconciliation while preserving last-known state.
	// +optional
	Suspended bool `json:"suspended,omitempty"`
}

// SyncPlanStatus defines the observed state of SyncPlan.
type SyncPlanStatus struct {
	// ObservedGeneration is the most recent metadata.generation reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions summarize the controller's view of the plan.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// LastSyncRun names the most recent SyncRun produced by this plan.
	// +optional
	// +kubebuilder:validation:MaxLength=253
	LastSyncRun string `json:"lastSyncRun,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Application",type=string,JSONPath=`.spec.application`
// +kubebuilder:printcolumn:name="AutoSync",type=boolean,JSONPath=`.spec.autoSync`
// +kubebuilder:printcolumn:name="Suspended",type=boolean,JSONPath=`.spec.suspended`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SyncPlan declares which DeploymentTargets receive an Application's manifests.
type SyncPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SyncPlanSpec   `json:"spec,omitempty"`
	Status SyncPlanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SyncPlanList contains a list of SyncPlan.
type SyncPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyncPlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SyncPlan{}, &SyncPlanList{})
}
