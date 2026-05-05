# Operator Observability

The team-operator emits OpenTelemetry metrics served via the standard `/metrics` endpoint
(Prometheus exporter) and optionally pushed via OTLP gRPC. This document covers Phase 1
(metrics) of the operator's observability rollout.

## Metrics Endpoint

`/metrics` serves two metric families on the same endpoint:

1. **controller-runtime built-ins** — always present, no configuration required:
   - `controller_runtime_reconcile_total{controller, result}`
   - `controller_runtime_reconcile_time_seconds{controller}` (histogram)
   - `controller_runtime_reconcile_errors_total{controller}`
   - `workqueue_*` metrics

2. **Domain-specific operator metrics** (`team_operator_*`) — described below.

## Domain Metrics

### `team_operator_resource_count` (Gauge)

Labels: `controller`, `namespace`, `phase`

How many CRs of a given type are in a given namespace and phase. Refreshed every
`--observability-metrics-export-interval` (default: 30s) by an async gauge callback.
Not on the reconcile hot path.

**Example PromQL:**
```promql
# Workbench CRs not yet ready in any namespace:
team_operator_resource_count{controller="workbench", phase!="ready"}

# Total CRs managed per controller:
sum by (controller) (team_operator_resource_count)
```

### `team_operator_status_transition_total` (Counter)

Labels: `controller`, `namespace`, `from_phase`, `to_phase`

Incremented each time a reconcile moves a CR between phases. Useful for detecting
flapping (repeated error→ready→error cycles) or stuck controllers.

The `from_phase` label reflects the CR's prior stable phase, derived from the existing
`Ready` condition's reason at the start of the reconcile. On a CR's first reconcile
(no prior conditions) `from_phase=unknown`. This lets dashboards distinguish
"fresh→ready" from "error→ready (recovery)".

**Example PromQL:**
```promql
# Rate of error transitions across all controllers:
rate(team_operator_status_transition_total{to_phase="error"}[5m])

# Check for Connect flapping between ready and error:
increase(team_operator_status_transition_total{controller="connect"}[1h])
```

### `team_operator_dependency_check_total` (Counter)

Labels: `controller`, `namespace`, `dependency`, `result`

Incremented each time a dependency check runs. `dependency` is one of:
`postgres`, `keycloak`, `secret`, `crd`. `result` is `success` or `error`.

**Example PromQL:**
```promql
# Postgres dependency check failure rate:
rate(team_operator_dependency_check_total{dependency="postgres", result="error"}[5m])
```

### `team_operator_reconcile_requeue_total` (Counter)

Labels: `controller`, `namespace`, `reason`

Distinguishes requeue reasons that controller-runtime collapses into "requeue".
`reason` is one of: `deps_not_ready`, `conflict`, `retry`, `rate_limit`.

**Example PromQL:**
```promql
# Requeues due to dependency wait:
rate(team_operator_reconcile_requeue_total{reason="deps_not_ready"}[5m])
```

## Configuration

### Flags

| Flag | Default | Purpose |
|------|---------|---------|
| `--observability-metrics-enabled` | `true` | Master toggle |
| `--observability-metrics-prometheus` | `true` | Prometheus exporter on `/metrics` |
| `--observability-metrics-otlp-endpoint` | `""` | OTLP gRPC push endpoint |
| `--observability-metrics-export-interval` | `30s` | OTLP export and gauge refresh cadence |
| `--observability-cluster-name` | `""` | `k8s.cluster.name` resource attribute |

### Environment Variables

Env vars are fallbacks for flags. Flag values take precedence.

| Variable | Purpose |
|----------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP endpoint fallback (all signals) |
| `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` | OTLP endpoint fallback (metrics only) |
| `OTEL_RESOURCE_ATTRIBUTES` | Free-form resource attributes (`key=value,key=value`) |
| `OTEL_SDK_DISABLED` | Kill switch — disables all OTel instrumentation |
| `POD_NAME` | Set to `metadata.name` via Kubernetes downward API for `service.instance.id` |

### Precedence

`flag value > OTEL_EXPORTER_OTLP_METRICS_ENDPOINT > OTEL_EXPORTER_OTLP_ENDPOINT > default`

## Enabling OTLP Push

Point at an OpenTelemetry Collector or Grafana Agent:

**Helm:**
```yaml
observability:
  metrics:
    otlpEndpoint: "otel-collector.monitoring.svc.cluster.local:4317"
```

**Kustomize** — apply the `config/observability/` overlay on top of `config/default/`.

Both Prometheus and OTLP push can be active simultaneously. Enabling OTLP push does not
disable the `/metrics` endpoint.

## Resource Attributes

Every metric carries these resource attributes:

| Attribute | Value | Source |
|-----------|-------|--------|
| `service.name` | `team-operator` | Hardcoded |
| `service.version` | Operator binary version | `internal.VersionString` |
| `service.instance.id` | Pod name | `$POD_NAME` env var |
| `k8s.cluster.name` | _(optional)_ | `--observability-cluster-name` flag |

## Cardinality

Worst case per metric: `controllers (7) × namespaces (~50) × enum values (≤10)` ≈ 3500 series.
This is comfortably within standard Prometheus limits. Per-CR-name labels are intentionally
excluded to prevent cardinality explosion at scale.
