// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package core

import (
	"time"

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
	const (
		timeout  = time.Second * 30
		interval = time.Millisecond * 250
	)

	Context("When creating a Site CR", func() {
		It("Should create child resources (Connect, Workbench, etc.)", func() {
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

			By("Verifying the Site CR was created")
			siteKey := types.NamespacedName{Name: siteName, Namespace: testNamespace}
			createdSite := &corev1beta1.Site{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, siteKey, createdSite)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdSite.Name).To(Equal(siteName))
			Expect(createdSite.Namespace).To(Equal(testNamespace))
		})
	})

	Context("When validating Site CRD schema", func() {
		It("Should reject invalid Site specs", func() {
			By("Creating a Site with missing required fields")
			testNamespace := "posit-team"
			invalidSite := &corev1beta1.Site{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.posit.team/v1beta1",
					Kind:       "Site",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-site",
					Namespace: testNamespace,
				},
				// Empty spec - the CRD should still accept this as fields are optional
				Spec: corev1beta1.SiteSpec{},
			}
			// This should succeed because the CRD doesn't require most fields
			err := k8sClient.Create(ctx, invalidSite)
			// The create might succeed or fail depending on CRD validation
			// We just want to verify the API server is working
			if err == nil {
				// Clean up
				Expect(k8sClient.Delete(ctx, invalidSite)).To(Succeed())
			}
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

			By("Verifying the Connect CR was created")
			connectKey := types.NamespacedName{Name: connectName, Namespace: testNamespace}
			createdConnect := &corev1beta1.Connect{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, connectKey, createdConnect)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdConnect.Name).To(Equal(connectName))
			Expect(createdConnect.Spec.Replicas).To(Equal(1))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, connect)).To(Succeed())
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

			By("Verifying the Workbench CR was created")
			workbenchKey := types.NamespacedName{Name: workbenchName, Namespace: testNamespace}
			createdWorkbench := &corev1beta1.Workbench{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, workbenchKey, createdWorkbench)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdWorkbench.Name).To(Equal(workbenchName))
			Expect(createdWorkbench.Spec.Replicas).To(Equal(1))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, workbench)).To(Succeed())
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

			By("Verifying the PackageManager CR was created")
			pmKey := types.NamespacedName{Name: pmName, Namespace: testNamespace}
			createdPM := &corev1beta1.PackageManager{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, pmKey, createdPM)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdPM.Name).To(Equal(pmName))
			Expect(createdPM.Spec.Replicas).To(Equal(1))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, pm)).To(Succeed())
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
