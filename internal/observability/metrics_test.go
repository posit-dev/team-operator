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

func attrsToMap(s attribute.Set) map[string]string {
	out := make(map[string]string, s.Len())
	for _, kv := range s.ToSlice() {
		out[string(kv.Key)] = kv.Value.Emit()
	}
	return out
}

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
			if mm.Name != observability.MetricStatusTransitionTotal {
				continue
			}
			found = true
			sum, ok := mm.Data.(metricdata.Sum[int64])
			require.True(t, ok, "expected Sum[int64] data type")
			require.Len(t, sum.DataPoints, 2, "expected 2 distinct label sets")
			for _, dp := range sum.DataPoints {
				attrs := attrsToMap(dp.Attributes)
				switch attrs[observability.LabelController] {
				case "site":
					assert.Equal(t, int64(2), dp.Value, "site->ready transition count")
					assert.Equal(t, map[string]string{
						observability.LabelController: "site",
						observability.LabelNamespace:  "posit-team",
						observability.LabelFromPhase:  observability.PhaseReconciling,
						observability.LabelToPhase:    observability.PhaseReady,
					}, attrs)
				case "connect":
					assert.Equal(t, int64(1), dp.Value, "connect->error transition count")
					assert.Equal(t, map[string]string{
						observability.LabelController: "connect",
						observability.LabelNamespace:  "posit-team",
						observability.LabelFromPhase:  observability.PhaseReconciling,
						observability.LabelToPhase:    observability.PhaseError,
					}, attrs)
				default:
					t.Fatalf("unexpected controller label %q in metric %q with attrs %v", attrs[observability.LabelController], mm.Name, attrs)
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
			if mm.Name != observability.MetricDependencyCheckTotal {
				continue
			}
			found = true
			sum, ok := mm.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			require.Len(t, sum.DataPoints, 2)
			for _, dp := range sum.DataPoints {
				attrs := attrsToMap(dp.Attributes)
				switch attrs[observability.LabelDependency] {
				case observability.DependencyPostgres:
					assert.Equal(t, map[string]string{
						observability.LabelController: "connect",
						observability.LabelNamespace:  "posit-team",
						observability.LabelDependency: observability.DependencyPostgres,
						observability.LabelResult:     observability.ResultSuccess,
					}, attrs)
				case observability.DependencySecret:
					assert.Equal(t, map[string]string{
						observability.LabelController: "connect",
						observability.LabelNamespace:  "posit-team",
						observability.LabelDependency: observability.DependencySecret,
						observability.LabelResult:     observability.ResultError,
					}, attrs)
				default:
					t.Fatalf("unexpected dependency label %q in metric %q with attrs %v", attrs[observability.LabelDependency], mm.Name, attrs)
				}
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
			if mm.Name != observability.MetricReconcileRequeueTotal {
				continue
			}
			found = true
			sum, ok := mm.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			require.Len(t, sum.DataPoints, 1)
			dp := sum.DataPoints[0]
			assert.Equal(t, int64(1), dp.Value)
			assert.Equal(t, map[string]string{
				observability.LabelController: "workbench",
				observability.LabelNamespace:  "posit-team",
				observability.LabelReason:     observability.RequeueReasonDepsNotReady,
			}, attrsToMap(dp.Attributes))
		}
	}
	assert.True(t, found)
}
