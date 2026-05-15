/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReleaseArtifactType enumerates the kinds of artifact a Release can pin.
// +kubebuilder:validation:Enum=image;chart;ociArtifact
type ReleaseArtifactType string

const (
	ReleaseArtifactImage    ReleaseArtifactType = "image"
	ReleaseArtifactChart    ReleaseArtifactType = "chart"
	ReleaseArtifactOCIBlob  ReleaseArtifactType = "ociArtifact"
)

// ReleaseArtifact is a single pinned artifact making up a Release.
type ReleaseArtifact struct {
	// Type of the artifact.
	Type ReleaseArtifactType `json:"type"`

	// Ref is the artifact reference (e.g., a container image ref or chart URL).
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	Ref string `json:"ref"`

	// Digest is the immutable content addressable identifier (e.g., sha256:...).
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Digest string `json:"digest,omitempty"`

	// Version is the human-readable version of this artifact (e.g., a chart
	// semver or image tag) for UI display.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Version string `json:"version,omitempty"`
}

// ReleaseProvenance captures audit context describing how the Release was built.
type ReleaseProvenance struct {
	// Commit is the source commit SHA used to produce the artifacts.
	// +optional
	// +kubebuilder:validation:MaxLength=128
	Commit string `json:"commit,omitempty"`

	// BuildURL points at the CI build that produced the Release.
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	BuildURL string `json:"buildUrl,omitempty"`

	// SBOMRef references the SBOM artifact associated with the Release.
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	SBOMRef string `json:"sbomRef,omitempty"`
}

// ReleaseSpec defines the desired state of Release.
type ReleaseSpec struct {
	// Application names the Application this Release is for.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Application string `json:"application"`

	// Artifacts pins the deployable artifacts that make up the Release.
	// +kubebuilder:validation:MinItems=1
	Artifacts []ReleaseArtifact `json:"artifacts"`

	// Provenance records how the Release was produced.
	// +optional
	Provenance ReleaseProvenance `json:"provenance,omitempty"`
}

// ReleaseStatus defines the observed state of Release.
type ReleaseStatus struct {
	// ObservedGeneration is the most recent metadata.generation reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions summarize the controller's view of the Release.
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
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Release is a deployable collection of pinned artifacts for an Application.
// See PROPOSAL §8.6.
type Release struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReleaseSpec   `json:"spec,omitempty"`
	Status ReleaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ReleaseList contains a list of Release.
type ReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Release `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Release{}, &ReleaseList{})
}
