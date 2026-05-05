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

// Force a build error if status.ReasonReconcileError is renamed/removed.
// PhaseError is documented as covering this Reason but the value transform is
// not 1:1, so it can't be asserted via camelToSnake below.
var _ = status.ReasonReconcileError

// TestPhaseMatchesStatusReason locks down phase strings that are expected to
// be the lowercase_underscore form of a status.Reason* constant. This catches
// the case where a Reason is renamed in the status package and dashboards
// silently break.
//
// Note: this test asserts two things at once — that phase strings track the
// matching Reason value, and that Reason values stay CamelCase. If a future
// change in internal/status switches Reason values to a different format
// (e.g., already-snake-cased or human-formatted strings) this test will fail
// even though the semantic mapping is unchanged; update camelToSnake or the
// expected phase strings accordingly.
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

// camelToSnake converts CamelCase to lowercase_underscore. It only handles
// one capital per word boundary (e.g., "DatabaseReady" -> "database_ready");
// consecutive capitals from acronyms like "HTTPReady" or "OIDCReady" are not
// supported and would produce incorrect output. None of the current
// status.Reason* values use acronyms; if one is added, this helper must be
// updated alongside the new test case.
func camelToSnake(s string) string {
	var b strings.Builder
	var prev rune
	for i, r := range s {
		isUpper := r >= 'A' && r <= 'Z'
		prevUpper := prev >= 'A' && prev <= 'Z'
		if i > 0 && isUpper && !prevUpper {
			b.WriteByte('_')
		}
		if isUpper {
			r += 'a' - 'A'
		}
		b.WriteRune(r)
		prev = rune(s[i])
	}
	return b.String()
}
