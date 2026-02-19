package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPackageManagerConfig_GenerateGcfg(t *testing.T) {
	minimal := PackageManagerConfig{
		Server: &PackageManagerServerConfig{
			LauncherDir: "/another/friend",
		},
	}
	str, err := minimal.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "LauncherDir = /another/friend")

	pmCfg := PackageManagerConfig{
		Server: &PackageManagerServerConfig{
			LauncherDir: "/test/friend",
			RVersion:    []string{"/some/path", "/another/path"},
		},
		Git: &PackageManagerGitConfig{
			AllowUnsandboxedGitBuilds: true,
		},
		Http: &PackageManagerHttpConfig{
			Listen: ":4242",
		},
	}
	str, err = pmCfg.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "/test/friend")
	require.Contains(t, str, "/some/path")
	require.Contains(t, str, "/another/path")
}

func TestPackageManagerConfig_AdditionalConfig(t *testing.T) {
	// Test basic string append
	cfg := PackageManagerConfig{
		Server: &PackageManagerServerConfig{
			Address: "some-address.com",
		},
		AdditionalConfig: "\n[NewSection]\nNewKey = custom-value\n",
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "[Server]")
	require.Contains(t, str, "Address = some-address.com")
	require.Contains(t, str, "[NewSection]")
	require.Contains(t, str, "NewKey = custom-value")
}

func TestPackageManagerConfig_AdditionalConfigEmpty(t *testing.T) {
	cfg := PackageManagerConfig{
		Server: &PackageManagerServerConfig{
			Address: "some-address.com",
		},
		AdditionalConfig: "",
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "Address = some-address.com")
}
