package v1beta1

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/posit-dev/team-operator/api/product"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v12 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

func checkCMVolume(t *testing.T, vol corev1.Volume, name, configMap string) {
	assert.Equal(t, name, vol.Name)
	assert.Equal(t, configMap, vol.ConfigMap.Name)
	assert.Equal(t, configMap, vol.ConfigMap.LocalObjectReference.Name)
}

func checkCsiVolume(t *testing.T, vol corev1.Volume, name, driver, key, value string, readOnly bool) {
	assert.Equal(t, name, vol.Name)
	assert.NotNil(t, vol.CSI)
	assert.Equal(t, driver, vol.CSI.Driver)
	assert.Equal(t, readOnly, *vol.CSI.ReadOnly)
	assert.Equal(t, value, vol.CSI.VolumeAttributes[key])
}

func checkEmptyVolume(t *testing.T, vol corev1.Volume, name string) {
	assert.Equal(t, name, vol.Name)
	assert.Nil(t, vol.ConfigMap)
	assert.Nil(t, vol.Secret)
	assert.Nil(t, vol.CSI)
	assert.NotNil(t, vol.EmptyDir)
}

func checkSecretVolume(t *testing.T, vol corev1.Volume, name, secretName, secretKey, secretPath string) {
	assert.Equal(t, name, vol.Name)
	assert.NotNil(t, vol.Secret)
	assert.Equal(t, secretName, vol.Secret.SecretName)

	// just a single item covered presently...
	assert.Len(t, vol.Secret.Items, 1)
	assert.Equal(t, secretKey, vol.Secret.Items[0].Key)
	assert.Equal(t, secretPath, vol.Secret.Items[0].Path)

	// TODO: defaultMode...
}

func checkVolumeMount(t *testing.T, vm corev1.VolumeMount, name, mountPath, subPath string, readOnly bool) {
	assert.Equal(t, mountPath, vm.MountPath)
	assert.Equal(t, subPath, vm.SubPath)
	assert.Equal(t, name, vm.Name)
	assert.Equal(t, readOnly, vm.ReadOnly)
}

func checkEnvVar(t *testing.T, e corev1.EnvVar, name, value string) {
	assert.Equal(t, name, e.Name)
	assert.Equal(t, value, e.Value)
	assert.Nil(t, e.ValueFrom)
}
func checkEnvVarFromSecret(t *testing.T, e corev1.EnvVar, name, secretName, secretKey string) {
	assert.Equal(t, name, e.Name)
	assert.NotNil(t, e.ValueFrom)
	assert.NotNil(t, e.ValueFrom.SecretKeyRef)
	assert.Equal(t, secretName, e.ValueFrom.SecretKeyRef.LocalObjectReference.Name)
	assert.Equal(t, secretKey, e.ValueFrom.SecretKeyRef.Key)
}
func TestCreateVolumeFactory_Basic(t *testing.T) {
	con := &Connect{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test",
			Namespace: "posit-team",
		},
	}
	conf := &ConnectConfig{}

	v := con.CreateVolumeFactory(conf)

	vols := v.Volumes()
	fmt.Printf("Vols: %+v\n", vols)
	assert.Len(t, vols, 1)
	checkCMVolume(t, vols[0], "config-volume", "test-connect")

	volMounts := v.VolumeMounts()
	fmt.Printf("VolMounts: %+v\n", volMounts)
	assert.Len(t, volMounts, 1)
	checkVolumeMount(t, volMounts[0], "config-volume",
		"/etc/rstudio-connect/rstudio-connect.gcfg", "rstudio-connect.gcfg", true)

	env := v.EnvVars()
	fmt.Printf("env: %+v\n", env)
	assert.Len(t, env, 0)
}

func TestCreateVolumeFactory_OffHost(t *testing.T) {
	con := &Connect{
		ObjectMeta: v1.ObjectMeta{
			Name:      "off-host",
			Namespace: "posit-team",
		},
		Spec: ConnectSpec{
			OffHostExecution: true,
		},
	}
	conf := &ConnectConfig{}

	v := con.CreateVolumeFactory(conf)

	vols := v.Volumes()
	fmt.Printf("Vols: %+v\n", vols)
	assert.Len(t, vols, 2)

	checkCMVolume(t, vols[0], "config-volume", "off-host-connect")
	checkCMVolume(t, vols[1], "template-config", "off-host-connect-templates")

	volMounts := v.VolumeMounts()
	fmt.Printf("VolMounts: %+v\n", volMounts)
	assert.Len(t, volMounts, 6)

	// config volumes
	checkVolumeMount(t, volMounts[0], "config-volume",
		"/etc/rstudio-connect/rstudio-connect.gcfg", "rstudio-connect.gcfg", true)
	checkVolumeMount(t, volMounts[1], "config-volume",
		"/etc/rstudio-connect/runtime.yaml", "runtime.yaml", true)
	checkVolumeMount(t, volMounts[2], "config-volume",
		"/etc/rstudio-connect/launcher/launcher.kubernetes.profiles.conf",
		"launcher.kubernetes.profiles.conf", true)

	// template volumes
	checkVolumeMount(t, volMounts[3], "template-config",
		"/var/lib/rstudio-connect-launcher/Kubernetes/rstudio-library-templates-data.tpl",
		"rstudio-library-templates-data.tpl", true)
	checkVolumeMount(t, volMounts[4], "template-config",
		"/var/lib/rstudio-connect-launcher/Kubernetes/job.tpl",
		"job.tpl", true)
	checkVolumeMount(t, volMounts[5], "template-config",
		"/var/lib/rstudio-connect-launcher/Kubernetes/service.tpl",
		"service.tpl", true)

	env := v.EnvVars()
	fmt.Printf("env: %+v\n", env)
	assert.Len(t, env, 0)
}

func TestCreateSecretVolumeFactory_AwsLicenseFile(t *testing.T) {
	con := &Connect{
		ObjectMeta: v1.ObjectMeta{
			Name:      "aws-license",
			Namespace: "posit-team",
		},
		Spec: ConnectSpec{
			Secret: SecretConfig{
				Type: product.SiteSecretAws,
			},
			License: product.LicenseSpec{
				Type: product.LicenseTypeFile,
			},
		},
	}
	v := con.CreateSecretVolumeFactory(&ConnectConfig{})

	vols := v.Volumes()
	fmt.Printf("Vols: %+v\n", vols)
	assert.Len(t, vols, 2)

	checkSecretVolume(t, vols[0], "key-volume", "aws-license-connect-secret-key", "secret.key", "secret.key")
	checkCsiVolume(t, vols[1], "license-volume", "secrets-store.csi.k8s.io", "secretProviderClass", "aws-license-connect-secrets", true)

	volMounts := v.VolumeMounts()
	fmt.Printf("VolMounts: %+v\n", volMounts)
	assert.Len(t, volMounts, 2)
	checkVolumeMount(t, volMounts[0], "key-volume", "/var/lib/rstudio-connect/db/secret.key", "secret.key", true)
	checkVolumeMount(t, volMounts[1], "license-volume", "/etc/rstudio-connect/license.lic", "pub.lic", true)

	env := v.EnvVars()
	fmt.Printf("env: %+v\n", env)
	assert.Len(t, env, 4)

	checkEnvVarFromSecret(t, env[0], "CONNECT_POSTGRES_PASSWORD", "aws-license-connect-db", "password")
	checkEnvVarFromSecret(t, env[1], "CONNECT_POSTGRES_INSTRUMENTATIONPASSWORD", "aws-license-connect-db", "password")
	checkEnvVarFromSecret(t, env[2], "DUMMY_SECRET_KEY", "aws-license-connect-secret-key", "secret.key")
	checkEnvVar(t, env[3], "RSC_LICENSE_FILE_PATH", "/etc/rstudio-connect/license.lic")
}

func TestCreateSecretVolumeFactory_AwsCsi(t *testing.T) {
	con := &Connect{
		ObjectMeta: v1.ObjectMeta{
			Name:      "aws-csi",
			Namespace: "posit-team",
		},
		Spec: ConnectSpec{
			Secret: SecretConfig{
				Type: product.SiteSecretAws,
			},
			License: product.LicenseSpec{
				Type: product.LicenseTypeKey,
				Key:  "test-key",
			},
		},
	}
	v := con.CreateSecretVolumeFactory(&ConnectConfig{})

	vols := v.Volumes()
	fmt.Printf("Vols: %+v\n", vols)
	assert.Len(t, vols, 2)

	checkSecretVolume(t, vols[0], "key-volume", "aws-csi-connect-secret-key", "secret.key", "secret.key")
	checkCsiVolume(t, vols[1], "secret-csi-volume", "secrets-store.csi.k8s.io", "secretProviderClass", "aws-csi-connect-secrets", true)

	volMounts := v.VolumeMounts()
	fmt.Printf("VolMounts: %+v\n", volMounts)
	assert.Len(t, volMounts, 2)
	checkVolumeMount(t, volMounts[0], "key-volume", "/var/lib/rstudio-connect/db/secret.key", "secret.key", true)
	checkVolumeMount(t, volMounts[1], "secret-csi-volume", "/mnt/dummy", "", true)

	env := v.EnvVars()
	fmt.Printf("env: %+v\n", env)
	assert.Len(t, env, 3)

	checkEnvVarFromSecret(t, env[0], "CONNECT_POSTGRES_PASSWORD", "aws-csi-connect-db", "password")
	checkEnvVarFromSecret(t, env[1], "CONNECT_POSTGRES_INSTRUMENTATIONPASSWORD", "aws-csi-connect-db", "password")
	checkEnvVarFromSecret(t, env[2], "DUMMY_SECRET_KEY", "aws-csi-connect-secret-key", "secret.key")
}

func TestCreateSecretVolumeFactory_Smtp(t *testing.T) {
	con := &Connect{
		ObjectMeta: v1.ObjectMeta{
			Name:      "aws-csi",
			Namespace: "posit-team",
		},
		Spec: ConnectSpec{
			Secret: SecretConfig{
				Type: product.SiteSecretAws,
			},
			License: product.LicenseSpec{
				Type: product.LicenseTypeKey,
				Key:  "test-key",
			},
		},
	}
	v := con.CreateSecretVolumeFactory(&ConnectConfig{
		Server: &ConnectServerConfig{
			EmailProvider: "SMTP",
		},
	})

	env := v.EnvVars()
	foundHost := false
	foundPort := false
	foundUser := false
	foundPassword := false
	for _, e := range env {
		if e.Name == "CONNECT_SMTP_HOST" {
			foundHost = true
		}
		if e.Name == "CONNECT_SMTP_PORT" {
			foundPort = true
		}
		if e.Name == "CONNECT_SMTP_USER" {
			foundUser = true
		}
		if e.Name == "CONNECT_SMTP_PASSWORD" {
			foundPassword = true
		}
	}
	assert.True(t, foundHost)
	assert.True(t, foundPort)
	assert.True(t, foundUser)
	assert.True(t, foundPassword)
}

func TestConnect_DefaultRuntimeYAML(t *testing.T) {
	r := require.New(t)

	con := &Connect{
		ObjectMeta: v1.ObjectMeta{
			Name:      "aws-csi",
			Namespace: "posit-team",
		},
		Spec: ConnectSpec{
			Secret: SecretConfig{
				Type: product.SiteSecretAws,
			},
			License: product.LicenseSpec{
				Type: product.LicenseTypeKey,
				Key:  "test-key",
			},
		},
	}

	runtimeYaml, err := con.DefaultRuntimeYAML()
	r.NoError(err)
	r.NotEqual(runtimeYaml, "")

	parsedRuntimeYaml := &product.ConnectRuntimeDefinition{}

	r.NoError(yaml.Unmarshal([]byte(runtimeYaml), parsedRuntimeYaml))

	r.Len(parsedRuntimeYaml.Images, 2)

	repo, _, ok := strings.Cut(parsedRuntimeYaml.Images[0].Name, ":")
	r.True(ok)
	r.Equal(repo, "ghcr.io/rstudio/content-pro")
}

func TestConnect_CreateSessionVolumeFactory(t *testing.T) {
	con := &Connect{
		ObjectMeta: v1.ObjectMeta{
			Name:      "aws-csi",
			Namespace: "posit-team",
		},
		Spec: ConnectSpec{
			Secret: SecretConfig{
				Type: product.SiteSecretAws,
			},
			License: product.LicenseSpec{
				Type: product.LicenseTypeKey,
				Key:  "test-key",
			},
		},
	}

	v := con.CreateSessionVolumeFactory(context.TODO())

	// should just be an init volume by default...
	vols := v.Volumes()
	assert.Len(t, vols, 1)

	checkEmptyVolume(t, vols[0], "init-volume")

	volMounts := v.VolumeMounts()
	assert.Len(t, volMounts, 1)
	checkVolumeMount(t, volMounts[0], "init-volume", "/opt/rstudio-connect/", "", false)

	env := v.EnvVars()
	assert.Len(t, env, 0)

	// and if you don't have a special secret env var
	con.Spec.SessionConfig = &product.SessionConfig{
		Pod: &product.PodConfig{
			Env: []corev1.EnvVar{
				{
					Name:  "TEST_ENV",
					Value: "secret://some-session/some-key",
				},
				{
					Name:  "ANOTHER_ENV",
					Value: "secret://site-chicken/another-key",
				},
			},
		},
	}

	v = con.CreateSessionVolumeFactory(context.TODO())

	vols = v.Volumes()
	assert.Len(t, vols, 1)

	checkEmptyVolume(t, vols[0], "init-volume")

	volMounts = v.VolumeMounts()
	assert.Len(t, volMounts, 1)
	checkVolumeMount(t, volMounts[0], "init-volume", "/opt/rstudio-connect/", "", false)

	env = v.EnvVars()
	assert.Len(t, env, 2)

	// but if you add some secret shenanigans... things get wild

	con.Spec.SessionConfig = &product.SessionConfig{
		Pod: &product.PodConfig{
			Env: []corev1.EnvVar{
				{
					Name:  "TEST_ENV",
					Value: "secret://site-session/the-key",
				},
			},
		},
	}

	v = con.CreateSessionVolumeFactory(context.TODO())

	vols = v.Volumes()
	assert.Len(t, vols, 2)

	checkEmptyVolume(t, vols[0], "init-volume")
	// csi volume is present...
	checkCsiVolume(t, vols[1], "session-csi", "secrets-store.csi.k8s.io", "secretProviderClass", "aws-csi-connect-site-session", true)

	volMounts = v.VolumeMounts()
	assert.Len(t, volMounts, 2)
	checkVolumeMount(t, volMounts[0], "init-volume", "/opt/rstudio-connect/", "", false)
	// dummy mount is present
	checkVolumeMount(t, volMounts[1], "session-csi", "/mnt/all-secrets", "", true)

	env = v.EnvVars()
	assert.Len(t, env, 1)
	checkEnvVarFromSecret(t, env[0], "TEST_ENV", "aws-csi-connect-site-session", "the-key")

	// and if you use secret shenanigans AND DSN... you don't need the dummy mount
	con.Spec.DsnSecret = "some-dsn-secret-key"

	v = con.CreateSessionVolumeFactory(context.TODO())

	vols = v.Volumes()
	assert.Len(t, vols, 2)

	checkCsiVolume(t, vols[0], "dsn-volume", "secrets-store.csi.k8s.io", "secretProviderClass", "aws-csi-connect-site-session", true)
	checkEmptyVolume(t, vols[1], "init-volume")

	volMounts = v.VolumeMounts()
	assert.Len(t, volMounts, 2)
	// no dummy mount!
	checkVolumeMount(t, volMounts[0], "dsn-volume", "/etc/odbc.ini", "odbc.ini", true)
	checkVolumeMount(t, volMounts[1], "init-volume", "/opt/rstudio-connect/", "", false)

	env = v.EnvVars()
	assert.Len(t, env, 1)
	checkEnvVarFromSecret(t, env[0], "TEST_ENV", "aws-csi-connect-site-session", "the-key")
}

func TestConnect_SiteSessionSecretProviderClass(t *testing.T) {
	con := &Connect{
		ObjectMeta: v1.ObjectMeta{
			Name:      "aws-csi",
			Namespace: "posit-team",
		},
		Spec: ConnectSpec{
			Secret: SecretConfig{
				Type: product.SiteSecretAws,
			},
			License: product.LicenseSpec{
				Type: product.LicenseTypeKey,
				Key:  "test-key",
			},
		},
	}

	// should be nil when no secret env vars defined
	spc, err := con.SiteSessionSecretProviderClass(context.TODO())
	assert.Nil(t, err)
	assert.Nil(t, spc)

	// and nil when no valid secrets in env vars

	con.Spec.SessionConfig = &product.SessionConfig{
		Pod: &product.PodConfig{
			Env: []corev1.EnvVar{
				{
					Name:  "TEST_ENV",
					Value: "secret://some-session/some-key",
				},
				{
					Name:  "ANOTHER_ENV",
					Value: "secret://site-chicken/some-key",
				},
			},
		},
	}
	spc, err = con.SiteSessionSecretProviderClass(context.TODO())
	assert.Nil(t, err)
	assert.Nil(t, spc)

	// should have a SPC defined when there is a secret env var defined
	con.Spec.SessionConfig = &product.SessionConfig{
		Pod: &product.PodConfig{
			Env: []corev1.EnvVar{
				{
					Name:  "TEST_ENV",
					Value: "secret://site-session/some-key",
				},
			},
		},
	}

	spc, err = con.SiteSessionSecretProviderClass(context.TODO())
	assert.Nil(t, err)
	assert.NotNil(t, spc)
	assert.Equal(t, v12.Provider("aws"), spc.Spec.Provider)
	assert.Contains(t, spc.Spec.Parameters["objects"], "some-key")
	assert.NotContains(t, spc.Spec.Parameters["objects"], "/some-key")

	// TODO: note that a secret like "secret://site-session//a-key" would look for "/a-key" in the secret
	//   this is currently untested behavior, but should be courtesy of TrimPrefix
}
