// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package observability_test

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/posit-dev/team-operator/internal/observability"
)

func TestNewProvider_NoopWhenDisabled(t *testing.T) {
	t.Setenv("OTEL_SDK_DISABLED", "true")
	p := observability.NewProvider(context.Background(), observability.Config{})
	require.NotNil(t, p)

	// Meter should work without panicking (noop meter)
	m := p.Meter("test")
	counter, err := m.Int64Counter("test_counter")
	require.NoError(t, err)
	counter.Add(context.Background(), 1) // noop, should not panic

	require.NoError(t, p.Shutdown(context.Background()))
}

func TestNewProvider_PrometheusOnly(t *testing.T) {
	// Use a fresh registry so the test is idempotent across `go test -count=N`
	// runs and does not pollute prometheus.DefaultRegisterer.
	reg := prometheus.NewRegistry()
	p := observability.NewProvider(context.Background(), observability.Config{
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

// TestNewProvider_NilRegistererDefaultsToCRMetrics pins the production wiring:
// when PrometheusRegisterer is nil (as main.go calls it), the exporter must
// register onto sigs.k8s.io/controller-runtime/pkg/metrics.Registry — the
// registry that controller-runtime's metrics server actually serves /metrics
// from. NOT prometheus.DefaultRegisterer (the global default), which is a
// SEPARATE registry that controller-runtime ignores. Regression test for a
// production bug found during AKS reference cluster validation where
// team_operator_* metrics emitted into a registry no HTTP handler served.
//
// Note: this test mutates global crmetrics.Registry state.
// `go test -count > 1` will fail with a duplicate-collector registration error.
func TestNewProvider_NilRegistererDefaultsToCRMetrics(t *testing.T) {
	p := observability.NewProvider(context.Background(), observability.Config{
		// PrometheusRegisterer intentionally nil — this is how main.go calls it.
	})
	require.NotNil(t, p)
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	counter, err := p.Meter("team-operator/regression").Int64Counter("crmetrics_registry_regression_total")
	require.NoError(t, err)
	counter.Add(context.Background(), 1)

	gatherer, ok := crmetrics.Registry.(prometheus.Gatherer)
	require.True(t, ok, "controller-runtime metrics.Registry must implement prometheus.Gatherer")
	families, err := gatherer.Gather()
	require.NoError(t, err)
	for _, mf := range families {
		if mf.GetName() == "crmetrics_registry_regression_total" {
			return
		}
	}
	t.Fatalf("metric crmetrics_registry_regression_total not found in crmetrics.Registry; nil registerer did not default to controller-runtime's Registry")
}

func TestNewProvider_OTLPEndpointSet(t *testing.T) {
	// Smoke test: provider init with an OTLP endpoint set must succeed; gRPC
	// connect is lazy so an unreachable collector does not fail at init time.
	// Shutdown may return an error when the collector is unreachable (the SDK
	// flushes pending exports), which is fine — callers tolerate the error.
	reg := prometheus.NewRegistry()
	p := observability.NewProvider(context.Background(), observability.Config{
		PrometheusRegisterer: reg,
		OTLPEndpoint:         "localhost:4317",
		OTLPInsecure:         true,
	})
	require.NotNil(t, p)
	_ = p.Shutdown(context.Background())
}

func TestNewProvider_EnvVarFallback(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	reg := prometheus.NewRegistry()
	p := observability.NewProvider(context.Background(), observability.Config{
		PrometheusRegisterer: reg,
		OTLPEndpoint:         "", // empty — should fall back to env var
		OTLPInsecure:         true,
	})
	require.NotNil(t, p)
	_ = p.Shutdown(context.Background())
}
