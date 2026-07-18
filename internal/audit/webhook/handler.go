/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/skaphos/keleustes/internal/audit"
	"github.com/skaphos/keleustes/internal/audit/payloads"
	"github.com/skaphos/keleustes/internal/audit/redaction"
)

// Handler is the admission handler that emits one audit envelope per
// kube-apiserver CRD write. Construct it with NewHandler and register with
// controller-runtime's webhook server via mgr.GetWebhookServer().Register.
type Handler struct {
	emitter     audit.Emitter
	log         logr.Logger
	clusterName string
}

// NewHandler returns a Handler that publishes to em and reports
// context.clusterName=clusterName on every envelope. Pass "" to skip the
// cluster-name field.
func NewHandler(em audit.Emitter, log logr.Logger, clusterName string) *Handler {
	return &Handler{emitter: em, log: log, clusterName: clusterName}
}

// Handle satisfies admission.Handler. The handler is mutation-free; it
// always returns Allow=true unless the audit emit fails, in which case it
// returns Denied with the underlying error as Reason. Read-only operations
// (Connect) are allowed through without emit.
func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	verb, ok := verbFor(req.Operation)
	if !ok {
		// Connect / unknown — no audit obligation.
		return admission.Allowed("")
	}

	env, payload, err := h.buildEnvelope(req, verb)
	if err != nil {
		h.log.Error(err, "audit envelope construction failed", "operation", req.Operation, "kind", req.Kind.Kind, "name", req.Name)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if _, err := audit.Emit(ctx, h.emitter, env, payload); err != nil {
		// Plan §11.4: state-changing verbs MUST refuse to act on emit
		// failure. Return Denied so the kube-apiserver doesn't persist
		// the change.
		h.log.Error(err, "audit emit failed; denying admission",
			"operation", req.Operation, "kind", req.Kind.Kind, "name", req.Name)
		return admission.Denied(fmt.Sprintf("audit emit failed: %v", err))
	}
	return admission.Allowed("")
}

// verbFor maps kube-apiserver admission operations to the registered audit
// verbs. Unknown operations (CONNECT) return ok=false so the handler can
// allow them through without emit.
func verbFor(op admissionv1.Operation) (audit.Verb, bool) {
	switch op {
	case admissionv1.Create:
		return audit.VerbCreate, true
	case admissionv1.Update:
		return audit.VerbEdit, true
	case admissionv1.Delete:
		return audit.VerbDelete, true
	default:
		return "", false
	}
}

// buildEnvelope constructs a populated Envelope plus the CRDWriteV1
// payload that pairs with it. Snapshots are redacted in place via the
// shared redaction package (plan §8.2).
func (h *Handler) buildEnvelope(req admission.Request, verb audit.Verb) (audit.Envelope, audit.Payload, error) {
	before, err := redactedSnapshot(req.OldObject.Raw, req.Kind.Kind)
	if err != nil {
		return audit.Envelope{}, nil, fmt.Errorf("decode oldObject: %w", err)
	}
	after, err := redactedSnapshot(req.Object.Raw, req.Kind.Kind)
	if err != nil {
		return audit.Envelope{}, nil, fmt.Errorf("decode object: %w", err)
	}

	env := audit.Envelope{
		RequestID: string(req.UID),
		Actor:     buildActor(req.UserInfo),
		Action: audit.Action{
			Verb: verb,
			Subject: audit.ActionSubject{
				APIGroup:  req.Kind.Group,
				Version:   req.Kind.Version,
				Kind:      req.Kind.Kind,
				Namespace: req.Namespace,
				Name:      req.Name,
			},
		},
		Context: audit.Context{
			ClusterName: h.clusterName,
		},
		Result: audit.Result{
			Outcome: audit.OutcomeSuccess,
			Before:  before,
			After:   after,
		},
	}

	return env, payloads.CRDWriteV1{
		SubresourceWrite: req.SubResource != "",
		DryRun:           req.DryRun != nil && *req.DryRun,
	}, nil
}

// buildActor maps the AdmissionRequest UserInfo to an audit.Actor. The
// IdP-aware normalization in plan §6.3 lands when the IdentityProvider
// CRD ships (SKA-330); for MVP 0 we attribute system service accounts to
// ActorSystem and everything else to ActorHuman.
func buildActor(u authenticationv1.UserInfo) audit.Actor {
	a := audit.Actor{
		Type:    actorTypeFor(u.Username),
		Subject: u.Username,
		Groups:  u.Groups,
	}
	if u.UID != "" {
		a.SubjectID = u.UID
	}
	return a
}

// actorTypeFor is a deliberately small heuristic: anything authenticated
// as a Kubernetes service account is treated as system. CI workload
// identity (`system:serviceaccount:default:github-actions-runner`) maps to
// the same bucket; richer ci/agent attribution lands with SKA-330.
func actorTypeFor(username string) audit.ActorType {
	if strings.HasPrefix(username, "system:serviceaccount:") || strings.HasPrefix(username, "system:") {
		return audit.ActorSystem
	}
	return audit.ActorHuman
}

// redactedSnapshot unmarshals a raw JSON object snapshot and applies the
// shared redaction rules. Returns (nil, nil) when raw is empty — the
// caller writes a JSON null to the envelope, which matches the plan §8.1
// convention for create/delete.
func redactedSnapshot(raw []byte, kind string) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	return redaction.Apply(kind, obj), nil
}
