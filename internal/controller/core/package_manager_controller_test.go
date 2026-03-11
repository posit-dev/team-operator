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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
