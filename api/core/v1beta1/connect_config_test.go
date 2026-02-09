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

func TestConnectConfig_AdditionalPassthrough(t *testing.T) {
	// Test passthrough-only values (new section and key via Additional)
	cfg := ConnectConfig{
		Additional: map[string]string{
			"NewSection.NewKey":        "custom-value",
			"Server.CustomServerField": "server-custom",
			"Database.Timeout":         "30",
		},
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)
	t.Logf("Generated gcfg with passthrough-only:\n%s", str)

	// Check that passthrough values are present
	require.Contains(t, str, "[NewSection]")
	require.Contains(t, str, "NewKey = custom-value")
	require.Contains(t, str, "[Server]")
	require.Contains(t, str, "CustomServerField = server-custom")
	require.Contains(t, str, "[Database]")
	require.Contains(t, str, "Timeout = 30")
}

func TestConnectConfig_AdditionalOverride(t *testing.T) {
	// Test override behavior (typed field + same key in Additional, passthrough wins)
	cfg := ConnectConfig{
		Server: &ConnectServerConfig{
			Address:                "typed-address.com",
			HideEmailAddresses:     false,
			DefaultContentListView: ContentListViewExpanded,
		},
		Additional: map[string]string{
			"Server.Address":                "passthrough-address.com", // Should override typed
			"Server.HideEmailAddresses":     "true",                    // Should override typed
			"Server.DefaultContentListView": "card",                    // Should override typed
			"Server.CustomField":            "custom-value",            // New field
		},
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)
	t.Logf("Generated gcfg with overrides:\n%s", str)

	// Check that passthrough values override typed values
	require.Contains(t, str, "[Server]")
	require.Contains(t, str, "Address = passthrough-address.com")
	require.Contains(t, str, "HideEmailAddresses = true")
	require.Contains(t, str, "DefaultContentListView = card")
	require.Contains(t, str, "CustomField = custom-value")

	// Original typed values should not be present
	require.NotContains(t, str, "Address = typed-address.com")
	require.NotContains(t, str, "HideEmailAddresses = false")
	require.NotContains(t, str, "DefaultContentListView = expanded")
}

func TestConnectConfig_AdditionalEmpty(t *testing.T) {
	// Test empty Additional map (no effect on output)
	cfg := ConnectConfig{
		Server: &ConnectServerConfig{
			Address: "some-address.com",
		},
		Additional: map[string]string{}, // Empty map
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)
	t.Logf("Generated gcfg with empty Additional:\n%s", str)

	// Should contain normal typed fields
	require.Contains(t, str, "[Server]")
	require.Contains(t, str, "Address = some-address.com")

	// Should not have any extra sections or fields
	require.Equal(t, 1, countOccurrences(str, "[Server]"))
}

func TestConnectConfig_AdditionalNil(t *testing.T) {
	// Test nil Additional map (no effect on output)
	cfg := ConnectConfig{
		Server: &ConnectServerConfig{
			Address: "some-address.com",
		},
		Additional: nil, // Nil map
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)

	// Should contain normal typed fields
	require.Contains(t, str, "[Server]")
	require.Contains(t, str, "Address = some-address.com")
}

func TestConnectConfig_AdditionalMalformedKey(t *testing.T) {
	// Test malformed key in Additional (no "." separator — should be skipped)
	cfg := ConnectConfig{
		Additional: map[string]string{
			"MalformedKey":     "should-be-skipped", // No section separator
			"Server.ValidKey":  "should-be-included",
			"AnotherBadKey":    "also-skipped",
			"Good.Section.Key": "multi-dot-ok", // Multiple dots are OK (first is section separator)
		},
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)
	t.Logf("Generated gcfg with malformed keys:\n%s", str)

	// Valid keys should be present
	require.Contains(t, str, "[Server]")
	require.Contains(t, str, "ValidKey = should-be-included")
	require.Contains(t, str, "[Good]")
	require.Contains(t, str, "Section.Key = multi-dot-ok")

	// Malformed keys should be skipped
	require.NotContains(t, str, "MalformedKey")
	require.NotContains(t, str, "should-be-skipped")
	require.NotContains(t, str, "AnotherBadKey")
	require.NotContains(t, str, "also-skipped")
}

func TestConnectConfig_AdditionalComplexScenario(t *testing.T) {
	// Test complex scenario with multiple sections, overrides, and new fields
	cfg := ConnectConfig{
		Server: &ConnectServerConfig{
			Address: "original.com",
		},
		Http: &ConnectHttpConfig{
			Listen: ":3939",
		},
		Applications: &ConnectApplicationsConfig{
			ScheduleConcurrency: 2,
		},
		Additional: map[string]string{
			// Override existing fields
			"Server.Address":                   "override.com",
			"Http.Listen":                      ":8080",
			"Applications.ScheduleConcurrency": "10",

			// Add new fields to existing sections
			"Server.NewServerField": "server-new",
			"Http.Timeout":          "60",

			// Add entirely new sections
			"CustomSection.Field1":   "value1",
			"CustomSection.Field2":   "value2",
			"AnotherSection.Setting": "some-setting",
		},
	}
	str, err := cfg.GenerateGcfg()
	require.Nil(t, err)
	t.Logf("Generated complex gcfg:\n%s", str)

	// Check overrides
	require.Contains(t, str, "Address = override.com")
	require.Contains(t, str, "Listen = :8080")
	require.Contains(t, str, "ScheduleConcurrency = 10")

	// Check new fields in existing sections
	require.Contains(t, str, "NewServerField = server-new")
	require.Contains(t, str, "Timeout = 60")

	// Check new sections
	require.Contains(t, str, "[CustomSection]")
	require.Contains(t, str, "Field1 = value1")
	require.Contains(t, str, "Field2 = value2")
	require.Contains(t, str, "[AnotherSection]")
	require.Contains(t, str, "Setting = some-setting")

	// Original values should not be present (they were overridden)
	require.NotContains(t, str, "Address = original.com")
	require.NotContains(t, str, "Listen = :3939")
	require.NotContains(t, str, "ScheduleConcurrency = 2")
}

// Helper function to count occurrences of a substring
func countOccurrences(str, substr string) int {
	count := 0
	for i := 0; i+len(substr) <= len(str); i++ {
		if str[i:i+len(substr)] == substr {
			count++
		}
	}
	return count
}
