// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package status

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func TestSetDeploymentHealth(t *testing.T) {
	t.Run("ready when replicas meet desired", func(t *testing.T) {
		conditions := []metav1.Condition{}
		SetDeploymentHealth(&conditions, 3, 2, 2)

		readyCond := findCondition(conditions, TypeReady)
		require.NotNil(t, readyCond, "expected Ready condition to be set")
		assert.Equal(t, metav1.ConditionTrue, readyCond.Status)
		assert.Equal(t, ReasonDeploymentReady, readyCond.Reason)

		progCond := findCondition(conditions, TypeProgressing)
		require.NotNil(t, progCond, "expected Progressing condition to be set")
		assert.Equal(t, metav1.ConditionFalse, progCond.Status)
		assert.Equal(t, ReasonReconcileComplete, progCond.Reason)
	})

	t.Run("not ready when replicas below desired", func(t *testing.T) {
		conditions := []metav1.Condition{}
		SetDeploymentHealth(&conditions, 3, 1, 3)

		readyCond := findCondition(conditions, TypeReady)
		require.NotNil(t, readyCond, "expected Ready condition to be set")
		assert.Equal(t, metav1.ConditionFalse, readyCond.Status)
		assert.Equal(t, ReasonDeploymentNotReady, readyCond.Reason)
		assert.Contains(t, readyCond.Message, "1/3")

		progCond := findCondition(conditions, TypeProgressing)
		require.NotNil(t, progCond, "expected Progressing condition to be set")
		assert.Equal(t, metav1.ConditionTrue, progCond.Status)
		assert.Equal(t, ReasonReconciling, progCond.Reason)
	})
}

func TestSetStatefulSetHealth(t *testing.T) {
	t.Run("ready when replicas meet desired", func(t *testing.T) {
		conditions := []metav1.Condition{}
		SetStatefulSetHealth(&conditions, 5, 3, 3)

		readyCond := findCondition(conditions, TypeReady)
		require.NotNil(t, readyCond, "expected Ready condition to be set")
		assert.Equal(t, metav1.ConditionTrue, readyCond.Status)
		assert.Equal(t, ReasonStatefulSetReady, readyCond.Reason)

		progCond := findCondition(conditions, TypeProgressing)
		require.NotNil(t, progCond, "expected Progressing condition to be set")
		assert.Equal(t, metav1.ConditionFalse, progCond.Status)
		assert.Equal(t, ReasonReconcileComplete, progCond.Reason)
	})

	t.Run("not ready when replicas below desired", func(t *testing.T) {
		conditions := []metav1.Condition{}
		SetStatefulSetHealth(&conditions, 5, 0, 1)

		readyCond := findCondition(conditions, TypeReady)
		require.NotNil(t, readyCond, "expected Ready condition to be set")
		assert.Equal(t, metav1.ConditionFalse, readyCond.Status)
		assert.Equal(t, ReasonStatefulSetNotReady, readyCond.Reason)
		assert.Contains(t, readyCond.Message, "0/1")

		progCond := findCondition(conditions, TypeProgressing)
		require.NotNil(t, progCond, "expected Progressing condition to be set")
		assert.Equal(t, metav1.ConditionTrue, progCond.Status)
		assert.Equal(t, ReasonReconciling, progCond.Reason)
	})
}

type fakeStatusWriter struct {
	patchCalled bool
}

func (f *fakeStatusWriter) Create(_ context.Context, _ client.Object, _ client.Object, _ ...client.SubResourceCreateOption) error {
	return nil
}
func (f *fakeStatusWriter) Update(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error {
	return nil
}
func (f *fakeStatusWriter) Patch(_ context.Context, _ client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
	f.patchCalled = true
	return nil
}

func TestPatchSuspendedStatus(t *testing.T) {
	t.Run("clears version and sets ready to false", func(t *testing.T) {
		conditions := []metav1.Condition{}
		var observedGen int64
		ready := true
		version := "2024.06.0"

		sw := &fakeStatusWriter{}
		err := PatchSuspendedStatus(
			context.Background(),
			sw,
			&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: "test", UID: types.UID("test-uid")}},
			client.MergeFrom(&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: "test", UID: types.UID("test-uid")}}),
			&conditions, 3, &observedGen, &ready, &version,
		)

		require.NoError(t, err)
		assert.True(t, sw.patchCalled, "expected status Patch to be called")
		assert.Equal(t, int64(3), observedGen)
		assert.False(t, ready)
		assert.Empty(t, version)

		readyCond := findCondition(conditions, TypeReady)
		require.NotNil(t, readyCond, "expected Ready condition to be set")
		assert.Equal(t, metav1.ConditionFalse, readyCond.Status)
		assert.Equal(t, ReasonSuspended, readyCond.Reason)

		progCond := findCondition(conditions, TypeProgressing)
		require.NotNil(t, progCond, "expected Progressing condition to be set")
		assert.Equal(t, metav1.ConditionFalse, progCond.Status)
		assert.Equal(t, ReasonSuspended, progCond.Reason)
	})
}

func TestIsSuspended(t *testing.T) {
	tests := []struct {
		name       string
		conditions []metav1.Condition
		expected   bool
	}{
		{
			name: "Ready condition with Suspended reason",
			conditions: []metav1.Condition{
				{Type: TypeReady, Status: metav1.ConditionFalse, Reason: ReasonSuspended},
			},
			expected: true,
		},
		{
			name: "Ready condition with different reason",
			conditions: []metav1.Condition{
				{Type: TypeReady, Status: metav1.ConditionFalse, Reason: ReasonReconcileError},
			},
			expected: false,
		},
		{
			name:       "No conditions",
			conditions: []metav1.Condition{},
			expected:   false,
		},
		{
			name: "Ready condition True is not suspended",
			conditions: []metav1.Condition{
				{Type: TypeReady, Status: metav1.ConditionTrue, Reason: ReasonDeploymentReady},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSuspended(tt.conditions)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPatchErrorStatus_TruncatesLongMessages(t *testing.T) {
	t.Run("short message is preserved", func(t *testing.T) {
		conditions := []metav1.Condition{}
		sw := &fakeStatusWriter{}
		shortErr := fmt.Errorf("short error")

		err := PatchErrorStatus(
			context.Background(),
			sw,
			&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: "test", UID: types.UID("test-uid")}},
			client.MergeFrom(&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: "test", UID: types.UID("test-uid")}}),
			&conditions, 1, shortErr,
		)

		require.NoError(t, err)
		readyCond := findCondition(conditions, TypeReady)
		require.NotNil(t, readyCond)
		assert.Equal(t, "short error", readyCond.Message)
	})

	t.Run("long message is truncated", func(t *testing.T) {
		conditions := []metav1.Condition{}
		sw := &fakeStatusWriter{}
		longMsg := strings.Repeat("x", 300)
		longErr := fmt.Errorf("%s", longMsg)

		err := PatchErrorStatus(
			context.Background(),
			sw,
			&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: "test", UID: types.UID("test-uid")}},
			client.MergeFrom(&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: "test", UID: types.UID("test-uid")}}),
			&conditions, 1, longErr,
		)

		require.NoError(t, err)
		readyCond := findCondition(conditions, TypeReady)
		require.NotNil(t, readyCond)
		assert.Len(t, readyCond.Message, maxConditionMessageLength)
		assert.True(t, strings.HasSuffix(readyCond.Message, "..."))
	})
}

func TestTruncateMessage(t *testing.T) {
	t.Run("short message unchanged", func(t *testing.T) {
		assert.Equal(t, "hello", TruncateMessage("hello"))
	})

	t.Run("exactly at limit unchanged", func(t *testing.T) {
		msg := strings.Repeat("a", maxConditionMessageLength)
		assert.Equal(t, msg, TruncateMessage(msg))
	})

	t.Run("long ASCII message truncated", func(t *testing.T) {
		msg := strings.Repeat("x", 300)
		result := TruncateMessage(msg)
		assert.Len(t, result, maxConditionMessageLength)
		assert.True(t, strings.HasSuffix(result, "..."))
	})

	t.Run("multi-byte UTF-8 at boundary is not split", func(t *testing.T) {
		// Each '日' is 3 bytes. Fill up to near the limit with multi-byte chars
		// so that a naive byte-slice would split a rune.
		prefix := strings.Repeat("a", maxConditionMessageLength-5) // 251 ASCII bytes
		// Add two 3-byte runes (6 bytes total) → 257 bytes, over limit
		msg := prefix + "日日"
		result := TruncateMessage(msg)
		assert.True(t, len(result) <= maxConditionMessageLength)
		assert.True(t, strings.HasSuffix(result, "..."))
		// Verify the result is valid UTF-8 (no split runes)
		assert.True(t, len(result) > 0)
		for _, r := range result {
			assert.NotEqual(t, rune(65533), r, "should not contain replacement character")
		}
	})
}

func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}
