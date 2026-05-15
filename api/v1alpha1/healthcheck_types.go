/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HealthState enumerates resource and application health outcomes. See PROPOSAL §12.
// +kubebuilder:validation:Enum=Healthy;Progressing;Degraded;Suspended;Missing;Unknown
type HealthState string

const (
	HealthStateHealthy     HealthState = "Healthy"
	HealthStateProgressing HealthState = "Progressing"
	HealthStateDegraded    HealthState = "Degraded"
	HealthStateSuspended   HealthState = "Suspended"
	HealthStateMissing     HealthState = "Missing"
	HealthStateUnknown     HealthState = "Unknown"
)

// HealthCheckSpec defines the desired state of HealthCheck. In Keleustes a
// HealthCheck describes how to evaluate the health of an Application on a
// DeploymentTarget; the implementation is staged across MVPs.
type HealthCheckSpec struct {
	// Application names the Application this check applies to.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Application string `json:"application"`

	// TargetRef references the DeploymentTarget the check evaluates.
	// +optional
	TargetRef *LocalObjectReference `json:"targetRef,omitempty"`
}

// HealthCheckStatus defines the observed state of HealthCheck.
type HealthCheckStatus struct {
	// ObservedGeneration is the most recent metadata.generation reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// State is the most recent aggregated health state.
	// +optional
	State HealthState `json:"state,omitempty"`

	// Summary is a human-readable one-line outcome description.
	// +optional
	// +kubebuilder:validation:MaxLength=1024
	Summary string `json:"summary,omitempty"`

	// Conditions summarize the controller's view of the check.
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
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// HealthCheck describes an application/target health evaluation.
type HealthCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HealthCheckSpec   `json:"spec,omitempty"`
	Status HealthCheckStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HealthCheckList contains a list of HealthCheck.
type HealthCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HealthCheck `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HealthCheck{}, &HealthCheckList{})
}
