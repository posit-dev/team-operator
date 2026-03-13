package core

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
)

func TestPPMAuthTokenExchangeScript(t *testing.T) {
	script := PPMAuthTokenExchangeScript()
	require.Contains(t, script, "exchange_token")
	require.Contains(t, script, "wget")
	require.Contains(t, script, "grant_type=urn:ietf:params:oauth:grant-type:token-exchange")
	require.Contains(t, script, "sidecar")
	require.Contains(t, script, "netrc")
	// Verify curlrc is written for R support
	require.Contains(t, script, "CURLRC_PATH")
	require.Contains(t, script, "--netrc-file")
	require.Contains(t, script, ".curlrc")
	// Verify null token validation
	require.Contains(t, script, `[ "$PPM_TOKEN" = "null" ]`)
	// Verify sidecar resilience with failure counter
	require.Contains(t, script, "FAIL_COUNT")
	require.Contains(t, script, "MAX_FAILURES")
	require.Contains(t, script, "WARNING: token refresh failed")
	require.Contains(t, script, "ERROR: token refresh failed")
	// Verify extract_json_field helper is present
	require.Contains(t, script, "extract_json_field")
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
	// Verify liveness probe is set to recover from consecutive failures
	require.NotNil(t, c.LivenessProbe, "Sidecar should have a liveness probe")
	require.NotNil(t, c.LivenessProbe.Exec, "Liveness probe should use exec")
	require.Contains(t, c.LivenessProbe.Exec.Command[2], ppmAuthNetrcPath, "Liveness probe should check netrc file")
}

func TestPPMAuthSidecarContainerCustomRefresh(t *testing.T) {
	c := PPMAuthSidecarContainer("", "https://ppm.example.com", "1800")
	require.Equal(t, "1800", c.Env[2].Value)
}

func TestPPMAuthVolumes(t *testing.T) {
	vols := PPMAuthVolumes("mysite", "sts.amazonaws.com")
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

func TestPPMAuthVolumesEmptyAudience(t *testing.T) {
	vols := PPMAuthVolumes("mysite", "")
	require.Len(t, vols, 3)

	// When audience is empty, the ServiceAccountToken projection has an empty audience,
	// which defaults to the API server audience — not the intended OIDC provider.
	// SetupPPMAuth guards against this; this test documents the raw behavior.
	require.Equal(t, "ppm-sa-token", vols[0].Name)
	require.NotNil(t, vols[0].Projected)
	require.Len(t, vols[0].Projected.Sources, 1)
	require.Equal(t, "", vols[0].Projected.Sources[0].ServiceAccountToken.Audience)
}

func TestSetupPPMAuthDisabled(t *testing.T) {
	sink := &testLogSink{}
	setup := SetupPPMAuth(false, "https://ppm.example.com", "", "sts.amazonaws.com", "mysite", logr.New(sink))
	require.Empty(t, setup.Volumes)
	require.Empty(t, setup.InitContainers)
}

func TestSetupPPMAuthEmptyURL(t *testing.T) {
	sink := &testLogSink{}
	setup := SetupPPMAuth(true, "", "", "sts.amazonaws.com", "mysite", logr.New(sink))
	require.Empty(t, setup.Volumes)
	require.Empty(t, setup.InitContainers)
	require.Contains(t, sink.messages, "AuthenticatedRepos is enabled but PPMUrl is empty; skipping PPM auth setup")
}

func TestSetupPPMAuthEmptyAudience(t *testing.T) {
	sink := &testLogSink{}
	setup := SetupPPMAuth(true, "https://ppm.example.com", "", "", "mysite", logr.New(sink))
	require.Empty(t, setup.Volumes)
	require.Empty(t, setup.InitContainers)
	require.Contains(t, sink.messages, "AuthenticatedRepos is enabled but PPMAuthAudience is empty; skipping PPM auth setup (projected SA token requires an audience)")
}

func TestSetupPPMAuthFullSetup(t *testing.T) {
	sink := &testLogSink{}
	setup := SetupPPMAuth(true, "https://ppm.example.com", "", "sts.amazonaws.com", "mysite", logr.New(sink))
	require.Len(t, setup.Volumes, 3)
	require.Len(t, setup.VolumeMounts, 1)
	require.Len(t, setup.EnvVars, 2)
	require.Len(t, setup.InitContainers, 1)
	require.Len(t, setup.SidecarContainers, 1)
	// Default image when ppmAuthImage is empty
	require.Equal(t, "alpine:3", setup.InitContainers[0].Image)
	require.Equal(t, "alpine:3", setup.SidecarContainers[0].Image)
}

func TestSetupPPMAuthCustomImage(t *testing.T) {
	sink := &testLogSink{}
	setup := SetupPPMAuth(true, "https://ppm.example.com", "registry.example.com/ppm-auth:v2", "sts.amazonaws.com", "mysite", logr.New(sink))
	require.Len(t, setup.InitContainers, 1)
	require.Len(t, setup.SidecarContainers, 1)
	require.Equal(t, "registry.example.com/ppm-auth:v2", setup.InitContainers[0].Image)
	require.Equal(t, "registry.example.com/ppm-auth:v2", setup.SidecarContainers[0].Image)
}

// testLogSink is a minimal logr.LogSink for testing SetupPPMAuth
type testLogSink struct {
	messages []string
}

func (l *testLogSink) Init(logr.RuntimeInfo)                  {}
func (l *testLogSink) Enabled(int) bool                       { return true }
func (l *testLogSink) Error(error, string, ...interface{})    {}
func (l *testLogSink) WithValues(...interface{}) logr.LogSink { return l }
func (l *testLogSink) WithName(string) logr.LogSink           { return l }
func (l *testLogSink) Info(_ int, msg string, _ ...interface{}) {
	l.messages = append(l.messages, msg)
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

func TestContainsGcfgKey(t *testing.T) {
	config := "[OpenIDConnect]\nScope = repos:read:*\nRoleClaim = roles\n"
	require.True(t, containsGcfgKey(config, "OpenIDConnect", "Scope"))
	require.True(t, containsGcfgKey(config, "OpenIDConnect", "RoleClaim"))
	require.False(t, containsGcfgKey(config, "OpenIDConnect", "GroupToScopeMapping"))
	require.False(t, containsGcfgKey(config, "OtherSection", "Scope"))
}

func TestContainsGcfgKeySkipsComments(t *testing.T) {
	config := "[OpenIDConnect]\n; Scope = repos:read:*\n# RoleClaim = roles\n"
	require.False(t, containsGcfgKey(config, "OpenIDConnect", "Scope"), "commented-out key with ; should be ignored")
	require.False(t, containsGcfgKey(config, "OpenIDConnect", "RoleClaim"), "commented-out key with # should be ignored")
}

func TestPpmAuthLivenessThresholdSeconds(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"valid interval", "1500", 3000},
		{"default interval", "3000", 6000},
		{"empty string", "", 6000},
		{"zero", "0", 6000},
		{"negative", "-1", 6000},
		{"unit suffix", "30s", 6000},
		{"non-numeric", "abc", 6000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, ppmAuthLivenessThresholdSeconds(tt.input))
		})
	}
}

// TestPPMAuthTokenExchangeScriptJWTValidation is a source-level smoke check
// that the JWT dot-count validation hasn't been accidentally removed from the
// shell script. It does not execute the script; behavioral coverage comes from
// integration/e2e tests.
func TestPPMAuthTokenExchangeScriptJWTValidation(t *testing.T) {
	script := PPMAuthTokenExchangeScript()
	require.Contains(t, script, "DOT_COUNT")
	require.Contains(t, script, "does not look like a JWT")
}

func TestSanitizePPMUrl(t *testing.T) {
	require.Equal(t, "https://ppm.example.com", SanitizePPMUrl("ppm.example.com"))
	require.Equal(t, "https://ppm.example.com", SanitizePPMUrl("https://ppm.example.com"))
	require.Equal(t, "https://ppm.example.com", SanitizePPMUrl("http://ppm.example.com"))
	require.Equal(t, "", SanitizePPMUrl(""))
	require.Equal(t, "https://ppm.example.com:8080/api", SanitizePPMUrl("ppm.example.com:8080/api"))
	// Trailing slashes are stripped to avoid double-slash in URLs like //__api__/token
	require.Equal(t, "https://ppm.example.com", SanitizePPMUrl("https://ppm.example.com/"))
	require.Equal(t, "https://ppm.example.com", SanitizePPMUrl("https://ppm.example.com///"))
	require.Equal(t, "", SanitizePPMUrl("https:///"))
}
