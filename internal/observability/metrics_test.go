// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package observability_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/posit-dev/team-operator/internal/observability"
)

func TestRecordStatusTransition(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	m := mp.Meter("test")

	observability.RecordStatusTransition(context.Background(), m,
		"site", "posit-team", observability.PhaseReconciling, observability.PhaseReady)
	observability.RecordStatusTransition(context.Background(), m,
		"site", "posit-team", observability.PhaseReconciling, observability.PhaseReady)
	observability.RecordStatusTransition(context.Background(), m,
		"connect", "posit-team", observability.PhaseReconciling, observability.PhaseError)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	var found bool
	for _, sm := range rm.ScopeMetrics {
		for _, mm := range sm.Metrics {
			if mm.Name == observability.MetricStatusTransitionTotal {
				found = true
				sum, ok := mm.Data.(metricdata.Sum[int64])
				require.True(t, ok, "expected Sum[int64] data type")
				assert.Len(t, sum.DataPoints, 2, "expected 2 distinct label sets")
				for _, dp := range sum.DataPoints {
					controller, _ := dp.Attributes.Value(attribute.Key(observability.LabelController))
					fromPhase, _ := dp.Attributes.Value(attribute.Key(observability.LabelFromPhase))
					toPhase, _ := dp.Attributes.Value(attribute.Key(observability.LabelToPhase))
					if controller.AsString() == "site" {
						assert.Equal(t, int64(2), dp.Value, "site->ready transition count")
						assert.Equal(t, observability.PhaseReconciling, fromPhase.AsString())
						assert.Equal(t, observability.PhaseReady, toPhase.AsString())
					}
					if controller.AsString() == "connect" {
						assert.Equal(t, int64(1), dp.Value, "connect->error transition count")
					}
				}
			}
		}
	}
	assert.True(t, found, "metric %s not found in output", observability.MetricStatusTransitionTotal)
}

func TestRecordDependencyCheck(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	m := mp.Meter("test")

	observability.RecordDependencyCheck(context.Background(), m,
		"connect", "posit-team", observability.DependencyPostgres, observability.ResultSuccess)
	observability.RecordDependencyCheck(context.Background(), m,
		"connect", "posit-team", observability.DependencySecret, observability.ResultError)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	var found bool
	for _, sm := range rm.ScopeMetrics {
		for _, mm := range sm.Metrics {
			if mm.Name == observability.MetricDependencyCheckTotal {
				found = true
				sum, ok := mm.Data.(metricdata.Sum[int64])
				require.True(t, ok)
				assert.Len(t, sum.DataPoints, 2)
			}
		}
	}
	assert.True(t, found)
}

func TestRecordReconcileRequeue(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	m := mp.Meter("test")

	observability.RecordReconcileRequeue(context.Background(), m,
		"workbench", "posit-team", observability.RequeueReasonDepsNotReady)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	var found bool
	for _, sm := range rm.ScopeMetrics {
		for _, mm := range sm.Metrics {
			if mm.Name == observability.MetricReconcileRequeueTotal {
				found = true
				sum, ok := mm.Data.(metricdata.Sum[int64])
				require.True(t, ok)
				require.Len(t, sum.DataPoints, 1)
				assert.Equal(t, int64(1), sum.DataPoints[0].Value)
			}
		}
	}
	assert.True(t, found)
}
