package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPPMAuthTokenExchangeScript(t *testing.T) {
	script := PPMAuthTokenExchangeScript()
	require.Contains(t, script, "exchange_token")
	require.Contains(t, script, "curl")
	require.Contains(t, script, "grant_type=urn:ietf:params:oauth:grant-type:token-exchange")
	require.Contains(t, script, "sidecar")
	require.Contains(t, script, "netrc")
	// Verify curlrc is written for R support
	require.Contains(t, script, "CURLRC_PATH")
	require.Contains(t, script, "--netrc-file")
	require.Contains(t, script, ".curlrc")
	// Verify null token validation
	require.Contains(t, script, `[ "$PPM_TOKEN" = "null" ]`)
	// Verify sidecar resilience
	require.Contains(t, script, "WARNING: token refresh failed, will retry")
}

func TestPPMAuthConfigMapName(t *testing.T) {
	name := PPMAuthConfigMapName("mysite")
	require.Equal(t, "mysite-ppm-auth-script", name)
}

func TestPPMAuthInitContainer(t *testing.T) {
	c := PPMAuthInitContainer("", "https://packagemanager.example.com")
	require.Equal(t, "ppm-auth-init", c.Name)
	require.Equal(t, "alpine:3", c.Image)
	require.Len(t, c.Env, 2)
	require.Equal(t, "PPM_URL", c.Env[0].Name)
	require.Equal(t, "https://packagemanager.example.com", c.Env[0].Value)
	require.Equal(t, "MODE", c.Env[1].Name)
	require.Equal(t, "init", c.Env[1].Value)
	require.Len(t, c.VolumeMounts, 3)
	// Verify non-root security context (alpine:3 runs as root by default)
	require.NotNil(t, c.SecurityContext)
	require.NotNil(t, c.SecurityContext.RunAsUser)
	require.Equal(t, int64(65534), *c.SecurityContext.RunAsUser)
	require.True(t, *c.SecurityContext.RunAsNonRoot)
}

func TestPPMAuthInitContainerCustomImage(t *testing.T) {
	c := PPMAuthInitContainer("custom-image:v1", "https://ppm.example.com")
	require.Equal(t, "custom-image:v1", c.Image)
}

func TestPPMAuthSidecarContainer(t *testing.T) {
	c := PPMAuthSidecarContainer("", "https://packagemanager.example.com", "")
	require.Equal(t, "ppm-auth-sidecar", c.Name)
	require.Equal(t, "alpine:3", c.Image)
	require.Len(t, c.Env, 3)
	require.Equal(t, "MODE", c.Env[1].Name)
	require.Equal(t, "sidecar", c.Env[1].Value)
	require.Equal(t, "REFRESH_INTERVAL", c.Env[2].Name)
	require.Equal(t, "3000", c.Env[2].Value)
	// Verify non-root security context (alpine:3 runs as root by default)
	require.NotNil(t, c.SecurityContext)
	require.NotNil(t, c.SecurityContext.RunAsUser)
	require.Equal(t, int64(65534), *c.SecurityContext.RunAsUser)
	require.True(t, *c.SecurityContext.RunAsNonRoot)
}

func TestPPMAuthSidecarContainerCustomRefresh(t *testing.T) {
	c := PPMAuthSidecarContainer("", "https://ppm.example.com", "1800")
	require.Equal(t, "1800", c.Env[2].Value)
}

func TestPPMAuthVolumes(t *testing.T) {
	vols := PPMAuthVolumes("mysite", "https://packagemanager.example.com", "sts.amazonaws.com")
	require.Len(t, vols, 3)

	// Projected SA token volume
	require.Equal(t, "ppm-sa-token", vols[0].Name)
	require.NotNil(t, vols[0].Projected)
	require.Len(t, vols[0].Projected.Sources, 1)
	require.Equal(t, "sts.amazonaws.com", vols[0].Projected.Sources[0].ServiceAccountToken.Audience)

	// Shared emptyDir
	require.Equal(t, "ppm-auth", vols[1].Name)
	require.NotNil(t, vols[1].EmptyDir)

	// Script ConfigMap
	require.Equal(t, "ppm-auth-script", vols[2].Name)
	require.NotNil(t, vols[2].ConfigMap)
	require.Equal(t, "mysite-ppm-auth-script", vols[2].ConfigMap.Name)
}

func TestPPMAuthVolumeMounts(t *testing.T) {
	mounts := PPMAuthVolumeMounts()
	require.Len(t, mounts, 1)
	require.Equal(t, "ppm-auth", mounts[0].Name)
	require.Equal(t, "/mnt/ppm-auth", mounts[0].MountPath)
	require.True(t, mounts[0].ReadOnly)
}

func TestPPMAuthEnvVars(t *testing.T) {
	envs := PPMAuthEnvVars()
	require.Len(t, envs, 2)
	require.Equal(t, "NETRC", envs[0].Name)
	require.Equal(t, "/mnt/ppm-auth/netrc", envs[0].Value)
	require.Equal(t, "CURL_HOME", envs[1].Name)
	require.Equal(t, "/mnt/ppm-auth", envs[1].Value)
}

func TestSanitizePPMUrl(t *testing.T) {
	require.Equal(t, "https://ppm.example.com", SanitizePPMUrl("ppm.example.com"))
	require.Equal(t, "https://ppm.example.com", SanitizePPMUrl("https://ppm.example.com"))
	require.Equal(t, "https://ppm.example.com", SanitizePPMUrl("http://ppm.example.com"))
}
