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

// ReleaseReconciler reconciles a Release object. Release-cut behavior is
// part of MVP 2; this stub marks each Release as accepted.
type ReleaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=keleustes.skaphos.io,resources=releases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keleustes.skaphos.io,resources=releases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=keleustes.skaphos.io,resources=releases/finalizers,verbs=update

func (r *ReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var obj keleustesv1alpha1.Release
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
		Message:            "Release accepted; release-cut behavior arrives with MVP 2.",
	})

	if equality.Semantic.DeepEqual(before, &obj.Status) {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, r.Status().Update(ctx, &obj)
}

// SetupWithManager registers the reconciler with the controller manager.
func (r *ReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keleustesv1alpha1.Release{}).
		Named("release").
		Complete(r)
}
