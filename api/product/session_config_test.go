package product

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func TestSessionConfig_GenerateSessionConfigTemplate(t *testing.T) {
	nothing := SessionConfig{}

	str, err := nothing.GenerateSessionConfigTemplate()
	require.Nil(t, err)

	minimal := SessionConfig{
		Pod: &PodConfig{
			ImagePullPolicy: "Always",
		},
	}
	str, err = minimal.GenerateSessionConfigTemplate()
	require.Nil(t, err)
	require.Contains(t, str, "\"imagePullPolicy\":\"Always\"")

	complex := SessionConfig{
		Pod: &PodConfig{
			Volumes: []v1.Volume{
				{
					Name: "volume",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				},
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "volume",
					MountPath: "/mnt/tmp",
				},
			},
			InitContainers: []v1.Container{
				{
					Name:            "init",
					Image:           "some-image",
					ImagePullPolicy: v1.PullIfNotPresent,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "volume",
							MountPath: "/mnt/tmp-init",
						},
					},
				},
			},
		},
	}

	str, err = complex.GenerateSessionConfigTemplate()
	require.Nil(t, err)
	require.Contains(t, str, "\"imagePullPolicy\":\"IfNotPresent\"")
	require.Contains(t, str, "\"name\":\"volume\"")
	require.Contains(t, str, "\"image\":\"some-image\"")
	require.Contains(t, str, "\"mountPath\":\"/mnt/tmp-init\"")
	require.Contains(t, str, "\"mountPath\":\"/mnt/tmp\"")
}

func TestSiteSessionVaultName(t *testing.T) {
	t.Skip("Need to create a TestProduct struct to test this behavior")
}
