// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package observability

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"

	"github.com/posit-dev/team-operator/internal"
)

// Config holds all flags/env that control OTel SDK initialization.
// Flags take precedence over environment variables; defaults are applied last.
type Config struct {
	// MetricsEnabled is the master toggle. When false, a noop provider is returned.
	MetricsEnabled bool
	// PrometheusEnabled registers the OTel Prometheus exporter onto prometheus.DefaultRegisterer.
	PrometheusEnabled bool
	// OTLPEndpoint is the gRPC endpoint for OTLP metric push (e.g. "otel-collector:4317").
	// Empty string means OTLP push is disabled unless OTEL_EXPORTER_OTLP_ENDPOINT is set.
	// The OTel SDK reads OTEL_EXPORTER_OTLP_ENDPOINT automatically when this is empty.
	OTLPEndpoint string
	// ResourceCountInterval is the cadence for the async resource-count gauge collection.
	ResourceCountInterval time.Duration
	// ClusterName is written to the k8s.cluster.name resource attribute when non-empty.
	ClusterName string
	// InstanceID is service.instance.id, typically $POD_NAME. Filled from env in main.go.
	InstanceID string
}

// Provider wraps the OTel MeterProvider and exposes a Meter factory and Shutdown.
// All fields are unexported; callers interact only via Meter() and Shutdown().
type Provider struct {
	mp metric.MeterProvider
}

// NewProvider initialises the OTel metrics SDK based on cfg.
// If MetricsEnabled is false, OTEL_SDK_DISABLED=true, or SDK init fails,
// a noop provider is returned with nil error so the operator always boots.
func NewProvider(ctx context.Context, cfg Config) (*Provider, error) {
	// Kill switch: OTEL_SDK_DISABLED env var (standard OTel convention).
	if os.Getenv("OTEL_SDK_DISABLED") == "true" {
		return &Provider{mp: noop.NewMeterProvider()}, nil
	}

	if !cfg.MetricsEnabled {
		return &Provider{mp: noop.NewMeterProvider()}, nil
	}

	mp, err := buildMeterProvider(ctx, cfg)
	if err != nil {
		// Degraded mode: log warning and return noop so the operator still starts.
		// Caller (main.go) should log this.
		fmt.Fprintf(os.Stderr, "observability: SDK init failed (%v); falling back to noop metrics\n", err)
		return &Provider{mp: noop.NewMeterProvider()}, nil
	}

	// Set as global so controller-runtime's default metrics still share the same provider
	// if needed in the future.
	otel.SetMeterProvider(mp)

	return &Provider{mp: mp}, nil
}

// Meter returns a named metric.Meter. name should be the controller/component name,
// e.g. "team-operator/site" or "team-operator/connect".
func (p *Provider) Meter(name string) metric.Meter {
	return p.mp.Meter(name)
}

// Shutdown flushes pending exports and releases SDK resources.
// Call this from the signal handler, after mgr.Start() returns.
// Export errors during shutdown (e.g. unreachable OTLP endpoint) are logged
// but not returned — the operator must be able to exit cleanly regardless.
func (p *Provider) Shutdown(ctx context.Context) error {
	if sdk, ok := p.mp.(*sdkmetric.MeterProvider); ok {
		if err := sdk.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "observability: SDK shutdown error (non-fatal): %v\n", err)
		}
	}
	// noop provider has no resources to release
	return nil
}

func buildMeterProvider(ctx context.Context, cfg Config) (*sdkmetric.MeterProvider, error) {
	res, err := buildResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("building OTel resource: %w", err)
	}

	var opts []sdkmetric.Option
	opts = append(opts, sdkmetric.WithResource(res))

	// Prometheus exporter — registers onto prometheus.DefaultRegisterer so /metrics
	// serves both controller-runtime built-ins and OTel metrics from one endpoint.
	if cfg.PrometheusEnabled {
		promExp, err := promexporter.New()
		if err != nil {
			return nil, fmt.Errorf("creating Prometheus exporter: %w", err)
		}
		opts = append(opts, sdkmetric.WithReader(promExp))
	}

	// OTLP gRPC exporter. The OTel SDK automatically reads OTEL_EXPORTER_OTLP_ENDPOINT
	// and OTEL_EXPORTER_OTLP_METRICS_ENDPOINT from the environment. If cfg.OTLPEndpoint
	// is set it takes precedence (passed via WithEndpoint option). If neither is set and
	// PrometheusEnabled is also false, the provider will have no readers — valid but useless.
	otlpEndpoint := cfg.OTLPEndpoint
	if otlpEndpoint == "" {
		otlpEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")
	}
	if otlpEndpoint == "" {
		otlpEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if otlpEndpoint != "" {
		otlpExp, err := otlpmetricgrpc.New(ctx,
			otlpmetricgrpc.WithEndpoint(otlpEndpoint),
			otlpmetricgrpc.WithInsecure(), // TLS is a follow-up; default off for simplicity
		)
		if err != nil {
			return nil, fmt.Errorf("creating OTLP metric exporter: %w", err)
		}
		interval := cfg.ResourceCountInterval
		if interval <= 0 {
			interval = 30 * time.Second
		}
		opts = append(opts, sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(otlpExp, sdkmetric.WithInterval(interval)),
		))
	}

	return sdkmetric.NewMeterProvider(opts...), nil
}

func buildResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName("team-operator"),
		semconv.ServiceVersion(internal.VersionString),
	}
	if cfg.InstanceID != "" {
		attrs = append(attrs, semconv.ServiceInstanceID(cfg.InstanceID))
	}
	if cfg.ClusterName != "" {
		attrs = append(attrs, attribute.String("k8s.cluster.name", cfg.ClusterName))
	}

	// Merge with OTEL_RESOURCE_ATTRIBUTES env var (OTel SDK handles this automatically
	// when we use resource.New with WithProcess or Detect, but we build manually here
	// so we apply env vars via resource.WithFromEnv()).
	return resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithAttributes(attrs...),
	)
}
