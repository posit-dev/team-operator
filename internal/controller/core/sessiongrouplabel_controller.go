// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// sessionGroupLabelsInjectedMarker is set on pods after group labels have been
	// injected to prevent reprocessing on subsequent reconciliations.
	sessionGroupLabelsInjectedMarker = "posit.co/session-group-labels-injected"

	// launcherInstanceIDLabel identifies Workbench session pods created by the launcher.
	launcherInstanceIDLabel = "launcher-instance-id"

	// containerUserGroupsArg is the container arg whose next value holds the
	// comma-separated list of user groups for a Workbench session.
	containerUserGroupsArg = "--container-user-groups"
)

// SessionGroupLabelConfig holds configuration for extracting Entra group names
// from Workbench session pod args and writing them as numbered pod labels.
type SessionGroupLabelConfig struct {
	// LabelKeyPrefix is the base of the numbered label keys.
	// Default "user-group-" produces "user-group-1", "user-group-2", etc.
	LabelKeyPrefix string

	// MatchPattern is the regex applied to each comma-separated entry in the
	// --container-user-groups value to decide whether to include it.
	// Default: "_entra_[^ ,]+"
	MatchPattern string

	// TrimPrefix is stripped from the start of each matched group name
	// before it becomes the label value. Matches the ltrimstr("_") behaviour
	// in the original script: "_entra_team" → "entra_team".
	// Default: "_"
	TrimPrefix string
}

// SessionGroupLabelReconciler watches Workbench session pods and writes one
// numbered label per Entra group found in the --container-user-groups arg,
// enabling per-group cost attribution in OpenCost/Infracost.
//
// Label format (mirrors the original pod-labeler script):
//
//	user-group-1: entra_research_team
//	user-group-2: entra_data_science
type SessionGroupLabelReconciler struct {
	client.Client
	Log    logr.Logger
	Config SessionGroupLabelConfig

	matchRegex *regexp.Regexp
}

// Reconcile handles pod events. For each unlabeled Workbench session pod it
// extracts group names from the --container-user-groups arg and patches one
// numbered label per group onto the pod.
func (r *SessionGroupLabelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := r.getLogger(ctx).WithValues(
		"controller", "SessionGroupLabel",
		"pod", req.NamespacedName,
	)

	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		l.Error(err, "failed to get pod")
		return ctrl.Result{}, err
	}

	// Skip if not a Workbench session pod
	if _, ok := pod.Labels[launcherInstanceIDLabel]; !ok {
		return ctrl.Result{}, nil
	}

	// Skip if group labels were already injected
	if _, ok := pod.Labels[sessionGroupLabelsInjectedMarker]; ok {
		return ctrl.Result{}, nil
	}

	// Skip pods that are terminating
	if pod.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	// Extract one numbered label per group from --container-user-groups arg
	groupLabels := r.extractGroupLabels(&pod)
	if len(groupLabels) == 0 {
		l.V(1).Info("no matching groups found in --container-user-groups arg")
		return ctrl.Result{}, nil
	}

	// Add marker so we don't reprocess this pod
	groupLabels[sessionGroupLabelsInjectedMarker] = "true"

	// Patch the pod with the new labels
	patch := client.MergeFrom(pod.DeepCopy())
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	for k, v := range groupLabels {
		pod.Labels[k] = v
	}

	if err := r.Patch(ctx, &pod, patch); err != nil {
		if apierrors.IsNotFound(err) {
			// Pod deleted between Get and Patch — nothing to do
			return ctrl.Result{}, nil
		}
		l.Error(err, "failed to patch group labels onto pod")
		return ctrl.Result{}, err
	}

	l.Info("injected session group labels", "labels", groupLabels)
	return ctrl.Result{}, nil
}

// extractGroupLabels reads the value of the --container-user-groups arg from
// containers[0], splits it by comma, filters entries with the configured regex,
// and returns a map of numbered label keys to group name values.
//
// Example output for "--container-user-groups _entra_research_team,_entra_data_science":
//
//	"user-group-1" → "entra_research_team"
//	"user-group-2" → "entra_data_science"
//
// Only containers[0] is checked because Workbench session pods always carry
// the launcher args there. Group order matches the original comma-separated
// list, so label numbering is stable across reconciliations.
func (r *SessionGroupLabelReconciler) extractGroupLabels(pod *corev1.Pod) map[string]string {
	labels := make(map[string]string)

	if len(pod.Spec.Containers) == 0 {
		return labels
	}

	args := pod.Spec.Containers[0].Args
	for i, arg := range args {
		if arg != containerUserGroupsArg || i+1 >= len(args) {
			continue
		}

		n := 1
		for _, entry := range strings.Split(args[i+1], ",") {
			entry = strings.TrimSpace(entry)
			if !r.matchRegex.MatchString(entry) {
				continue
			}
			// Replace = with - to match the original script's gsub("="; "-")
			entry = strings.ReplaceAll(entry, "=", "-")
			// Strip TrimPrefix to match ltrimstr("_"): "_entra_team" → "entra_team"
			entry = strings.TrimPrefix(entry, r.Config.TrimPrefix)
			entry = sanitizeGroupLabelValue(entry)
			if entry != "" {
				labels[fmt.Sprintf("%s%d", r.Config.LabelKeyPrefix, n)] = entry
				n++
			}
		}
		break
	}

	return labels
}

// sanitizeGroupLabelValue cleans a group name for use as a Kubernetes label
// value. Label values must be 63 characters or less, start and end with an
// alphanumeric character, and contain only alphanumerics, dashes, underscores,
// and dots.
func sanitizeGroupLabelValue(s string) string {
	s = regexp.MustCompile(`[^a-zA-Z0-9._-]`).ReplaceAllString(s, "_")
	s = regexp.MustCompile(`_{2,}`).ReplaceAllString(s, "_")
	if len(s) > 63 {
		s = s[:63]
	}
	return strings.Trim(s, "_.-")
}

// SetupWithManager registers the controller with the manager and compiles the
// match regex. Only pods that already carry the launcher-instance-id label
// (i.e. Workbench session pods) are enqueued.
func (r *SessionGroupLabelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var err error
	r.matchRegex, err = regexp.Compile(r.Config.MatchPattern)
	if err != nil {
		return err
	}

	sessionPodPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return false
		}
		_, ok = pod.Labels[launcherInstanceIDLabel]
		return ok
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(sessionPodPredicate).
		Complete(r)
}

func (r *SessionGroupLabelReconciler) getLogger(ctx context.Context) logr.Logger {
	if v, err := logr.FromContext(ctx); err == nil {
		return v
	}
	return r.Log
}
