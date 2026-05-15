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

// PromotionReconciler reconciles a Promotion object. The MVP 2 Promotion
// Engine — policy evaluation, Git mutation, sync orchestration — is not yet
// implemented; this stub keeps Phase=Proposed until the engine lands.
type PromotionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=keleustes.skaphos.dev,resources=promotions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keleustes.skaphos.dev,resources=promotions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=keleustes.skaphos.dev,resources=promotions/finalizers,verbs=update
// +kubebuilder:rbac:groups=keleustes.skaphos.dev,resources=releases;applications;promotionpolicies,verbs=get;list;watch

func (r *PromotionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var promo keleustesv1alpha1.Promotion
	if err := r.Get(ctx, req.NamespacedName, &promo); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	before := promo.Status.DeepCopy()
	promo.Status.ObservedGeneration = promo.Generation
	if promo.Status.Phase == "" {
		promo.Status.Phase = keleustesv1alpha1.PromotionPhaseProposed
	}
	apiMeta.SetStatusCondition(&promo.Status.Conditions, metav1.Condition{
		Type:               conditionAccepted,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: promo.Generation,
		Reason:             reasonSpecAccepted,
		Message:            "Promotion accepted; Promotion Engine arrives with MVP 2.",
	})

	if equality.Semantic.DeepEqual(before, &promo.Status) {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, r.Status().Update(ctx, &promo)
}

// SetupWithManager registers the reconciler with the controller manager.
func (r *PromotionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keleustesv1alpha1.Promotion{}).
		Named("promotion").
		Complete(r)
}
