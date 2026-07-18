/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package v1alpha1

// LocalObjectReference references an object in the same namespace by name.
type LocalObjectReference struct {
	// Name of the referenced object.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`
}

// SecretKeyRef references a key inside a Secret in the same namespace.
type SecretKeyRef struct {
	// Name of the Secret.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`

	// Key inside the Secret. When empty the consumer chooses a sensible default.
	// +optional
	// +kubebuilder:validation:MaxLength=253
	Key string `json:"key,omitempty"`
}

// OwnerInfo records who owns an Application. It is descriptive metadata that
// downstream UIs surface; the operator does not enforce its contents.
type OwnerInfo struct {
	// Team is the owning team identifier.
	// +optional
	// +kubebuilder:validation:MaxLength=253
	Team string `json:"team,omitempty"`

	// Contact is a human-readable contact channel (e.g., Slack channel,
	// email distribution list).
	// +optional
	// +kubebuilder:validation:MaxLength=253
	Contact string `json:"contact,omitempty"`
}
