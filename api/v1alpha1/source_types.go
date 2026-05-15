/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SourceType enumerates the deployable input streams a Source can describe.
// +kubebuilder:validation:Enum=containerImage;gitRepository;helmChart;ociArtifact
type SourceType string

const (
	SourceTypeContainerImage SourceType = "containerImage"
	SourceTypeGitRepository  SourceType = "gitRepository"
	SourceTypeHelmChart      SourceType = "helmChart"
	SourceTypeOCIArtifact    SourceType = "ociArtifact"
)

// SourceVerify configures provenance and signing checks the Source Engine
// applies before a revision is considered eligible for a Release.
type SourceVerify struct {
	// Cosign enables cosign signature verification on every observed revision.
	// +optional
	Cosign bool `json:"cosign,omitempty"`

	// RequireSBOM rejects revisions that have no associated SBOM.
	// +optional
	RequireSBOM bool `json:"requireSBOM,omitempty"`
}

// SourceSpec defines the desired state of Source.
type SourceSpec struct {
	// Type of the source stream.
	Type SourceType `json:"type"`

	// Image is the container image reference watched when Type is containerImage.
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	Image string `json:"image,omitempty"`

	// Repo is the Git repository URL when Type is gitRepository.
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	Repo string `json:"repo,omitempty"`

	// Chart references a Helm or OCI chart when Type is helmChart or ociArtifact.
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	Chart string `json:"chart,omitempty"`

	// Semver restricts observed revisions to those matching a semver range.
	// +optional
	// +kubebuilder:validation:MaxLength=128
	Semver string `json:"semver,omitempty"`

	// Verify configures provenance checks against each observed revision.
	// +optional
	Verify SourceVerify `json:"verify,omitempty"`
}

// SourceObservedRevision describes a single revision observed by the Source Engine.
type SourceObservedRevision struct {
	// Tag is the human-readable revision (e.g., a Git tag or image tag).
	// +kubebuilder:validation:MaxLength=253
	Tag string `json:"tag"`

	// Digest is the immutable content addressable identifier of the revision.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Digest string `json:"digest,omitempty"`

	// ObservedAt is when the Source Engine first observed this revision.
	// +optional
	ObservedAt *metav1.Time `json:"observedAt,omitempty"`
}

// SourceStatus defines the observed state of Source.
type SourceStatus struct {
	// ObservedGeneration is the most recent metadata.generation reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions summarize the controller's view of the Source.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// LatestRevision is the most recent revision observed by the Source Engine.
	// +optional
	LatestRevision *SourceObservedRevision `json:"latestRevision,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Latest",type=string,JSONPath=`.status.latestRevision.tag`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Source is a stream of deployable inputs observed by the Source Engine. See PROPOSAL §8.2.
type Source struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SourceSpec   `json:"spec,omitempty"`
	Status SourceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SourceList contains a list of Source.
type SourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Source `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Source{}, &SourceList{})
}
