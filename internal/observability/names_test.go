// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package observability_test

import (
	"strings"
	"testing"

	"github.com/posit-dev/team-operator/internal/observability"
	"github.com/posit-dev/team-operator/internal/status"
)

func TestMetricNamesHaveTeamOperatorPrefix(t *testing.T) {
	const prefix = "team_operator_"
	for _, name := range []string{
		observability.MetricResourceCount,
		observability.MetricStatusTransitionTotal,
		observability.MetricDependencyCheckTotal,
		observability.MetricReconcileRequeueTotal,
	} {
		if !strings.HasPrefix(name, prefix) {
			t.Errorf("metric %q missing %q prefix", name, prefix)
		}
	}
}

func TestLabelValueEnumsHaveNoDuplicates(t *testing.T) {
	groups := map[string][]string{
		"dependency": {
			observability.DependencyPostgres,
			observability.DependencyKeycloak,
			observability.DependencySecret,
			observability.DependencyCRD,
		},
		"result": {
			observability.ResultSuccess,
			observability.ResultError,
		},
		"requeue_reason": {
			observability.RequeueReasonDepsNotReady,
			observability.RequeueReasonConflict,
			observability.RequeueReasonRetry,
			observability.RequeueReasonRateLimit,
		},
		"phase": {
			observability.PhaseReconciling,
			observability.PhaseReady,
			observability.PhaseError,
			observability.PhaseSuspended,
			observability.PhaseDatabaseReady,
			observability.PhaseComponentsReady,
			observability.PhaseUnknown,
		},
	}
	for group, values := range groups {
		seen := make(map[string]struct{}, len(values))
		for _, v := range values {
			if _, dup := seen[v]; dup {
				t.Errorf("%s group has duplicate value %q", group, v)
			}
			seen[v] = struct{}{}
		}
	}
}

// TestPhaseMatchesStatusReason locks down phase strings that are expected to
// be the lowercase_underscore form of a status.Reason* constant. This catches
// the case where a Reason is renamed in the status package and dashboards
// silently break.
func TestPhaseMatchesStatusReason(t *testing.T) {
	cases := []struct {
		phase  string
		reason string
	}{
		{observability.PhaseReconciling, status.ReasonReconciling},
		{observability.PhaseSuspended, status.ReasonSuspended},
		{observability.PhaseDatabaseReady, status.ReasonDatabaseReady},
		{observability.PhaseComponentsReady, status.ReasonAllComponentsReady},
	}
	for _, c := range cases {
		if got := camelToSnake(c.reason); got != c.phase {
			t.Errorf("status.%s expected to map to phase %q, got %q", c.reason, c.phase, got)
		}
	}
}

func camelToSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}
