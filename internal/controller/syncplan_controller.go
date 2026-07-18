/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
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

// SyncPlanReconciler reconciles a SyncPlan object. The MVP 1 Sync Engine —
// kustomize/helm/raw render, server-side apply, prune, drift — is not yet
// implemented; this stub marks each SyncPlan as accepted.
type SyncPlanReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=keleustes.skaphos.io,resources=syncplans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keleustes.skaphos.io,resources=syncplans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=keleustes.skaphos.io,resources=syncplans/finalizers,verbs=update
// +kubebuilder:rbac:groups=keleustes.skaphos.io,resources=syncruns,verbs=get;list;watch;create;update;patch;delete

func (r *SyncPlanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var plan keleustesv1alpha1.SyncPlan
	if err := r.Get(ctx, req.NamespacedName, &plan); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	before := plan.Status.DeepCopy()
	plan.Status.ObservedGeneration = plan.Generation
	apiMeta.SetStatusCondition(&plan.Status.Conditions, metav1.Condition{
		Type:               conditionAccepted,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: plan.Generation,
		Reason:             reasonScaffoldReconciler,
		Message:            "SyncPlan accepted; Sync Engine arrives with MVP 1.",
	})

	if equality.Semantic.DeepEqual(before, &plan.Status) {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, r.Status().Update(ctx, &plan)
}

// SetupWithManager registers the reconciler with the controller manager.
func (r *SyncPlanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keleustesv1alpha1.SyncPlan{}).
		Named("syncplan").
		Complete(r)
}
