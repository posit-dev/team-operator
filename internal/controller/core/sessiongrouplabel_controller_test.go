// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package core

import (
	"testing"

	"github.com/go-logr/logr"
	v1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSanitizeGroupLabelValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean group name unchanged",
			input:    "entra_research_team",
			expected: "entra_research_team",
		},
		{
			name:     "hyphen from equals replacement preserved",
			input:    "entra_team-alpha",
			expected: "entra_team-alpha",
		},
		{
			name:     "invalid characters replaced with underscore",
			input:    "entra_team@domain",
			expected: "entra_team_domain",
		},
		{
			name:     "multiple underscores collapsed",
			input:    "entra___team",
			expected: "entra_team",
		},
		{
			name:     "leading and trailing underscores stripped",
			input:    "_entra_team_",
			expected: "entra_team",
		},
		{
			name:     "truncated to 63 characters",
			input:    "entra_this_is_a_very_long_group_name_that_exceeds_the_kubernetes_label_value_limit",
			expected: "entra_this_is_a_very_long_group_name_that_exceeds_the_kubernete",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only underscores",
			input:    "___",
			expected: "",
		},
		{
			name:     "uppercase preserved",
			input:    "entra_Research_Team",
			expected: "entra_Research_Team",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, sanitizeGroupLabelValue(tt.input))
		})
	}
}

func TestWalkJSONPath(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		path     string
		expected interface{}
		wantErr  bool
	}{
		{
			name:     "empty path returns root",
			input:    map[string]interface{}{"a": "b"},
			path:     "",
			expected: map[string]interface{}{"a": "b"},
		},
		{
			name:     "single field",
			input:    map[string]interface{}{"spec": map[string]interface{}{"foo": "bar"}},
			path:     "spec",
			expected: map[string]interface{}{"foo": "bar"},
		},
		{
			name: "nested field",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{"k": "v"},
				},
			},
			path:     "metadata.annotations",
			expected: map[string]interface{}{"k": "v"},
		},
		{
			name: "array index",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "first"},
						map[string]interface{}{"name": "second"},
					},
				},
			},
			path:     "spec.containers[1]",
			expected: map[string]interface{}{"name": "second"},
		},
		{
			name: "array index then field",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"args": []interface{}{"--flag", "value"},
						},
					},
				},
			},
			path:     "spec.containers[0].args",
			expected: []interface{}{"--flag", "value"},
		},
		{
			name:    "missing field returns error",
			input:   map[string]interface{}{"a": "b"},
			path:    "missing",
			wantErr: true,
		},
		{
			name: "index out of range returns error",
			input: map[string]interface{}{
				"items": []interface{}{"only"},
			},
			path:    "items[5]",
			wantErr: true,
		},
		{
			name:    "non-object at segment returns error",
			input:   map[string]interface{}{"a": "string-not-object"},
			path:    "a.b",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := walkJSONPath(tt.input, tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}

func TestSessionGroupLabelReconciler_ExtractGroupLabels(t *testing.T) {
	// defaultCfg matches real defaults — empty strings fall through to package defaults.
	defaultCfg := &v1beta1.SessionLabelsConfig{}

	tests := []struct {
		name           string
		cfg            *v1beta1.SessionLabelsConfig
		pod            *corev1.Pod
		expectedLabels map[string]string
	}{
		{
			// Matches the actual observed pod label in the cluster:
			// user-group-1: entra_posit_workbench_users-1032
			name: "default: matches actual deployed label format",
			cfg:  defaultCfg,
			pod: podWithArgs("session-abc", []string{
				"--container-user-groups",
				"_entra_posit_workbench_users-1032",
			}),
			expectedLabels: map[string]string{
				"user-group-1": "entra_posit_workbench_users-1032",
			},
		},
		{
			name: "default: multiple groups get sequential numbered labels",
			cfg:  defaultCfg,
			pod: podWithArgs("session-abc", []string{
				"--container-user-groups",
				"_entra_research_team,_entra_data_science,other_group",
			}),
			expectedLabels: map[string]string{
				"user-group-1": "entra_research_team",
				"user-group-2": "entra_data_science",
			},
		},
		{
			name: "default: equals sign replaced with dash before stripping prefix",
			cfg:  defaultCfg,
			pod: podWithArgs("session-abc", []string{
				"--container-user-groups",
				"_entra_team=alpha",
			}),
			expectedLabels: map[string]string{
				"user-group-1": "entra_team-alpha",
			},
		},
		{
			name: "default: non-entra groups skipped, numbering contiguous",
			cfg:  defaultCfg,
			pod: podWithArgs("session-abc", []string{
				"--container-user-groups",
				"regular_group,_entra_research_team,another_regular,_entra_data_science",
			}),
			expectedLabels: map[string]string{
				"user-group-1": "entra_research_team",
				"user-group-2": "entra_data_science",
			},
		},
		{
			// Mirrors what is actually seen in the cluster: a mix of _entra_ groups
			// and group_uuid entries. Only _entra_ entries produce labels.
			name: "default: mixed entra and non-entra groups",
			cfg:  defaultCfg,
			pod: podWithArgs("session-abc", []string{
				"--container-user-groups",
				"_entra_posit_workbench_users=1032,group_651c3700-ffe5-4f50-931c-f3-1004,_entra_data_science,group_7deca400-765c-4c50-85f9-44-1005",
			}),
			expectedLabels: map[string]string{
				"user-group-1": "entra_posit_workbench_users-1032",
				"user-group-2": "entra_data_science",
			},
		},
		{
			name:           "default: no matching groups produces empty map",
			cfg:            defaultCfg,
			pod:            podWithArgs("session-abc", []string{"--container-user-groups", "regular_group"}),
			expectedLabels: map[string]string{},
		},
		{
			name:           "default: missing flag produces empty map",
			cfg:            defaultCfg,
			pod:            podWithArgs("session-abc", []string{"--some-other-arg", "value"}),
			expectedLabels: map[string]string{},
		},
		{
			name:           "default: flag with no value is ignored",
			cfg:            defaultCfg,
			pod:            podWithArgs("session-abc", []string{"--container-user-groups"}),
			expectedLabels: map[string]string{},
		},
		{
			name: "default: only reads containers[0] — sidecar ignored",
			cfg:  defaultCfg,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "session-abc",
					Namespace: "posit-team",
					Labels: map[string]string{
						v1beta1.LauncherInstanceIDKey: "abc",
						v1beta1.SiteLabelKey:          "qa",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "session", Args: []string{"--container-user-groups", "regular_group"}},
						{Name: "sidecar", Args: []string{"--container-user-groups", "_entra_research_team"}},
					},
				},
			},
			expectedLabels: map[string]string{},
		},
		{
			name: "default: no containers produces empty map",
			cfg:  defaultCfg,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "session-abc",
					Namespace: "posit-team",
					Labels: map[string]string{
						v1beta1.LauncherInstanceIDKey: "abc",
						v1beta1.SiteLabelKey:          "qa",
					},
				},
				Spec: corev1.PodSpec{},
			},
			expectedLabels: map[string]string{},
		},
		{
			// Custom sourceField and sourceKey — reads from a non-default flag.
			name: "custom args path and flag name",
			cfg: &v1beta1.SessionLabelsConfig{
				LabelKeyPrefix: "team-",
				SourceField:    "spec.containers[0].args",
				SourceKey:      "--user-groups",
				SearchRegex:    `_entra_[^ ,]+`,
				TrimPrefix:     "_",
			},
			pod: podWithArgs("session-abc", []string{
				"--container-user-groups", "_entra_wrong_arg",
				"--user-groups", "_entra_correct_team",
			}),
			expectedLabels: map[string]string{
				"team-1": "entra_correct_team",
			},
		},
		{
			// Read groups from an annotation using metadata.annotations path.
			name: "annotation source via metadata.annotations",
			cfg: &v1beta1.SessionLabelsConfig{
				SourceField: "metadata.annotations",
				SourceKey:   "posit.co/user-groups",
				SearchRegex: `_entra_[^ ,]+`,
				TrimPrefix:  "_",
			},
			pod: podWithAnnotations("session-abc", map[string]string{
				"posit.co/user-groups": "_entra_research_team,_entra_data_science",
			}),
			expectedLabels: map[string]string{
				"user-group-1": "entra_research_team",
				"user-group-2": "entra_data_science",
			},
		},
		{
			// Read groups from pod labels using metadata.labels path.
			name: "label source via metadata.labels",
			cfg: &v1beta1.SessionLabelsConfig{
				SourceField: "metadata.labels",
				SourceKey:   "posit.co/group",
				SearchRegex: `_entra_[^ ,]+`,
				TrimPrefix:  "_",
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "session-abc",
					Namespace: "posit-team",
					Labels: map[string]string{
						v1beta1.LauncherInstanceIDKey: "abc123",
						v1beta1.SiteLabelKey:          "qa",
						"posit.co/group":              "_entra_platform_team",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "session"}},
				},
			},
			expectedLabels: map[string]string{
				"user-group-1": "entra_platform_team",
			},
		},
		{
			name: "annotation source: missing key produces empty map",
			cfg: &v1beta1.SessionLabelsConfig{
				SourceField: "metadata.annotations",
				SourceKey:   "posit.co/user-groups",
				SearchRegex: `_entra_[^ ,]+`,
				TrimPrefix:  "_",
			},
			pod:            podWithAnnotations("session-abc", map[string]string{"other-key": "value"}),
			expectedLabels: map[string]string{},
		},
		{
			name: "invalid SourceField path silently produces empty map",
			cfg: &v1beta1.SessionLabelsConfig{
				SourceField: "does.not.exist",
				SourceKey:   "--container-user-groups",
				SearchRegex: `_entra_[^ ,]+`,
			},
			pod:            podWithArgs("session-abc", []string{"--container-user-groups", "_entra_team"}),
			expectedLabels: map[string]string{},
		},
	}

	r := &SessionGroupLabelReconciler{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels, err := r.extractGroupLabels(logr.Discard(), tt.pod, tt.cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedLabels, labels)
		})
	}
}

// podWithArgs builds a minimal Workbench session pod with a single container.
func podWithArgs(name string, args []string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "posit-team",
			Labels: map[string]string{
				v1beta1.LauncherInstanceIDKey: "abc123",
				v1beta1.SiteLabelKey:          "qa",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "session", Args: args},
			},
		},
	}
}

// podWithAnnotations builds a minimal Workbench session pod with the given annotations.
func podWithAnnotations(name string, annotations map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "posit-team",
			Labels: map[string]string{
				v1beta1.LauncherInstanceIDKey: "abc123",
				v1beta1.SiteLabelKey:          "qa",
			},
			Annotations: annotations,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "session"}},
		},
	}
}
