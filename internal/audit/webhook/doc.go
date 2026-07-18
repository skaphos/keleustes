/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

// Package webhook hosts the audit-emitting admission handler.
//
// The handler is a sigs.k8s.io/controller-runtime/pkg/webhook/admission
// validator: it Allow=trues every state-changing request and emits one
// audit envelope per request as a side effect. Per audit-event-schema plan
// §11.1 "write-then-act," a failure from the underlying emitter causes the
// handler to Deny the request — the kube-apiserver then refuses to persist
// the change, so no state escapes without audit coverage.
//
// Registration of the ValidatingWebhookConfiguration is a follow-up: the
// config/ kustomize bases need cert-manager wiring + caBundle injection
// before the apiserver routes requests here. See README in this package
// for the production-wiring TODO.
package webhook
