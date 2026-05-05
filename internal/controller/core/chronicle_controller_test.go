// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package core

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/localtest"
	"github.com/posit-dev/team-operator/internal/observability"
	"github.com/posit-dev/team-operator/internal/status"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
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

// TestChronicleReconciler_Metrics verifies that a status transition metric is recorded
// when ReconcileChronicle processes a suspended Chronicle (PhaseSuspended path).
func TestChronicleReconciler_Metrics(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "chronicle-metrics"

	fakeEnv := localtest.FakeTestEnv{}
	cli, scheme, log := fakeEnv.Start(loadSchemes)

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer mp.Shutdown(context.Background())

	r := &ChronicleReconciler{
		Client: cli,
		Scheme: scheme,
		Log:    log,
		Meter:  mp.Meter("test"),
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

	// ReconcileChronicle with Suspended=true exercises the PhaseSuspended recording path.
	_, _ = r.ReconcileChronicle(ctx, req, c)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &rm))
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == observability.MetricStatusTransitionTotal {
				found = true
			}
		}
	}
	assert.True(t, found, "expected status transition to be recorded")
}
