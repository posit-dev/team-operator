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

func TestPackageManagerConfig_OpenIDConnect(t *testing.T) {
	cfg := PackageManagerConfig{
		OpenIDConnect: &PackageManagerOIDCConfig{
			ClientId:     "my-client-id",
			ClientSecret: "/etc/rstudio-pm/oidc-client-secret",
			Issuer:       "https://login.example.com",
			RequireLogin: true,
		},
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "[OpenIDConnect]")
	require.Contains(t, str, "ClientId = my-client-id")
	require.Contains(t, str, "ClientSecret = /etc/rstudio-pm/oidc-client-secret")
	require.Contains(t, str, "Issuer = https://login.example.com")
	require.Contains(t, str, "RequireLogin = true")
}

func TestPackageManagerConfig_IdentityFederation(t *testing.T) {
	cfg := PackageManagerConfig{
		IdentityFederation: []PackageManagerIdentityFederationConfig{
			{
				Name:     "connect",
				Issuer:   "https://oidc.eks.us-east-1.amazonaws.com/id/EXAMPLE",
				Audience: "sts.amazonaws.com",
				Subject:  "system:serviceaccount:posit-team:mysite-connect",
				Scope:    "repos:read:*",
			},
			{
				Name:     "workbench",
				Issuer:   "https://oidc.eks.us-east-1.amazonaws.com/id/EXAMPLE",
				Audience: "sts.amazonaws.com",
				Subject:  "system:serviceaccount:posit-team:mysite-workbench",
				Scope:    "repos:read:*",
			},
		},
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, `[IdentityFederation "connect"]`)
	require.Contains(t, str, `[IdentityFederation "workbench"]`)
	require.Contains(t, str, "Issuer = https://oidc.eks.us-east-1.amazonaws.com/id/EXAMPLE")
	require.Contains(t, str, "Audience = sts.amazonaws.com")
	require.Contains(t, str, "Subject = system:serviceaccount:posit-team:mysite-connect")
	require.Contains(t, str, "Scope = repos:read:*")
}

func TestPackageManagerConfig_OpenIDConnectAndIdentityFederation(t *testing.T) {
	cfg := PackageManagerConfig{
		Server: &PackageManagerServerConfig{
			Address: "https://packagemanager.example.com",
		},
		OpenIDConnect: &PackageManagerOIDCConfig{
			ClientId:     "ppm-client",
			ClientSecret: "/etc/rstudio-pm/oidc-client-secret",
			Issuer:       "https://login.example.com",
			RequireLogin: true,
		},
		IdentityFederation: []PackageManagerIdentityFederationConfig{
			{
				Name:          "connect",
				Issuer:        "https://oidc.eks.us-east-1.amazonaws.com/id/EXAMPLE",
				Audience:      "sts.amazonaws.com",
				Subject:       "system:serviceaccount:posit-team:.*-connect",
				Scope:         "repos:read:*",
				TokenLifetime: "1h",
			},
		},
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "[Server]")
	require.Contains(t, str, "[OpenIDConnect]")
	require.Contains(t, str, `[IdentityFederation "connect"]`)
	require.Contains(t, str, "TokenLifetime = 1h")
}
