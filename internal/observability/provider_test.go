// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package observability_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/posit-dev/team-operator/internal/observability"
)

func TestNewProvider_NoopWhenDisabled(t *testing.T) {
	t.Setenv("OTEL_SDK_DISABLED", "true")
	p, err := observability.NewProvider(context.Background(), observability.Config{
		MetricsEnabled:    true,
		PrometheusEnabled: true,
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	// Meter should work without panicking (noop meter)
	m := p.Meter("test")
	counter, err := m.Int64Counter("test_counter")
	require.NoError(t, err)
	counter.Add(context.Background(), 1) // noop, should not panic

	require.NoError(t, p.Shutdown(context.Background()))
}

func TestNewProvider_MetricsDisabled(t *testing.T) {
	p, err := observability.NewProvider(context.Background(), observability.Config{
		MetricsEnabled: false,
	})
	require.NoError(t, err)
	require.NotNil(t, p)
	require.NoError(t, p.Shutdown(context.Background()))
}

func TestNewProvider_PrometheusOnly(t *testing.T) {
	p, err := observability.NewProvider(context.Background(), observability.Config{
		MetricsEnabled:    true,
		PrometheusEnabled: true,
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	m := p.Meter("team-operator/site")
	counter, err := m.Int64Counter("test_init_counter")
	require.NoError(t, err)
	counter.Add(context.Background(), 5)

	require.NoError(t, p.Shutdown(context.Background()))
}

func TestNewProvider_OTLPEndpointSet(t *testing.T) {
	// Unreachable endpoint — exporter should fail gracefully at export time,
	// not at init time. Provider init must succeed.
	p, err := observability.NewProvider(context.Background(), observability.Config{
		MetricsEnabled:    true,
		PrometheusEnabled: false,
		OTLPEndpoint:      "localhost:4317",
	})
	require.NoError(t, err)
	require.NotNil(t, p)
	require.NoError(t, p.Shutdown(context.Background()))
}

func TestNewProvider_EnvVarFallback(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	p, err := observability.NewProvider(context.Background(), observability.Config{
		MetricsEnabled:    true,
		PrometheusEnabled: false,
		OTLPEndpoint:      "", // empty — should fall back to env var
	})
	require.NoError(t, err)
	require.NotNil(t, p)
	require.NoError(t, p.Shutdown(context.Background()))
}

var _ = assert.New // suppress unused import warning
