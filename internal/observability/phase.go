// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package observability

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/posit-dev/team-operator/internal/status"
)

// PhaseFromConditions returns the current Phase value derived from the most
// recent Ready condition's Reason. It is intended to be called early in a
// Reconcile loop — before the controller sets Ready=Reconciling — so the
// returned value reflects the prior stable state.
//
// Returns PhaseUnknown if no Ready condition is present (first reconcile, or
// CR was just created) or if the Reason is not recognized.
func PhaseFromConditions(conds []metav1.Condition) string {
	for i := range conds {
		if conds[i].Type == status.TypeReady {
			return phaseFromReason(conds[i].Reason)
		}
	}
	return PhaseUnknown
}

func phaseFromReason(reason string) string {
	switch reason {
	case status.ReasonReconciling:
		return PhaseReconciling
	case status.ReasonReconcileError:
		return PhaseError
	case status.ReasonReconcileComplete, status.ReasonDeploymentReady, status.ReasonStatefulSetReady:
		return PhaseReady
	case status.ReasonAllComponentsReady:
		return PhaseComponentsReady
	case status.ReasonComponentsNotReady:
		return PhaseProgressing
	case status.ReasonSuspended:
		return PhaseSuspended
	case status.ReasonDatabaseReady:
		return PhaseDatabaseReady
	case status.ReasonDeploymentNotReady, status.ReasonStatefulSetNotReady:
		return PhaseUnknown
	default:
		return PhaseUnknown
	}
}
