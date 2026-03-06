// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package status

import (
	"context"
	"fmt"
	"strings"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	ReasonSuspended           = "Suspended"
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

// IsSuspended returns true if the Ready condition exists with ReasonSuspended.
func IsSuspended(conditions []metav1.Condition) bool {
	c := apimeta.FindStatusCondition(conditions, TypeReady)
	return c != nil && c.Reason == ReasonSuspended
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

// SetDeploymentHealth sets Ready and Progressing conditions based on Deployment replica counts.
func SetDeploymentHealth(conditions *[]metav1.Condition, generation int64, readyReplicas, desiredReplicas int32) {
	if readyReplicas >= desiredReplicas {
		SetReady(conditions, generation, metav1.ConditionTrue, ReasonDeploymentReady, "Deployment has minimum availability")
		SetProgressing(conditions, generation, metav1.ConditionFalse, ReasonReconcileComplete, "Reconciliation complete")
	} else {
		SetReady(conditions, generation, metav1.ConditionFalse, ReasonDeploymentNotReady,
			fmt.Sprintf("Deployment has %d/%d ready replicas", readyReplicas, desiredReplicas))
		SetProgressing(conditions, generation, metav1.ConditionTrue, ReasonReconciling, "Deployment rollout in progress")
	}
}

// SetStatefulSetHealth sets Ready and Progressing conditions based on StatefulSet replica counts.
func SetStatefulSetHealth(conditions *[]metav1.Condition, generation int64, readyReplicas, desiredReplicas int32) {
	if readyReplicas >= desiredReplicas {
		SetReady(conditions, generation, metav1.ConditionTrue, ReasonStatefulSetReady, "StatefulSet has minimum availability")
		SetProgressing(conditions, generation, metav1.ConditionFalse, ReasonReconcileComplete, "Reconciliation complete")
	} else {
		SetReady(conditions, generation, metav1.ConditionFalse, ReasonStatefulSetNotReady,
			fmt.Sprintf("StatefulSet has %d/%d ready replicas", readyReplicas, desiredReplicas))
		SetProgressing(conditions, generation, metav1.ConditionTrue, ReasonReconciling, "StatefulSet rollout in progress")
	}
}

// PatchSuspendedStatus is a best-effort helper that sets ObservedGeneration, Ready
// and Progressing to False with ReasonSuspended, then patches the status subresource.
// It also sets the product-level ready bool to false and clears the version string
// via the provided pointers. If the status patch fails, the conditions will be set
// on the in-memory object but not persisted; the next reconcile will retry.
func PatchSuspendedStatus(ctx context.Context, statusWriter client.StatusWriter, obj client.Object, patchBase client.Patch, conditions *[]metav1.Condition, generation int64, observedGeneration *int64, ready *bool, version *string) error {
	*observedGeneration = generation
	SetReady(conditions, generation, metav1.ConditionFalse, ReasonSuspended, "Product is suspended")
	SetProgressing(conditions, generation, metav1.ConditionFalse, ReasonSuspended, "Product is suspended")
	*ready = false
	*version = ""
	return statusWriter.Patch(ctx, obj, patchBase)
}

// maxConditionMessageLength is the maximum length for condition messages to avoid
// leaking verbose internal details (connection strings, hostnames, etc.) in status.
const maxConditionMessageLength = 256

// PatchErrorStatus is a best-effort helper that sets Ready and Progressing to False
// with ReasonReconcileError, then patches the status subresource. The caller should
// log the returned error but still return the original reconcile error.
// If the status patch itself fails (e.g., due to a conflict), the conditions will be
// set on the in-memory object but not persisted; the next reconcile will retry.
func PatchErrorStatus(ctx context.Context, statusWriter client.StatusWriter, obj client.Object, patchBase client.Patch, conditions *[]metav1.Condition, generation int64, reconcileErr error) error {
	msg := truncateMessage(reconcileErr.Error())
	SetReady(conditions, generation, metav1.ConditionFalse, ReasonReconcileError, msg)
	SetProgressing(conditions, generation, metav1.ConditionFalse, ReasonReconcileError, msg)
	return statusWriter.Patch(ctx, obj, patchBase)
}

func truncateMessage(msg string) string {
	if len(msg) <= maxConditionMessageLength {
		return msg
	}
	return msg[:maxConditionMessageLength-3] + "..."
}
