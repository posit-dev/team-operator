// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package crdapply

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
