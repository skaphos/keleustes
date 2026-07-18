/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package controller

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	keleustesv1alpha1 "github.com/skaphos/keleustes/api/v1alpha1"
)

// scaffoldCase wires one CRD into the shared scaffold-reconciler assertion.
// The minimum-valid Spec for each kind comes from kubebuilder validation
// markers (see api/v1alpha1).
type scaffoldCase struct {
	kind       string
	makeSpec   func(name, namespace string) client.Object
	fresh      func() client.Object
	reconciler func() reconcile.Reconciler
	extract    func(client.Object) (observedGen int64, conditions []metav1.Condition)
}

var scaffoldCases = []scaffoldCase{
	{
		kind: "Application",
		makeSpec: func(name, ns string) client.Object {
			return &keleustesv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec: keleustesv1alpha1.ApplicationSpec{
					Deployment: keleustesv1alpha1.ApplicationDeployment{
						Strategy: keleustesv1alpha1.ApplicationDeploymentGitOps,
						Manifest: keleustesv1alpha1.ApplicationManifest{
							Type: keleustesv1alpha1.ApplicationManifestKustomize,
						},
					},
				},
			}
		},
		fresh: func() client.Object { return &keleustesv1alpha1.Application{} },
		reconciler: func() reconcile.Reconciler {
			return &ApplicationReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		},
		extract: func(o client.Object) (int64, []metav1.Condition) {
			x := o.(*keleustesv1alpha1.Application)
			return x.Status.ObservedGeneration, x.Status.Conditions
		},
	},
	{
		kind: "Source",
		makeSpec: func(name, ns string) client.Object {
			return &keleustesv1alpha1.Source{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec:       keleustesv1alpha1.SourceSpec{Type: keleustesv1alpha1.SourceTypeContainerImage},
			}
		},
		fresh:      func() client.Object { return &keleustesv1alpha1.Source{} },
		reconciler: func() reconcile.Reconciler { return &SourceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()} },
		extract: func(o client.Object) (int64, []metav1.Condition) {
			x := o.(*keleustesv1alpha1.Source)
			return x.Status.ObservedGeneration, x.Status.Conditions
		},
	},
	{
		kind: "Release",
		makeSpec: func(name, ns string) client.Object {
			return &keleustesv1alpha1.Release{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec: keleustesv1alpha1.ReleaseSpec{
					Application: "app",
					Artifacts: []keleustesv1alpha1.ReleaseArtifact{
						{Type: keleustesv1alpha1.ReleaseArtifactImage, Ref: "ghcr.io/example/app:1.0.0"},
					},
				},
			}
		},
		fresh:      func() client.Object { return &keleustesv1alpha1.Release{} },
		reconciler: func() reconcile.Reconciler { return &ReleaseReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()} },
		extract: func(o client.Object) (int64, []metav1.Condition) {
			x := o.(*keleustesv1alpha1.Release)
			return x.Status.ObservedGeneration, x.Status.Conditions
		},
	},
	{
		kind: "Deployment",
		makeSpec: func(name, ns string) client.Object {
			return &keleustesv1alpha1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec: keleustesv1alpha1.DeploymentSpec{
					Application: "app",
					TargetRef:   keleustesv1alpha1.LocalObjectReference{Name: "target"},
				},
			}
		},
		fresh: func() client.Object { return &keleustesv1alpha1.Deployment{} },
		reconciler: func() reconcile.Reconciler {
			return &DeploymentReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		},
		extract: func(o client.Object) (int64, []metav1.Condition) {
			x := o.(*keleustesv1alpha1.Deployment)
			return x.Status.ObservedGeneration, x.Status.Conditions
		},
	},
	{
		kind: "Environment",
		makeSpec: func(name, ns string) client.Object {
			return &keleustesv1alpha1.Environment{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec:       keleustesv1alpha1.EnvironmentSpec{Order: 10},
			}
		},
		fresh: func() client.Object { return &keleustesv1alpha1.Environment{} },
		reconciler: func() reconcile.Reconciler {
			return &EnvironmentReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		},
		extract: func(o client.Object) (int64, []metav1.Condition) {
			x := o.(*keleustesv1alpha1.Environment)
			return x.Status.ObservedGeneration, x.Status.Conditions
		},
	},
	{
		kind: "Cell",
		makeSpec: func(name, ns string) client.Object {
			return &keleustesv1alpha1.Cell{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec:       keleustesv1alpha1.CellSpec{Environment: "prod"},
			}
		},
		fresh:      func() client.Object { return &keleustesv1alpha1.Cell{} },
		reconciler: func() reconcile.Reconciler { return &CellReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()} },
		extract: func(o client.Object) (int64, []metav1.Condition) {
			x := o.(*keleustesv1alpha1.Cell)
			return x.Status.ObservedGeneration, x.Status.Conditions
		},
	},
	{
		kind: "DeploymentTarget",
		makeSpec: func(name, ns string) client.Object {
			return &keleustesv1alpha1.DeploymentTarget{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec: keleustesv1alpha1.DeploymentTargetSpec{
					Environment: "prod",
					Cluster:     keleustesv1alpha1.DeploymentTargetCluster{Name: "cluster-a"},
				},
			}
		},
		fresh: func() client.Object { return &keleustesv1alpha1.DeploymentTarget{} },
		reconciler: func() reconcile.Reconciler {
			return &DeploymentTargetReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		},
		extract: func(o client.Object) (int64, []metav1.Condition) {
			x := o.(*keleustesv1alpha1.DeploymentTarget)
			return x.Status.ObservedGeneration, x.Status.Conditions
		},
	},
	{
		kind: "Promotion",
		makeSpec: func(name, ns string) client.Object {
			return &keleustesv1alpha1.Promotion{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec: keleustesv1alpha1.PromotionSpec{
					Application: "app",
					Release:     "app-1.0.0",
					From:        keleustesv1alpha1.PromotionFrom{Environment: "qa"},
					To:          keleustesv1alpha1.PromotionTo{Environment: "prod"},
					Mode:        keleustesv1alpha1.PromotionModePullRequest,
				},
			}
		},
		fresh: func() client.Object { return &keleustesv1alpha1.Promotion{} },
		reconciler: func() reconcile.Reconciler {
			return &PromotionReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		},
		extract: func(o client.Object) (int64, []metav1.Condition) {
			x := o.(*keleustesv1alpha1.Promotion)
			return x.Status.ObservedGeneration, x.Status.Conditions
		},
	},
	{
		kind: "PromotionPolicy",
		makeSpec: func(name, ns string) client.Object {
			return &keleustesv1alpha1.PromotionPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec:       keleustesv1alpha1.PromotionPolicySpec{Required: []string{"imageSigned"}},
			}
		},
		fresh: func() client.Object { return &keleustesv1alpha1.PromotionPolicy{} },
		reconciler: func() reconcile.Reconciler {
			return &PromotionPolicyReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		},
		extract: func(o client.Object) (int64, []metav1.Condition) {
			x := o.(*keleustesv1alpha1.PromotionPolicy)
			return x.Status.ObservedGeneration, x.Status.Conditions
		},
	},
	{
		kind: "Approval",
		makeSpec: func(name, ns string) client.Object {
			return &keleustesv1alpha1.Approval{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec: keleustesv1alpha1.ApprovalSpec{
					PromotionRef: keleustesv1alpha1.LocalObjectReference{Name: "promo"},
					Decision:     keleustesv1alpha1.ApprovalDecisionApproved,
					Reviewer:     "alice",
				},
			}
		},
		fresh:      func() client.Object { return &keleustesv1alpha1.Approval{} },
		reconciler: func() reconcile.Reconciler { return &ApprovalReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()} },
		extract: func(o client.Object) (int64, []metav1.Condition) {
			x := o.(*keleustesv1alpha1.Approval)
			return x.Status.ObservedGeneration, x.Status.Conditions
		},
	},
	{
		kind: "FreezeWindow",
		makeSpec: func(name, ns string) client.Object {
			start := metav1.NewTime(time.Now().Add(time.Hour))
			end := metav1.NewTime(time.Now().Add(2 * time.Hour))
			return &keleustesv1alpha1.FreezeWindow{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec: keleustesv1alpha1.FreezeWindowSpec{
					Reason: "scheduled maintenance",
					Start:  start,
					End:    end,
				},
			}
		},
		fresh: func() client.Object { return &keleustesv1alpha1.FreezeWindow{} },
		reconciler: func() reconcile.Reconciler {
			return &FreezeWindowReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		},
		extract: func(o client.Object) (int64, []metav1.Condition) {
			x := o.(*keleustesv1alpha1.FreezeWindow)
			return x.Status.ObservedGeneration, x.Status.Conditions
		},
	},
	{
		kind: "SyncPlan",
		makeSpec: func(name, ns string) client.Object {
			return &keleustesv1alpha1.SyncPlan{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec: keleustesv1alpha1.SyncPlanSpec{
					Application: "app",
					TargetRefs:  []keleustesv1alpha1.LocalObjectReference{{Name: "target"}},
				},
			}
		},
		fresh:      func() client.Object { return &keleustesv1alpha1.SyncPlan{} },
		reconciler: func() reconcile.Reconciler { return &SyncPlanReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()} },
		extract: func(o client.Object) (int64, []metav1.Condition) {
			x := o.(*keleustesv1alpha1.SyncPlan)
			return x.Status.ObservedGeneration, x.Status.Conditions
		},
	},
	{
		kind: "SyncRun",
		makeSpec: func(name, ns string) client.Object {
			return &keleustesv1alpha1.SyncRun{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec: keleustesv1alpha1.SyncRunSpec{
					PlanRef:   keleustesv1alpha1.LocalObjectReference{Name: "plan"},
					TargetRef: keleustesv1alpha1.LocalObjectReference{Name: "target"},
				},
			}
		},
		fresh:      func() client.Object { return &keleustesv1alpha1.SyncRun{} },
		reconciler: func() reconcile.Reconciler { return &SyncRunReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()} },
		extract: func(o client.Object) (int64, []metav1.Condition) {
			x := o.(*keleustesv1alpha1.SyncRun)
			return x.Status.ObservedGeneration, x.Status.Conditions
		},
	},
	{
		kind: "HealthCheck",
		makeSpec: func(name, ns string) client.Object {
			return &keleustesv1alpha1.HealthCheck{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec:       keleustesv1alpha1.HealthCheckSpec{Application: "app"},
			}
		},
		fresh: func() client.Object { return &keleustesv1alpha1.HealthCheck{} },
		reconciler: func() reconcile.Reconciler {
			return &HealthCheckReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		},
		extract: func(o client.Object) (int64, []metav1.Condition) {
			x := o.(*keleustesv1alpha1.HealthCheck)
			return x.Status.ObservedGeneration, x.Status.Conditions
		},
	},
	{
		kind: "Notifier",
		makeSpec: func(name, ns string) client.Object {
			return &keleustesv1alpha1.Notifier{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec: keleustesv1alpha1.NotifierSpec{
					Endpoint: keleustesv1alpha1.NotifierEndpoint{
						Webhook: &keleustesv1alpha1.NotifierWebhookEndpoint{
							URL: "https://hooks.example.com/keleustes",
						},
					},
				},
			}
		},
		fresh: func() client.Object { return &keleustesv1alpha1.Notifier{} },
		reconciler: func() reconcile.Reconciler {
			return &NotifierReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		},
		extract: func(o client.Object) (int64, []metav1.Condition) {
			x := o.(*keleustesv1alpha1.Notifier)
			return x.Status.ObservedGeneration, x.Status.Conditions
		},
	},
}

var _ = Describe("Scaffold reconcilers", func() {
	for _, tc := range scaffoldCases {
		tc := tc

		It(fmt.Sprintf("%s reconciler marks the object Accepted with reason ScaffoldReconciler", tc.kind), func() {
			name := fmt.Sprintf("%s-scaffold", strings.ToLower(tc.kind))
			obj := tc.makeSpec(name, "default")
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(ctx, obj)
			})

			res, err := tc.reconciler().Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: "default", Name: name},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.RequeueAfter).To(BeZero())

			fetched := tc.fresh()
			Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: name}, fetched)).To(Succeed())

			observedGen, conditions := tc.extract(fetched)
			Expect(observedGen).To(Equal(fetched.GetGeneration()))

			cond := apiMeta.FindStatusCondition(conditions, conditionAccepted)
			Expect(cond).NotTo(BeNil(), "Accepted condition must be set")
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).To(Equal(reasonScaffoldReconciler))
			Expect(cond.ObservedGeneration).To(Equal(fetched.GetGeneration()))
		})
	}
})

var _ = Describe("Notifier endpoint XOR validation", func() {
	It("rejects a Notifier with both builtin and webhook set", func() {
		obj := &keleustesv1alpha1.Notifier{
			ObjectMeta: metav1.ObjectMeta{Name: "notifier-xor-both", Namespace: "default"},
			Spec: keleustesv1alpha1.NotifierSpec{
				Endpoint: keleustesv1alpha1.NotifierEndpoint{
					Builtin: "slack",
					Webhook: &keleustesv1alpha1.NotifierWebhookEndpoint{
						URL: "https://hooks.example.com/x",
					},
				},
			},
		}
		err := k8sClient.Create(ctx, obj)
		Expect(err).To(HaveOccurred(), "create should be rejected by the endpoint XOR rule")
		Expect(err.Error()).To(ContainSubstring("exactly one of endpoint.builtin or endpoint.webhook"))
	})

	It("rejects a Notifier with neither builtin nor webhook set", func() {
		obj := &keleustesv1alpha1.Notifier{
			ObjectMeta: metav1.ObjectMeta{Name: "notifier-xor-neither", Namespace: "default"},
			Spec:       keleustesv1alpha1.NotifierSpec{Endpoint: keleustesv1alpha1.NotifierEndpoint{}},
		}
		err := k8sClient.Create(ctx, obj)
		Expect(err).To(HaveOccurred(), "create should be rejected by the endpoint XOR rule")
		Expect(err.Error()).To(ContainSubstring("exactly one of endpoint.builtin or endpoint.webhook"))
	})

	It("accepts a Notifier with only builtin set", func() {
		obj := &keleustesv1alpha1.Notifier{
			ObjectMeta: metav1.ObjectMeta{Name: "notifier-xor-builtin", Namespace: "default"},
			Spec: keleustesv1alpha1.NotifierSpec{
				Endpoint: keleustesv1alpha1.NotifierEndpoint{Builtin: "stdout"},
			},
		}
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
	})
})
