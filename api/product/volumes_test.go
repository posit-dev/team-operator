package product

import (
	"testing"

	"github.com/rstudio/goex/ptr"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func TestVolumeMountHash(t *testing.T) {
	hash := VolumeMountHash(v1.VolumeMount{
		Name:      "test-mount",
		ReadOnly:  false,
		MountPath: "/tmp/path",
		SubPath:   "path",
	})
	require.Contains(t, hash, "test-mount")
	require.Contains(t, hash, "false")
	require.Contains(t, hash, "/tmp/path")
	require.Contains(t, hash, "path")
}

func initTestVolumeFactory() *VolumeFactory {
	return &VolumeFactory{
		Vols: map[string]*VolumeDef{
			"one": {
				Source: &v1.VolumeSource{},
				Mounts: []*VolumeMountDef{
					{"/path/one", "one", false},
					{"/path/two", "two", true},
				},
				Env: []v1.EnvVar{
					{
						Name:  "ONE",
						Value: "TWO",
					},
				},
			},
			"zzz": {
				Source: &v1.VolumeSource{},
				Mounts: []*VolumeMountDef{
					{"/path/last", "two", true},
				},
				Env: []v1.EnvVar{
					{Name: "THREE", Value: "FOUR"},
					{Name: "FIVE", Value: "SIX"},
				},
			},
		},
	}
}

func initTestSecretVolumeFactory() *SecretVolumeFactory {
	return &SecretVolumeFactory{
		Vols: map[string]*VolumeDef{
			"one": {
				Source: &v1.VolumeSource{},
				Mounts: []*VolumeMountDef{
					{"/path/one", "one", false},
					{"/path/two", "two", true},
				},
				Env: []v1.EnvVar{
					{
						Name:  "ONE",
						Value: "TWO",
					},
				},
			},
			"zzz": {
				Source: &v1.VolumeSource{},
				Mounts: []*VolumeMountDef{
					{"/path/last", "two", true},
				},
				Env: []v1.EnvVar{
					{Name: "THREE", Value: "FOUR"},
					{Name: "FIVE", Value: "SIX"},
				},
			},
		},
		Env: []v1.EnvVar{
			{Name: "FUN", Value: "OTHER"},
		},
		CsiEntries: map[string]*CSIDef{
			"test-csi-vol": &CSIDef{
				Driver:           "some-driver",
				ReadOnly:         ptr.To(false),
				VolumeAttributes: map[string]string{"some-attribute": "some-value"},
				DummyVolumeMount: []*VolumeMountDef{
					{MountPath: "/mnt/dummy", ReadOnly: true, SubPath: "dummy"},
				},
			},
		},
		CsiAllSecrets: map[string]string{
			"new-secret-key": "old-secret-key",
		},
		CsiKubernetesSecrets: map[string]map[string]string{
			"some-k8s-secret": {
				"final-secret-key": "new-secret-key",
			},
		},
	}
}

func TestVolumeFactory_Volumes(t *testing.T) {
	vFactory := initTestVolumeFactory()

	volumes := vFactory.Volumes()
	require.Equal(t, "one", volumes[0].Name)
	require.Equal(t, "zzz", volumes[1].Name)
}

func TestVolumeFactory_VolumeMounts(t *testing.T) {
	vFactory := initTestVolumeFactory()

	vms := vFactory.VolumeMounts()

	require.Equal(t, "one", vms[0].Name)
	require.Equal(t, "/path/one", vms[0].MountPath)
	require.Equal(t, "one", vms[1].Name)
	require.Equal(t, "/path/two", vms[1].MountPath)
	require.Equal(t, "zzz", vms[2].Name)
}

func TestVolumeFactory_EnvVars(t *testing.T) {
	vFactory := initTestVolumeFactory()

	env := vFactory.EnvVars()

	require.Equal(t, "ONE", env[0].Name)
	require.Equal(t, "THREE", env[1].Name)
	require.Equal(t, "FIVE", env[2].Name)
}

func TestSecretVolumeFactory(t *testing.T) {
	vFactory := initTestSecretVolumeFactory()

	// env vars
	env := vFactory.EnvVars()

	require.Equal(t, "ONE", env[0].Name)
	require.Equal(t, "THREE", env[1].Name)
	require.Equal(t, "FIVE", env[2].Name)
	require.Equal(t, "FUN", env[3].Name)

	// volumes
	vols := vFactory.Volumes()
	require.Len(t, vols, 3)
	require.Equal(t, "one", vols[0].Name)
	require.Equal(t, "test-csi-vol", vols[1].Name)
	require.Equal(t, "some-driver", vols[1].CSI.Driver)
	require.Equal(t, "zzz", vols[2].Name)

	// volume mounts
	vm := vFactory.VolumeMounts()
	require.Len(t, vm, 4)
	require.Equal(t, "one", vm[0].Name)
	require.Equal(t, "one", vm[0].SubPath)
	require.Equal(t, "one", vm[1].Name)
	require.Equal(t, "two", vm[1].SubPath)
	require.Equal(t, "test-csi-vol", vm[2].Name)
	require.Equal(t, "dummy", vm[2].SubPath)
	require.Equal(t, "zzz", vm[3].Name)
	require.Equal(t, "two", vm[3].SubPath)
}
