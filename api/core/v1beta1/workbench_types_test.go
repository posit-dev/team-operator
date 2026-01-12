package v1beta1

import (
	"context"
	"fmt"
	"testing"

	"github.com/posit-dev/team-operator/api/product"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func anyTrue[T any](l []T, predicate func(T) bool) bool {
	for _, elt := range l {
		if predicate(elt) {
			return true
		}
	}
	fmt.Printf("List is not true: %v+\n", l)
	return false
}

func allTrue[T any](l []T, predicate func(T) bool) bool {
	for _, elt := range l {
		if !predicate(elt) {
			return false
		}
	}
	return true
}

func TestWorkbench_CreateVolumeFactory_NonRoot(t *testing.T) {
	w := &Workbench{
		ObjectMeta: v1.ObjectMeta{
			Name:      "non-root",
			Namespace: "ns",
		},
		Spec: WorkbenchSpec{
			NonRoot: true,
		},
	}
	cfg := &WorkbenchConfig{}
	vf := w.CreateVolumeFactory(cfg)

	// if we set NonRoot, then the supervisor-volume and supervisor-cover-volume should get created
	vols := vf.Volumes()
	foundVol := false
	foundCoverVol := false
	for _, v := range vols {
		if v.Name == "supervisor-volume" {
			foundVol = true
		}
		if v.Name == "supervisor-cover-volume" {
			foundCoverVol = true
		}
	}
	assert.True(t, foundVol)
	assert.True(t, foundCoverVol)
	vms := vf.VolumeMounts()

	foundVm := false
	foundCoverVm := false
	for _, vm := range vms {
		if vm.Name == "supervisor-volume" {
			foundVm = true
		}
		if vm.Name == "supervisor-cover-volume" {
			foundCoverVm = true
		}
	}
	assert.True(t, foundVm)
	assert.True(t, foundCoverVm)

	// if not set... should not get created
	w.Spec.NonRoot = false
	vf2 := w.CreateVolumeFactory(cfg)

	// if we set NonRoot, then the supervisor-volume and supervisor-cover-volume should get created
	vols2 := vf2.Volumes()
	foundVol2 := false
	foundCoverVol2 := false
	for _, v := range vols2 {
		if v.Name == "supervisor-volume" {
			foundVol2 = true
		}
		if v.Name == "supervisor-cover-volume" {
			foundCoverVol2 = true
		}
	}
	assert.False(t, foundVol2)
	assert.False(t, foundCoverVol2)
	vms2 := vf2.VolumeMounts()

	foundVm2 := false
	foundCoverVm2 := false
	for _, vm := range vms2 {
		if vm.Name == "supervisor-volume" {
			foundVm2 = true
		}
		if vm.Name == "supervisor-cover-volume" {
			foundCoverVm2 = true
		}
	}
	assert.False(t, foundVm2)
	assert.False(t, foundCoverVm2)
}

func TestWorkbench_CreateSecretVolumeFactory_Kubernetes(t *testing.T) {

	w := &Workbench{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-k8s",
			Namespace: "ns",
		},
		Spec: WorkbenchSpec{
			Secret: SecretConfig{
				VaultName: "test-vault",
				Type:      product.SiteSecretKubernetes,
			},
		},
	}

	vf := w.CreateSecretVolumeFactory()

	e := vf.EnvVars()
	require.Len(t, e, 2)
	require.True(t, anyTrue(e, func(envVar corev1.EnvVar) bool {
		return envVar.Name == "WORKBENCH_POSTGRES_PASSWORD" &&
			envVar.ValueFrom != nil &&
			envVar.ValueFrom.SecretKeyRef != nil &&
			envVar.ValueFrom.SecretKeyRef.LocalObjectReference.Name == "test-vault"
	}))

	v := vf.Volumes()
	require.Len(t, v, 5)

	vm := vf.VolumeMounts()
	require.Len(t, vm, 4)
}

func TestWorkbench_CreateSecretVolumeFactory_Aws(t *testing.T) {

	w := &Workbench{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-aws",
			Namespace: "ns",
		},
		Spec: WorkbenchSpec{
			Secret: SecretConfig{
				VaultName: "aws-vault",
				Type:      product.SiteSecretAws,
			},
		},
	}

	vf := w.CreateSecretVolumeFactory()

	e := vf.EnvVars()
	require.Len(t, e, 2)
	require.True(t, anyTrue(e, func(envVar corev1.EnvVar) bool {
		return envVar.Name == "WORKBENCH_POSTGRES_PASSWORD" &&
			envVar.ValueFrom != nil &&
			envVar.ValueFrom.SecretKeyRef != nil &&
			envVar.ValueFrom.SecretKeyRef.LocalObjectReference.Name == "test-aws-workbench-secret"
	}))

	v := vf.Volumes()
	require.Len(t, v, 5)

	vm := vf.VolumeMounts()
	// TODO: should we be mounting the licensing config if it is not set in the workbench spec...?
	//    (we currently are... and I think that might be a bug...?)
	require.Len(t, vm, 4)
}

func TestWorkbench_InitializeNonRootSupervisorConfig(t *testing.T) {

	w := &Workbench{
		ObjectMeta: v1.ObjectMeta{
			Name:      "non-root",
			Namespace: "ns",
		},
		Spec: WorkbenchSpec{
			NonRoot: true,
		},
	}
	cfg := &WorkbenchConfig{}

	// no config present
	assert.Nil(t, cfg.SupervisordIniConfig.Programs)

	err := w.InitializeNonRootSupervisorConfig(context.TODO(), cfg)
	assert.Nil(t, err)

	// now config is present!
	assert.Contains(t, cfg.SupervisordIniConfig.Programs, "launcher.conf")
	assert.Contains(t, cfg.SupervisordIniConfig.Programs["launcher.conf"], "rstudio-launcher")
	assert.Equal(t, "root", cfg.SupervisordIniConfig.Programs["launcher.conf"]["rstudio-launcher"].User)

	// make sure failure modes are ok
	cfgFile := &WorkbenchConfig{
		SupervisordIniConfig: SupervisordIniConfig{
			Programs: map[string]map[string]*SupervisordProgramConfig{
				"custom-launcher.conf": {
					"something": {
						User: "my-user",
					},
				},
			},
		},
	}

	err = w.InitializeNonRootSupervisorConfig(context.TODO(), cfgFile)
	assert.Nil(t, err)
	assert.Contains(t, cfgFile.SupervisordIniConfig.Programs, "custom-launcher.conf")
	assert.NotContains(t, cfgFile.SupervisordIniConfig.Programs, "launcher.conf")
	assert.Contains(t, cfgFile.SupervisordIniConfig.Programs["custom-launcher.conf"], "something")
	assert.NotContains(t, cfgFile.SupervisordIniConfig.Programs["custom-launcher.conf"], "rstudio-launcher")

	// and the other...
	cfgProg := &WorkbenchConfig{
		SupervisordIniConfig: SupervisordIniConfig{
			Programs: map[string]map[string]*SupervisordProgramConfig{
				"something.conf": {
					"some-launcher-program": {
						User: "my-user",
					},
				},
			},
		},
	}

	err = w.InitializeNonRootSupervisorConfig(context.TODO(), cfgProg)
	assert.Nil(t, err)
	assert.Contains(t, cfgProg.SupervisordIniConfig.Programs, "something.conf")
	assert.NotContains(t, cfgProg.SupervisordIniConfig.Programs, "launcher.conf")
	assert.Contains(t, cfgProg.SupervisordIniConfig.Programs["something.conf"], "some-launcher-program")
	assert.NotContains(t, cfgProg.SupervisordIniConfig.Programs["something.conf"], "rstudio-launcher")
}
