package v1beta1

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestWorkbenchSecretConfig_GenerateSecretData(t *testing.T) {
	wb := WorkbenchSecretConfig{
		WorkbenchSecretIniConfig{
			Database: &WorkbenchDatabaseConfig{
				Provider: WorkbenchDatabaseProviderPostgres,
				Database: "chicken",
				Port:     "5432",
				Host:     "myhost.com",
				Username: "user",
			},
			OpenidClientSecret: &WorkbenchOpenidClientSecret{
				ClientId: "your-client-id-test",
			},
			Databricks: map[string]*WorkbenchDatabricksConfig{
				"posit-test": &WorkbenchDatabricksConfig{
					Name: "posit-test",
				},
			},
		},
	}

	res, err := wb.GenerateSecretData()
	require.Nil(t, err)
	require.Contains(t, res["database.conf"], "provider=postgresql")
	require.Contains(t, res["database.conf"], "database=chicken")
	require.Contains(t, res["database.conf"], "port=5432")
	require.Contains(t, res["database.conf"], "host=myhost.com")
	require.Contains(t, res["database.conf"], "username=user")
	require.Contains(t, res["openid-client-secret"], "client-id=your-client-id-test")
	require.Contains(t, res["databricks.conf"], "name=posit-test")
	require.Len(t, res, 3)
}

func TestWorkbenchConfig_GenerateConfigmap(t *testing.T) {
	wb := WorkbenchConfig{
		WorkbenchIniConfig: WorkbenchIniConfig{
			Launcher: &WorkbenchLauncherConfig{
				Server: &WorkbenchLauncherServerConfig{
					Address: "127.0.0.1",
					Port:    "5559",
				},
			},
		},
	}

	res, err := wb.GenerateConfigmap()
	require.Nil(t, err)
	require.Equal(t, "\n[server]\naddress=127.0.0.1\nport=5559\nauthorization-enabled=0\nthread-pool-size=0\nunprivileged=0\n", res["launcher.conf"])
	require.Len(t, res, 1) // other items should not render
	require.Equal(t, res["launcher-env"], "")

	all := WorkbenchConfig{
		WorkbenchIniConfig: WorkbenchIniConfig{
			Launcher: &WorkbenchLauncherConfig{
				Server: &WorkbenchLauncherServerConfig{
					Address: "127.0.0.1",
					Port:    "5559",
				},
				Cluster: &WorkbenchLauncherClusterConfig{
					Name: "Kubernetes",
					Type: "Kubernetes",
				},
			},
			VsCode: &WorkbenchVsCodeConfig{
				Enabled: 1,
				Args:    "some-arg",
			},
			Logging: &WorkbenchLoggingConfig{
				All: &WorkbenchLoggingSection{
					LogLevel:   "info",
					LoggerType: "stderr",
				},
			},
			Jupyter: &WorkbenchJupyterConfig{
				LabsEnabled:                  1,
				NotebooksEnabled:             1,
				JupyterExe:                   "/usr/bin/jupyter",
				LabVersion:                   "3.4.0",
				NotebookVersion:              "6.5.0",
				SessionCullMinutes:           0,
				SessionShutdownMinutes:       10,
				DefaultSessionCluster:        "default-cluster",
				DefaultSessionContainerImage: "jupyter/scipy-notebook:latest",
			},
			RServer: &WorkbenchRServerConfig{
				ServerHealthCheckEnabled:               1,
				AuthPamSessionsEnabled:                 0,
				AdminEnabled:                           1,
				WwwPort:                                8787,
				ServerProjectSharing:                   0,
				LauncherAddress:                        "127.0.0.1",
				LauncherPort:                           5559,
				LauncherSessionsEnabled:                1,
				LauncherSessionsAutoUpdate:             1,
				LauncherSessionsInitContainerImageName: "my-init-container",
				LauncherSessionsInitContainerImageTag:  "v1.0.0",
				AuthOpenidScopes:                       []string{"openid", "profile", "email", "offline_access"},
			},
			Resources: map[string]*WorkbenchLauncherKubnernetesResourcesConfigSection{
				"*": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name: "test-name",
					Cpus: "6CPUS",
				},
			},
		},
		WorkbenchSessionIniConfig: WorkbenchSessionIniConfig{
			RSession: &WorkbenchRSessionConfig{
				DefaultRSConnectServer: "https://my-connect.com",
			},
			Repos: &WorkbenchRepoConfig{
				RSPM: "https://rspm.com",
				CRAN: "https://cran.com",
			},
		},
		WorkbenchProfilesConfig: WorkbenchProfilesConfig{
			map[string]WorkbenchLauncherKubernetesProfilesConfigSection{
				"*": WorkbenchLauncherKubernetesProfilesConfigSection{
					ContainerImages:      []string{"one", "two"},
					AllowCustomResources: 1,
					AllowUnknownImages:   1,
				},
			},
		},
		WorkbenchDcfConfig: WorkbenchDcfConfig{
			LauncherEnv: &WorkbenchLauncherEnvConfig{
				JobType:     "any",
				Environment: map[string]string{"PATH": "/usr/bin"},
			},
		},
	}

	res, err = all.GenerateConfigmap()
	require.Nil(t, err)

	require.Contains(t, res["launcher.conf"], "[server]\naddress=127.0.0.1\nport=5559\n")
	require.Contains(t, res["launcher.conf"], "[cluster]\nname=Kubernetes\ntype=Kubernetes\n")

	require.Contains(t, res["rserver.conf"], "admin-enabled=1\n")
	require.Contains(t, res["rserver.conf"], "launcher-sessions-auto-update=1\n")
	require.Contains(t, res["rserver.conf"], "launcher-sessions-init-container-image-name=my-init-container\n")
	require.Contains(t, res["rserver.conf"], "launcher-sessions-init-container-image-tag=v1.0.0\n")
	require.Contains(t, res["rserver.conf"], "auth-openid-scopes=openid profile email offline_access\n")
	require.Contains(t, res["vscode.conf"], "enabled=1\n")
	require.Contains(t, res["jupyter.conf"], "labs-enabled=1\n")
	require.Contains(t, res["jupyter.conf"], "notebooks-enabled=1\n")
	require.Contains(t, res["jupyter.conf"], "jupyter-exe=/usr/bin/jupyter\n")
	require.Contains(t, res["jupyter.conf"], "lab-version=3.4.0\n")
	require.Contains(t, res["jupyter.conf"], "notebook-version=6.5.0\n")
	require.Contains(t, res["jupyter.conf"], "session-cull-minutes=0\n")
	require.Contains(t, res["jupyter.conf"], "session-shutdown-minutes=10\n")
	require.Contains(t, res["jupyter.conf"], "default-session-cluster=default-cluster\n")
	require.Contains(t, res["jupyter.conf"], "default-session-container-image=jupyter/scipy-notebook:latest\n")

	require.Contains(t, res["launcher.kubernetes.profiles.conf"], "\n[*]\n")
	require.Contains(t, res["launcher.kubernetes.profiles.conf"], "\ncontainer-images=one,two")

	require.Contains(t, res["repos.conf"], "CRAN=https://cran.com")
	require.Contains(t, res["repos.conf"], "RSPM=https://rspm.com")
	require.Contains(t, res["rsession.conf"], "default-rsconnect-server=https://my-connect.com\n")
	require.Contains(t, res["launcher.kubernetes.resources.conf"], "name=test-name\ncpus=6CPUS\n")
	require.Contains(t, res["launcher-env"], "JobType: any")
	require.Contains(t, res["launcher-env"], "Environment: PATH=/usr/bin")
	require.Contains(t, res["logging.conf"], "[*]\nlog-level")
}

func TestWorkbenchConfig_GenerateSupervisorConfigmap(t *testing.T) {
	wbc := WorkbenchConfig{
		SupervisordIniConfig: SupervisordIniConfig{
			map[string]map[string]*SupervisordProgramConfig{
				"launcher.conf": {
					"rstudio-launcher": {
						Command:               "/usr/lib/rstudio-server/bin/rstudio-launcher",
						AutoRestart:           false,
						NumProcs:              1,
						StdOutLogFile:         "/dev/stdout",
						StdOutLogFileMaxBytes: 0,
						StdErrLogFile:         "/dev/stderr",
						StdErrLogFileMaxBytes: 0,
						User:                  "rserver",
					},
				},
			},
		},
	}

	cm, err := wbc.GenerateSupervisorConfigmap(context.TODO())
	require.Nil(t, err)
	require.Contains(t, cm["launcher.conf"], "[program:rstudio-launcher")
	require.Contains(t, cm["launcher.conf"], "numprocs=1")
	require.Contains(t, cm["launcher.conf"], "stdout_logfile_maxbytes=0")
	require.Contains(t, cm["launcher.conf"], "user=rserver")
}

func TestWorkbenchConfig_GenerateSessionConfigmap(t *testing.T) {
	wbc := WorkbenchConfig{
		WorkbenchSessionNewlineConfig: WorkbenchSessionNewlineConfig{
			VsCodeExtensionsConf: []string{"ext1", "ext2"},
		},
		WorkbenchSessionJsonConfig: WorkbenchSessionJsonConfig{
			VSCodeUserSettingsJson: map[string]*apiextensionsv1.JSON{
				"one": {Raw: []byte("\"two\"")},
			},
			PositronUserSettingsJson: map[string]*apiextensionsv1.JSON{
				"extensions.autoUpdate":         {Raw: []byte("true")},
				"quarto.path":                   {Raw: []byte("\"/some/path\"")},
				"positron.assistant.enable":     {Raw: []byte("true")},
				"positron.assistant.testModels": {Raw: []byte("false")},
				"some.number.setting":           {Raw: []byte("42")},
			},
		},
		WorkbenchSessionIniConfig: WorkbenchSessionIniConfig{
			Positron: &WorkbenchPositronConfig{
				Enabled: 1,
				Exe:     "/some/path",
			},
		},
	}

	cm, err := wbc.GenerateSessionConfigmap()
	require.Nil(t, err)
	fmt.Printf("Session: %+v\n", cm)
	require.Contains(t, cm["vscode-user-settings.json"], "\"one\":\"two\"")
	require.Contains(t, cm["positron-user-settings.json"], "\"quarto.path\":\"/some/path\"")
	// Verify boolean values are not quoted
	require.Contains(t, cm["positron-user-settings.json"], "\"extensions.autoUpdate\":true")
	require.Contains(t, cm["positron-user-settings.json"], "\"positron.assistant.enable\":true")
	require.Contains(t, cm["positron-user-settings.json"], "\"positron.assistant.testModels\":false")
	// Verify numeric values are not quoted
	require.Contains(t, cm["positron-user-settings.json"], "\"some.number.setting\":42")
	require.Contains(t, cm["positron.conf"], "exe=/some/path")
}

func TestWorkbenchConfig_GenerateDcfConfigmap(t *testing.T) {
	wbc := WorkbenchConfig{
		WorkbenchDcfConfig: WorkbenchDcfConfig{
			LauncherEnv: &WorkbenchLauncherEnvConfig{
				JobType:     "any",
				Environment: map[string]string{"PATH": "/usr/bin", "TEST": "something"},
			},
		},
	}

	cm, err := wbc.GenerateConfigmap()
	require.Nil(t, err)
	fmt.Printf("Session: %+v\n", cm)
	require.Contains(t, cm["launcher-env"], "\nJobType: any\nEnvironment: PATH=/usr/bin\n  TEST=something\n")
}

func TestVsCodeUserSettingsJsonMarshal(t *testing.T) {
	us := map[string]*apiextensionsv1.JSON{
		"one": {Raw: []byte("\"two\"")},
	}

	b, err := json.Marshal(us)
	require.Nil(t, err)
	require.Contains(t, string(b[:]), "\"one\"")
	require.Contains(t, string(b[:]), "\"two\"")
}

func TestWorkbenchConfig_GenerateLoginConfigmap(t *testing.T) {
	r := require.New(t)
	wbc := WorkbenchConfig{}

	cm, err := wbc.GenerateLoginConfigmapData(context.TODO())
	r.NoError(err)
	r.NotNil(cm)

	r.Contains(cm, "login.defs")
	r.Contains(cm["login.defs"], "UMASK")

	r.Contains(cm, "common-session")
	r.Contains(cm["common-session"], "umask=0077")

	r.Contains(cm, "99-ptd.sh")
	r.Contains(cm["99-ptd.sh"], "umask 077")
}

func TestParseResourceQuantity(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string // Expected string representation
	}{
		{"empty string", "", "0"},
		{"integer", "2", "2"},
		{"float", "1.5", "1500m"},
		{"millicores", "500m", "500m"},
		{"millicores full core", "1000m", "1"},
		{"millicores multiple", "2500m", "2500m"},
		{"memory bytes", "1024", "1024"},
		{"memory Ki", "1024Ki", "1Mi"},
		{"memory Mi", "512Mi", "512Mi"},
		{"memory Gi", "1Gi", "1Gi"},
		{"memory Gi decimal", "1.5Gi", "1536Mi"},
		{"memory Ti", "1Ti", "1Ti"},
		{"invalid format", "abc", "0"},
		{"invalid number with unit", "xyzGi", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseResourceQuantity(tt.input)
			expected := resource.MustParse(tt.expected)
			require.True(t, result.Equal(expected), "got %v, want %v", result.String(), expected.String())
		})
	}
}

func TestCompareResourceProfiles(t *testing.T) {
	tests := []struct {
		name     string
		profile1 *WorkbenchLauncherKubnernetesResourcesConfigSection
		profile2 *WorkbenchLauncherKubnernetesResourcesConfigSection
		expected bool // true if profile1 should come before profile2
	}{
		{
			name: "lower CPU comes first",
			profile1: &WorkbenchLauncherKubnernetesResourcesConfigSection{
				Name:  "small",
				Cpus:  "1",
				MemMb: "1024",
			},
			profile2: &WorkbenchLauncherKubnernetesResourcesConfigSection{
				Name:  "large",
				Cpus:  "2",
				MemMb: "1024",
			},
			expected: true,
		},
		{
			name: "equal CPU, lower memory comes first",
			profile1: &WorkbenchLauncherKubnernetesResourcesConfigSection{
				Name:  "small",
				Cpus:  "1",
				MemMb: "512",
			},
			profile2: &WorkbenchLauncherKubnernetesResourcesConfigSection{
				Name:  "medium",
				Cpus:  "1",
				MemMb: "1024",
			},
			expected: true,
		},
		{
			name: "millicores comparison",
			profile1: &WorkbenchLauncherKubnernetesResourcesConfigSection{
				Name:  "small",
				Cpus:  "500m",
				MemMb: "1024",
			},
			profile2: &WorkbenchLauncherKubnernetesResourcesConfigSection{
				Name:  "medium",
				Cpus:  "1",
				MemMb: "1024",
			},
			expected: true,
		},
		{
			name: "use request if higher than limit",
			profile1: &WorkbenchLauncherKubnernetesResourcesConfigSection{
				Name:        "profile1",
				Cpus:        "0.5",
				CpusRequest: "1", // Higher request should be used
				MemMb:       "1024",
			},
			profile2: &WorkbenchLauncherKubnernetesResourcesConfigSection{
				Name:        "profile2",
				Cpus:        "2",
				CpusRequest: "0.5",
				MemMb:       "1024",
			},
			expected: true,
		},
		{
			name: "equal resources, sort by name",
			profile1: &WorkbenchLauncherKubnernetesResourcesConfigSection{
				Name:  "a-profile",
				Cpus:  "1",
				MemMb: "1024",
			},
			profile2: &WorkbenchLauncherKubnernetesResourcesConfigSection{
				Name:  "b-profile",
				Cpus:  "1",
				MemMb: "1024",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use profile names as keys for comparison
			key1 := "profile1"
			key2 := "profile2"
			if tt.name == "equal resources, sort by name" {
				// For the name-based test, use actual names
				key1 = "a-profile"
				key2 = "b-profile"
			}
			result := compareResourceProfiles(tt.profile1, tt.profile2, key1, key2)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestWorkbenchConfig_GenerateConfigmap_ResourcesSorted(t *testing.T) {
	wb := WorkbenchConfig{
		WorkbenchIniConfig: WorkbenchIniConfig{
			Resources: map[string]*WorkbenchLauncherKubnernetesResourcesConfigSection{
				"large": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:  "Large Instance",
					Cpus:  "4",
					MemMb: "8192",
				},
				"small": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:  "Small Instance",
					Cpus:  "1",
					MemMb: "1024",
				},
				"medium": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:  "Medium Instance",
					Cpus:  "2",
					MemMb: "4096",
				},
				"tiny": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:  "Tiny Instance",
					Cpus:  "500m",
					MemMb: "512",
				},
			},
		},
	}

	res, err := wb.GenerateConfigmap()
	require.Nil(t, err)

	resourcesConfig := res["launcher.kubernetes.resources.conf"]
	require.NotEmpty(t, resourcesConfig)

	// Check that profiles appear in the correct order (tiny -> small -> medium -> large)
	tinyIdx := strings.Index(resourcesConfig, "[tiny]")
	smallIdx := strings.Index(resourcesConfig, "[small]")
	mediumIdx := strings.Index(resourcesConfig, "[medium]")
	largeIdx := strings.Index(resourcesConfig, "[large]")

	require.NotEqual(t, -1, tinyIdx, "tiny profile should be present")
	require.NotEqual(t, -1, smallIdx, "small profile should be present")
	require.NotEqual(t, -1, mediumIdx, "medium profile should be present")
	require.NotEqual(t, -1, largeIdx, "large profile should be present")

	// Verify order: tiny < small < medium < large
	require.Less(t, tinyIdx, smallIdx, "tiny should come before small")
	require.Less(t, smallIdx, mediumIdx, "small should come before medium")
	require.Less(t, mediumIdx, largeIdx, "medium should come before large")
}

func TestGetEffectiveResource(t *testing.T) {
	tests := []struct {
		name     string
		limit    string
		request  string
		expected string
	}{
		{"limit higher", "2", "1", "2"},
		{"request higher", "1", "2", "2"},
		{"both equal", "2", "2", "2"},
		{"limit empty", "", "1", "1"},
		{"request empty", "1", "", "1"},
		{"both empty", "", "", "0"},
		{"with units", "1Gi", "512Mi", "1Gi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getEffectiveResource(tt.limit, tt.request)
			expected := resource.MustParse(tt.expected)
			require.True(t, result.Equal(expected), "got %v, want %v", result.String(), expected.String())
		})
	}
}

func TestWorkbenchConfig_GenerateConfigmap_DefaultProfileFirst(t *testing.T) {
	wb := WorkbenchConfig{
		WorkbenchIniConfig: WorkbenchIniConfig{
			Resources: map[string]*WorkbenchLauncherKubnernetesResourcesConfigSection{
				"large": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:  "Large Instance",
					Cpus:  "4",
					MemMb: "8192",
				},
				"default": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:  "Default Instance",
					Cpus:  "2", // Medium-sized, but should still appear first
					MemMb: "2048",
				},
				"tiny": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:  "Tiny Instance",
					Cpus:  "500m",
					MemMb: "512",
				},
			},
		},
	}

	res, err := wb.GenerateConfigmap()
	require.Nil(t, err)

	resourcesConfig := res["launcher.kubernetes.resources.conf"]
	require.NotEmpty(t, resourcesConfig)

	// Check that profiles are present
	defaultIdx := strings.Index(resourcesConfig, "[default]")
	tinyIdx := strings.Index(resourcesConfig, "[tiny]")
	largeIdx := strings.Index(resourcesConfig, "[large]")

	require.NotEqual(t, -1, defaultIdx, "default profile should be present")
	require.NotEqual(t, -1, tinyIdx, "tiny profile should be present")
	require.NotEqual(t, -1, largeIdx, "large profile should be present")

	// Verify that default comes first, regardless of its resource values
	require.Less(t, defaultIdx, tinyIdx, "default should come before tiny")
	require.Less(t, defaultIdx, largeIdx, "default should come before large")

	// Verify remaining profiles are still sorted by resources (tiny < large)
	require.Less(t, tinyIdx, largeIdx, "tiny should come before large")
}

func TestWorkbenchConfig_GenerateConfigmap_MemoryUnits(t *testing.T) {
	wb := WorkbenchConfig{
		WorkbenchIniConfig: WorkbenchIniConfig{
			Resources: map[string]*WorkbenchLauncherKubnernetesResourcesConfigSection{
				"xlarge": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:  "XLarge Instance",
					Cpus:  "8",
					MemMb: "16Gi", // 16384 Mi
				},
				"small": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:  "Small Instance",
					Cpus:  "1",
					MemMb: "512Mi", // 512 Mi
				},
				"medium": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:  "Medium Instance",
					Cpus:  "2",
					MemMb: "2Gi", // 2048 Mi
				},
				"large": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:  "Large Instance",
					Cpus:  "4",
					MemMb: "8Gi", // 8192 Mi
				},
			},
		},
	}

	res, err := wb.GenerateConfigmap()
	require.Nil(t, err)

	resourcesConfig := res["launcher.kubernetes.resources.conf"]
	require.NotEmpty(t, resourcesConfig)

	// Check that profiles appear in the correct order based on memory units
	smallIdx := strings.Index(resourcesConfig, "[small]")
	mediumIdx := strings.Index(resourcesConfig, "[medium]")
	largeIdx := strings.Index(resourcesConfig, "[large]")
	xlargeIdx := strings.Index(resourcesConfig, "[xlarge]")

	require.NotEqual(t, -1, smallIdx, "small profile should be present")
	require.NotEqual(t, -1, mediumIdx, "medium profile should be present")
	require.NotEqual(t, -1, largeIdx, "large profile should be present")
	require.NotEqual(t, -1, xlargeIdx, "xlarge profile should be present")

	// Verify order: small (512Mi) < medium (2Gi) < large (8Gi) < xlarge (16Gi)
	require.Less(t, smallIdx, mediumIdx, "small should come before medium")
	require.Less(t, mediumIdx, largeIdx, "medium should come before large")
	require.Less(t, largeIdx, xlargeIdx, "large should come before xlarge")
}

// TestParseConstraintsString tests parsing of comma-separated placement constraints
func TestParseConstraintsString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single constraint",
			input:    "node-type=gpu",
			expected: []string{"node-type=gpu"},
		},
		{
			name:     "multiple constraints",
			input:    "node-type=gpu,zone=us-west",
			expected: []string{"node-type=gpu", "zone=us-west"},
		},
		{
			name:     "constraints with spaces",
			input:    " node-type=gpu , zone=us-west ",
			expected: []string{"node-type=gpu", "zone=us-west"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: []string{},
		},
		{
			name:     "trailing comma",
			input:    "node-type=gpu,",
			expected: []string{"node-type=gpu"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail until we implement the function
			result := parseConstraintsString(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestMergeConstraints tests merging and deduplicating constraint arrays
func TestMergeConstraints(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		new      []string
		expected []string
	}{
		{
			name:     "merge empty with new",
			existing: []string{},
			new:      []string{"node-type=gpu"},
			expected: []string{"node-type=gpu"},
		},
		{
			name:     "merge existing with empty",
			existing: []string{"zone=us-west"},
			new:      []string{},
			expected: []string{"zone=us-west"},
		},
		{
			name:     "merge non-overlapping",
			existing: []string{"zone=us-west"},
			new:      []string{"node-type=gpu"},
			expected: []string{"zone=us-west", "node-type=gpu"},
		},
		{
			name:     "merge with duplicates",
			existing: []string{"zone=us-west", "node-type=gpu"},
			new:      []string{"node-type=gpu", "tier=high"},
			expected: []string{"zone=us-west", "node-type=gpu", "tier=high"},
		},
		{
			name:     "nil existing",
			existing: nil,
			new:      []string{"node-type=gpu"},
			expected: []string{"node-type=gpu"},
		},
		{
			name:     "nil new",
			existing: []string{"zone=us-west"},
			new:      nil,
			expected: []string{"zone=us-west"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail until we implement the function
			result := mergeConstraints(tt.existing, tt.new)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestValidateConstraintFormat tests validation of key=value and key:value formats
func TestValidateConstraintFormat(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		valid      bool
	}{
		{
			name:       "valid constraint with equals",
			constraint: "node-type=gpu",
			valid:      true,
		},
		{
			name:       "valid constraint with colon",
			constraint: "node-type:gpu",
			valid:      true,
		},
		{
			name:       "valid with hyphens and equals",
			constraint: "node-class=high-memory",
			valid:      true,
		},
		{
			name:       "valid with hyphens and colon",
			constraint: "node-class:high-memory",
			valid:      true,
		},
		{
			name:       "valid with dots and equals",
			constraint: "kubernetes.io/hostname=node1",
			valid:      true,
		},
		{
			name:       "valid with dots and colon",
			constraint: "kubernetes.io/hostname:node1",
			valid:      true,
		},
		{
			name:       "valid kubernetes hostname format",
			constraint: "kubernetes.io/hostname:k8s-control",
			valid:      true,
		},
		{
			name:       "missing separator",
			constraint: "node-type",
			valid:      false,
		},
		{
			name:       "empty key with equals",
			constraint: "=value",
			valid:      false,
		},
		{
			name:       "empty key with colon",
			constraint: ":value",
			valid:      false,
		},
		{
			name:       "empty value with equals",
			constraint: "key=",
			valid:      false,
		},
		{
			name:       "empty value with colon",
			constraint: "key:",
			valid:      false,
		},
		{
			name:       "multiple equals",
			constraint: "key=value=extra",
			valid:      false,
		},
		{
			name:       "multiple colons",
			constraint: "key:value:extra",
			valid:      false,
		},
		{
			name:       "empty string",
			constraint: "",
			valid:      false,
		},
		{
			name:       "spaces in key",
			constraint: "node type=gpu",
			valid:      false,
		},
		{
			name:       "spaces in value",
			constraint: "node-type=gpu node",
			valid:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail until we implement the function
			result := validateConstraintFormat(tt.constraint)
			require.Equal(t, tt.valid, result)
		})
	}
}

// TestWorkbenchConfig_GenerateConfigmap_WithPlacementConstraintsSync tests synchronization
// of placement constraints from resource profiles to launcher profiles
func TestWorkbenchConfig_GenerateConfigmap_WithPlacementConstraintsSync(t *testing.T) {
	wb := WorkbenchConfig{
		WorkbenchIniConfig: WorkbenchIniConfig{
			Resources: map[string]*WorkbenchLauncherKubnernetesResourcesConfigSection{
				"gpu-enabled": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:                 "GPU Instance",
					Cpus:                 "4",
					MemMb:                "8192",
					NvidiaGpus:           "1",
					PlacementConstraints: "node-type=gpu,gpu-vendor=nvidia",
				},
				"compute-heavy": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:                 "Compute Heavy",
					Cpus:                 "16",
					MemMb:                "32768",
					PlacementConstraints: "node-class=compute",
				},
			},
		},
		WorkbenchProfilesConfig: WorkbenchProfilesConfig{
			LauncherKubernetesProfiles: map[string]WorkbenchLauncherKubernetesProfilesConfigSection{
				"data-science": WorkbenchLauncherKubernetesProfilesConfigSection{
					ContainerImages:       []string{"rstudio/ml:latest"},
					DefaultContainerImage: "rstudio/ml:latest",
					ResourceProfiles:      []string{"gpu-enabled", "compute-heavy"},
					PlacementConstraints:  []string{}, // Should be populated with constraints from referenced resources
				},
			},
		},
	}

	res, err := wb.GenerateConfigmap()
	require.Nil(t, err)

	profilesConfig := res["launcher.kubernetes.profiles.conf"]
	require.NotEmpty(t, profilesConfig)

	// Check that placement constraints were synchronized
	require.Contains(t, profilesConfig, "placement-constraints")
	require.Contains(t, profilesConfig, "node-type=gpu")
	require.Contains(t, profilesConfig, "gpu-vendor=nvidia")
	require.Contains(t, profilesConfig, "node-class=compute")
}

// TestWorkbenchConfig_GenerateConfigmap_PreservesExistingConstraints tests that
// existing placement constraints in profiles are preserved when syncing
func TestWorkbenchConfig_GenerateConfigmap_PreservesExistingConstraints(t *testing.T) {
	wb := WorkbenchConfig{
		WorkbenchIniConfig: WorkbenchIniConfig{
			Resources: map[string]*WorkbenchLauncherKubnernetesResourcesConfigSection{
				"gpu-enabled": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:                 "GPU Instance",
					Cpus:                 "4",
					MemMb:                "8192",
					PlacementConstraints: "node-type=gpu",
				},
			},
		},
		WorkbenchProfilesConfig: WorkbenchProfilesConfig{
			LauncherKubernetesProfiles: map[string]WorkbenchLauncherKubernetesProfilesConfigSection{
				"production": WorkbenchLauncherKubernetesProfilesConfigSection{
					ContainerImages:       []string{"rstudio/prod:latest"},
					DefaultContainerImage: "rstudio/prod:latest",
					ResourceProfiles:      []string{"gpu-enabled"},
					PlacementConstraints:  []string{"environment=prod", "tier=critical"}, // Existing constraints
				},
			},
		},
	}

	res, err := wb.GenerateConfigmap()
	require.Nil(t, err)

	profilesConfig := res["launcher.kubernetes.profiles.conf"]
	require.NotEmpty(t, profilesConfig)

	// Check that both existing and synced constraints are present
	require.Contains(t, profilesConfig, "environment=prod")
	require.Contains(t, profilesConfig, "tier=critical")
	require.Contains(t, profilesConfig, "node-type=gpu")
}

// TestWorkbenchConfig_GenerateConfigmap_WithColonFormat tests synchronization
// with colon-separated placement constraints (kubernetes.io/hostname:k8s-control format)
func TestWorkbenchConfig_GenerateConfigmap_WithColonFormat(t *testing.T) {
	wb := WorkbenchConfig{
		WorkbenchIniConfig: WorkbenchIniConfig{
			Resources: map[string]*WorkbenchLauncherKubnernetesResourcesConfigSection{
				"default": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:                 "Default Instance",
					Cpus:                 "1.0",
					MemMb:                "2048",
					PlacementConstraints: "kubernetes.io/hostname:k8s-control",
				},
				"small": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:                 "Small Instance",
					Cpus:                 "1.0",
					MemMb:                "512",
					PlacementConstraints: "kubernetes.io/hostname:k8s-worker2",
				},
				"medium": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:                 "Medium Instance",
					Cpus:                 "2.0",
					MemMb:                "4096",
					PlacementConstraints: "kubernetes.io/hostname:k8s-worker1",
				},
			},
		},
		WorkbenchProfilesConfig: WorkbenchProfilesConfig{
			LauncherKubernetesProfiles: map[string]WorkbenchLauncherKubernetesProfilesConfigSection{
				"*": WorkbenchLauncherKubernetesProfilesConfigSection{
					ContainerImages:       []string{"rstudio/workbench:latest"},
					DefaultContainerImage: "rstudio/workbench:latest",
					ResourceProfiles:      []string{"default", "small", "medium"},
					PlacementConstraints:  []string{}, // Should be populated with constraints from referenced resources
				},
			},
		},
	}

	res, err := wb.GenerateConfigmap()
	require.Nil(t, err)

	profilesConfig := res["launcher.kubernetes.profiles.conf"]
	require.NotEmpty(t, profilesConfig)

	// Check that colon-separated placement constraints were synchronized
	require.Contains(t, profilesConfig, "placement-constraints")
	require.Contains(t, profilesConfig, "kubernetes.io/hostname:k8s-control")
	require.Contains(t, profilesConfig, "kubernetes.io/hostname:k8s-worker1")
	require.Contains(t, profilesConfig, "kubernetes.io/hostname:k8s-worker2")
}

// TestWorkbenchConfig_GenerateConfigmap_HandlesEmptyConstraints tests handling
// of empty or nil placement constraints
func TestWorkbenchConfig_GenerateConfigmap_HandlesEmptyConstraints(t *testing.T) {
	wb := WorkbenchConfig{
		WorkbenchIniConfig: WorkbenchIniConfig{
			Resources: map[string]*WorkbenchLauncherKubnernetesResourcesConfigSection{
				"basic": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:                 "Basic Instance",
					Cpus:                 "2",
					MemMb:                "4096",
					PlacementConstraints: "", // Empty constraints
				},
				"standard": &WorkbenchLauncherKubnernetesResourcesConfigSection{
					Name:  "Standard Instance",
					Cpus:  "4",
					MemMb: "8192",
					// No PlacementConstraints field set
				},
			},
		},
		WorkbenchProfilesConfig: WorkbenchProfilesConfig{
			LauncherKubernetesProfiles: map[string]WorkbenchLauncherKubernetesProfilesConfigSection{
				"default": WorkbenchLauncherKubernetesProfilesConfigSection{
					ContainerImages:       []string{"rstudio/default:latest"},
					DefaultContainerImage: "rstudio/default:latest",
					ResourceProfiles:      []string{"basic", "standard"},
					PlacementConstraints:  nil, // Nil constraints
				},
			},
		},
	}

	res, err := wb.GenerateConfigmap()
	require.Nil(t, err)

	profilesConfig := res["launcher.kubernetes.profiles.conf"]
	require.NotEmpty(t, profilesConfig)

	// The profile section should exist but placement-constraints should be empty or not present
	require.Contains(t, profilesConfig, "[default]")
}

// TestWorkbenchConfig_ForceAdminUiEnabled tests force-admin-ui-enabled behavior
func TestWorkbenchConfig_ForceAdminUiEnabled(t *testing.T) {
	// Test when explicitly set to 1
	wbEnabled := WorkbenchConfig{
		WorkbenchIniConfig: WorkbenchIniConfig{
			RServer: &WorkbenchRServerConfig{
				AdminEnabled:        1,
				ForceAdminUiEnabled: 1,
			},
		},
	}

	res, err := wbEnabled.GenerateConfigmap()
	require.Nil(t, err)
	require.Contains(t, res["rserver.conf"], "force-admin-ui-enabled=1\n")

	// Test when set to 0 (disabled) - still appears in config
	wbDisabled := WorkbenchConfig{
		WorkbenchIniConfig: WorkbenchIniConfig{
			RServer: &WorkbenchRServerConfig{
				AdminEnabled:        1,
				ForceAdminUiEnabled: 0,
			},
		},
	}

	res, err = wbDisabled.GenerateConfigmap()
	require.Nil(t, err)
	require.Contains(t, res["rserver.conf"], "force-admin-ui-enabled=0\n")
}
