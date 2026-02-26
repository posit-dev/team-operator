package v1beta1

import (
	"testing"

	"github.com/posit-dev/team-operator/api/product"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPackageManager_CreateSecretVolumeFactory_K8sSecret(t *testing.T) {
	pm := &PackageManager{
		ObjectMeta: v1.ObjectMeta{
			Name:      "k8s-secret",
			Namespace: "ns",
		},
		Spec: PackageManagerSpec{
			Secret: SecretConfig{
				Name: "my-pkg-secret",
			},
			License: product.LicenseSpec{
				Type: product.LicenseTypeFile,
			},
		},
	}

	vf := pm.CreateSecretVolumeFactory()

	env := vf.EnvVars()

	foundPassword := false
	foundUsagePassword := false
	foundSecretKey := false
	for _, e := range env {
		if e.Name == "PACKAGEMANAGER_POSTGRES_PASSWORD" {
			foundPassword = true
			assert.Equal(t, "my-pkg-secret", e.ValueFrom.SecretKeyRef.Name)
			assert.Equal(t, "pkg-db-password", e.ValueFrom.SecretKeyRef.Key)
		}
		if e.Name == "PACKAGEMANAGER_POSTGRES_USAGEDATAPASSWORD" {
			foundUsagePassword = true
			assert.Equal(t, "my-pkg-secret", e.ValueFrom.SecretKeyRef.Name)
			assert.Equal(t, "pkg-db-usagedata-password", e.ValueFrom.SecretKeyRef.Key)
		}
		if e.Name == "PACKAGEMANAGER_SECRET_KEY" {
			foundSecretKey = true
			assert.Equal(t, "my-pkg-secret", e.ValueFrom.SecretKeyRef.Name)
			assert.Equal(t, "pkg-secret-key", e.ValueFrom.SecretKeyRef.Key)
		}
	}
	assert.True(t, foundPassword)
	assert.True(t, foundUsagePassword)
	assert.True(t, foundSecretKey)
}
