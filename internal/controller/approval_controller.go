/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	keleustesv1alpha1 "github.com/skaphos/keleustes/api/v1alpha1"
)

// ApprovalReconciler reconciles an Approval object. Approval semantics —
// recording who approved what, tying back to Promotions — arrive with MVP 2;
// this stub marks each Approval as accepted.
type ApprovalReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=keleustes.skaphos.io,resources=approvals,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keleustes.skaphos.io,resources=approvals/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=keleustes.skaphos.io,resources=approvals/finalizers,verbs=update

func (r *ApprovalReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var obj keleustesv1alpha1.Approval
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	before := obj.Status.DeepCopy()
	obj.Status.ObservedGeneration = obj.Generation
	apiMeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type:               conditionAccepted,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: obj.Generation,
		Reason:             reasonScaffoldReconciler,
		Message:            "Approval accepted; approval semantics arrive with MVP 2.",
	})

	if equality.Semantic.DeepEqual(before, &obj.Status) {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, r.Status().Update(ctx, &obj)
}

// SetupWithManager registers the reconciler with the controller manager.
func (r *ApprovalReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keleustesv1alpha1.Approval{}).
		Named("approval").
		Complete(r)
}
