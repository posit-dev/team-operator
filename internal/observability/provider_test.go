// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package observability_test

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/posit-dev/team-operator/internal/observability"
)

func TestNewProvider_NoopWhenDisabled(t *testing.T) {
	t.Setenv("OTEL_SDK_DISABLED", "true")
	p := observability.NewProvider(context.Background(), observability.Config{
		MetricsEnabled:    true,
		PrometheusEnabled: true,
	})
	require.NotNil(t, p)

	// Meter should work without panicking (noop meter)
	m := p.Meter("test")
	counter, err := m.Int64Counter("test_counter")
	require.NoError(t, err)
	counter.Add(context.Background(), 1) // noop, should not panic

	require.NoError(t, p.Shutdown(context.Background()))
}

func TestNewProvider_MetricsDisabled(t *testing.T) {
	p := observability.NewProvider(context.Background(), observability.Config{
		MetricsEnabled: false,
	})
	require.NotNil(t, p)
	require.NoError(t, p.Shutdown(context.Background()))
}

func TestNewProvider_PrometheusOnly(t *testing.T) {
	// Use a fresh registry so the test is idempotent across `go test -count=N`
	// runs and does not pollute prometheus.DefaultRegisterer.
	reg := prometheus.NewRegistry()
	p := observability.NewProvider(context.Background(), observability.Config{
		MetricsEnabled:       true,
		PrometheusEnabled:    true,
		PrometheusRegisterer: reg,
	})
	require.NotNil(t, p)

	m := p.Meter("team-operator/site")
	counter, err := m.Int64Counter("test_init_counter")
	require.NoError(t, err)
	counter.Add(context.Background(), 5)

	require.NoError(t, p.Shutdown(context.Background()))
}

func TestNewProvider_PrometheusGather(t *testing.T) {
	// Verify the contract that the OTel Prometheus exporter feeds the configured
	// Registerer / Gatherer — i.e. recorded counters appear in /metrics output.
	reg := prometheus.NewRegistry()
	p := observability.NewProvider(context.Background(), observability.Config{
		MetricsEnabled:       true,
		PrometheusEnabled:    true,
		PrometheusRegisterer: reg,
	})
	require.NotNil(t, p)
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	m := p.Meter("team-operator/test")
	counter, err := m.Int64Counter("provider_gather_test_total")
	require.NoError(t, err)
	counter.Add(context.Background(), 3)

	families, err := reg.Gather()
	require.NoError(t, err)

	var found bool
	for _, mf := range families {
		if mf.GetName() == "provider_gather_test_total" {
			found = true
			break
		}
	}
	require.True(t, found, "OTel counter must appear in Prometheus gather output")
}

// TestNewProvider_NilRegistererDefaultsToGlobal pins the production wiring:
// when PrometheusRegisterer is nil (as main.go calls it), the exporter must
// register onto prometheus.DefaultRegisterer so controller-runtime's metrics
// server serves both controller_runtime_* built-ins and team_operator_* metrics
// from the same /metrics endpoint. Regression test for a bug found during
// real-cluster validation where promexporter.New() without WithRegisterer
// silently created its own internal registry that no HTTP handler served.
//
// Note: this test mutates global prometheus.DefaultRegisterer state.
// `go test -count > 1` will fail with a duplicate-collector registration error.
func TestNewProvider_NilRegistererDefaultsToGlobal(t *testing.T) {
	p := observability.NewProvider(context.Background(), observability.Config{
		MetricsEnabled:    true,
		PrometheusEnabled: true,
		// PrometheusRegisterer intentionally nil — this is how main.go calls it.
	})
	require.NotNil(t, p)
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	counter, err := p.Meter("team-operator/regression").Int64Counter("default_registerer_regression_total")
	require.NoError(t, err)
	counter.Add(context.Background(), 1)

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, mf := range families {
		if mf.GetName() == "default_registerer_regression_total" {
			return
		}
	}
	t.Fatalf("metric default_registerer_regression_total not found in prometheus.DefaultGatherer; nil registerer did not default to DefaultRegisterer")
}

func TestNewProvider_OTLPEndpointSet(t *testing.T) {
	// Smoke test: provider init with an OTLP endpoint set must succeed; gRPC
	// connect is lazy so an unreachable collector does not fail at init time.
	// Shutdown may return an error when the collector is unreachable (the SDK
	// flushes pending exports), which is fine — callers tolerate the error.
	p := observability.NewProvider(context.Background(), observability.Config{
		MetricsEnabled:    true,
		PrometheusEnabled: false,
		OTLPEndpoint:      "localhost:4317",
		OTLPInsecure:      true,
	})
	require.NotNil(t, p)
	_ = p.Shutdown(context.Background())
}

func TestNewProvider_EnvVarFallback(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	p := observability.NewProvider(context.Background(), observability.Config{
		MetricsEnabled:    true,
		PrometheusEnabled: false,
		OTLPEndpoint:      "", // empty — should fall back to env var
		OTLPInsecure:      true,
	})
	require.NotNil(t, p)
	_ = p.Shutdown(context.Background())
}
