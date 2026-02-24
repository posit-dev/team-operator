// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package crdapply

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestParseCRDs(t *testing.T) {
	crds, err := ParseCRDs()
	require.NoError(t, err)
	require.NotEmpty(t, crds, "expected at least one CRD to be embedded")

	names := make([]string, len(crds))
	for i, crd := range crds {
		names[i] = crd.Name
		require.NotEmpty(t, crd.Spec.Group, "CRD %s should have a group", crd.Name)
		require.NotEmpty(t, crd.Spec.Names.Kind, "CRD %s should have a kind", crd.Name)
	}
	t.Logf("embedded CRDs: %v", names)
}

func TestPermanentErrorType(t *testing.T) {
	sentinel := errors.New("config failure")
	pe := permanentError{sentinel}
	require.Equal(t, "config failure", pe.Error())
	require.True(t, errors.Is(pe, sentinel), "permanentError should unwrap to inner error")
	var target permanentError
	require.True(t, errors.As(pe, &target), "errors.As should match permanentError")
}

func newFakeClient(t *testing.T) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, apiextensionsv1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).Build()
}

// TestPollApplyCRDsPermanentErrorStopsPoll verifies that a permanentError causes the
// poll loop to stop after the first attempt, without waiting for the context to expire.
func TestPollApplyCRDsPermanentErrorStopsPoll(t *testing.T) {
	callCount := 0
	fn := func(_ context.Context, _ client.Client, _ logr.Logger) error {
		callCount++
		return permanentError{fmt.Errorf("no CRDs found")}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := pollApplyCRDs(ctx, newFakeClient(t), logr.Discard(), 5*time.Second, fn)
	require.Error(t, err)
	require.Equal(t, 1, callCount, "permanent error should stop the poll after one attempt")
	require.Contains(t, err.Error(), "no CRDs found")
}

// TestPollApplyCRDsTransientErrorRetried verifies that transient errors cause the poll
// loop to retry until the function succeeds.
func TestPollApplyCRDsTransientErrorRetried(t *testing.T) {
	callCount := 0
	fn := func(_ context.Context, _ client.Client, _ logr.Logger) error {
		callCount++
		if callCount < 3 {
			return fmt.Errorf("transient error %d", callCount)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := pollApplyCRDs(ctx, newFakeClient(t), logr.Discard(), 10*time.Millisecond, fn)
	require.NoError(t, err)
	require.Equal(t, 3, callCount, "should retry until success")
}

// TestPollApplyCRDsContextCancelWrapsErrors verifies that when the context is cancelled
// after transient errors, the returned error includes both the poll error and the last
// apply error.
func TestPollApplyCRDsContextCancelWrapsErrors(t *testing.T) {
	fn := func(_ context.Context, _ client.Client, _ logr.Logger) error {
		return fmt.Errorf("transient patch error")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := pollApplyCRDs(ctx, newFakeClient(t), logr.Discard(), 5*time.Second, fn)
	require.Error(t, err)
	require.Contains(t, err.Error(), "transient patch error", "last apply error should be included in returned error")
}

// TestPollApplyCRDsContextCancelUnwrapsErrors verifies that both the poll error and the
// last apply error are independently unwrappable via errors.Is from the combined error.
// This exercises the Go 1.20+ multi-error wrapping via fmt.Errorf with two %w verbs.
func TestPollApplyCRDsContextCancelUnwrapsErrors(t *testing.T) {
	sentinel := errors.New("sentinel transient error")
	fn := func(_ context.Context, _ client.Client, _ logr.Logger) error {
		return sentinel
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := pollApplyCRDs(ctx, newFakeClient(t), logr.Discard(), 5*time.Second, fn)
	require.Error(t, err)
	require.True(t, errors.Is(err, context.DeadlineExceeded), "poll error (context deadline) should be unwrappable via errors.Is")
	require.True(t, errors.Is(err, sentinel), "last apply error should be unwrappable via errors.Is (requires Go 1.20+ dual %%w)")
}

// TestApplyCRDs is a structural test that verifies all embedded CRDs are stored after
// applyCRDs is called. It uses the controller-runtime fake client, which implements
// Patch(Apply) as a simplified create-or-update and does not enforce SSA field-manager
// ownership or conflict detection. Real SSA semantics are only exercised in integration
// or e2e tests against a live API server.
func TestApplyCRDs(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, apiextensionsv1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	err := applyCRDs(context.Background(), fakeClient, logr.Discard())
	require.NoError(t, err)

	// Verify each CRD was actually stored by the fake client.
	crds, err := ParseCRDs()
	require.NoError(t, err)
	for _, expected := range crds {
		stored := &apiextensionsv1.CustomResourceDefinition{}
		require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: expected.Name}, stored),
			"CRD %s should have been applied", expected.Name)
	}
}
