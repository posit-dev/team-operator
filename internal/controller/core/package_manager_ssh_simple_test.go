package core_test

import (
	"fmt"
	"testing"

	v1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPackageManager_SSHKeysAsSeparateSecrets(t *testing.T) {
	tests := []struct {
		name              string
		sshKeys           []v1beta1.SSHKeyConfig
		secretType        product.SiteSecretType
		expectedSSHVolume bool
	}{
		{
			name:              "No SSH keys",
			sshKeys:           nil,
			secretType:        product.SiteSecretAws,
			expectedSSHVolume: false,
		},
		{
			name: "Single AWS SSH key",
			sshKeys: []v1beta1.SSHKeyConfig{
				{
					Name: "github",
					Host: "github.com",
					SecretRef: v1beta1.SecretReference{
						Source: "aws-secrets-manager",
						Name:   "github", // Name is now just the key name in the vault
					},
				},
			},
			secretType:        product.SiteSecretAws,
			expectedSSHVolume: true,
		},
		{
			name: "Multiple AWS SSH keys",
			sshKeys: []v1beta1.SSHKeyConfig{
				{
					Name: "github",
					Host: "github.com",
					SecretRef: v1beta1.SecretReference{
						Source: "aws-secrets-manager",
						Name:   "github",
					},
				},
				{
					Name: "gitlab",
					Host: "gitlab.com",
					SecretRef: v1beta1.SecretReference{
						Source: "aws-secrets-manager",
						Name:   "gitlab",
					},
				},
			},
			secretType:        product.SiteSecretAws,
			expectedSSHVolume: true,
		},
		{
			name: "Mixed secret sources - AWS SSH volume created",
			sshKeys: []v1beta1.SSHKeyConfig{
				{
					Name: "github",
					Host: "github.com",
					SecretRef: v1beta1.SecretReference{
						Source: "aws-secrets-manager",
						Name:   "github",
					},
				},
				{
					Name: "gitlab",
					Host: "gitlab.com",
					SecretRef: v1beta1.SecretReference{
						Source: "kubernetes",
						Name:   "gitlab-ssh",
						Key:    "private-key",
					},
				},
			},
			secretType:        product.SiteSecretAws,
			expectedSSHVolume: true, // AWS SSH keys still create volume
		},
		{
			name: "Kubernetes secret type - No SSH volume",
			sshKeys: []v1beta1.SSHKeyConfig{
				{
					Name: "github",
					Host: "github.com",
					SecretRef: v1beta1.SecretReference{
						Source: "aws-secrets-manager",
						Name:   "github",
					},
				},
			},
			secretType:        product.SiteSecretKubernetes,
			expectedSSHVolume: false, // AWS SSH keys not added when using Kubernetes secrets
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &v1beta1.PackageManager{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test",
					Namespace: "test-namespace",
				},
				Spec: v1beta1.PackageManagerSpec{
					WorkloadCompoundName: "test-workload",
					Secret: v1beta1.SecretConfig{
						Type:      tt.secretType,
						VaultName: "test-vault",
					},
					GitSSHKeys: tt.sshKeys,
				},
			}

			// SSH keys should be in a separate CSI volume
			// This is done through CreateSecretVolumeFactory
			factory := pm.CreateSecretVolumeFactory()

			// Check if SSH CSI volume exists
			_, hasSSHVolume := factory.CsiEntries["ssh-secrets-volume"]
			assert.Equal(t, tt.expectedSSHVolume, hasSSHVolume, "SSH CSI volume presence mismatch")

			// If SSH volume exists, verify it has the correct SecretProviderClass
			if hasSSHVolume {
				sshVolume := factory.CsiEntries["ssh-secrets-volume"]
				assert.Equal(t, "secrets-store.csi.k8s.io", sshVolume.Driver)
				assert.Equal(t, "test-packagemanager-ssh-secrets", sshVolume.VolumeAttributes["secretProviderClass"])
				assert.True(t, *sshVolume.ReadOnly)

				// Check that individual SSH key volumes are created using CSI
				for _, sshKey := range tt.sshKeys {
					if sshKey.SecretRef.Source == "aws-secrets-manager" {
						volName := fmt.Sprintf("ssh-key-%s", sshKey.Name)
						vol, exists := factory.Vols[volName]
						assert.True(t, exists, "Volume for SSH key %s should exist", sshKey.Name)
						if exists {
							assert.NotNil(t, vol.Source.CSI, "Volume should use CSI source")
							assert.Equal(t, "secrets-store.csi.k8s.io", vol.Source.CSI.Driver)
							assert.Equal(t, "test-packagemanager-ssh-secrets", vol.Source.CSI.VolumeAttributes["secretProviderClass"])
						}
					}
				}
			}
		})
	}
}
