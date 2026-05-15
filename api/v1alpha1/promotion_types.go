/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PromotionMode enumerates how a Promotion mutates Git or cluster state.
// See PROPOSAL §8.7.
// +kubebuilder:validation:Enum=directCommit;pullRequest;manualRecordOnly;dryRun
type PromotionMode string

const (
	PromotionModeDirectCommit     PromotionMode = "directCommit"
	PromotionModePullRequest      PromotionMode = "pullRequest"
	PromotionModeManualRecordOnly PromotionMode = "manualRecordOnly"
	PromotionModeDryRun           PromotionMode = "dryRun"
)

// PromotionPhase enumerates the lifecycle states of a Promotion. See PROPOSAL §8.7.
// +kubebuilder:validation:Enum=Proposed;Evaluating;Blocked;Approved;MutatingGit;WaitingForMerge;WaitingForSync;Verifying;Succeeded;Failed;RolledBack;Canceled
type PromotionPhase string

const (
	PromotionPhaseProposed        PromotionPhase = "Proposed"
	PromotionPhaseEvaluating      PromotionPhase = "Evaluating"
	PromotionPhaseBlocked         PromotionPhase = "Blocked"
	PromotionPhaseApproved        PromotionPhase = "Approved"
	PromotionPhaseMutatingGit     PromotionPhase = "MutatingGit"
	PromotionPhaseWaitingForMerge PromotionPhase = "WaitingForMerge"
	PromotionPhaseWaitingForSync  PromotionPhase = "WaitingForSync"
	PromotionPhaseVerifying       PromotionPhase = "Verifying"
	PromotionPhaseSucceeded       PromotionPhase = "Succeeded"
	PromotionPhaseFailed          PromotionPhase = "Failed"
	PromotionPhaseRolledBack      PromotionPhase = "RolledBack"
	PromotionPhaseCanceled        PromotionPhase = "Canceled"
)

// PromotionFrom narrows the source side of a promotion.
type PromotionFrom struct {
	// Environment is the source Environment name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Environment string `json:"environment"`
}

// PromotionTo narrows the destination side of a promotion.
type PromotionTo struct {
	// Environment is the destination Environment name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Environment string `json:"environment"`

	// Cells optionally restricts the promotion to the listed Cells.
	// +optional
	Cells []string `json:"cells,omitempty"`

	// Regions optionally restricts the promotion to the listed regions.
	// +optional
	Regions []string `json:"regions,omitempty"`
}

// PromotionChange records an external change-management identifier.
type PromotionChange struct {
	// Provider names the change-management system (e.g., servicenow, jira).
	// +optional
	// +kubebuilder:validation:MaxLength=253
	Provider string `json:"provider,omitempty"`

	// ID is the provider-local change identifier.
	// +optional
	// +kubebuilder:validation:MaxLength=253
	ID string `json:"id,omitempty"`
}

// PromotionSpec defines the desired state of Promotion.
type PromotionSpec struct {
	// Application names the Application being promoted.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Application string `json:"application"`

	// Release names the Release being promoted.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Release string `json:"release"`

	// From is the source of the promotion.
	From PromotionFrom `json:"from"`

	// To is the destination of the promotion.
	To PromotionTo `json:"to"`

	// Mode controls how Keleustes mutates Git or cluster state to enact the promotion.
	// +kubebuilder:default=pullRequest
	Mode PromotionMode `json:"mode"`

	// Change is an optional external change-management reference.
	// +optional
	Change PromotionChange `json:"change,omitempty"`

	// PolicyRefs names PromotionPolicy resources that must pass before the
	// Promotion can proceed.
	// +optional
	PolicyRefs []LocalObjectReference `json:"policyRefs,omitempty"`
}

// PromotionStatus defines the observed state of Promotion.
type PromotionStatus struct {
	// ObservedGeneration is the most recent metadata.generation reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is the current Promotion lifecycle phase.
	// +optional
	Phase PromotionPhase `json:"phase,omitempty"`

	// Blockers is a list of human-readable reasons the promotion is not advancing.
	// +optional
	Blockers []string `json:"blockers,omitempty"`

	// Conditions summarize the controller's view of the Promotion.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Application",type=string,JSONPath=`.spec.application`
// +kubebuilder:printcolumn:name="Release",type=string,JSONPath=`.spec.release`
// +kubebuilder:printcolumn:name="To",type=string,JSONPath=`.spec.to.environment`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Promotion is a requested movement of a Release into one or more deployment
// targets. See PROPOSAL §8.7.
type Promotion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PromotionSpec   `json:"spec,omitempty"`
	Status PromotionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PromotionList contains a list of Promotion.
type PromotionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Promotion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Promotion{}, &PromotionList{})
}
