package core

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/posit-dev/team-operator/api/localtest"
	"github.com/stretchr/testify/require"
)

// siteTypesMarker returns the first capture group of re applied to api/core/v1beta1/site_types.go, so tests can
// compare the kubebuilder markers (the source of the CRD schema) with the controller's Go logic.
func siteTypesMarker(t *testing.T, re string) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(localtest.RootDir, "api", "core", "v1beta1", "site_types.go"))
	require.NoError(t, err)
	m := regexp.MustCompile(re).FindSubmatch(src)
	require.NotNil(t, m, "marker %q not found in site_types.go", re)
	return string(m[1])
}

// TestDefaultWorkbenchResourceProfilesMatchCEL keeps the default profile keys hard-coded in the CEL rule on
// InternalWorkbenchExperimentalFeatures in sync with defaultWorkbenchResourceProfiles().
func TestDefaultWorkbenchResourceProfilesMatchCEL(t *testing.T) {
	list := siteTypesMarker(t, `XValidation:rule="[^"]*resourceProfileAccess[^"]*: p in \[([^\]]*)\]`)

	var celKeys []string
	for _, item := range strings.Split(list, ",") {
		celKeys = append(celKeys, strings.Trim(strings.TrimSpace(item), "'"))
	}
	require.Equal(t, getResourceProfileKeys(defaultWorkbenchResourceProfiles()), celKeys)
}

// TestResourceProfileAccessMatchGrammarParity checks that the controller's match validation agrees with the Pattern
// marker the API server enforces.
func TestResourceProfileAccessMatchGrammarParity(t *testing.T) {
	pattern := regexp.MustCompile(siteTypesMarker(t, "(?m)Pattern=`([^`]*)`\\s*\\n\\s*Match string"))

	tests := []struct {
		match string
		valid bool
	}{
		{"*", true},
		{"jdoe", true},
		{"j.doe-1_x", true},
		{"@power-users", true},
		{"@a@b", true},
		{"a@b", true},
		{"a*", true},
		{"@a*", true},
		{"@a b", true}, // non-ASCII space: RE2's \s is ASCII-only, so both accept it
		{"", false},
		{"@", false},
		{"@@", false},
		{"@@admins", false},
		{"**", false},
		{"*x", false},
		{"*admins", false},
		{"@*", false},
		{"@*admins", false},
		{"my user", false},
		{"@my group", false},
		{"tab\there", false},
		{"new\nline", false},
		{"cr\rx", false},
		{"ff\fx", false},
		{" lead", false},
		{"trail ", false},
		{"a[b", false},
		{"a]b", false},
		{"[x]", false},
		{"@grp]\n[*", false},
	}
	for _, tt := range tests {
		t.Run(tt.match, func(t *testing.T) {
			goErr := validateResourceProfileAccessMatch(tt.match)
			patternOK := pattern.MatchString(tt.match)
			require.Equal(t, tt.valid, goErr == nil, "Go validator: %v", goErr)
			require.Equal(t, tt.valid, patternOK, "Pattern %s", pattern)
		})
	}
}
