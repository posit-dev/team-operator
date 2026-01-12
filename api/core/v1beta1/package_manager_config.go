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
}

func (configStruct *PackageManagerConfig) GenerateGcfg() (string, error) {

	var builder strings.Builder

	configStructValsPtr := reflect.ValueOf(configStruct)
	configStructVals := reflect.Indirect(configStructValsPtr)

	for i := 0; i < configStructVals.NumField(); i++ {
		fieldName := configStructVals.Type().Field(i).Name
		fieldValue := configStructVals.Field(i)

		if fieldValue.IsNil() {
			continue
		}

		builder.WriteString("\n[" + fieldName + "]\n")

		sectionStructVals := reflect.Indirect(fieldValue)

		for j := 0; j < sectionStructVals.NumField(); j++ {
			sectionFieldName := sectionStructVals.Type().Field(j).Name
			sectionFieldValue := sectionStructVals.Field(j)

			if sectionStructVals.Field(j).String() != "" {
				if sectionFieldValue.Kind() == reflect.Slice {
					for k := 0; k < sectionFieldValue.Len(); k++ {
						arrayValue := sectionFieldValue.Index(k).String()
						if arrayValue != "" {
							builder.WriteString(fmt.Sprintf("%v", sectionFieldName) + " = " + fmt.Sprintf("%v", arrayValue) + "\n")
						}
					}

				} else {
					builder.WriteString(fmt.Sprintf("%v", sectionFieldName) + " = " + fmt.Sprintf("%v", sectionFieldValue) + "\n")
				}
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
