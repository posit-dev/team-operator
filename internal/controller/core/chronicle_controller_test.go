// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package core

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/localtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	// SetProgressing should not be applied when suspended
	updated := &positcov1beta1.Chronicle{}
	require.NoError(t, cli.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, updated))
	assert.Empty(t, updated.Status.Conditions,
		"no status conditions should be set when suspended")
	for _, cond := range updated.Status.Conditions {
		assert.NotEqual(t, "Progressing", cond.Type,
			"SetProgressing should not be applied when suspended")
	}
}
