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
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	keleustesv1alpha1 "github.com/skaphos/keleustes/api/v1alpha1"
)

const (
	conditionAccepted = "Accepted"
	reasonSpecAccepted = "SpecAccepted"
)

// ApplicationReconciler reconciles an Application object.
type ApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=keleustes.skaphos.dev,resources=applications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keleustes.skaphos.dev,resources=applications/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=keleustes.skaphos.dev,resources=applications/finalizers,verbs=update

func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx).WithValues("namespacedName", req.NamespacedName)

	var app keleustesv1alpha1.Application
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	before := app.Status.DeepCopy()
	app.Status.ObservedGeneration = app.Generation
	apiMeta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type:               conditionAccepted,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: app.Generation,
		Reason:             reasonSpecAccepted,
		Message:            "Application specification accepted; reconciliation engines pending.",
	})

	if equality.Semantic.DeepEqual(before, &app.Status) {
		return ctrl.Result{}, nil
	}
	if err := r.Status().Update(ctx, &app); err != nil {
		return ctrl.Result{}, err
	}
	log.V(1).Info("updated Application status")
	return ctrl.Result{}, nil
}

// SetupWithManager registers the reconciler with the controller manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keleustesv1alpha1.Application{}).
		Named("application").
		Complete(r)
}
