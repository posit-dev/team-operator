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

func TestPackageManagerConfig_GenerateGcfg_WithAdditional(t *testing.T) {
	// Test passthrough-only: Additional sets a value in a new section
	pmCfg := PackageManagerConfig{
		Additional: map[string]string{
			"NewSection.SomeKey": "some-value",
		},
	}
	str, err := pmCfg.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "[NewSection]")
	require.Contains(t, str, "SomeKey = some-value")

	// Test passthrough override: typed field + Additional for same key
	pmCfg = PackageManagerConfig{
		Server: &PackageManagerServerConfig{
			LauncherDir: "/typed/path",
		},
		Additional: map[string]string{
			"Server.LauncherDir": "/passthrough/path",
		},
	}
	str, err = pmCfg.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "LauncherDir = /passthrough/path")
	require.NotContains(t, str, "/typed/path") // passthrough wins

	// Test empty Additional map
	pmCfg = PackageManagerConfig{
		Server: &PackageManagerServerConfig{
			LauncherDir: "/test/path",
		},
		Additional: map[string]string{},
	}
	str, err = pmCfg.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "LauncherDir = /test/path")

	// Test Additional adding to existing section
	pmCfg = PackageManagerConfig{
		Server: &PackageManagerServerConfig{
			LauncherDir: "/test/path",
		},
		Additional: map[string]string{
			"Server.DataDir": "/data/dir",
		},
	}
	str, err = pmCfg.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "[Server]")
	require.Contains(t, str, "LauncherDir = /test/path")
	require.Contains(t, str, "DataDir = /data/dir")

	// Test malformed key (no ".") - should be skipped gracefully
	pmCfg = PackageManagerConfig{
		Server: &PackageManagerServerConfig{
			LauncherDir: "/test/path",
		},
		Additional: map[string]string{
			"InvalidKey": "some-value",
		},
	}
	str, err = pmCfg.GenerateGcfg()
	require.Nil(t, err)
	require.NotContains(t, str, "InvalidKey")
	require.Contains(t, str, "LauncherDir = /test/path")

	// Test passthrough with multiple values
	pmCfg = PackageManagerConfig{
		Server: &PackageManagerServerConfig{
			RVersion: []string{"/opt/R/4.2.0", "/opt/R/4.3.0"},
		},
		Additional: map[string]string{
			"Server.RVersion": "/opt/R/custom",
		},
	}
	str, err = pmCfg.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "RVersion = /opt/R/custom")
	require.NotContains(t, str, "/opt/R/4.2.0")
	require.NotContains(t, str, "/opt/R/4.3.0")
}
