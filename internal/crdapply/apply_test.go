// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package crdapply

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCRDs(t *testing.T) {
	crds, err := ParseCRDs()
	require.NoError(t, err)
	require.Len(t, crds, 7, "expected 7 CRDs to be embedded")

	names := make([]string, len(crds))
	for i, crd := range crds {
		names[i] = crd.Name
		require.NotEmpty(t, crd.Spec.Group, "CRD %s should have a group", crd.Name)
		require.NotEmpty(t, crd.Spec.Names.Kind, "CRD %s should have a kind", crd.Name)
	}
	t.Logf("embedded CRDs: %v", names)
}
