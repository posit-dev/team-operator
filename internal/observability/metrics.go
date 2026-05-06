// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package observability

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

// Instruments are initialized lazily per Meter instance and cached by Meter identity
// to avoid re-creating instruments on every call. The OTel SDK is idempotent for
// same-name instruments from the same meter, but caching avoids the per-call
// allocation in the hot reconcile path.

var (
	statusTransitionMu   sync.Mutex
	statusTransitionInst = map[metric.Meter]metric.Int64Counter{}

	dependencyCheckMu   sync.Mutex
	dependencyCheckInst = map[metric.Meter]metric.Int64Counter{}

	reconcileRequeueMu   sync.Mutex
	reconcileRequeueInst = map[metric.Meter]metric.Int64Counter{}

	noopMeter = noop.NewMeterProvider().Meter("team-operator-noop")
)

// RecordStatusTransition increments team_operator_status_transition_total.
// controller is the controller name (e.g. "site", "connect").
// fromPhase and toPhase should be Phase* constants from names.go.
// A nil meter is a safe no-op. Calls where fromPhase == toPhase are also
// no-ops: the metric tracks transitions, not steady-state reconciles, and
// counting "same phase as before" pollutes flapping detection. Use
// controller_runtime_reconcile_total for "how often did this controller
// reconcile in state X."
func RecordStatusTransition(ctx context.Context, m metric.Meter, controller, namespace, fromPhase, toPhase string) {
	if m == nil || fromPhase == toPhase {
		return
	}
	counter := getOrCreateCounter(&statusTransitionMu, statusTransitionInst, m,
		MetricStatusTransitionTotal,
		"Number of status phase transitions, partitioned by controller, namespace, from_phase, and to_phase.")
	counter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String(LabelController, controller),
			attribute.String(LabelNamespace, namespace),
			attribute.String(LabelFromPhase, fromPhase),
			attribute.String(LabelToPhase, toPhase),
		),
	)
}

// RecordDependencyCheck increments team_operator_dependency_check_total.
// dependency should be a Dependency* constant. result should be a Result* constant.
// A nil meter is a safe no-op.
func RecordDependencyCheck(ctx context.Context, m metric.Meter, controller, namespace, dependency, result string) {
	if m == nil {
		return
	}
	counter := getOrCreateCounter(&dependencyCheckMu, dependencyCheckInst, m,
		MetricDependencyCheckTotal,
		"Number of dependency checks, partitioned by controller, namespace, dependency type, and result.")
	counter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String(LabelController, controller),
			attribute.String(LabelNamespace, namespace),
			attribute.String(LabelDependency, dependency),
			attribute.String(LabelResult, result),
		),
	)
}

// RecordReconcileRequeue increments team_operator_reconcile_requeue_total.
// reason should be a RequeueReason* constant from names.go.
// A nil meter is a safe no-op.
func RecordReconcileRequeue(ctx context.Context, m metric.Meter, controller, namespace, reason string) {
	if m == nil {
		return
	}
	counter := getOrCreateCounter(&reconcileRequeueMu, reconcileRequeueInst, m,
		MetricReconcileRequeueTotal,
		"Number of reconcile requeues, partitioned by controller, namespace, and reason.")
	counter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String(LabelController, controller),
			attribute.String(LabelNamespace, namespace),
			attribute.String(LabelReason, reason),
		),
	)
}

// getOrCreateCounter retrieves or creates an Int64Counter from the cache.
// Cache miss creates the instrument via the supplied Meter; if creation fails
// (e.g. duplicate conflicting registration), fall back to a noop counter so
// the recording call is a safe no-op rather than a panic.
func getOrCreateCounter(mu *sync.Mutex, cache map[metric.Meter]metric.Int64Counter, m metric.Meter, name, desc string) metric.Int64Counter {
	mu.Lock()
	defer mu.Unlock()
	if c, ok := cache[m]; ok {
		return c
	}
	c, err := m.Int64Counter(name, metric.WithDescription(desc))
	if err != nil {
		// Fallback to a noop counter so the recording call is a safe no-op.
		c, _ = noopMeter.Int64Counter(name)
	}
	cache[m] = c
	return c
}
