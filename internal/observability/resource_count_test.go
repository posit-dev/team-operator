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

type mockResourceLister struct {
	results []observability.ResourceCount
}

func (m *mockResourceLister) List(ctx context.Context) ([]observability.ResourceCount, error) {
	return m.results, nil
}

func TestRegisterResourceCountGauge(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer mp.Shutdown(context.Background())
	m := mp.Meter("test")

	lister := &mockResourceLister{
		results: []observability.ResourceCount{
			{Controller: "connect", Namespace: "posit-team", Phase: "ready", Count: 3},
			{Controller: "connect", Namespace: "posit-team", Phase: "error", Count: 1},
		},
	}

	err := observability.RegisterResourceCountGauge(m, lister)
	require.NoError(t, err)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	var found int
	for _, sm := range rm.ScopeMetrics {
		for _, mm := range sm.Metrics {
			if mm.Name == observability.MetricResourceCount {
				gauge, ok := mm.Data.(metricdata.Gauge[int64])
				require.True(t, ok)
				for _, dp := range gauge.DataPoints {
					found++
					controller, _ := dp.Attributes.Value(attribute.Key(observability.LabelController))
					phase, _ := dp.Attributes.Value(attribute.Key(observability.LabelPhase))
					if controller.AsString() == "connect" && phase.AsString() == "ready" {
						assert.Equal(t, int64(3), dp.Value)
					}
					if controller.AsString() == "connect" && phase.AsString() == "error" {
						assert.Equal(t, int64(1), dp.Value)
					}
				}
			}
		}
	}
	assert.Equal(t, 2, found, "expected 2 gauge data points")
}
