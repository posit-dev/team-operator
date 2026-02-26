// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package core

import (
	"context"
	"testing"

	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	localtest "github.com/posit-dev/team-operator/api/localtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPackageManagerReconciler_SiteWatchMap(t *testing.T) {
	ctx := context.Background()
	ns := "test-ns"

	fakeEnv := localtest.FakeTestEnv{}
	cli, cliScheme, cliLog := fakeEnv.Start(loadSchemes)
	r := &PackageManagerReconciler{
		Client: cli,
		Scheme: cliScheme,
		Log:    cliLog,
	}

	// PackageManager.SiteName() returns pm.Name (the metadata name), so no Spec is needed.
	// Create a PackageManager that matches the Site name
	require.NoError(t, cli.Create(ctx, &positcov1beta1.PackageManager{
		ObjectMeta: metav1.ObjectMeta{Name: "my-site", Namespace: ns},
	}))
	// Create a PackageManager with a different name (should not be enqueued)
	require.NoError(t, cli.Create(ctx, &positcov1beta1.PackageManager{
		ObjectMeta: metav1.ObjectMeta{Name: "other-site", Namespace: ns},
	}))
	// Create a PackageManager in a different namespace (should not be enqueued)
	require.NoError(t, cli.Create(ctx, &positcov1beta1.PackageManager{
		ObjectMeta: metav1.ObjectMeta{Name: "my-site", Namespace: "other-ns"},
	}))

	site := &positcov1beta1.Site{
		ObjectMeta: metav1.ObjectMeta{Name: "my-site", Namespace: ns},
	}
	requests := r.siteToPackageManagerRequests(ctx, site)

	require.Len(t, requests, 1)
	assert.Equal(t, "my-site", requests[0].Name)
	assert.Equal(t, ns, requests[0].Namespace)
}
