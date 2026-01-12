package v1beta1

import (
	"fmt"

	"reflect"
	"strings"
)

const RPackageRepositoryMapKey = "RPackageRepositories"

type ConnectConfig struct {
	Server         *ConnectServerConfig         `json:"Server,omitempty"`
	Http           *ConnectHttpConfig           `json:"Http,omitempty"`
	Metrics        *ConnectMetricsConfig        `json:"Metrics,omitempty"`
	Database       *ConnectDatabaseConfig       `json:"Database,omitempty"`
	Postgres       *ConnectPostgresConfig       `json:"Postgres,omitempty"`
	Logging        *ConnectLoggingConfig        `json:"Logging,omitempty"`
	Launcher       *ConnectLauncherConfig       `json:"Launcher,omitempty"`
	Authentication *ConnectAuthenticationConfig `json:"Authentication,omitempty"`
	Authorization  *ConnectAuthorizationConfig  `json:"Authorization,omitempty"`
	Applications   *ConnectApplicationsConfig   `json:"Applications,omitempty"`
	OAuth2         *ConnectOAuth2Config         `json:"OAuth2,omitempty"`
	R              *ConnectRConfig              `json:"R,omitempty"`
	Python         *ConnectPythonConfig         `json:"Python,omitempty"`
	Quarto         *ConnectQuartoConfig         `json:"Quarto,omitempty"`
	SAML           *ConnectSamlConfig           `json:"SAML,omitempty"`
	Scheduler      *ConnectSchedulerConfig      `json:"Scheduler,omitempty"`
	// exclude this from default JSON marshalling... that way we can handle directly and unwrap the keys
	// at the top level of the JSON output
	// see the GenerateGcfg method for our custom handling
	RPackageRepository map[string]RPackageRepositoryConfig `json:"RPackageRepositories,omitempty"`
	TableauIntegration *ConnectTableauIntegrationConfig    `json:"TableauIntegration,omitempty"`
}

type RPackageRepositoryConfig struct {
	Url string `json:"Url,omitempty"`
}

type ConnectUserRole string

const (
	ConnectViewerRole        ConnectUserRole = "viewer"
	ConnectPublisherRole                     = "publisher"
	ConnectAdministratorRole                 = "administrator"
)

type ConnectPackageManagerUrlRewrite string

const (
	ConnectPackageManagerUrlRewriteAuto        ConnectPackageManagerUrlRewrite = "auto"
	ConnectPackageManagerUrlRewriteForceSource                                 = "force-source"
	ConnectPackageManagerUrlRewriteForceBinary                                 = "force-binary"
	ConnectPackageManagerUrlRewriteNone                                        = "none"
)

type ConnectQuartoConfig struct {
	Enabled bool `json:"Enabled,omitempty"`
}
type ConnectRConfig struct {
	Enabled                         bool                            `json:"Enabled,omitempty"`
	PositPackageManagerURLRewriting ConnectPackageManagerUrlRewrite `json:"PositPackageManagerURLRewriting,omitempty"`
}
type ConnectAuthorizationConfig struct {
	DefaultUserRole             ConnectUserRole `json:"DefaultUserRole,omitempty"`
	PublishersCanManageVanities bool            `json:"PublishersCanManageVanities,omitempty"`
	ViewersCanOnlySeeThemselves bool            `json:"ViewersCanOnlySeeThemselves,omitempty"`
	UserRoleGroupMapping        bool            `json:"UserRoleGroupMapping,omitempty"`
	ViewerRoleMapping           []string        `json:"ViewerRoleMapping,omitempty"`
	PublisherRoleMapping        []string        `json:"PublisherRoleMapping,omitempty"`
	AdministratorRoleMapping    []string        `json:"AdministratorRoleMapping,omitempty"`
}
type ConnectAuthenticationConfig struct {
	Provider AuthType `json:"Provider,omitempty"`
}

type ConnectApplicationsConfig struct {
	BundleRetentionLimit     int  `json:"BundleRetentionLimit,omitempty"`
	PythonEnvironmentReaping bool `json:"PythonEnvironmentReaping,omitempty"`
	OAuthIntegrationsEnabled bool `json:"OAuthIntegrationsEnabled,omitempty"`
	ScheduleConcurrency      int  `json:"ScheduleConcurrency,omitempty"`
}

type ConnectOAuth2Config struct {
	ClientId             string   `json:"ClientId,omitempty"`
	ClientSecretFile     string   `json:"ClientSecretFile,omitempty"`
	OpenIDConnectIssuer  string   `json:"OpenIDConnectIssuer,omitempty"`
	RequireUsernameClaim bool     `json:"RequireUsernameClaim,omitempty"`
	CustomScope          []string `json:"CustomScope,omitempty"`
	GroupsAutoProvision  bool     `json:"GroupsAutoProvision,omitempty"`
	UniqueIdClaim        string   `json:"UniqueIdClaim,omitempty"`
	EmailClaim           string   `json:"EmailClaim,omitempty"`
	UsernameClaim        string   `json:"UsernameClaim,omitempty"`
	GroupsClaim          *string  `json:"GroupsClaim,omitempty"`
	Logging              bool     `json:"Logging,omitempty"`
}
type ConnectSamlConfig struct {
	IdPMetaDataURL      string `json:"IdPMetaDataURL,omitempty"`
	IdPAttributeProfile string `json:"IdPAttributeProfile,omitempty"`
	UsernameAttribute   string `json:"UsernameAttribute,omitempty"`
	FirstNameAttribute  string `json:"FirstNameAttribute,omitempty"`
	LastNameAttribute   string `json:"LastNameAttribute,omitempty"`
	EmailAttribute      string `json:"EmailAttribute,omitempty"`
}

type ConnectSchedulerConfig struct {
	MaxCPURequest     int `json:"MaxCPURequest,omitempty"`
	MaxCPULimit       int `json:"MaxCPULimit,omitempty"`
	MaxMemoryRequest  int `json:"MaxMemoryRequest,omitempty"`
	MaxMemoryLimit    int `json:"MaxMemoryLimit,omitempty"`
	NvidiaGPULimit    int `json:"NvidiaGPULimit,omitempty"`
	MaxNvidiaGPULimit int `json:"MaxNvidiaGPULimit,omitempty"`
	AMDGPULimit       int `json:"AMDGPULimit,omitempty"`
	MaxAMDGPULimit    int `json:"MaxAMDGPULimit,omitempty"`
}

type ContentListView string

const (
	ContentListViewCompact  ContentListView = "compact"
	ContentListViewExpanded ContentListView = "expanded"
)

type ConnectServerConfig struct {
	Address                string          `json:"Address,omitempty"`
	DataDir                string          `json:"DataDir,omitempty"`
	FrameOptionsContent    string          `json:"FrameOptionsContent,omitempty"`
	FrameOptionsDashboard  string          `json:"FrameOptionsDashboard,omitempty"`
	DefaultContentListView ContentListView `json:"DefaultContentListView,omitempty"`
	LoggedInWarning        string          `json:"LoggedInWarning,omitempty"`
	PublicWarning          string          `json:"PublicWarning,omitempty"`
	ProxyHeaderLogging     bool            `json:"ProxyHeaderLogging,omitempty"`
	// EmailProvider sets the email provider for Connect emails. If set, secrets will be mounted for SMTP connections
	EmailProvider          string `json:"EmailProvider,omitempty"`
	SenderEmail            string `json:"SenderEmail,omitempty"`
	SenderEmailDisplayName string `json:"SenderEmailDisplayName,omitempty"`
	EmailTo                string `json:"EmailTo,omitempty"`
	HideEmailAddresses     bool   `json:"HideEmailAddresses,omitempty"`
}

type ConnectDatabaseConfig struct {
	Provider string `json:"Provider,omitempty"`
}

type ConnectPostgresConfig struct {
	URL                string `json:"URL,omitempty"`
	InstrumentationURL string `json:"InstrumentationURL,omitempty"`
}

type ConnectHttpConfig struct {
	ForceSecure bool   `json:"ForceSecure,omitempty"`
	Listen      string `json:"Listen,omitempty"`
}

type ConnectServiceLogFormat string

const (
	ConnectServiceLogFormatText ConnectServiceLogFormat = "TEXT"
	ConnectServiceLogFormatJson                         = "JSON"
)

type ConnectAccessLogFormat string

const (
	ConnectAccessLogFormatCommon   ConnectAccessLogFormat = "COMMON"
	ConnectAccessLogFormatCombined                        = "COMBINED"
	ConnectAccessLogFormatJson                            = "JSON"
)

type ConnectLoggingConfig struct {
	ServiceLog       string                  `json:"ServiceLog,omitempty"`
	ServiceLogLevel  string                  `json:"ServiceLogLevel,omitempty"`
	ServiceLogFormat ConnectServiceLogFormat `json:"ServiceLogFormat,omitempty"`
	AccessLog        string                  `json:"AccessLog,omitempty"`
	AccessLogFormat  ConnectAccessLogFormat  `json:"AccessLogFormat,omitempty"`
	AuditLog         string                  `json:"AuditLog,omitempty"`
	AuditLogFormat   ConnectServiceLogFormat `json:"AuditLogFormat,omitempty"`
}

type ConnectTableauIntegrationConfig struct {
	Logging bool `json:"Logging,omitempty"`
}

type ConnectPythonConfig struct {
	Enabled bool `json:"Enabled,omitempty"`
}

type ConnectLauncherConfig struct {
	Enabled                  bool     `json:"Enabled,omitempty"`
	Kubernetes               bool     `json:"Kubernetes,omitempty"`
	ClusterDefinition        []string `json:"ClusterDefinition,omitempty"`
	KubernetesNamespace      string   `json:"KubernetesNamespace,omitempty"`
	KubernetesProfilesConfig string   `json:"KubernetesProfilesConfig,omitempty"`
	DataDirPVCName           string   `json:"DataDirPVCName,omitempty"`
	KubernetesUseTemplates   bool     `json:"KubernetesUseTemplates,omitempty"`
	ScratchPath              string   `json:"ScratchPath,omitempty"`
}

type ConnectMetricsConfig struct {
	PrometheusListen string `json:"PrometheusListen,omitempty"`
}

func (configStruct *ConnectConfig) GenerateGcfg() (string, error) {

	var builder strings.Builder

	configStructValsPtr := reflect.ValueOf(configStruct)
	configStructVals := reflect.Indirect(configStructValsPtr)

	for i := 0; i < configStructVals.NumField(); i++ {
		fieldName := configStructVals.Type().Field(i).Name
		fieldValue := configStructVals.Field(i)

		if fieldValue.IsNil() {
			continue
		}

		sectionStructVals := reflect.Indirect(fieldValue)

		// This is to handle the case of the RPackageRepositories
		if fieldValue.Kind() == reflect.Map {
			iter := sectionStructVals.MapRange()

			for iter.Next() {
				repoName := iter.Key()
				repoValue := iter.Value()

				builder.WriteString("\n[" + fieldName + " \"" + fmt.Sprintf("%v", repoName) + "\"" + "]\n")

				if repoValue.Kind() == reflect.Struct {
					builder.WriteString("Url = " + fmt.Sprintf("%v", repoValue.FieldByName("Url")) + "\n")
				}
			}
		} else {
			builder.WriteString("\n[" + fieldName + "]\n")

			for j := 0; j < sectionStructVals.NumField(); j++ {
				sectionFieldName := sectionStructVals.Type().Field(j).Name
				sectionFieldValue := sectionStructVals.Field(j)

				// Handle pointer fields (like *string)
				if sectionFieldValue.Kind() == reflect.Ptr {
					if !sectionFieldValue.IsNil() {
						derefValue := sectionFieldValue.Elem()
						// Always write pointer fields when they're not nil, even if empty string
						builder.WriteString(fmt.Sprintf("%v", sectionFieldName) + " = " + fmt.Sprintf("%v", derefValue) + "\n")
					}
					// Skip nil pointers entirely
					continue
				}

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

	}
	return builder.String(), nil
}
