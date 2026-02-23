// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package core

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
)

var _ = Describe("Site Controller (envtest)", func() {

	Context("When creating a Site CR", func() {
		It("Should be able to create and retrieve a Site CR", func() {
			By("Creating a test namespace")
			testNamespace := "envtest-site-ns"
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: testNamespace,
				},
			}
			// Namespace might already exist from another test
			err := k8sClient.Create(ctx, ns)
			if err != nil && !isAlreadyExistsError(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			By("Creating a Site CR")
			siteName := "test-site-envtest"
			site := &corev1beta1.Site{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.posit.team/v1beta1",
					Kind:       "Site",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      siteName,
					Namespace: testNamespace,
				},
				Spec: corev1beta1.SiteSpec{
					WorkloadSecret: corev1beta1.SecretConfig{
						VaultName: "workload-vault",
						Type:      product.SiteSecretTest,
					},
					MainDatabaseCredentialSecret: corev1beta1.SecretConfig{
						VaultName: "test-vault",
						Type:      product.SiteSecretTest,
					},
					Flightdeck: corev1beta1.InternalFlightdeckSpec{
						Image: "test-image:latest",
					},
				},
			}
			Expect(k8sClient.Create(ctx, site)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, site)).To(Succeed())
			})

			By("Verifying the Site CR was created")
			siteKey := types.NamespacedName{Name: siteName, Namespace: testNamespace}
			createdSite := &corev1beta1.Site{}
			Expect(k8sClient.Get(ctx, siteKey, createdSite)).To(Succeed())

			Expect(createdSite.Name).To(Equal(siteName))
			Expect(createdSite.Namespace).To(Equal(testNamespace))
		})
	})

	Context("When testing Connect CRD", func() {
		It("Should be able to create a Connect resource directly", func() {
			testNamespace := "posit-team"
			connectName := "test-connect-envtest"

			connect := &corev1beta1.Connect{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.posit.team/v1beta1",
					Kind:       "Connect",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      connectName,
					Namespace: testNamespace,
				},
				Spec: corev1beta1.ConnectSpec{
					Debug:    false,
					Replicas: 1,
				},
			}
			Expect(k8sClient.Create(ctx, connect)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, connect)).To(Succeed())
			})

			By("Verifying the Connect CR was created")
			connectKey := types.NamespacedName{Name: connectName, Namespace: testNamespace}
			createdConnect := &corev1beta1.Connect{}
			Expect(k8sClient.Get(ctx, connectKey, createdConnect)).To(Succeed())

			Expect(createdConnect.Name).To(Equal(connectName))
			Expect(createdConnect.Spec.Replicas).To(Equal(1))
		})
	})

	Context("When testing Workbench CRD", func() {
		It("Should be able to create a Workbench resource directly", func() {
			testNamespace := "posit-team"
			workbenchName := "test-workbench-envtest"

			workbench := &corev1beta1.Workbench{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.posit.team/v1beta1",
					Kind:       "Workbench",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      workbenchName,
					Namespace: testNamespace,
				},
				Spec: corev1beta1.WorkbenchSpec{
					Replicas: 1,
				},
			}
			Expect(k8sClient.Create(ctx, workbench)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, workbench)).To(Succeed())
			})

			By("Verifying the Workbench CR was created")
			workbenchKey := types.NamespacedName{Name: workbenchName, Namespace: testNamespace}
			createdWorkbench := &corev1beta1.Workbench{}
			Expect(k8sClient.Get(ctx, workbenchKey, createdWorkbench)).To(Succeed())

			Expect(createdWorkbench.Name).To(Equal(workbenchName))
			Expect(createdWorkbench.Spec.Replicas).To(Equal(1))
		})
	})

	Context("When testing PackageManager CRD", func() {
		It("Should be able to create a PackageManager resource directly", func() {
			testNamespace := "posit-team"
			pmName := "test-pm-envtest"

			pm := &corev1beta1.PackageManager{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.posit.team/v1beta1",
					Kind:       "PackageManager",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      pmName,
					Namespace: testNamespace,
				},
				Spec: corev1beta1.PackageManagerSpec{
					Replicas: 1,
				},
			}
			Expect(k8sClient.Create(ctx, pm)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, pm)).To(Succeed())
			})

			By("Verifying the PackageManager CR was created")
			pmKey := types.NamespacedName{Name: pmName, Namespace: testNamespace}
			createdPM := &corev1beta1.PackageManager{}
			Expect(k8sClient.Get(ctx, pmKey, createdPM)).To(Succeed())

			Expect(createdPM.Name).To(Equal(pmName))
			Expect(createdPM.Spec.Replicas).To(Equal(1))
		})
	})
})

// Helper to check if error is "already exists"
func isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	return client.IgnoreAlreadyExists(err) == nil
}
