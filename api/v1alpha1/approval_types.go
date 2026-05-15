/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApprovalDecision enumerates the possible outcomes of an Approval.
// +kubebuilder:validation:Enum=Pending;Approved;Rejected;Withdrawn
type ApprovalDecision string

const (
	ApprovalDecisionPending   ApprovalDecision = "Pending"
	ApprovalDecisionApproved  ApprovalDecision = "Approved"
	ApprovalDecisionRejected  ApprovalDecision = "Rejected"
	ApprovalDecisionWithdrawn ApprovalDecision = "Withdrawn"
)

// ApprovalSpec records an approver's decision on a referenced Promotion.
type ApprovalSpec struct {
	// PromotionRef references the Promotion this Approval applies to.
	PromotionRef LocalObjectReference `json:"promotionRef"`

	// Decision is the approver's decision.
	Decision ApprovalDecision `json:"decision"`

	// Reviewer identifies the approver (typically a user or group identifier).
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Reviewer string `json:"reviewer"`

	// Comment is an optional human-readable note attached to the decision.
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	Comment string `json:"comment,omitempty"`
}

// ApprovalStatus defines the observed state of Approval.
type ApprovalStatus struct {
	// ObservedGeneration is the most recent metadata.generation reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// RecordedAt is when the Approval was first observed by the controller.
	// +optional
	RecordedAt *metav1.Time `json:"recordedAt,omitempty"`

	// Conditions summarize the controller's view of the Approval.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Promotion",type=string,JSONPath=`.spec.promotionRef.name`
// +kubebuilder:printcolumn:name="Decision",type=string,JSONPath=`.spec.decision`
// +kubebuilder:printcolumn:name="Reviewer",type=string,JSONPath=`.spec.reviewer`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Approval records an approver's decision on a Promotion.
type Approval struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApprovalSpec   `json:"spec,omitempty"`
	Status ApprovalStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApprovalList contains a list of Approval.
type ApprovalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Approval `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Approval{}, &ApprovalList{})
}
