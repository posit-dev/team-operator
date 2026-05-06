// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package observability_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/posit-dev/team-operator/internal/observability"
	"github.com/posit-dev/team-operator/internal/status"
)

func TestPhaseFromConditions(t *testing.T) {
	cases := []struct {
		name  string
		conds []metav1.Condition
		want  string
	}{
		{"empty conditions returns Unknown", nil, observability.PhaseUnknown},
		{"reconciling reason", []metav1.Condition{{Type: status.TypeReady, Reason: status.ReasonReconciling}}, observability.PhaseReconciling},
		{"reconcile error reason", []metav1.Condition{{Type: status.TypeReady, Reason: status.ReasonReconcileError}}, observability.PhaseError},
		{"reconcile complete reason", []metav1.Condition{{Type: status.TypeReady, Reason: status.ReasonReconcileComplete}}, observability.PhaseReady},
		{"deployment ready reason", []metav1.Condition{{Type: status.TypeReady, Reason: status.ReasonDeploymentReady}}, observability.PhaseReady},
		{"statefulset ready reason", []metav1.Condition{{Type: status.TypeReady, Reason: status.ReasonStatefulSetReady}}, observability.PhaseReady},
		{"all components ready reason", []metav1.Condition{{Type: status.TypeReady, Reason: status.ReasonAllComponentsReady}}, observability.PhaseComponentsReady},
		{"suspended reason", []metav1.Condition{{Type: status.TypeReady, Reason: status.ReasonSuspended}}, observability.PhaseSuspended},
		{"database ready reason", []metav1.Condition{{Type: status.TypeReady, Reason: status.ReasonDatabaseReady}}, observability.PhaseDatabaseReady},
		{"deployment not ready reason", []metav1.Condition{{Type: status.TypeReady, Reason: status.ReasonDeploymentNotReady}}, observability.PhaseUnknown},
		{"statefulset not ready reason", []metav1.Condition{{Type: status.TypeReady, Reason: status.ReasonStatefulSetNotReady}}, observability.PhaseUnknown},
		{"components not ready reason", []metav1.Condition{{Type: status.TypeReady, Reason: status.ReasonComponentsNotReady}}, observability.PhaseProgressing},
		{"unrecognized reason returns Unknown", []metav1.Condition{{Type: status.TypeReady, Reason: "SomethingElse"}}, observability.PhaseUnknown},
		{"non-Ready condition is ignored", []metav1.Condition{{Type: status.TypeProgressing, Reason: status.ReasonReconcileComplete}}, observability.PhaseUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, observability.PhaseFromConditions(tc.conds))
		})
	}
}
