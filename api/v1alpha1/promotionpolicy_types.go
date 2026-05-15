/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PromotionPolicySpec defines the desired state of PromotionPolicy. See PROPOSAL §8.8.
type PromotionPolicySpec struct {
	// Required lists native policy gate identifiers (e.g., imageSigned,
	// sbomPresent, vulnThreshold, sourceHealthy, targetUnlocked, changeApproved,
	// ownerApproved) that must succeed for a referencing Promotion to proceed.
	// +kubebuilder:validation:MinItems=1
	Required []string `json:"required"`
}

// PromotionPolicyStatus defines the observed state of PromotionPolicy.
type PromotionPolicyStatus struct {
	// ObservedGeneration is the most recent metadata.generation reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions summarize the controller's view of the policy.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// PromotionPolicy declares the gates required by a Promotion.
type PromotionPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PromotionPolicySpec   `json:"spec,omitempty"`
	Status PromotionPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PromotionPolicyList contains a list of PromotionPolicy.
type PromotionPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PromotionPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PromotionPolicy{}, &PromotionPolicyList{})
}
