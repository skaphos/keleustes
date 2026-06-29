/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package crdsource

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	keleustesv1alpha1 "github.com/skaphos/keleustes/api/v1alpha1"
	"github.com/skaphos/keleustes/internal/api/openapi"
	"github.com/skaphos/keleustes/internal/api/readmodel"
)

// accept stamps the scaffold Accepted=True condition on a status object, the way
// the reconcilers do, so the status mapping has something real to read.
func accept(conds *[]metav1.Condition, gen int64) {
	apiMeta.SetStatusCondition(conds, metav1.Condition{
		Type:               conditionAccepted,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: gen,
		Reason:             "Test",
		Message:            "accepted",
	})
}

var _ = Describe("crdsource ReadModel", func() {
	var src *Source

	BeforeEach(func() {
		src = New(k8sClient)
	})

	It("maps an Application onto the product-concept shape", func() {
		app := &keleustesv1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "checkout",
				Namespace: "default",
				Labels:    map[string]string{labelPartOf: "payments"},
			},
			Spec: keleustesv1alpha1.ApplicationSpec{
				Owner: keleustesv1alpha1.OwnerInfo{Team: "team-pay"},
				Deployment: keleustesv1alpha1.ApplicationDeployment{
					Strategy: keleustesv1alpha1.ApplicationDeploymentGitOps,
					Manifest: keleustesv1alpha1.ApplicationManifest{
						Type:     keleustesv1alpha1.ApplicationManifestKustomize,
						Repo:     "github.com/example/state",
						BasePath: "apps/checkout",
					},
				},
				Topology: keleustesv1alpha1.ApplicationTopology{
					Environments: []string{"dev", "prod"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		accept(&app.Status.Conditions, app.Generation)
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())

		got, err := src.GetApplication(ctx, "checkout")
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Name).To(Equal("checkout"))
		Expect(got.Ulid).To(Equal("")) // no identity engine in the scaffold
		Expect(got.Owner).To(HaveValue(Equal("team-pay")))
		Expect(got.Project).To(HaveValue(Equal("payments")))
		Expect(got.Source).NotTo(BeNil())
		Expect(got.Source.Repo).To(HaveValue(Equal("github.com/example/state")))
		Expect(got.Source.Path).To(HaveValue(Equal("apps/checkout")))
		Expect(got.Status).To(Equal(openapi.StatusProgressing))

		// The same object surfaces through the list, with the snapshot stamp.
		page, err := src.ListApplications(ctx, readmodel.ApplicationFilter{Project: "payments"})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.AsOf).NotTo(BeZero())
		Expect(page.Items).To(ContainElement(HaveField("Name", "checkout")))

		// A non-matching project filter excludes it.
		other, err := src.ListApplications(ctx, readmodel.ApplicationFilter{Project: "logistics"})
		Expect(err).NotTo(HaveOccurred())
		Expect(other.Items).NotTo(ContainElement(HaveField("Name", "checkout")))
	})

	It("returns ErrNotFound for an unknown application", func() {
		_, err := src.GetApplication(ctx, "does-not-exist")
		Expect(err).To(MatchError(readmodel.ErrNotFound))
	})

	It("maps DeploymentTargets and derives a best-effort matrix", func() {
		tgt := &keleustesv1alpha1.DeploymentTarget{
			ObjectMeta: metav1.ObjectMeta{Name: "prod-us", Namespace: "default"},
			Spec: keleustesv1alpha1.DeploymentTargetSpec{
				Environment: "prod",
				Region:      "us-east-1",
				Cell:        "cell-a",
				Cluster:     keleustesv1alpha1.DeploymentTargetCluster{Name: "prod-cluster"},
			},
		}
		Expect(k8sClient.Create(ctx, tgt)).To(Succeed())
		accept(&tgt.Status.Conditions, tgt.Generation)
		Expect(k8sClient.Status().Update(ctx, tgt)).To(Succeed())

		targets, err := src.ListTargets(ctx)
		Expect(err).NotTo(HaveOccurred())
		var found *openapi.DeploymentTarget
		for i := range targets {
			if targets[i].Name == "prod-us" {
				found = &targets[i]
			}
		}
		Expect(found).NotTo(BeNil())
		Expect(found.Env).To(HaveValue(Equal("prod")))
		Expect(found.Region).To(HaveValue(Equal("us-east-1")))
		Expect(found.Cell).To(HaveValue(Equal("cell-a")))
		Expect(found.Cluster).To(HaveValue(Equal("prod-cluster")))
		Expect(found.Status).To(Equal(openapi.StatusProgressing))

		m, err := src.GetMatrix(ctx, "all")
		Expect(err).NotTo(HaveOccurred())
		Expect(m.AsOf).NotTo(BeZero())
		// Columns and rows are always non-nil (valid even when sparse).
		Expect(m.Columns).NotTo(BeNil())
		Expect(m.Rows).NotTo(BeNil())
		Expect(m.Columns).To(ContainElement(HaveField("Region", HaveValue(Equal("us-east-1")))))

		// Drift and health are valid-but-empty for an existing target.
		drift, err := src.GetTargetDrift(ctx, "prod-us")
		Expect(err).NotTo(HaveOccurred())
		Expect(drift.Entries).NotTo(BeNil())

		health, err := src.GetTargetHealth(ctx, "prod-us")
		Expect(err).NotTo(HaveOccurred())
		Expect(health).NotTo(BeNil())

		// Drift/health for a missing target is ErrNotFound.
		_, err = src.GetTargetDrift(ctx, "ghost")
		Expect(err).To(MatchError(readmodel.ErrNotFound))
	})

	It("returns an empty audit page (audit lives in JetStream, not CRDs)", func() {
		page, err := src.QueryAudit(ctx, readmodel.AuditQuery{})
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Items).NotTo(BeNil())
		Expect(page.Items).To(BeEmpty())
		Expect(page.NextCursor).To(Equal(""))
	})
})
