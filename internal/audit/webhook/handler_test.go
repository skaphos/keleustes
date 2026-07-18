/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/skaphos/keleustes/internal/audit"
	"github.com/skaphos/keleustes/internal/audit/redaction"
)

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func newAppObject(name string) map[string]any {
	return map[string]any{
		"apiVersion": "keleustes.skaphos.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]any{"name": name, "namespace": "default"},
		"spec": map[string]any{
			"deployment": map[string]any{
				"strategy": "gitops",
				"manifest": map[string]any{"type": "kustomize"},
			},
		},
	}
}

func newReq(t *testing.T, op admissionv1.Operation, user string, obj, old any) admission.Request {
	t.Helper()
	var (
		objRaw, oldRaw []byte
	)
	if obj != nil {
		objRaw = mustJSON(t, obj)
	}
	if old != nil {
		oldRaw = mustJSON(t, old)
	}
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "req-001",
			Operation: op,
			Kind: metav1.GroupVersionKind{
				Group: "keleustes.skaphos.io", Version: "v1alpha1", Kind: "Application",
			},
			Namespace: "default",
			Name:      "checkout",
			Object:    runtime.RawExtension{Raw: objRaw},
			OldObject: runtime.RawExtension{Raw: oldRaw},
			UserInfo: authenticationv1.UserInfo{
				Username: user,
				UID:      "uid-" + user,
				Groups:   []string{"system:authenticated"},
			},
		},
	}
}

func TestHandle_CreateEmitsCreateVerbWithAfterOnly(t *testing.T) {
	t.Parallel()
	em := audit.NewInMemoryEmitter()
	h := NewHandler(em, logr.Discard(), "hub-test")

	req := newReq(t, admissionv1.Create, "alice@example.com", newAppObject("checkout"), nil)
	resp := h.Handle(context.Background(), req)
	if !resp.Allowed {
		t.Fatalf("create should be allowed, got %+v", resp)
	}
	events := em.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	got := events[0]
	if got.Action.Verb != audit.VerbCreate {
		t.Errorf("verb: got %q, want %q", got.Action.Verb, audit.VerbCreate)
	}
	if got.Result.Before != nil {
		t.Errorf("create: before should be nil, got %v", got.Result.Before)
	}
	if got.Result.After == nil {
		t.Errorf("create: after should be populated")
	}
	if got.Actor.Type != audit.ActorHuman {
		t.Errorf("actor.type for alice@example.com: got %q, want human", got.Actor.Type)
	}
	if got.Context.ClusterName != "hub-test" {
		t.Errorf("context.clusterName: got %q, want hub-test", got.Context.ClusterName)
	}
}

func TestHandle_DeleteEmitsDeleteVerbWithBeforeOnly(t *testing.T) {
	t.Parallel()
	em := audit.NewInMemoryEmitter()
	h := NewHandler(em, logr.Discard(), "")

	req := newReq(t, admissionv1.Delete, "alice@example.com", nil, newAppObject("checkout"))
	resp := h.Handle(context.Background(), req)
	if !resp.Allowed {
		t.Fatalf("delete should be allowed, got %+v", resp)
	}
	got := em.Events()[0]
	if got.Action.Verb != audit.VerbDelete {
		t.Errorf("verb: got %q, want %q", got.Action.Verb, audit.VerbDelete)
	}
	if got.Result.Before == nil {
		t.Errorf("delete: before should be populated")
	}
	if got.Result.After != nil {
		t.Errorf("delete: after should be nil, got %v", got.Result.After)
	}
}

func TestHandle_ServiceAccountUsernameMapsToSystem(t *testing.T) {
	t.Parallel()
	em := audit.NewInMemoryEmitter()
	h := NewHandler(em, logr.Discard(), "")

	req := newReq(t, admissionv1.Update,
		"system:serviceaccount:keleustes-system:controller-manager",
		newAppObject("checkout"), newAppObject("checkout"))
	_ = h.Handle(context.Background(), req)
	got := em.Events()[0]
	if got.Actor.Type != audit.ActorSystem {
		t.Errorf("system SA actor.type: got %q, want system", got.Actor.Type)
	}
}

func TestHandle_SecretBytesAreRedactedInSnapshot(t *testing.T) {
	t.Parallel()
	em := audit.NewInMemoryEmitter()
	h := NewHandler(em, logr.Discard(), "")

	secretObj := map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   map[string]any{"name": "creds"},
		"data": map[string]any{
			"password": "c2VjcmV0", // base64(secret)
		},
	}
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "req-secret",
			Operation: admissionv1.Create,
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
			Namespace: "default",
			Name:      "creds",
			Object:    runtime.RawExtension{Raw: mustJSON(t, secretObj)},
			UserInfo:  authenticationv1.UserInfo{Username: "alice@example.com"},
		},
	}
	resp := h.Handle(context.Background(), req)
	if !resp.Allowed {
		t.Fatalf("should be allowed, got %+v", resp)
	}
	got := em.Events()[0]
	after, ok := got.Result.After.(map[string]any)
	if !ok {
		t.Fatalf("after should be map[string]any, got %T", got.Result.After)
	}
	data, ok := after["data"].(map[string]any)
	if !ok {
		t.Fatalf("after.data should be map, got %T", after["data"])
	}
	marker, ok := data["password"].(redaction.Marker)
	if !ok {
		t.Fatalf("after.data.password should be redaction.Marker, got %T", data["password"])
	}
	if marker.Redacted != redaction.ClassSecretBytes {
		t.Errorf("redaction class: got %q, want %q", marker.Redacted, redaction.ClassSecretBytes)
	}
}

func TestHandle_EmitErrorDeniesAdmission(t *testing.T) {
	t.Parallel()
	h := NewHandler(failingEmitter{}, logr.Discard(), "")
	req := newReq(t, admissionv1.Create, "alice@example.com", newAppObject("checkout"), nil)
	resp := h.Handle(context.Background(), req)
	if resp.Allowed {
		t.Fatalf("emit failure should deny admission, got Allowed=true")
	}
	// Per plan §11.1 write-then-act: the kube-apiserver must not persist
	// changes that escape audit coverage.
	if resp.Result == nil || resp.Result.Code == http.StatusOK {
		t.Errorf("response should carry a non-OK status: %+v", resp.Result)
	}
}

func TestHandle_ConnectAllowedWithoutEmit(t *testing.T) {
	t.Parallel()
	em := audit.NewInMemoryEmitter()
	h := NewHandler(em, logr.Discard(), "")
	req := newReq(t, admissionv1.Connect, "alice@example.com", nil, nil)
	resp := h.Handle(context.Background(), req)
	if !resp.Allowed {
		t.Fatalf("connect should be allowed without emit, got %+v", resp)
	}
	if len(em.Events()) != 0 {
		t.Errorf("connect should not emit audit, got %d events", len(em.Events()))
	}
}

// failingEmitter is a test double that always returns an error from Emit.
type failingEmitter struct{}

func (failingEmitter) Emit(_ context.Context, _ audit.Envelope, _ audit.Payload) (string, error) {
	return "", errors.New("synthetic emit failure")
}
