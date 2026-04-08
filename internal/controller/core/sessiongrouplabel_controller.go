// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	v1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// sessionGroupLabelsInjectedMarker is set on pods after the controller has
	// processed them, preventing reprocessing on subsequent reconciliations.
	sessionGroupLabelsInjectedMarker = "posit.co/session-group-labels-injected"

	// launcherInstanceIDLabel identifies Workbench session pods created by the launcher.
	launcherInstanceIDLabel = "launcher-instance-id"

	// defaultSourceField is the dot-path used when SessionLabelsConfig.SourceField is empty.
	defaultSourceField = "spec.containers[0].args"

	// defaultSourceKey is used when SessionLabelsConfig.SourceKey is empty.
	defaultSourceKey = "--container-user-groups"

	// defaultSearchRegex is used when SessionLabelsConfig.SearchRegex is empty.
	defaultSearchRegex = `_entra_[^ ,]+`

	// defaultLabelKeyPrefix is used when SessionLabelsConfig.LabelKeyPrefix is empty.
	defaultLabelKeyPrefix = "user-group-"

	// defaultTrimPrefix is used when SessionLabelsConfig.TrimPrefix is empty.
	defaultTrimPrefix = "_"
)

var (
	// sanitizeInvalidChars replaces characters that are not valid in Kubernetes
	// label values with underscores.
	sanitizeInvalidChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	// sanitizeMultiUnderscore collapses consecutive underscores to a single one.
	sanitizeMultiUnderscore = regexp.MustCompile(`_{2,}`)
)

// SessionGroupLabelReconciler watches Workbench session pods and writes one
// numbered label per entry that matches the configured regex in the configured
// pod field.
//
// Configuration is read per-site from the Workbench CR's SessionLabels field.
// If the Workbench CR has no SessionLabels, the pod is marked as processed and
// skipped. Default label format (when all defaults apply):
//
//	user-group-1: entra_research_team
//	user-group-2: entra_data_science
type SessionGroupLabelReconciler struct {
	client.Client
	Log logr.Logger
}

// Reconcile handles pod events. For each unprocessed Workbench session pod it
// looks up the site's Workbench CR to read the SessionLabels config, then
// extracts group names from the configured field and patches numbered labels.
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

	// Skip if not a Workbench session pod (missing launcher label)
	if _, ok := pod.Labels[launcherInstanceIDLabel]; !ok {
		return ctrl.Result{}, nil
	}

	// Skip if already processed (whether or not groups were found)
	if _, ok := pod.Labels[sessionGroupLabelsInjectedMarker]; ok {
		return ctrl.Result{}, nil
	}

	// Skip pods that are terminating
	if pod.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	// Derive the site name from the pod label — maps to the Workbench CR name.
	siteName := pod.Labels[v1beta1.SiteLabelKey]
	if siteName == "" {
		l.V(1).Info("pod has no posit.team/site label, skipping")
		return ctrl.Result{}, nil
	}

	// Look up the Workbench CR for this site to read the SessionLabels config.
	var workbench v1beta1.Workbench
	if err := r.Get(ctx, types.NamespacedName{Name: siteName, Namespace: pod.Namespace}, &workbench); err != nil {
		if apierrors.IsNotFound(err) {
			// Workbench may not be reconciled yet; leave pod unmarked so we retry.
			l.V(1).Info("workbench CR not found, will retry", "site", siteName)
			return ctrl.Result{}, nil
		}
		l.Error(err, "failed to get workbench CR", "site", siteName)
		return ctrl.Result{}, err
	}

	// If the Workbench has no sessionLabels config, mark and skip — feature is
	// not enabled for this site.
	if workbench.Spec.SessionLabels == nil {
		return r.markProcessed(ctx, &pod)
	}

	cfg := workbench.Spec.SessionLabels

	// Extract one numbered label per matching entry from the configured field.
	groupLabels, err := r.extractGroupLabels(&pod, cfg)
	if err != nil {
		l.Error(err, "failed to extract labels",
			"sourceField", cfg.SourceField, "site", siteName)
		return ctrl.Result{}, err
	}

	if len(groupLabels) == 0 {
		l.V(1).Info("no matching entries found, marking pod to skip future reconciles", "site", siteName)
	}

	// Always set the processed marker so we don't re-reconcile this pod.
	groupLabels[sessionGroupLabelsInjectedMarker] = "true"

	// Patch the pod with the extracted labels + marker.
	patch := client.MergeFrom(pod.DeepCopy())
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	for k, v := range groupLabels {
		pod.Labels[k] = v
	}

	if err := r.Patch(ctx, &pod, patch); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		l.Error(err, "failed to patch labels onto pod")
		return ctrl.Result{}, err
	}

	l.Info("processed session group labels", "labels", groupLabels, "site", siteName)
	return ctrl.Result{}, nil
}

// markProcessed patches only the processed marker onto the pod without adding
// any labels. Used when the feature is disabled for the site.
func (r *SessionGroupLabelReconciler) markProcessed(ctx context.Context, pod *corev1.Pod) (ctrl.Result, error) {
	patch := client.MergeFrom(pod.DeepCopy())
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels[sessionGroupLabelsInjectedMarker] = "true"
	if err := r.Patch(ctx, pod, patch); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// extractGroupLabels resolves the configured field path in the pod, reads the
// raw comma-separated value string, and returns a map of numbered label keys
// to sanitized values for each entry matching the configured regex.
func (r *SessionGroupLabelReconciler) extractGroupLabels(pod *corev1.Pod, cfg *v1beta1.SessionLabelsConfig) (map[string]string, error) {
	labels := make(map[string]string)

	raw, ok, err := r.extractSourceValue(pod, cfg)
	if err != nil {
		return nil, err
	}
	if !ok || raw == "" {
		return labels, nil
	}

	searchRegex := cfg.SearchRegex
	if searchRegex == "" {
		searchRegex = defaultSearchRegex
	}
	re, err := regexp.Compile(searchRegex)
	if err != nil {
		return nil, fmt.Errorf("invalid searchRegex %q: %w", searchRegex, err)
	}

	prefix := cfg.LabelKeyPrefix
	if prefix == "" {
		prefix = defaultLabelKeyPrefix
	}
	trimPrefix := cfg.TrimPrefix
	if trimPrefix == "" {
		trimPrefix = defaultTrimPrefix
	}

	n := 1
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if !re.MatchString(entry) {
			continue
		}
		// Replace = with - to match the original script's gsub("="; "-")
		entry = strings.ReplaceAll(entry, "=", "-")
		// Strip configured prefix: "_entra_team" → "entra_team"
		entry = strings.TrimPrefix(entry, trimPrefix)
		entry = sanitizeGroupLabelValue(entry)
		if entry != "" {
			labels[fmt.Sprintf("%s%d", prefix, n)] = entry
			n++
		}
	}

	return labels, nil
}

// extractSourceValue walks SourceField in the pod's JSON representation and
// returns the raw comma-separated group string. See walkJSONPath for path syntax.
//
// Leaf-type behaviour:
//   - string: returned directly; SourceKey is ignored.
//   - []string: SourceKey is the flag name; the next element is the value.
//   - map[string]string: SourceKey is the map key.
func (r *SessionGroupLabelReconciler) extractSourceValue(pod *corev1.Pod, cfg *v1beta1.SessionLabelsConfig) (string, bool, error) {
	field := cfg.SourceField
	if field == "" {
		field = defaultSourceField
	}
	key := cfg.SourceKey
	if key == "" {
		key = defaultSourceKey
	}

	data, err := json.Marshal(pod)
	if err != nil {
		return "", false, fmt.Errorf("marshaling pod: %w", err)
	}
	var root interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return "", false, fmt.Errorf("unmarshaling pod JSON: %w", err)
	}

	leaf, err := walkJSONPath(root, field)
	if err != nil {
		// Field absent or invalid path — treat as "not found", not a hard error.
		return "", false, nil
	}

	switch v := leaf.(type) {
	case string:
		return v, true, nil
	case []interface{}:
		for i, elem := range v {
			if s, ok := elem.(string); ok && s == key && i+1 < len(v) {
				if next, ok := v[i+1].(string); ok {
					return next, true, nil
				}
			}
		}
		return "", false, nil
	case map[string]interface{}:
		if val, ok := v[key]; ok {
			if s, ok := val.(string); ok {
				return s, true, nil
			}
		}
		return "", false, nil
	}
	return "", false, nil
}

// walkJSONPath navigates a decoded JSON value using a dot-path that supports
// array index notation. Examples:
//
//	"spec.containers[0].args"   navigates object→array[0]→object
//	"metadata.annotations"      navigates object→object
func walkJSONPath(v interface{}, path string) (interface{}, error) {
	if path == "" {
		return v, nil
	}

	seg, rest, _ := strings.Cut(path, ".")

	if open := strings.Index(seg, "["); open >= 0 {
		close := strings.Index(seg, "]")
		if close < open {
			return nil, fmt.Errorf("invalid path segment %q", seg)
		}
		arrayKey := seg[:open]
		idx, err := strconv.Atoi(seg[open+1 : close])
		if err != nil {
			return nil, fmt.Errorf("non-integer array index in %q", seg)
		}
		m, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected object at %q, got %T", arrayKey, v)
		}
		arr, ok := m[arrayKey].([]interface{})
		if !ok {
			return nil, fmt.Errorf("expected array at %q", arrayKey)
		}
		if idx < 0 || idx >= len(arr) {
			return nil, fmt.Errorf("index %d out of range for %q (len %d)", idx, arrayKey, len(arr))
		}
		return walkJSONPath(arr[idx], rest)
	}

	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected object at %q, got %T", seg, v)
	}
	child, ok := m[seg]
	if !ok {
		return nil, fmt.Errorf("field %q not found", seg)
	}
	return walkJSONPath(child, rest)
}

// sanitizeGroupLabelValue cleans a string for use as a Kubernetes label
// value. Label values must be 63 characters or less, start and end with an
// alphanumeric character, and contain only alphanumerics, dashes, underscores,
// and dots.
func sanitizeGroupLabelValue(s string) string {
	s = sanitizeInvalidChars.ReplaceAllString(s, "_")
	s = sanitizeMultiUnderscore.ReplaceAllString(s, "_")
	if len(s) > 63 {
		s = s[:63]
	}
	return strings.Trim(s, "_.-")
}

// SetupWithManager registers the controller with the manager. Only pods that
// carry the launcher-instance-id label (i.e. Workbench session pods) are enqueued.
func (r *SessionGroupLabelReconciler) SetupWithManager(mgr ctrl.Manager) error {
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
