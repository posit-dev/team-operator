package core

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/localtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func initPackageManagerReconciler(t *testing.T) (context.Context, *PackageManagerReconciler, client.Client) {
	t.Helper()
	fakeEnv := localtest.FakeTestEnv{}
	cli, scheme, log := fakeEnv.Start(loadSchemes)
	r := &PackageManagerReconciler{
		Client: cli,
		Scheme: scheme,
		Log:    log,
	}
	ctx := logr.NewContext(context.Background(), log)
	return ctx, r, cli
}

func makePackageManager(ns, name, storageClass string) *positcov1beta1.PackageManager {
	return &positcov1beta1.PackageManager{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PackageManager",
			APIVersion: "core.posit.team/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
			UID:       "pm-test-uid",
		},
		Spec: positcov1beta1.PackageManagerSpec{
			PackageManagerStorageClassName: storageClass,
		},
	}
}

// TestCreateStorageClassPVC_Creation verifies that on first call the PVC is created
// with the correct annotation, labels, and StorageClass.
func TestCreateStorageClassPVC_Creation(t *testing.T) {
	ctx, r, cli := initPackageManagerReconciler(t)
	ns := "posit-team"
	pm := makePackageManager(ns, "mysite", "my-storage-class")

	require.NoError(t, cli.Create(ctx, pm))

	err := r.createStorageClassPVC(ctx, pm)
	require.NoError(t, err)

	pvc := &corev1.PersistentVolumeClaim{}
	pvcName := pm.ComponentName() + "-storage"
	require.NoError(t, cli.Get(ctx, types.NamespacedName{Namespace: ns, Name: pvcName}, pvc))

	// Annotation must be set
	assert.Equal(t, pm.Name+"/package-manager", pvc.Annotations["nfs.io/storage-path"])

	// Labels must be set
	labels := pm.KubernetesLabels()
	for k, v := range labels {
		assert.Equal(t, v, pvc.Labels[k], "label %s", k)
	}

	// StorageClass must be set
	require.NotNil(t, pvc.Spec.StorageClassName)
	assert.Equal(t, "my-storage-class", *pvc.Spec.StorageClassName)
}

// TestCreateStorageClassPVC_AnnotationOnUpdate verifies that the nfs.io/storage-path
// annotation is maintained on subsequent reconciles (not lost after creation).
func TestCreateStorageClassPVC_AnnotationOnUpdate(t *testing.T) {
	ctx, r, cli := initPackageManagerReconciler(t)
	ns := "posit-team"
	pm := makePackageManager(ns, "mysite", "my-storage-class")

	require.NoError(t, cli.Create(ctx, pm))

	// First reconcile – creates the PVC
	require.NoError(t, r.createStorageClassPVC(ctx, pm))

	// Second reconcile – should preserve (not drop) the annotation
	require.NoError(t, r.createStorageClassPVC(ctx, pm))

	pvc := &corev1.PersistentVolumeClaim{}
	pvcName := pm.ComponentName() + "-storage"
	require.NoError(t, cli.Get(ctx, types.NamespacedName{Namespace: ns, Name: pvcName}, pvc))

	assert.Equal(t, pm.Name+"/package-manager", pvc.Annotations["nfs.io/storage-path"],
		"annotation must be preserved across reconciles")
}

// TestCreateStorageClassPVC_LabelsMergedOnUpdate verifies that labels are merged
// on every reconcile, not only on creation.
func TestCreateStorageClassPVC_LabelsMergedOnUpdate(t *testing.T) {
	ctx, r, cli := initPackageManagerReconciler(t)
	ns := "posit-team"
	pm := makePackageManager(ns, "mysite", "my-storage-class")

	require.NoError(t, cli.Create(ctx, pm))

	// First reconcile – creates the PVC
	require.NoError(t, r.createStorageClassPVC(ctx, pm))

	// Simulate an existing PVC that has lost its labels (e.g. manual edit)
	pvcName := pm.ComponentName() + "-storage"
	pvc := &corev1.PersistentVolumeClaim{}
	require.NoError(t, cli.Get(ctx, types.NamespacedName{Namespace: ns, Name: pvcName}, pvc))
	pvc.Labels = map[string]string{}
	require.NoError(t, cli.Update(ctx, pvc))

	// Second reconcile – labels should be merged back
	require.NoError(t, r.createStorageClassPVC(ctx, pm))

	require.NoError(t, cli.Get(ctx, types.NamespacedName{Namespace: ns, Name: pvcName}, pvc))
	for k, v := range pm.KubernetesLabels() {
		assert.Equal(t, v, pvc.Labels[k], "label %s must be present after second reconcile", k)
	}
}

// TestCreateStorageClassPVC_MissingStorageClass verifies that an error is returned
// when PackageManagerStorageClassName is empty.
func TestCreateStorageClassPVC_MissingStorageClass(t *testing.T) {
	ctx, r, cli := initPackageManagerReconciler(t)
	ns := "posit-team"
	pm := makePackageManager(ns, "mysite", "")

	require.NoError(t, cli.Create(ctx, pm))

	err := r.createStorageClassPVC(ctx, pm)
	assert.Error(t, err)
}

// TestCreateStorageClassPVC_StorageClassMismatch verifies that when an existing PVC
// has a different StorageClassName, no error is returned and the PVC's StorageClass
// is left unchanged (the mismatch is logged as a warning).
func TestCreateStorageClassPVC_StorageClassMismatch(t *testing.T) {
	ctx, r, cli := initPackageManagerReconciler(t)
	ns := "posit-team"
	pm := makePackageManager(ns, "mysite", "sc-a")

	require.NoError(t, cli.Create(ctx, pm))

	// First reconcile – creates the PVC with StorageClass "sc-a"
	require.NoError(t, r.createStorageClassPVC(ctx, pm))

	// Change the requested StorageClass to "sc-b"
	pm.Spec.PackageManagerStorageClassName = "sc-b"

	// Second reconcile – should log a warning and return no error
	err := r.createStorageClassPVC(ctx, pm)
	require.NoError(t, err)

	// PVC's StorageClass must remain "sc-a"
	pvc := &corev1.PersistentVolumeClaim{}
	pvcName := pm.ComponentName() + "-storage"
	require.NoError(t, cli.Get(ctx, types.NamespacedName{Namespace: ns, Name: pvcName}, pvc))
	require.NotNil(t, pvc.Spec.StorageClassName)
	assert.Equal(t, "sc-a", *pvc.Spec.StorageClassName, "StorageClass must not change on existing PVC")
}
