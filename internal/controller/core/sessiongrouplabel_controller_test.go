// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package core

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
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
		// Case is preserved (unlike key sanitization) — matches actual deployed label values
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

func TestSessionGroupLabelReconciler_ExtractGroupLabels(t *testing.T) {
	// defaultConfig matches the defaults used in main.go and values.yaml
	defaultConfig := SessionGroupLabelConfig{
		LabelKeyPrefix: "user-group-",
		MatchPattern:   `_entra_[^ ,]+`,
		TrimPrefix:     "_",
	}

	tests := []struct {
		name           string
		config         SessionGroupLabelConfig
		pod            *corev1.Pod
		expectedLabels map[string]string
	}{
		{
			// Matches the actual observed pod label in the cluster:
			// user-group-1: entra_posit_workbench_users-1032
			name:   "matches actual deployed label format",
			config: defaultConfig,
			pod: podWithArgs("session-abc", []string{
				"--container-user-groups",
				"_entra_posit_workbench_users-1032",
			}),
			expectedLabels: map[string]string{
				"user-group-1": "entra_posit_workbench_users-1032",
			},
		},
		{
			name:   "multiple groups get sequential numbered labels",
			config: defaultConfig,
			pod: podWithArgs("session-abc", []string{
				"--container-user-groups",
				"_entra_research_team,_entra_data_science,other_group",
			}),
			expectedLabels: map[string]string{
				"user-group-1": "entra_research_team",
				"user-group-2": "entra_data_science",
				// "other_group" does not match _entra_ pattern and is skipped
			},
		},
		{
			name:   "equals sign replaced with dash before stripping prefix",
			config: defaultConfig,
			pod: podWithArgs("session-abc", []string{
				"--container-user-groups",
				"_entra_team=alpha",
			}),
			expectedLabels: map[string]string{
				"user-group-1": "entra_team-alpha",
			},
		},
		{
			name:   "non-entra groups are skipped, numbering is contiguous",
			config: defaultConfig,
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
			// and group_uuid entries (non-Entra groups added by the container runtime).
			// Only _entra_ entries should produce labels; uuid groups are filtered out.
			// Numbering must be contiguous despite the skipped entries.
			name:   "mixed entra and non-entra groups — only entra groups labelled",
			config: defaultConfig,
			pod: podWithArgs("session-abc", []string{
				"--container-user-groups",
				"_entra_posit_workbench_users=1032,group_651c3700-ffe5-4f50-931c-f3-1004,_entra_data_science,group_7deca400-765c-4c50-85f9-44-1005",
			}),
			expectedLabels: map[string]string{
				// = replaced with - then leading _ stripped
				"user-group-1": "entra_posit_workbench_users-1032",
				// uuid group skipped
				"user-group-2": "entra_data_science",
				// second uuid group skipped, numbering stays contiguous
			},
		},
		{
			name:   "no matching groups produces empty map",
			config: defaultConfig,
			pod: podWithArgs("session-abc", []string{
				"--container-user-groups",
				"regular_group,another_group",
			}),
			expectedLabels: map[string]string{},
		},
		{
			name:   "missing --container-user-groups arg produces empty map",
			config: defaultConfig,
			pod: podWithArgs("session-abc", []string{
				"--some-other-arg", "value",
			}),
			expectedLabels: map[string]string{},
		},
		{
			name:   "--container-user-groups as last arg with no value is ignored",
			config: defaultConfig,
			pod: podWithArgs("session-abc", []string{
				"--container-user-groups",
			}),
			expectedLabels: map[string]string{},
		},
		{
			name:   "only reads containers[0] — groups in sidecar are ignored",
			config: defaultConfig,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "session-abc",
					Namespace: "posit-team",
					Labels:    map[string]string{launcherInstanceIDLabel: "abc"},
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
			name:   "no containers produces empty map",
			config: defaultConfig,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "session-abc",
					Namespace: "posit-team",
					Labels:    map[string]string{launcherInstanceIDLabel: "abc"},
				},
				Spec: corev1.PodSpec{},
			},
			expectedLabels: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &SessionGroupLabelReconciler{
				Config:     tt.config,
				matchRegex: regexp.MustCompile(tt.config.MatchPattern),
			}
			assert.Equal(t, tt.expectedLabels, r.extractGroupLabels(tt.pod))
		})
	}
}

// podWithArgs is a test helper that builds a minimal Workbench session pod
// with a single container using the given args.
func podWithArgs(name string, args []string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "posit-team",
			Labels:    map[string]string{launcherInstanceIDLabel: "abc123"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "session", Args: args},
			},
		},
	}
}
