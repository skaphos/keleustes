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

// DeploymentTargetReconciler reconciles a DeploymentTarget object.
type DeploymentTargetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=keleustes.skaphos.io,resources=deploymenttargets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keleustes.skaphos.io,resources=deploymenttargets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=keleustes.skaphos.io,resources=deploymenttargets/finalizers,verbs=update

func (r *DeploymentTargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var target keleustesv1alpha1.DeploymentTarget
	if err := r.Get(ctx, req.NamespacedName, &target); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	before := target.Status.DeepCopy()
	target.Status.ObservedGeneration = target.Generation
	apiMeta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
		Type:               conditionAccepted,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: target.Generation,
		Reason:             reasonSpecAccepted,
		Message:            "DeploymentTarget specification accepted; cluster connectivity is not yet probed.",
	})

	if equality.Semantic.DeepEqual(before, &target.Status) {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, r.Status().Update(ctx, &target)
}

// SetupWithManager registers the reconciler with the controller manager.
func (r *DeploymentTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keleustesv1alpha1.DeploymentTarget{}).
		Named("deploymenttarget").
		Complete(r)
}
