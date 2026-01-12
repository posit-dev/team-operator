package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestConnectConfig_GenerateGcfg(t *testing.T) {
	minimal := ConnectConfig{
		Server: &ConnectServerConfig{
			Address: "some-address.com",
		},
	}

	str, err := minimal.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "Address = some-address.com")

	cfg := ConnectConfig{
		Server: &ConnectServerConfig{
			Address: "some-address.com",
		},
		Http: &ConnectHttpConfig{
			ForceSecure: true,
			Listen:      ":3939",
		},
	}
	str, err = cfg.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "Listen = :3939")
	require.Contains(t, str, "ForceSecure = true")
}

func TestConnectConfig_GenerateGcfgRepositories(t *testing.T) {
	c := ConnectConfig{
		RPackageRepository: map[string]RPackageRepositoryConfig{
			"CRAN": {
				Url: "https://p3m.dev/cran/latest",
			},
		},
	}

	str, err := c.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "[RPackageRepository \"CRAN\"]")
	require.Contains(t, str, "Url = https://p3m.dev/cran/latest")
	require.NotContains(t, str, "["+RPackageRepositoryMapKey+"]")

	cBlank := ConnectConfig{}
	str, err = cBlank.GenerateGcfg()
	require.Nil(t, err)
	require.NotContains(t, str, "RPackageRepository")
	require.NotContains(t, str, RPackageRepositoryMapKey)
	require.NotContains(t, str, "["+RPackageRepositoryMapKey+"]")
}

func TestConnectConfig_ScheduleConcurrency(t *testing.T) {
	cfg := ConnectConfig{
		Applications: &ConnectApplicationsConfig{
			ScheduleConcurrency: 5,
		},
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "[Applications]")
	require.Contains(t, str, "ScheduleConcurrency = 5")

	// Test with explicit zero value (disables scheduled concurrency)
	cfgNoSchedule := ConnectConfig{
		Applications: &ConnectApplicationsConfig{
			ScheduleConcurrency: 0,
		},
	}
	str, err = cfgNoSchedule.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "[Applications]")
	require.Contains(t, str, "ScheduleConcurrency = 0")

	// Test that nil Applications section generates no config
	cfgDefault := ConnectConfig{}
	str, err = cfgDefault.GenerateGcfg()
	require.Nil(t, err)
	require.NotContains(t, str, "[Applications]")
	require.NotContains(t, str, "ScheduleConcurrency")
}

func TestConnectConfig_RoleMappings(t *testing.T) {
	cfg := ConnectConfig{
		Authorization: &ConnectAuthorizationConfig{
			UserRoleGroupMapping:     true,
			ViewerRoleMapping:        []string{"viewers-group", "read-only-users"},
			PublisherRoleMapping:     []string{"publishers-group"},
			AdministratorRoleMapping: []string{"admins-group", "super-admins"},
		},
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)
	t.Logf("Generated gcfg:\n%s", str)
	require.Contains(t, str, "[Authorization]")
	require.Contains(t, str, "UserRoleGroupMapping = true")
	require.Contains(t, str, "ViewerRoleMapping = viewers-group")
	require.Contains(t, str, "ViewerRoleMapping = read-only-users")
	require.Contains(t, str, "PublisherRoleMapping = publishers-group")
	require.Contains(t, str, "AdministratorRoleMapping = admins-group")
	require.Contains(t, str, "AdministratorRoleMapping = super-admins")

	// Test with empty mappings
	cfgEmpty := ConnectConfig{
		Authorization: &ConnectAuthorizationConfig{
			DefaultUserRole: ConnectPublisherRole,
		},
	}
	str, err = cfgEmpty.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "[Authorization]")
	require.Contains(t, str, "DefaultUserRole = publisher")
	require.NotContains(t, str, "ViewerRoleMapping")
	require.NotContains(t, str, "PublisherRoleMapping")
	require.NotContains(t, str, "AdministratorRoleMapping")
}

func TestConnectConfig_GroupsClaim(t *testing.T) {
	// Test with GroupsClaim set
	cfg := ConnectConfig{
		OAuth2: &ConnectOAuth2Config{
			ClientId:            "test-client",
			OpenIDConnectIssuer: "https://example.com",
			GroupsAutoProvision: true,
			GroupsClaim:         ptr.To("groups"),
		},
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)
	t.Logf("Generated gcfg:\n%s", str)
	require.Contains(t, str, "[OAuth2]")
	require.Contains(t, str, "ClientId = test-client")
	require.Contains(t, str, "OpenIDConnectIssuer = https://example.com")
	require.Contains(t, str, "GroupsAutoProvision = true")
	require.Contains(t, str, "GroupsClaim = groups")

	// Test with no GroupsClaim (nil pointer)
	cfgEmpty := ConnectConfig{
		OAuth2: &ConnectOAuth2Config{
			ClientId:            "test-client",
			OpenIDConnectIssuer: "https://example.com",
			GroupsAutoProvision: true,
			GroupsClaim:         nil, // Not set
		},
	}
	str, err = cfgEmpty.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "[OAuth2]")
	require.Contains(t, str, "ClientId = test-client")
	require.Contains(t, str, "OpenIDConnectIssuer = https://example.com")
	require.Contains(t, str, "GroupsAutoProvision = true")
	require.NotContains(t, str, "GroupsClaim")

	// Test with explicitly empty GroupsClaim (empty string pointer)
	cfgExplicitEmpty := ConnectConfig{
		OAuth2: &ConnectOAuth2Config{
			ClientId:            "test-client",
			OpenIDConnectIssuer: "https://example.com",
			GroupsAutoProvision: true,
			GroupsClaim:         ptr.To(""), // Explicitly set to empty
		},
	}
	str, err = cfgExplicitEmpty.GenerateGcfg()
	require.Nil(t, err)
	t.Logf("Generated gcfg with explicit empty:\n%s", str)
	require.Contains(t, str, "[OAuth2]")
	require.Contains(t, str, "ClientId = test-client")
	require.Contains(t, str, "OpenIDConnectIssuer = https://example.com")
	require.Contains(t, str, "GroupsAutoProvision = true")
	require.Contains(t, str, "GroupsClaim = ", "Explicitly empty GroupsClaim should be written to config")
}

func TestConnectConfig_CustomScope(t *testing.T) {
	// Test with CustomScope
	cfg := ConnectConfig{
		OAuth2: &ConnectOAuth2Config{
			ClientId:            "test-client",
			OpenIDConnectIssuer: "https://example.com",
			GroupsAutoProvision: true,
			CustomScope:         []string{"openid", "email", "profile"},
		},
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)
	t.Logf("Generated gcfg with CustomScope:\n%s", str)
	require.Contains(t, str, "[OAuth2]")
	require.Contains(t, str, "ClientId = test-client")
	require.Contains(t, str, "OpenIDConnectIssuer = https://example.com")
	require.Contains(t, str, "CustomScope = openid")
	require.Contains(t, str, "CustomScope = email")
	require.Contains(t, str, "CustomScope = profile")

	// Test with no CustomScope
	cfgNoScope := ConnectConfig{
		OAuth2: &ConnectOAuth2Config{
			ClientId:            "test-client",
			OpenIDConnectIssuer: "https://example.com",
		},
	}
	str, err = cfgNoScope.GenerateGcfg()
	require.Nil(t, err)
	require.Contains(t, str, "[OAuth2]")
	require.Contains(t, str, "ClientId = test-client")
	require.Contains(t, str, "OpenIDConnectIssuer = https://example.com")
	require.NotContains(t, str, "CustomScope")
}
