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

// DeploymentReconciler reconciles a Deployment object. Live-state reporting
// arrives with the Sync Engine; this stub marks each Deployment as accepted.
type DeploymentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=keleustes.skaphos.io,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keleustes.skaphos.io,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=keleustes.skaphos.io,resources=deployments/finalizers,verbs=update

func (r *DeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var obj keleustesv1alpha1.Deployment
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
		Message:            "Deployment accepted; live-state reporting arrives with the Sync Engine.",
	})

	if equality.Semantic.DeepEqual(before, &obj.Status) {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, r.Status().Update(ctx, &obj)
}

// SetupWithManager registers the reconciler with the controller manager.
func (r *DeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keleustesv1alpha1.Deployment{}).
		Named("deployment").
		Complete(r)
}
