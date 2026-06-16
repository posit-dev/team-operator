// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package core

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/localtest"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal/status"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestPackageManagerReconciler_Suspended verifies that when PackageManager has Suspended=true,
// ReconcilePackageManager does not create a Deployment and does not apply SetProgressing.
func TestPackageManagerReconciler_Suspended(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "pm-suspended"

	fakeEnv := localtest.FakeTestEnv{}
	cli, scheme, log := fakeEnv.Start(loadSchemes)

	r := &PackageManagerReconciler{
		Client: cli,
		Scheme: scheme,
		Log:    log,
	}

	ctx = logr.NewContext(ctx, log)
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: ns, Name: name},
	}

	suspended := true
	pm := &positcov1beta1.PackageManager{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PackageManager",
			APIVersion: "core.posit.team/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec:       positcov1beta1.PackageManagerSpec{Suspended: &suspended},
	}

	err := cli.Create(ctx, pm)
	require.NoError(t, err)

	res, err := r.ReconcilePackageManager(ctx, req, pm)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	// No Deployment should be created when suspended
	dep := &appsv1.Deployment{}
	err = cli.Get(ctx, client.ObjectKey{Name: pm.ComponentName(), Namespace: ns}, dep)
	assert.True(t, apierrors.IsNotFound(err), "expected not-found error, got: %v", err)

	// Status should reflect the suspended state
	updated := &positcov1beta1.PackageManager{}
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

// TestPackageManagerReconciler_DeploymentHasProbes verifies that the rendered
// Package Manager Deployment includes both Readiness and Liveness HTTP probes
// pointing at /__ping__ on the named "http" port.
func TestPackageManagerReconciler_DeploymentHasProbes(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "pm-probes"

	fakeEnv := localtest.FakeTestEnv{}
	cli, scheme, log := fakeEnv.Start(loadSchemes)

	r := &PackageManagerReconciler{
		Client: cli,
		Scheme: scheme,
		Log:    log,
	}

	ctx = logr.NewContext(ctx, log)
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: ns, Name: name},
	}

	pm := &positcov1beta1.PackageManager{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PackageManager",
			APIVersion: "core.posit.team/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, UID: "pm-probes-uid"},
		Spec: positcov1beta1.PackageManagerSpec{
			Image: "ghcr.io/rstudio/rstudio-pm:test",
			Secret: positcov1beta1.SecretConfig{
				Type: product.SiteSecretKubernetes,
			},
			Config: &positcov1beta1.PackageManagerConfig{},
		},
	}

	require.NoError(t, cli.Create(ctx, pm))

	_, err := r.ensureDeployedService(ctx, req, pm)
	require.NoError(t, err)

	dep := &appsv1.Deployment{}
	err = cli.Get(ctx, client.ObjectKey{Name: pm.ComponentName(), Namespace: ns}, dep)
	require.NoError(t, err)

	container := dep.Spec.Template.Spec.Containers[0]

	httpPort := intstr.IntOrString{Type: intstr.String, StrVal: "http"}

	// Verify ReadinessProbe exists and targets /__ping__ on the http port
	require.NotNil(t, container.ReadinessProbe)
	require.NotNil(t, container.ReadinessProbe.HTTPGet)
	assert.Equal(t, "/__ping__", container.ReadinessProbe.HTTPGet.Path)
	assert.Equal(t, httpPort, container.ReadinessProbe.HTTPGet.Port)

	// Verify LivenessProbe exists and targets /__ping__ on the http port
	require.NotNil(t, container.LivenessProbe)
	require.NotNil(t, container.LivenessProbe.HTTPGet)
	assert.Equal(t, "/__ping__", container.LivenessProbe.HTTPGet.Path)
	assert.Equal(t, httpPort, container.LivenessProbe.HTTPGet.Port)
	assert.Equal(t, int32(10), container.LivenessProbe.InitialDelaySeconds)
}

// TestPackageManagerReconciler_EnvVars verifies that the envVars field, including a
// valueFrom.secretKeyRef entry, flows through to the rendered Package Manager
// container's Env.
func TestPackageManagerReconciler_EnvVars(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "pm-env-vars"

	fakeEnv := localtest.FakeTestEnv{}
	cli, scheme, log := fakeEnv.Start(loadSchemes)

	r := &PackageManagerReconciler{
		Client: cli,
		Scheme: scheme,
		Log:    log,
	}

	ctx = logr.NewContext(ctx, log)
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: ns, Name: name},
	}

	plainEnv := corev1.EnvVar{
		Name:  "PM_PLAIN",
		Value: "plain-value",
	}
	secretEnv := corev1.EnvVar{
		Name: "PM_FROM_SECRET",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
				Key:                  "api-key",
			},
		},
	}

	pm := &positcov1beta1.PackageManager{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PackageManager",
			APIVersion: "core.posit.team/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, UID: "pm-env-vars-uid"},
		Spec: positcov1beta1.PackageManagerSpec{
			Image: "ghcr.io/rstudio/rstudio-pm:test",
			Secret: positcov1beta1.SecretConfig{
				Type: product.SiteSecretKubernetes,
			},
			Config:  &positcov1beta1.PackageManagerConfig{},
			EnvVars: []corev1.EnvVar{plainEnv, secretEnv},
		},
	}

	require.NoError(t, cli.Create(ctx, pm))

	_, err := r.ensureDeployedService(ctx, req, pm)
	require.NoError(t, err)

	dep := &appsv1.Deployment{}
	err = cli.Get(ctx, client.ObjectKey{Name: pm.ComponentName(), Namespace: ns}, dep)
	require.NoError(t, err)

	container := dep.Spec.Template.Spec.Containers[0]
	assert.Contains(t, container.Env, plainEnv, "plain envVar should be rendered into the container Env")
	assert.Contains(t, container.Env, secretEnv, "secretKeyRef envVar should be rendered into the container Env")
}
