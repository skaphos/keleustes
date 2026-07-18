/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SyncRunPhase enumerates the lifecycle states of a single SyncRun.
// +kubebuilder:validation:Enum=Pending;Rendering;Applying;Verifying;Succeeded;Failed
type SyncRunPhase string

const (
	SyncRunPhasePending   SyncRunPhase = "Pending"
	SyncRunPhaseRendering SyncRunPhase = "Rendering"
	SyncRunPhaseApplying  SyncRunPhase = "Applying"
	SyncRunPhaseVerifying SyncRunPhase = "Verifying"
	SyncRunPhaseSucceeded SyncRunPhase = "Succeeded"
	SyncRunPhaseFailed    SyncRunPhase = "Failed"
)

// SyncRunSpec defines the desired state of SyncRun. SyncRuns are typically
// created by the SyncPlan controller, not authored by hand.
type SyncRunSpec struct {
	// PlanRef references the SyncPlan that produced this run.
	PlanRef LocalObjectReference `json:"planRef"`

	// TargetRef references the DeploymentTarget this run reconciles into.
	TargetRef LocalObjectReference `json:"targetRef"`

	// Revision is the source commit or digest this run renders.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Revision string `json:"revision,omitempty"`
}

// SyncRunStatus defines the observed state of SyncRun.
type SyncRunStatus struct {
	// ObservedGeneration is the most recent metadata.generation reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is the current SyncRun lifecycle phase.
	// +optional
	Phase SyncRunPhase `json:"phase,omitempty"`

	// StartedAt is when the SyncRun began.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// CompletedAt is when the SyncRun reached a terminal phase.
	// +optional
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`

	// Conditions summarize the controller's view of the run.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Plan",type=string,JSONPath=`.spec.planRef.name`
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.targetRef.name`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SyncRun records a single attempt by the Sync Engine to reconcile a SyncPlan
// against a DeploymentTarget.
type SyncRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SyncRunSpec   `json:"spec,omitempty"`
	Status SyncRunStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SyncRunList contains a list of SyncRun.
type SyncRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyncRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SyncRun{}, &SyncRunList{})
}
