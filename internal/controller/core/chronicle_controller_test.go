// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package core

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/localtest"
	"github.com/posit-dev/team-operator/internal/status"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestChronicleReconciler_Suspended verifies that when Chronicle has Suspended=true,
// ReconcileChronicle does not create a StatefulSet and does not apply SetProgressing.
func TestChronicleReconciler_Suspended(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "chronicle-suspended"

	fakeEnv := localtest.FakeTestEnv{}
	cli, scheme, log := fakeEnv.Start(loadSchemes)

	r := &ChronicleReconciler{
		Client: cli,
		Scheme: scheme,
		Log:    log,
	}

	ctx = logr.NewContext(ctx, log)
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: ns, Name: name},
	}

	suspended := true
	c := &positcov1beta1.Chronicle{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Chronicle",
			APIVersion: "core.posit.team/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec:       positcov1beta1.ChronicleSpec{Suspended: &suspended},
	}

	err := cli.Create(ctx, c)
	require.NoError(t, err)

	res, err := r.ReconcileChronicle(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	// No StatefulSet should be created when suspended
	sts := &appsv1.StatefulSet{}
	err = cli.Get(ctx, client.ObjectKey{Name: c.ComponentName(), Namespace: ns}, sts)
	assert.True(t, apierrors.IsNotFound(err), "expected not-found error, got: %v", err)

	// Status should reflect the suspended state
	updated := &positcov1beta1.Chronicle{}
	require.NoError(t, cli.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, updated))
	assert.False(t, updated.Status.Ready, "Ready bool should be false when suspended")
	readyCond := apimeta.FindStatusCondition(updated.Status.Conditions, status.TypeReady)
	require.NotNil(t, readyCond, "Ready condition should be set when suspended")
	assert.Equal(t, metav1.ConditionFalse, readyCond.Status)
	assert.Equal(t, status.ReasonSuspended, readyCond.Reason)
	progressCond := apimeta.FindStatusCondition(updated.Status.Conditions, status.TypeProgressing)
	require.NotNil(t, progressCond, "Progressing condition should be set when suspended")
	assert.Equal(t, metav1.ConditionFalse, progressCond.Status)
	assert.Equal(t, status.ReasonSuspended, progressCond.Reason)
}

// initChronicleReconciler creates a Chronicle reconciler backed by a fake client
// and registers the given Chronicle so its resources can be reconciled.
func initChronicleReconciler(t *testing.T, ctx context.Context, ns, name string, c *positcov1beta1.Chronicle) (context.Context, *ChronicleReconciler, ctrl.Request) {
	t.Helper()

	fakeEnv := localtest.FakeTestEnv{}
	cli, scheme, log := fakeEnv.Start(loadSchemes)

	r := &ChronicleReconciler{
		Client: cli,
		Scheme: scheme,
		Log:    log,
	}

	ctx = logr.NewContext(ctx, log)
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: ns, Name: name},
	}

	require.NoError(t, cli.Create(ctx, c))

	return ctx, r, req
}

func newChronicle(ns, name string) *positcov1beta1.Chronicle {
	return &positcov1beta1.Chronicle{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Chronicle",
			APIVersion: "core.posit.team/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec:       positcov1beta1.ChronicleSpec{Image: "ghcr.io/rstudio/chronicle:test"},
	}
}

func getChronicleContainerResources(t *testing.T, ctx context.Context, r *ChronicleReconciler, ns string, c *positcov1beta1.Chronicle) corev1.ResourceRequirements {
	t.Helper()

	sts := &appsv1.StatefulSet{}
	require.NoError(t, r.Get(ctx, client.ObjectKey{Name: c.ComponentName(), Namespace: ns}, sts))
	require.Len(t, sts.Spec.Template.Spec.Containers, 1)
	return sts.Spec.Template.Spec.Containers[0].Resources
}

// TestChronicleReconciler_DefaultResources verifies the chronicle-server container
// gets the default resource requests plus a memory limit when Spec.Resources is unset.
func TestChronicleReconciler_DefaultResources(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "chronicle-default-resources"

	c := newChronicle(ns, name)
	ctx, r, req := initChronicleReconciler(t, ctx, ns, name, c)

	_, err := r.ensureDeployedService(ctx, req, c)
	require.NoError(t, err)

	resources := getChronicleContainerResources(t, ctx, r, ns, c)

	assert.Equal(t, resource.MustParse("100m"), resources.Requests[corev1.ResourceCPU])
	assert.Equal(t, resource.MustParse("1Gi"), resources.Requests[corev1.ResourceMemory])
	// Chronicle deliberately sets a memory limit (unlike the main products) to
	// contain runaway aggregation memory as a cgroup OOMKill, and no CPU limit.
	assert.Equal(t, resource.MustParse("4Gi"), resources.Limits[corev1.ResourceMemory])
	_, hasCPULimit := resources.Limits[corev1.ResourceCPU]
	assert.False(t, hasCPULimit, "chronicle default resources should not set a CPU limit")
}

// TestChronicleReconciler_ResourcesOverride verifies an explicit Spec.Resources
// override replaces the defaults entirely. This is how a site burns down a large
// aggregation backlog with a higher bounded memory limit.
func TestChronicleReconciler_ResourcesOverride(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "chronicle-resources-override"

	override := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("8Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("56Gi"),
		},
	}

	c := newChronicle(ns, name)
	c.Spec.Resources = &override
	ctx, r, req := initChronicleReconciler(t, ctx, ns, name, c)

	_, err := r.ensureDeployedService(ctx, req, c)
	require.NoError(t, err)

	resources := getChronicleContainerResources(t, ctx, r, ns, c)

	assert.Equal(t, resource.MustParse("1"), resources.Requests[corev1.ResourceCPU])
	assert.Equal(t, resource.MustParse("8Gi"), resources.Requests[corev1.ResourceMemory])
	assert.Equal(t, resource.MustParse("56Gi"), resources.Limits[corev1.ResourceMemory])
	_, hasCPULimit := resources.Limits[corev1.ResourceCPU]
	assert.False(t, hasCPULimit, "override with no CPU limit should not introduce one")
}
