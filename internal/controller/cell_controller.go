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

// CellReconciler reconciles a Cell object.
type CellReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=keleustes.skaphos.dev,resources=cells,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keleustes.skaphos.dev,resources=cells/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=keleustes.skaphos.dev,resources=cells/finalizers,verbs=update

func (r *CellReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cell keleustesv1alpha1.Cell
	if err := r.Get(ctx, req.NamespacedName, &cell); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	before := cell.Status.DeepCopy()
	cell.Status.ObservedGeneration = cell.Generation
	apiMeta.SetStatusCondition(&cell.Status.Conditions, metav1.Condition{
		Type:               conditionAccepted,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: cell.Generation,
		Reason:             reasonSpecAccepted,
		Message:            "Cell specification accepted.",
	})

	if equality.Semantic.DeepEqual(before, &cell.Status) {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, r.Status().Update(ctx, &cell)
}

// SetupWithManager registers the reconciler with the controller manager.
func (r *CellReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keleustesv1alpha1.Cell{}).
		Named("cell").
		Complete(r)
}
