// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

// Package observability provides OTel-based metrics instrumentation for the team-operator.
package observability

// Metric names — all under the team_operator_* namespace.
const (
	MetricResourceCount         = "team_operator_resource_count"
	MetricStatusTransitionTotal = "team_operator_status_transition_total"
	MetricDependencyCheckTotal  = "team_operator_dependency_check_total"
	MetricReconcileRequeueTotal = "team_operator_reconcile_requeue_total"
)

// Label keys.
const (
	LabelController = "controller"
	LabelNamespace  = "namespace"
	LabelPhase      = "phase"
	LabelFromPhase  = "from_phase"
	LabelToPhase    = "to_phase"
	LabelDependency = "dependency"
	LabelResult     = "result"
	LabelReason     = "reason"
)

// Dependency enum values for LabelDependency.
const (
	DependencyPostgres = "postgres"
	DependencyKeycloak = "keycloak"
	DependencySecret   = "secret"
	DependencyCRD      = "crd"
)

// Result enum values for LabelResult.
const (
	ResultSuccess = "success"
	ResultError   = "error"
)

// Requeue reason enum values for LabelReason.
// Keep this small and operator-defined — never pass free-form strings.
const (
	RequeueReasonDepsNotReady = "deps_not_ready"
	RequeueReasonConflict     = "conflict"
	RequeueReasonRetry        = "retry"
	RequeueReasonRateLimit    = "rate_limit"
)

// Phase values for LabelPhase / LabelFromPhase / LabelToPhase.
// Where applicable these are the lowercase_underscore form of the matching
// status.Reason* constants in internal/status/status.go. The mapping is
// asserted by TestPhaseMatchesStatusReason in names_test.go — adding or
// renaming a Reason in the status package will break that test.
const (
	PhaseReconciling     = "reconciling"          // status.ReasonReconciling
	PhaseReady           = "ready"                // generic ready phase (not tied to a single Reason)
	PhaseError           = "error"                // generic error phase (covers status.ReasonReconcileError)
	PhaseSuspended       = "suspended"            // status.ReasonSuspended
	PhaseDatabaseReady   = "database_ready"       // status.ReasonDatabaseReady
	PhaseComponentsReady = "all_components_ready" // status.ReasonAllComponentsReady
	PhaseProgressing     = "progressing"          // status.ReasonComponentsNotReady (waiting on children)
	PhaseUnknown         = "unknown"              // sentinel for an untracked previous phase
)
