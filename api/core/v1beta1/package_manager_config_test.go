package v1beta1

import (
	"encoding/json"
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

func TestPackageManagerConfig_Authentication(t *testing.T) {
	cfg := PackageManagerConfig{
		Authentication: &PackageManagerAuthenticationConfig{
			APITokenAuth:          true,
			DeviceAuthType:        "oidc",
			NewReposAuthByDefault: true,
			Lifetime:              "30d",
			Inactivity:            "12h",
			CookieSweepDuration:   "5m",
		},
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "[Authentication]")
	require.Contains(t, str, "APITokenAuth = true")
	require.Contains(t, str, "DeviceAuthType = oidc")
	require.Contains(t, str, "NewReposAuthByDefault = true")
	require.Contains(t, str, "Lifetime = 30d")
	require.Contains(t, str, "Inactivity = 12h")
	require.Contains(t, str, "CookieSweepDuration = 5m")
}

func TestPackageManagerConfig_OIDCNewFields(t *testing.T) {
	cfg := PackageManagerConfig{
		OpenIDConnect: &PackageManagerOIDCConfig{
			ClientId:             "my-client",
			ClientSecretFile:     "/etc/rstudio-pm/oidc-secret",
			Issuer:               "https://auth.example.com",
			Logging:              true,
			TokenLifetime:        "1h",
			DisablePKCE:          true,
			UniqueIdClaim:        "sub",
			UsernameClaim:        "preferred_username",
			MaxAuthenticationAge: "24h",
			GroupsSeparator:      ",",
			RolesSeparator:       ";",
			CustomScope:          "profile email groups",
			NoAutoGroupsScope:    true,
			EnableDevicePKCE:     true,
		},
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "[OpenIDConnect]")
	require.Contains(t, str, "ClientId = my-client")
	require.Contains(t, str, "ClientSecretFile = /etc/rstudio-pm/oidc-secret")
	require.Contains(t, str, "Issuer = https://auth.example.com")
	require.Contains(t, str, "Logging = true")
	require.Contains(t, str, "TokenLifetime = 1h")
	require.Contains(t, str, "DisablePKCE = true")
	require.Contains(t, str, "UniqueIdClaim = sub")
	require.Contains(t, str, "UsernameClaim = preferred_username")
	require.Contains(t, str, "MaxAuthenticationAge = 24h")
	require.Contains(t, str, "GroupsSeparator = ,")
	require.Contains(t, str, "RolesSeparator = ;")
	require.Contains(t, str, "CustomScope = profile email groups")
	require.Contains(t, str, "NoAutoGroupsScope = true")
	require.Contains(t, str, "EnableDevicePKCE = true")
}

func TestPackageManagerConfig_IdentityFederationJSONRoundTrip(t *testing.T) {
	original := PackageManagerConfig{
		IdentityFederation: []PackageManagerIdentityFederationConfig{
			{
				Name:     "connect",
				Issuer:   "https://issuer.example.com",
				Audience: "my-audience",
				Subject:  "system:serviceaccount:posit-team:mysite-connect",
				Scope:    "repos:read:*",
			},
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var roundTripped PackageManagerConfig
	err = json.Unmarshal(data, &roundTripped)
	require.NoError(t, err)

	require.Len(t, roundTripped.IdentityFederation, 1)
	require.Equal(t, "connect", roundTripped.IdentityFederation[0].Name)
	require.Equal(t, "https://issuer.example.com", roundTripped.IdentityFederation[0].Issuer)
	require.Equal(t, "my-audience", roundTripped.IdentityFederation[0].Audience)
	require.Equal(t, "system:serviceaccount:posit-team:mysite-connect", roundTripped.IdentityFederation[0].Subject)
	require.Equal(t, "repos:read:*", roundTripped.IdentityFederation[0].Scope)
}

func TestPackageManagerConfig_IdentityFederationNewFields(t *testing.T) {
	cfg := PackageManagerConfig{
		IdentityFederation: []PackageManagerIdentityFederationConfig{
			{
				Name:              "my-idp",
				Issuer:            "https://issuer.example.com",
				Logging:           true,
				Audience:          "my-audience",
				CustomScope:       "read write",
				NoAutoGroupsScope: true,
				GroupsClaim:       "groups",
				GroupsSeparator:   ",",
				RoleClaim:         "roles",
				RolesSeparator:    ";",
				UniqueIdClaim:     "sub",
				UsernameClaim:     "preferred_username",
				TokenLifetime:     "2h",
			},
		},
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, `[IdentityFederation "my-idp"]`)
	require.Contains(t, str, "Issuer = https://issuer.example.com")
	require.Contains(t, str, "Logging = true")
	require.Contains(t, str, "Audience = my-audience")
	require.Contains(t, str, "CustomScope = read write")
	require.Contains(t, str, "NoAutoGroupsScope = true")
	require.Contains(t, str, "GroupsClaim = groups")
	require.Contains(t, str, "GroupsSeparator = ,")
	require.Contains(t, str, "RoleClaim = roles")
	require.Contains(t, str, "RolesSeparator = ;")
	require.Contains(t, str, "UniqueIdClaim = sub")
	require.Contains(t, str, "UsernameClaim = preferred_username")
	require.Contains(t, str, "TokenLifetime = 2h")
}

func TestPackageManagerConfig_IdentityFederationRejectsCarriageReturn(t *testing.T) {
	cfg := PackageManagerConfig{
		IdentityFederation: []PackageManagerIdentityFederationConfig{
			{
				Name:   "bad\rname",
				Issuer: "https://issuer.example.com",
			},
		},
	}
	_, err := cfg.GenerateGcfg()
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not contain")
}
