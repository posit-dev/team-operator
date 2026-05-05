// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package observability

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/posit-dev/team-operator/internal"
)

// Config holds all flags/env that control OTel SDK initialization.
// Flags take precedence over environment variables; defaults are applied last.
//
// Note on service.name precedence: Config sets service.name to "team-operator"
// after resource.WithFromEnv(), so the explicit attribute wins over the
// OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES env vars by design.
type Config struct {
	// MetricsEnabled is the master toggle. When false, a noop provider is returned.
	MetricsEnabled bool
	// PrometheusEnabled registers the OTel Prometheus exporter onto a Prometheus
	// Registerer. When PrometheusRegisterer is nil, prometheus.DefaultRegisterer is used.
	PrometheusEnabled bool
	// PrometheusRegisterer is the Prometheus registerer the exporter binds to.
	// When nil and PrometheusEnabled is true, prometheus.DefaultRegisterer is used.
	// Tests should pass a fresh prometheus.NewRegistry() to avoid polluting the
	// process-global default registerer.
	PrometheusRegisterer prometheus.Registerer
	// OTLPEndpoint is the gRPC endpoint for OTLP metric push (e.g. "otel-collector:4317").
	// Empty string means OTLP push is disabled unless OTEL_EXPORTER_OTLP_ENDPOINT is set.
	// The OTel SDK reads OTEL_EXPORTER_OTLP_ENDPOINT automatically when this is empty.
	OTLPEndpoint string
	// OTLPInsecure forces the gRPC exporter to plaintext. Default false (TLS is used).
	// Set true for in-cluster collectors reachable over the pod network without TLS.
	OTLPInsecure bool
	// MetricsExportInterval is the cadence for OTLP metric export and async gauge collection.
	MetricsExportInterval time.Duration
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

var providerLog = ctrl.Log.WithName("observability")

// NewProvider initialises the OTel metrics SDK based on cfg.
// If MetricsEnabled is false, OTEL_SDK_DISABLED=true, or SDK init fails,
// a noop provider is returned so the operator always boots.
func NewProvider(ctx context.Context, cfg Config) *Provider {
	// Kill switch: OTEL_SDK_DISABLED env var (standard OTel convention).
	if os.Getenv("OTEL_SDK_DISABLED") == "true" {
		return &Provider{mp: noop.NewMeterProvider()}
	}

	if !cfg.MetricsEnabled {
		return &Provider{mp: noop.NewMeterProvider()}
	}

	mp, err := buildMeterProvider(ctx, cfg)
	if err != nil {
		// Degraded mode: log warning and return noop so the operator still starts.
		providerLog.Error(err, "SDK init failed; falling back to noop metrics")
		return &Provider{mp: noop.NewMeterProvider()}
	}

	return &Provider{mp: mp}
}

// Meter returns a named metric.Meter. name should be the controller/component name,
// e.g. "team-operator/site" or "team-operator/connect".
func (p *Provider) Meter(name string) metric.Meter {
	return p.mp.Meter(name)
}

// Shutdown flushes pending exports and releases SDK resources.
// Call this from the signal handler, after mgr.Start() returns.
// Returns the SDK shutdown error so callers can choose to log or ignore it;
// the operator should still exit cleanly even when shutdown errors occur.
func (p *Provider) Shutdown(ctx context.Context) error {
	if sdk, ok := p.mp.(*sdkmetric.MeterProvider); ok {
		return sdk.Shutdown(ctx)
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

	// Prometheus exporter — registers onto a Prometheus Registerer so /metrics
	// serves both controller-runtime built-ins and OTel metrics from one endpoint.
	// Defaults to prometheus.DefaultRegisterer when cfg.PrometheusRegisterer is nil.
	if cfg.PrometheusEnabled {
		var promOpts []promexporter.Option
		if cfg.PrometheusRegisterer != nil {
			promOpts = append(promOpts, promexporter.WithRegisterer(cfg.PrometheusRegisterer))
		}
		promExp, err := promexporter.New(promOpts...)
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
		grpcOpts := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(otlpEndpoint),
		}
		if cfg.OTLPInsecure {
			providerLog.Info("OTLP push using insecure (plaintext) transport; ensure the collector is in-cluster or behind a service mesh", "endpoint", otlpEndpoint)
			grpcOpts = append(grpcOpts, otlpmetricgrpc.WithInsecure())
		}
		otlpExp, err := otlpmetricgrpc.New(ctx, grpcOpts...)
		if err != nil {
			return nil, fmt.Errorf("creating OTLP metric exporter: %w", err)
		}
		interval := cfg.MetricsExportInterval
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
	// Order matters: WithFromEnv runs first, then WithAttributes — so explicit
	// attrs (including service.name) take precedence over OTEL_SERVICE_NAME.
	return resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithAttributes(attrs...),
	)
}
