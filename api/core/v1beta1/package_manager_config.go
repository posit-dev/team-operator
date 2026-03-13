package v1beta1

import (
	"fmt"
	"reflect"
	"strings"
)

type PackageManagerConfig struct {
	Server             *PackageManagerServerConfig              `json:"Server,omitempty"`
	Http               *PackageManagerHttpConfig                `json:"Http,omitempty"`
	Git                *PackageManagerGitConfig                 `json:"Git,omitempty"`
	Database           *PackageManagerDatabaseConfig            `json:"Database,omitempty"`
	Postgres           *PackageManagerPostgresConfig            `json:"Postgres,omitempty"`
	Storage            *PackageManagerStorageConfig             `json:"Storage,omitempty"`
	S3Storage          *PackageManagerS3StorageConfig           `json:"S3Storage,omitempty"`
	Metrics            *PackageManagerMetricsConfig             `json:"Metrics,omitempty"`
	Repos              *PackageManagerReposConfig               `json:"Repos,omitempty"`
	Cran               *PackageManagerCRANConfig                `json:"CRAN,omitempty"`
	Debug              *PackageManagerDebugConfig               `json:"Debug,omitempty"`
	Authentication     *PackageManagerAuthenticationConfig      `json:"Authentication,omitempty"`
	OpenIDConnect      *PackageManagerOIDCConfig                `json:"OpenIDConnect,omitempty"`
	IdentityFederation []PackageManagerIdentityFederationConfig `json:"IdentityFederation,omitempty"`

	// AdditionalConfig allows appending arbitrary gcfg config content not covered by typed fields.
	// Note: the JSON key is "additionalConfig" (camelCase) for backward compatibility with v1.20.0.
	// The value is appended verbatim after the generated config. gcfg parsing naturally handles
	// conflicts: list values are combined, scalar values use the last occurrence.
	// +optional
	AdditionalConfig string `json:"additionalConfig,omitempty"`
}

func (configStruct *PackageManagerConfig) GenerateGcfg() (string, error) {

	var builder strings.Builder

	// Build an intermediate representation: ordered sections with key-value pairs.
	// We use ordered slices to preserve the deterministic output order from reflection.
	type sectionEntry struct {
		name   string
		keys   []string            // ordered key names (for non-slice values)
		values map[string]string   // key → value
		slices map[string][]string // key → multiple values (for gcfg multi-value keys)
	}
	sections := []sectionEntry{}
	sectionIndex := map[string]int{} // section name → index in sections slice

	configStructValsPtr := reflect.ValueOf(configStruct)
	configStructVals := reflect.Indirect(configStructValsPtr)

	for i := 0; i < configStructVals.NumField(); i++ {
		fieldName := configStructVals.Type().Field(i).Name
		fieldValue := configStructVals.Field(i)

		// Skip the AdditionalConfig string — we handle it at the end
		if fieldName == "AdditionalConfig" {
			continue
		}

		// Skip IdentityFederation - handled specially after the main loop
		if fieldName == "IdentityFederation" {
			continue
		}

		if fieldValue.IsNil() {
			continue
		}

		entry := sectionEntry{
			name:   fieldName,
			values: map[string]string{},
			slices: map[string][]string{},
		}

		sectionStructVals := reflect.Indirect(fieldValue)

		for j := 0; j < sectionStructVals.NumField(); j++ {
			sectionFieldName := sectionStructVals.Type().Field(j).Name
			sectionFieldValue := sectionStructVals.Field(j)

			if sectionStructVals.Field(j).String() != "" {
				if sectionFieldValue.Kind() == reflect.Slice {
					var vals []string
					for k := 0; k < sectionFieldValue.Len(); k++ {
						arrayValue := sectionFieldValue.Index(k).String()
						if arrayValue != "" {
							vals = append(vals, arrayValue)
						}
					}
					if len(vals) > 0 {
						entry.keys = append(entry.keys, sectionFieldName)
						entry.slices[sectionFieldName] = vals
					}
				} else {
					entry.keys = append(entry.keys, sectionFieldName)
					entry.values[sectionFieldName] = fmt.Sprintf("%v", sectionFieldValue)
				}
			}
		}

		sectionIndex[fieldName] = len(sections)
		sections = append(sections, entry)
	}

	// Render sections to gcfg format
	for _, section := range sections {
		builder.WriteString("\n[" + section.name + "]\n")
		for _, key := range section.keys {
			if vals, isSlice := section.slices[key]; isSlice {
				for _, v := range vals {
					builder.WriteString(key + " = " + v + "\n")
				}
			} else if val, ok := section.values[key]; ok {
				builder.WriteString(key + " = " + val + "\n")
			}
		}
	}

	// Render named IdentityFederation sections (these use the gcfg named subsection syntax)
	for _, idf := range configStruct.IdentityFederation {
		if strings.ContainsAny(idf.Name, "\"]\n") {
			return "", fmt.Errorf("invalid IdentityFederation name %q: must not contain '\"', ']', or newlines", idf.Name)
		}
		builder.WriteString(fmt.Sprintf("\n[IdentityFederation \"%s\"]\n", idf.Name))
		if idf.Issuer != "" {
			builder.WriteString("Issuer = " + idf.Issuer + "\n")
		}
		if idf.Logging {
			builder.WriteString("Logging = true\n")
		}
		if idf.Audience != "" {
			builder.WriteString("Audience = " + idf.Audience + "\n")
		}
		if idf.Subject != "" {
			builder.WriteString("Subject = " + idf.Subject + "\n")
		}
		if idf.AuthorizedParty != "" {
			builder.WriteString("AuthorizedParty = " + idf.AuthorizedParty + "\n")
		}
		if idf.Scope != "" {
			builder.WriteString("Scope = " + idf.Scope + "\n")
		}
		if idf.CustomScope != "" {
			builder.WriteString("CustomScope = " + idf.CustomScope + "\n")
		}
		if idf.NoAutoGroupsScope {
			builder.WriteString("NoAutoGroupsScope = true\n")
		}
		if idf.GroupsClaim != "" {
			builder.WriteString("GroupsClaim = " + idf.GroupsClaim + "\n")
		}
		if idf.GroupsSeparator != "" {
			builder.WriteString("GroupsSeparator = " + idf.GroupsSeparator + "\n")
		}
		if idf.RoleClaim != "" {
			builder.WriteString("RoleClaim = " + idf.RoleClaim + "\n")
		}
		if idf.RolesSeparator != "" {
			builder.WriteString("RolesSeparator = " + idf.RolesSeparator + "\n")
		}
		if idf.UniqueIdClaim != "" {
			builder.WriteString("UniqueIdClaim = " + idf.UniqueIdClaim + "\n")
		}
		if idf.UsernameClaim != "" {
			builder.WriteString("UsernameClaim = " + idf.UsernameClaim + "\n")
		}
		if idf.TokenLifetime != "" {
			builder.WriteString("TokenLifetime = " + idf.TokenLifetime + "\n")
		}
	}

	if configStruct.AdditionalConfig != "" {
		builder.WriteString(configStruct.AdditionalConfig)
	}

	return builder.String(), nil
}

type PackageManagerReposConfig struct {
	PyPI         string `json:"PyPI,omitempty"`
	CRAN         string `json:"CRAN,omitempty"`
	Bioconductor string `json:"Bioconductor,omitempty"`
}

// PackageManagerCRANConfig is deprecated TODO: deprecated! We will remove this soon!
type PackageManagerCRANConfig struct {
	RSF bool `json:"RSF,omitempty"`
}

type PackageManagerS3StorageConfig struct {
	Bucket string `json:"Bucket,omitempty"`
	Prefix string `json:"Prefix,omitempty"`
	Region string `json:"Region,omitempty"`
}

type PackageManagerStorageConfig struct {
	Default string `json:"Default,omitempty"`
}

type PackageManagerAccessLogFormat string

const (
	PackageManagerAccessLogFormatCommon   PackageManagerAccessLogFormat = "common"
	PackageManagerAccessLogFormatCombined                               = "combined"
)

type PackageManagerServerConfig struct {
	Address         string                        `json:"Address,omitempty"`
	RVersion        []string                      `json:"RVersion,omitempty"`
	LauncherDir     string                        `json:"LauncherDir,omitempty"`
	AccessLog       string                        `json:"AccessLog,omitempty"`
	AccessLogFormat PackageManagerAccessLogFormat `json:"AccessLogFormat,omitempty"`
	DataDir         string                        `json:"DataDir,omitempty"`
}

type PackageManagerDatabaseConfig struct {
	Provider string `json:"Provider,omitempty"`
}

type PackageManagerPostgresConfig struct {
	URL          string `json:"URL,omitempty"`
	UsageDataURL string `json:"UsageDataURL,omitempty"`
}

type PackageManagerHttpConfig struct {
	Listen string `json:"Listen,omitempty"`
}

type PackageManagerGitConfig struct {
	AllowUnsandboxedGitBuilds bool `json:"AllowUnsandboxedGitBuilds,omitempty"`
}

type PackageManagerMetricsConfig struct {
	Enabled bool `json:"Enabled,omitempty"`
}

type PackageManagerDebugConfig struct {
	Log string `json:"Log,omitempty"`
}

type PackageManagerAuthenticationConfig struct {
	APITokenAuth          bool   `json:"APITokenAuth,omitempty"`
	DeviceAuthType        string `json:"DeviceAuthType,omitempty"`
	NewReposAuthByDefault bool   `json:"NewReposAuthByDefault,omitempty"`
	Lifetime              string `json:"Lifetime,omitempty"`
	Inactivity            string `json:"Inactivity,omitempty"`
	CookieSweepDuration   string `json:"CookieSweepDuration,omitempty"`
}

type PackageManagerOIDCConfig struct {
	ClientId             string `json:"ClientId,omitempty"`
	ClientSecret         string `json:"ClientSecret,omitempty"`
	ClientSecretFile     string `json:"ClientSecretFile,omitempty"`
	Issuer               string `json:"Issuer,omitempty"`
	RequireLogin         bool   `json:"RequireLogin,omitempty"`
	Logging              bool   `json:"Logging,omitempty"`
	Scope                string `json:"Scope,omitempty"`
	CustomScope          string `json:"CustomScope,omitempty"`
	NoAutoGroupsScope    bool   `json:"NoAutoGroupsScope,omitempty"`
	GroupsClaim          string `json:"GroupsClaim,omitempty"`
	GroupsSeparator      string `json:"GroupsSeparator,omitempty"`
	RoleClaim            string `json:"RoleClaim,omitempty"`
	RolesSeparator       string `json:"RolesSeparator,omitempty"`
	UniqueIdClaim        string `json:"UniqueIdClaim,omitempty"`
	UsernameClaim        string `json:"UsernameClaim,omitempty"`
	TokenLifetime        string `json:"TokenLifetime,omitempty"`
	MaxAuthenticationAge string `json:"MaxAuthenticationAge,omitempty"`
	DisablePKCE          bool   `json:"DisablePKCE,omitempty"`
	EnableDevicePKCE     bool   `json:"EnableDevicePKCE,omitempty"`
}

type PackageManagerIdentityFederationConfig struct {
	Name              string `json:"Name"`
	Issuer            string `json:"Issuer,omitempty"`
	Logging           bool   `json:"Logging,omitempty"`
	Audience          string `json:"Audience,omitempty"`
	Subject           string `json:"Subject,omitempty"`
	AuthorizedParty   string `json:"AuthorizedParty,omitempty"`
	Scope             string `json:"Scope,omitempty"`
	CustomScope       string `json:"CustomScope,omitempty"`
	NoAutoGroupsScope bool   `json:"NoAutoGroupsScope,omitempty"`
	GroupsClaim       string `json:"GroupsClaim,omitempty"`
	GroupsSeparator   string `json:"GroupsSeparator,omitempty"`
	RoleClaim         string `json:"RoleClaim,omitempty"`
	RolesSeparator    string `json:"RolesSeparator,omitempty"`
	UniqueIdClaim     string `json:"UniqueIdClaim,omitempty"`
	UsernameClaim     string `json:"UsernameClaim,omitempty"`
	TokenLifetime     string `json:"TokenLifetime,omitempty"`
}

// SSHKeyConfig defines SSH key configuration for Git authentication
type SSHKeyConfig struct {
	// Name is a unique identifier for this SSH key configuration
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name"`

	// Host is the Git host domain this key applies to
	// Example: "github.com"
	// +kubebuilder:validation:Required
	Host string `json:"host"`

	// SecretRef references the secret containing the SSH private key
	// +kubebuilder:validation:Required
	SecretRef SecretReference `json:"secretRef"`

	// PassphraseSecretRef optionally references a secret containing the passphrase for an encrypted SSH key
	// +optional
	PassphraseSecretRef *SecretReference `json:"passphraseSecretRef,omitempty"`
}

// SecretReference defines a reference to a secret in various secret management systems
type SecretReference struct {
	// Source specifies the secret management system
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=aws-secrets-manager;kubernetes;azure-key-vault
	Source string `json:"source"`

	// Name is the secret name
	// For AWS: secret name in AWS Secrets Manager (e.g., "ptd/cluster/packagemanager/ssh/github")
	// For Kubernetes: secret name in the same namespace
	// For Azure: key vault secret name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Key is the specific key within the secret (primarily for Kubernetes secrets)
	// For AWS/Azure: usually not needed as the entire secret is used
	// For Kubernetes: the key within the Secret data
	// +optional
	Key string `json:"key,omitempty"`
}
