package v1beta1

import (
	"fmt"
	"reflect"
	"strings"
)

type PackageManagerConfig struct {
	Server    *PackageManagerServerConfig    `json:"Server,omitempty"`
	Http      *PackageManagerHttpConfig      `json:"Http,omitempty"`
	Git       *PackageManagerGitConfig       `json:"Git,omitempty"`
	Database  *PackageManagerDatabaseConfig  `json:"Database,omitempty"`
	Postgres  *PackageManagerPostgresConfig  `json:"Postgres,omitempty"`
	Storage   *PackageManagerStorageConfig   `json:"Storage,omitempty"`
	S3Storage *PackageManagerS3StorageConfig `json:"S3Storage,omitempty"`
	Metrics   *PackageManagerMetricsConfig   `json:"Metrics,omitempty"`
	Repos     *PackageManagerReposConfig     `json:"Repos,omitempty"`
	Cran      *PackageManagerCRANConfig      `json:"CRAN,omitempty"`
	Debug     *PackageManagerDebugConfig     `json:"Debug,omitempty"`

	// Additional allows setting arbitrary gcfg config values not covered by typed fields.
	// Keys should be in "Section.Key" format (e.g., "Server.DataDir", "Storage.Default").
	// Values set here take precedence over typed fields if both specify the same key.
	// +optional
	Additional map[string]string `json:"additional,omitempty"`
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

		// Skip the Additional map — we handle it after typed fields
		if fieldName == "Additional" {
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

	// Apply Additional (passthrough) overrides
	if configStruct.Additional != nil {
		for key, value := range configStruct.Additional {
			parts := strings.SplitN(key, ".", 2)
			if len(parts) != 2 {
				continue // skip malformed keys
			}
			sectionName := parts[0]
			keyName := parts[1]

			if idx, ok := sectionIndex[sectionName]; ok {
				// Override or add to existing section
				if _, exists := sections[idx].values[keyName]; !exists {
					// Check if it's overriding a slice key
					if _, sliceExists := sections[idx].slices[keyName]; !sliceExists {
						sections[idx].keys = append(sections[idx].keys, keyName)
					}
				}
				// Remove from slices if it was a multi-value key (passthrough replaces it)
				delete(sections[idx].slices, keyName)
				sections[idx].values[keyName] = value
			} else {
				// Create new section
				entry := sectionEntry{
					name:   sectionName,
					keys:   []string{keyName},
					values: map[string]string{keyName: value},
					slices: map[string][]string{},
				}
				sectionIndex[sectionName] = len(sections)
				sections = append(sections, entry)
			}
		}
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
