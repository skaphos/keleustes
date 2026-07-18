/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NotifierSpec defines the desired state of a Notifier. See
// docs/plans/2026-05-extensibility-plugin-surfaces.md §3.1.
//
// Notifiers route lifecycle events to external sinks. Delivery is async
// and fail-open per plan §3.1 — a broken sink never blocks a Promotion
// or a SyncRun.
type NotifierSpec struct {
	// Endpoint is the delivery target. Exactly one of Builtin or Webhook
	// must be set; the CEL rule on NotifierEndpoint enforces the XOR.
	Endpoint NotifierEndpoint `json:"endpoint"`

	// Events selects which event classes this Notifier receives. An empty
	// Include matches every event class; Exclude is applied after Include.
	// Event-class names follow the audit-event-schema §13 registry
	// (e.g. "Promotion.Blocked", "SyncRun.Failed").
	// +optional
	Events NotifierEventSelector `json:"events,omitempty"`

	// Filters narrow matched events by resource attributes.
	// +optional
	Filters NotifierFilters `json:"filters,omitempty"`

	// Delivery controls how the Notifier delivers messages. Defaults
	// chosen to match plan §3.1 ("always async, fail-open").
	// +optional
	Delivery NotifierDelivery `json:"delivery,omitempty"`
}

// NotifierEndpoint declares where a Notifier sends its events. Exactly one
// of Builtin or Webhook must be set.
//
// +kubebuilder:validation:XValidation:rule="(has(self.builtin) && !has(self.webhook)) || (!has(self.builtin) && has(self.webhook))",message="exactly one of endpoint.builtin or endpoint.webhook must be set"
type NotifierEndpoint struct {
	// Builtin selects a Skaphos-provided notifier implementation.
	// MVP 1 lands the first built-in (`webhook`); the others appear as
	// their implementations ship per the extensibility plan's phased
	// rollout (§11).
	// +kubebuilder:validation:Enum=stdout;k8sEvent;slack;pagerduty;msteams
	// +optional
	Builtin string `json:"builtin,omitempty"`

	// Webhook delivers via HTTP POST to a customer-managed receiver.
	// +optional
	Webhook *NotifierWebhookEndpoint `json:"webhook,omitempty"`
}

// NotifierWebhookEndpoint is the inline webhook receiver configuration.
type NotifierWebhookEndpoint struct {
	// URL is the HTTPS endpoint receiving the notification. Plaintext
	// HTTP is rejected to keep authSecretRef tokens off the wire.
	// +kubebuilder:validation:Pattern=`^https://`
	// +kubebuilder:validation:MaxLength=2048
	URL string `json:"url"`

	// AuthSecretRef points at a Secret carrying a bearer token, shared
	// HMAC key, or other auth material the receiver will validate.
	// +optional
	AuthSecretRef *SecretKeyRef `json:"authSecretRef,omitempty"`
}

// NotifierEventSelector chooses which event classes flow through a
// Notifier. See audit-event-schema plan §13 for the canonical class list.
type NotifierEventSelector struct {
	// Include lists event class names that should be delivered. An empty
	// list matches every class. Entries are matched literally
	// (e.g. "Promotion.Blocked", "SyncRun.Failed").
	// +optional
	// +listType=set
	Include []string `json:"include,omitempty"`

	// Exclude lists event class names that should never be delivered,
	// applied after Include resolution.
	// +optional
	// +listType=set
	Exclude []string `json:"exclude,omitempty"`
}

// NotifierFilters narrow event delivery by resource attributes. Globs in
// Applications follow Go's filepath.Match syntax.
type NotifierFilters struct {
	// Environments restricts deliveries to events whose subject's
	// Environment label is in the list.
	// +optional
	// +listType=set
	Environments []string `json:"environments,omitempty"`

	// Applications restricts deliveries to events whose subject
	// Application name matches one of the glob patterns.
	// +optional
	// +listType=set
	Applications []string `json:"applications,omitempty"`
}

// NotifierDeliveryMode selects the delivery model. Per plan §3.1
// Notifiers are always async and fail-open; sync is informational only
// and does not change failure semantics.
//
// +kubebuilder:validation:Enum=async;sync
type NotifierDeliveryMode string

const (
	NotifierDeliveryModeAsync NotifierDeliveryMode = "async"
	NotifierDeliveryModeSync  NotifierDeliveryMode = "sync"
)

// NotifierBackoff selects the retry-interval strategy.
//
// +kubebuilder:validation:Enum=constant;exponential
type NotifierBackoff string

const (
	NotifierBackoffConstant    NotifierBackoff = "constant"
	NotifierBackoffExponential NotifierBackoff = "exponential"
)

// NotifierDelivery controls the wire-level delivery behavior.
type NotifierDelivery struct {
	// Mode controls whether delivery is asynchronous (default) or
	// synchronous. Notifiers remain fail-open in either mode.
	// +optional
	// +kubebuilder:default=async
	Mode NotifierDeliveryMode `json:"mode,omitempty"`

	// Timeout for a single delivery attempt.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Retries is the maximum number of additional attempts after a
	// failed delivery. Zero disables retries.
	// +optional
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	Retries int32 `json:"retries,omitempty"`

	// Backoff selects the retry-interval strategy.
	// +optional
	// +kubebuilder:default=exponential
	Backoff NotifierBackoff `json:"backoff,omitempty"`
}

// NotifierStatus defines the observed state of Notifier.
type NotifierStatus struct {
	// ObservedGeneration is the most recent metadata.generation
	// reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions summarize the controller's view of this Notifier. The
	// `Accepted` condition reflects schema acceptance; delivery health
	// surfaces under `Ready` once MVP 1 lands the dispatcher.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.spec.endpoint.builtin`
// +kubebuilder:printcolumn:name="Mode",type=string,JSONPath=`.spec.delivery.mode`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Notifier routes Keleustes lifecycle events to external sinks. See
// docs/plans/2026-05-extensibility-plugin-surfaces.md §3.1 and ADR 0001
// (plugin extension model).
type Notifier struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NotifierSpec   `json:"spec,omitempty"`
	Status NotifierStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NotifierList contains a list of Notifier.
type NotifierList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Notifier `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Notifier{}, &NotifierList{})
}
