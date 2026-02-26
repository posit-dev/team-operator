package core

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/posit-dev/team-operator/api/localtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func initSiteReconciler(t *testing.T) (context.Context, *SiteReconciler, client.WithWatch) {
	t.Helper()
	fakeEnv := localtest.FakeTestEnv{}
	cli, scheme, log := fakeEnv.Start(loadSchemes)
	r := &SiteReconciler{
		Client: cli,
		Scheme: scheme,
		Log:    log,
	}
	ctx := logr.NewContext(context.Background(), log)
	return ctx, r, cli
}

// TestProvisionVolumeViaPVC_AnnotationPersistence verifies that nfs.io/storage-path
// is set on creation and preserved on subsequent reconciles.
func TestProvisionVolumeViaPVC_AnnotationPersistence(t *testing.T) {
	ctx, r, cli := initSiteReconciler(t)
	ns := "posit-team"
	site := defaultSite("mysite")

	require.NoError(t, cli.Create(ctx, site))

	size := resource.MustParse("10Gi")

	// First reconcile – creates the PVC
	require.NoError(t, r.provisionVolumeViaPVC(ctx, site, "mysite-pvc", "data", "my-sc", size))

	// Second reconcile – should preserve the annotation
	require.NoError(t, r.provisionVolumeViaPVC(ctx, site, "mysite-pvc", "data", "my-sc", size))

	pvc := &corev1.PersistentVolumeClaim{}
	require.NoError(t, cli.Get(ctx, types.NamespacedName{Namespace: ns, Name: "mysite-pvc"}, pvc))
	assert.Equal(t, "mysite/data", pvc.Annotations["nfs.io/storage-path"],
		"annotation must be preserved across reconciles")
}

// TestProvisionVolumeViaPVC_LabelsMergedOnUpdate verifies that labels are merged
// on every reconcile, not only on creation.
func TestProvisionVolumeViaPVC_LabelsMergedOnUpdate(t *testing.T) {
	ctx, r, cli := initSiteReconciler(t)
	ns := "posit-team"
	site := defaultSite("mysite")

	require.NoError(t, cli.Create(ctx, site))

	size := resource.MustParse("10Gi")

	// First reconcile – creates the PVC with labels
	require.NoError(t, r.provisionVolumeViaPVC(ctx, site, "mysite-pvc", "data", "my-sc", size))

	// Simulate a PVC that has lost its labels (e.g. manual edit)
	pvc := &corev1.PersistentVolumeClaim{}
	require.NoError(t, cli.Get(ctx, types.NamespacedName{Namespace: ns, Name: "mysite-pvc"}, pvc))
	pvc.Labels = map[string]string{}
	require.NoError(t, cli.Update(ctx, pvc))

	// Second reconcile – labels must be merged back
	require.NoError(t, r.provisionVolumeViaPVC(ctx, site, "mysite-pvc", "data", "my-sc", size))

	require.NoError(t, cli.Get(ctx, types.NamespacedName{Namespace: ns, Name: "mysite-pvc"}, pvc))
	for k, v := range site.KubernetesLabels() {
		assert.Equal(t, v, pvc.Labels[k], "label %s must be present after second reconcile", k)
	}
}
