// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package observability

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Instruments holds pre-created OTel counters for a single controller.
// Construct once at SetupWithManager time and reuse for the lifetime of the reconciler.
// A zero-value Instruments is a safe no-op (all Record* calls are silently dropped).
type Instruments struct {
	StatusTransition metric.Int64Counter
	DependencyCheck  metric.Int64Counter
	ReconcileRequeue metric.Int64Counter
}

// NewInstruments creates a complete set of counters from the given Meter.
// Passing a nil meter returns a zero-value Instruments — all Record* methods become no-ops.
func NewInstruments(m metric.Meter) Instruments {
	if m == nil {
		return Instruments{}
	}
	status, _ := m.Int64Counter(MetricStatusTransitionTotal,
		metric.WithDescription("Number of status phase transitions, partitioned by controller, namespace, from_phase, and to_phase."))
	dep, _ := m.Int64Counter(MetricDependencyCheckTotal,
		metric.WithDescription("Number of dependency checks, partitioned by controller, namespace, dependency type, and result."))
	requeue, _ := m.Int64Counter(MetricReconcileRequeueTotal,
		metric.WithDescription("Number of reconcile requeues, partitioned by controller, namespace, and reason."))
	return Instruments{StatusTransition: status, DependencyCheck: dep, ReconcileRequeue: requeue}
}

// RecordStatusTransition increments team_operator_status_transition_total.
// controller is the controller name (e.g. "site", "connect").
// fromPhase and toPhase should be Phase* constants from names.go.
// Calls where fromPhase == toPhase are no-ops: the metric tracks transitions,
// not steady-state reconciles. Use controller_runtime_reconcile_total for
// "how often did this controller reconcile in state X."
func (i Instruments) RecordStatusTransition(ctx context.Context, controller, namespace, fromPhase, toPhase string) {
	if i.StatusTransition == nil || fromPhase == toPhase {
		return
	}
	i.StatusTransition.Add(ctx, 1,
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
func (i Instruments) RecordDependencyCheck(ctx context.Context, controller, namespace, dependency, result string) {
	if i.DependencyCheck == nil {
		return
	}
	i.DependencyCheck.Add(ctx, 1,
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
func (i Instruments) RecordReconcileRequeue(ctx context.Context, controller, namespace, reason string) {
	if i.ReconcileRequeue == nil {
		return
	}
	i.ReconcileRequeue.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String(LabelController, controller),
			attribute.String(LabelNamespace, namespace),
			attribute.String(LabelReason, reason),
		),
	)
}
