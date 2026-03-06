// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package status

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExtractVersion(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "image with tag",
			image:    "ghcr.io/rstudio/rstudio-connect:2024.06.0",
			expected: "2024.06.0",
		},
		{
			name:     "image with latest tag returns empty",
			image:    "ghcr.io/rstudio/rstudio-connect:latest",
			expected: "",
		},
		{
			name:     "image with digest only",
			image:    "ghcr.io/rstudio/rstudio-connect@sha256:abc123",
			expected: "",
		},
		{
			name:     "image with tag and digest",
			image:    "ghcr.io/rstudio/rstudio-connect:2024.06.0@sha256:abc123",
			expected: "2024.06.0",
		},
		{
			name:     "registry with port and tag",
			image:    "localhost:5000/myimage:v1.0",
			expected: "v1.0",
		},
		{
			name:     "registry with port no tag",
			image:    "localhost:5000/myimage",
			expected: "",
		},
		{
			name:     "no tag",
			image:    "ghcr.io/rstudio/rstudio-connect",
			expected: "",
		},
		{
			name:     "empty string",
			image:    "",
			expected: "",
		},
		{
			name:     "complex registry with port and tag",
			image:    "registry.example.com:443/organization/repo:v2.3.4",
			expected: "v2.3.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractVersion(tt.image)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsReady(t *testing.T) {
	tests := []struct {
		name       string
		conditions []metav1.Condition
		expected   bool
	}{
		{
			name: "Ready condition is True",
			conditions: []metav1.Condition{
				{Type: TypeReady, Status: metav1.ConditionTrue},
			},
			expected: true,
		},
		{
			name: "Ready condition is False",
			conditions: []metav1.Condition{
				{Type: TypeReady, Status: metav1.ConditionFalse},
			},
			expected: false,
		},
		{
			name:       "Ready condition absent",
			conditions: []metav1.Condition{},
			expected:   false,
		},
		{
			name: "Multiple conditions, Ready is True",
			conditions: []metav1.Condition{
				{Type: TypeProgressing, Status: metav1.ConditionTrue},
				{Type: TypeReady, Status: metav1.ConditionTrue},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsReady(tt.conditions)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSetReady(t *testing.T) {
	t.Run("adds Ready condition when absent", func(t *testing.T) {
		conditions := []metav1.Condition{}
		SetReady(&conditions, 1, metav1.ConditionTrue, ReasonReconcileComplete, "All good")

		assert.Len(t, conditions, 1)
		assert.Equal(t, TypeReady, conditions[0].Type)
		assert.Equal(t, metav1.ConditionTrue, conditions[0].Status)
		assert.Equal(t, ReasonReconcileComplete, conditions[0].Reason)
		assert.Equal(t, "All good", conditions[0].Message)
		assert.Equal(t, int64(1), conditions[0].ObservedGeneration)
	})

	t.Run("updates Ready condition when present", func(t *testing.T) {
		conditions := []metav1.Condition{
			{Type: TypeReady, Status: metav1.ConditionFalse, Reason: "OldReason", Message: "Old message"},
		}
		SetReady(&conditions, 2, metav1.ConditionTrue, ReasonReconcileComplete, "Updated message")

		assert.Len(t, conditions, 1)
		assert.Equal(t, TypeReady, conditions[0].Type)
		assert.Equal(t, metav1.ConditionTrue, conditions[0].Status)
		assert.Equal(t, ReasonReconcileComplete, conditions[0].Reason)
		assert.Equal(t, "Updated message", conditions[0].Message)
		assert.Equal(t, int64(2), conditions[0].ObservedGeneration)
	})
}

func TestSetProgressing(t *testing.T) {
	t.Run("adds Progressing condition when absent", func(t *testing.T) {
		conditions := []metav1.Condition{}
		SetProgressing(&conditions, 1, metav1.ConditionTrue, ReasonReconciling, "In progress")

		assert.Len(t, conditions, 1)
		assert.Equal(t, TypeProgressing, conditions[0].Type)
		assert.Equal(t, metav1.ConditionTrue, conditions[0].Status)
		assert.Equal(t, ReasonReconciling, conditions[0].Reason)
		assert.Equal(t, "In progress", conditions[0].Message)
		assert.Equal(t, int64(1), conditions[0].ObservedGeneration)
	})

	t.Run("updates Progressing condition when present", func(t *testing.T) {
		conditions := []metav1.Condition{
			{Type: TypeProgressing, Status: metav1.ConditionTrue, Reason: ReasonReconciling, Message: "Old"},
		}
		SetProgressing(&conditions, 2, metav1.ConditionFalse, ReasonReconcileComplete, "Done")

		assert.Len(t, conditions, 1)
		assert.Equal(t, TypeProgressing, conditions[0].Type)
		assert.Equal(t, metav1.ConditionFalse, conditions[0].Status)
		assert.Equal(t, ReasonReconcileComplete, conditions[0].Reason)
		assert.Equal(t, "Done", conditions[0].Message)
		assert.Equal(t, int64(2), conditions[0].ObservedGeneration)
	})

	t.Run("preserves other conditions", func(t *testing.T) {
		conditions := []metav1.Condition{
			{Type: TypeReady, Status: metav1.ConditionTrue},
		}
		SetProgressing(&conditions, 1, metav1.ConditionTrue, ReasonReconciling, "In progress")

		assert.Len(t, conditions, 2)
		// Verify both conditions exist
		ready := false
		progressing := false
		for _, c := range conditions {
			if c.Type == TypeReady {
				ready = true
			}
			if c.Type == TypeProgressing {
				progressing = true
			}
		}
		assert.True(t, ready, "Ready condition should still exist")
		assert.True(t, progressing, "Progressing condition should be added")
	})
}
