// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package status

import (
	"strings"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Condition type constants
const (
	TypeReady       = "Ready"
	TypeProgressing = "Progressing"
)

// Reason constants
const (
	ReasonReconciling         = "Reconciling"
	ReasonReconcileComplete   = "ReconcileComplete"
	ReasonReconcileError      = "ReconcileError"
	ReasonDeploymentReady     = "DeploymentReady"
	ReasonDeploymentNotReady  = "DeploymentNotReady"
	ReasonStatefulSetReady    = "StatefulSetReady"
	ReasonStatefulSetNotReady = "StatefulSetNotReady"
	ReasonAllComponentsReady  = "AllComponentsReady"
	ReasonComponentsNotReady  = "ComponentsNotReady"
	ReasonDatabaseReady       = "DatabaseReady"
)

// SetReady sets the Ready condition on the given conditions slice.
func SetReady(conditions *[]metav1.Condition, generation int64, status metav1.ConditionStatus, reason, message string) {
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               TypeReady,
		Status:             status,
		ObservedGeneration: generation,
		Reason:             reason,
		Message:            message,
	})
}

// SetProgressing sets the Progressing condition on the given conditions slice.
func SetProgressing(conditions *[]metav1.Condition, generation int64, status metav1.ConditionStatus, reason, message string) {
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               TypeProgressing,
		Status:             status,
		ObservedGeneration: generation,
		Reason:             reason,
		Message:            message,
	})
}

// IsReady returns true if the Ready condition is True.
func IsReady(conditions []metav1.Condition) bool {
	return apimeta.IsStatusConditionTrue(conditions, TypeReady)
}

// ExtractVersion extracts a version string from a container image reference.
// For example, "ghcr.io/rstudio/rstudio-connect:2024.06.0" returns "2024.06.0".
// Also handles digest references: "image:2024.06.0@sha256:abc" returns "2024.06.0".
// Returns empty string if no tag is found.
func ExtractVersion(image string) string {
	// Strip digest suffix if present (image:tag@sha256:...)
	if idx := strings.LastIndex(image, "@"); idx != -1 {
		image = image[:idx]
	}
	// Isolate the last path segment to avoid matching registry port colons
	lastSlash := strings.LastIndex(image, "/")
	nameTag := image
	if lastSlash != -1 {
		nameTag = image[lastSlash+1:]
	}
	if idx := strings.LastIndex(nameTag, ":"); idx != -1 {
		tag := nameTag[idx+1:]
		// Skip "latest" as it's not a useful version
		if tag == "latest" {
			return ""
		}
		return tag
	}
	return ""
}
