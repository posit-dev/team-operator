// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2025 Posit Software, PBC
//+k8s:openapi-gen=true

package v1beta1

import (
	"github.com/posit-dev/team-operator/api/product"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	KubernetesInstanceLabelKey          = "app.kubernetes.io/instance"
	KubernetesMetadataNameKey           = "kubernetes.io/metadata.name"
	KubernetesNameLabelKey              = "app.kubernetes.io/name"
	LauncherInstanceIDKey               = "launcher-instance-id"
	ManagedByLabelKey                   = "app.kubernetes.io/managed-by"
	ManagedByLabelValue                 = "team-operator"
	SiteLabelKey                        = "posit.team/site"
	ComponentLabelKey                   = "posit.team/component"
	ComponentLabelValueConnect          = "connect"
	ComponentLabelValueConnectSession   = "connect-session"
	ComponentLabelValueWorkbench        = "workbench"
	ComponentLabelValueWorkbenchSession = "workbench-session"
)

// SiteSpec defines the desired state of Site
type SiteSpec struct {
	// AwsAccountId is the account Id for this AWS Account. It is used to create EKS-to-IAM annotations
	AwsAccountId string `json:"awsAccountId,omitempty"`

	// ClusterDate is the date id (YYYYmmdd) for the cluster. It is used to create EKS-to-IAM annotations
	ClusterDate string `json:"clusterDate,omitempty"`

	// WorkloadCompoundName is the name for the workload
	WorkloadCompoundName string `json:"workloadCompoundName,omitempty"`

	// Domain is the core domain name associated with the Posit Team Site
	Domain string `json:"domain"`

	// SecretType is the type of secret that we should use to store values (i.e. database passwords)
	// *NOTE*: this field is deprecated and will be removed in the future
	// +optional
	SecretType product.SiteSecretType `json:"secretType"`

	// Flightdeck contains Flightdeck configuration
	Flightdeck InternalFlightdeckSpec `json:"flightdeck,omitempty"`

	// PackageManager contains Posit Package Manager configuration
	PackageManager InternalPackageManagerSpec `json:"packageManager,omitempty"`

	// Connect contains Posit Connect configuration
	Connect InternalConnectSpec `json:"connect,omitempty"`

	// Workbench contains Posit Workbench configuration
	Workbench InternalWorkbenchSpec `json:"workbench,omitempty"`

	// Chronicle contains Posit Chronicle configuration
	Chronicle InternalChronicleSpec `json:"chronicle,omitempty"`

	// Keycloak contains the Keycloak configuration details
	Keycloak InternalKeycloakSpec `json:"keycloak,omitempty"`

	// IngressClass is the ingress class to be used when creating ingress routes
	IngressClass string `json:"ingressClass,omitempty"`

	// IngressAnnotations is a set of annotations to be applied to all ingress routes
	IngressAnnotations map[string]string `json:"ingressAnnotations,omitempty"`

	// ImagePullSecrets is a set of image pull secrets to use for all image pulls. These names / secrets
	// must already exist in the namespace in question.
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`

	// VolumeSource is a definition of where volumes should be created from. Usually a site targets a single
	// shared resource (i.e. FSx instance) to provision all of its shared data
	VolumeSource VolumeSource `json:"volumeSource,omitempty"`

	// SharedDirectory is the name of a directory mounted into Workbench and Connect at /mnt/<sharedDirectory>. It should
	// NOT contain any slashes.
	SharedDirectory string `json:"sharedDirectory,omitempty"`

	// VolumeSubdirJobOff turns off the VolumeSubdir provisioning kubernetes job
	VolumeSubdirJobOff bool `json:"volumeSubdirJobOff,omitempty"`

	// ExtraSiteServiceAccounts will be prefixed by "<siteName>-" and created as service accounts in Kubernetes
	ExtraSiteServiceAccounts []ServiceAccountConfig `json:"extraSiteServiceAccounts,omitempty"`

	// Secret configures the secret management for this Site
	Secret SecretConfig `json:"secret,omitempty"`

	// WorkloadSecret configures the managed persistent secret for the entire workload account
	WorkloadSecret SecretConfig `json:"workloadSecret,omitempty"`

	DisablePrePullImages bool `json:"disablePrePullImages,omitempty"`

	// MainDatabaseCredentialSecret configures the secret used for storing the main database credentials
	MainDatabaseCredentialSecret SecretConfig `json:"mainDatabaseCredentialSecret,omitempty"`

	DropDatabaseOnTeardown bool `json:"dropDatabaseOnTearDown,omitempty"`

	Debug     bool              `json:"debug,omitempty"`
	LogFormat product.LogFormat `json:"logFormat,omitempty"`

	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=100
	// +kubebuilder:validation:Type=integer
	NetworkTrust NetworkTrust `json:"networkTrust,omitempty"`

	// PackageManagerUrl specifies the Package Manager URL for Workbench to use
	// If empty, Workbench will use the local Package Manager URL by default
	PackageManagerUrl string `json:"packageManagerUrl,omitempty"`

	// EFSEnabled indicates whether EFS is enabled for this site
	// When true, network policies will allow workbench sessions to access EFS mount targets
	EFSEnabled bool `json:"efsEnabled,omitempty"`

	// VPCCIDR is the CIDR block for the VPC, used for EFS network policies
	VPCCIDR string `json:"vpcCIDR,omitempty"`

	// EnableFQDNHealthChecks controls whether Grafana Alloy generates FQDN-based health check targets
	// for this site's products. When false, only internal cluster health checks are generated.
	// Defaults to true.
	// +kubebuilder:default=true
	EnableFQDNHealthChecks *bool `json:"enableFqdnHealthChecks,omitempty"`
}

type ServiceAccountConfig struct {
	NameSuffix  string            `json:"nameSuffix,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type SiteDomainType string

const (
	SiteSubDomain  SiteDomainType = ""
	SiteDashDomain                = "dash"
)

type AzureFilesConfig struct {
	// StorageClassName is the name of the Kubernetes StorageClass that uses the Azure Files CSI driver
	StorageClassName string `json:"storageClassName,omitempty"`

	// ShareSizeGiB is the size of the Azure File Share to create
	ShareSizeGiB int `json:"shareSizeGiB,omitempty"`
}

type InternalFlightdeckSpec struct {
	// Enabled controls whether Flightdeck is deployed. Defaults to true if not specified.
	// Set to false to explicitly disable Flightdeck deployment.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Image is the container image for Flightdeck.
	// Can be a tag (e.g., "v1.2.3") which will be combined with the default registry,
	// or a full image path (e.g., "my-registry.io/flightdeck:v1.0.0").
	// Defaults to "docker.io/posit/ptd-flightdeck:latest" if not specified.
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullPolicy controls when the kubelet pulls the image
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Replicas is the number of Flightdeck pods to run
	Replicas int `json:"replicas,omitempty"`

	// FeatureEnabler controls which features are enabled in Flightdeck
	FeatureEnabler FeatureEnablerConfig `json:"featureEnabler,omitempty"`

	// LogLevel sets the logging verbosity (debug, info, warn, error)
	// +kubebuilder:default="info"
	LogLevel string `json:"logLevel,omitempty"`

	// LogFormat sets the log output format (text, json)
	// +kubebuilder:default="text"
	LogFormat string `json:"logFormat,omitempty"`
}

type FeatureEnablerConfig struct {
	// ShowConfig enables the configuration page
	// +kubebuilder:default=false
	ShowConfig bool `json:"showConfig,omitempty"`

	// ShowAcademy enables the academy page
	// +kubebuilder:default=false
	ShowAcademy bool `json:"showAcademy,omitempty"`
}

type InternalPackageManagerSpec struct {
	License product.LicenseSpec `json:"license,omitempty"`

	Volume *product.VolumeSpec `json:"volume,omitempty"`

	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	AddEnv map[string]string `json:"addEnv,omitempty"`

	Image string `json:"image,omitempty"`

	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	S3Bucket string `json:"s3Bucket,omitempty"`

	Replicas int `json:"replicas,omitempty"`

	// +kubebuilder:default=packagemanager
	DomainPrefix string `json:"domainPrefix,omitempty"`

	// GitSSHKeys defines SSH key configurations for Git authentication in Package Manager
	// These SSH keys will be made available to Package Manager for Git Builders
	// +optional
	GitSSHKeys []SSHKeyConfig `json:"gitSSHKeys,omitempty"`

	// AzureFiles configures Azure Files integration for persistent storage
	// +optional
	AzureFiles *AzureFilesConfig `json:"azureFiles,omitempty"`
}

type InternalConnectSpec struct {
	License product.LicenseSpec `json:"license,omitempty"`

	Volume *product.VolumeSpec `json:"volume,omitempty"`

	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	Auth AuthSpec `json:"auth,omitempty"`

	AddEnv map[string]string `json:"addEnv,omitempty"`

	Image string `json:"image,omitempty"`

	SessionImage string `json:"sessionImage,omitempty"`

	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	Databricks *DatabricksConfig `json:"databricks,omitempty"`

	LoggedInWarning string `json:"loggedInWarning,omitempty"`

	PublicWarning string `json:"publicWarning,omitempty"`

	Replicas int `json:"replicas,omitempty"`

	ExperimentalFeatures *InternalConnectExperimentalFeatures `json:"experimentalFeatures,omitempty"`

	// +kubebuilder:default=connect
	DomainPrefix string `json:"domainPrefix,omitempty"`

	// GPUSettings allows configuring GPU resource requests and limits
	GPUSettings *GPUSettings `json:"gpuSettings,omitempty"`

	DatabaseSettings *DatabaseSettings `json:"databaseSettings,omitempty"`

	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=0
	ScheduleConcurrency int `json:"scheduleConcurrency,omitempty"`
}

type DatabaseSettings struct {
	Schema                string `json:"schema,omitempty"`
	InstrumentationSchema string `json:"instrumentationSchema,omitempty"`
}

type GPUSettings struct {
	NvidiaGPULimit    int `json:"nvidiaGPULimit,omitempty"`
	MaxNvidiaGPULimit int `json:"maxNvidiaGPULimit,omitempty"`
	AMDGPULimit       int `json:"amdGPULimit,omitempty"`
	MaxAMDGPULimit    int `json:"maxAMDGPULimit,omitempty"`
}

type InternalConnectExperimentalFeatures struct {
	// MailSender enables SMTP and sets the sender email address
	MailSender string `json:"mailSender,omitempty"`

	// MailDisplayName Is the display name of the server in the email subject
	MailDisplayName string `json:"mailDisplayName,omitempty"`

	// MailTarget sets the target email address. Can be useful for avoiding junk mail
	MailTarget string `json:"mailTarget,omitempty"`

	// DsnSecret is a key for the default SiteSecretType to embed in sessions as a DSN (/etc/odbc.ini)
	DsnSecret string `json:"dsnSecret,omitempty"`

	// SessionEnvVars is a map of environment variables to set in the session pods
	SessionEnvVars []corev1.EnvVar `json:"sessionEnvVars,omitempty"`

	// SessionImagePullPolicy is the image pull policy for session pods
	SessionImagePullPolicy corev1.PullPolicy `json:"sessionImagePullPolicy,omitempty"`

	// SessionServiceAccountName is the name of the service account to use as default for ALL session pods.
	// This can be overwritten in the UI
	SessionServiceAccountName string `json:"sessionServiceAccountName,omitempty"`

	// ChronicleSidecarProductApiKeyEnabled assumes the api key for this product has been added to a secret and
	// injects the secret as an environment variable to the sidecar
	ChronicleSidecarProductApiKeyEnabled bool `json:"chronicleSidecarProductApiKeyEnabled,omitempty"`
}

type InternalWorkbenchSpec struct {
	Databricks map[string]DatabricksConfig `json:"databricks,omitempty"`

	Snowflake SnowflakeConfig `json:"snowflake,omitempty"`

	License product.LicenseSpec `json:"license,omitempty"`

	Volume *product.VolumeSpec `json:"volume,omitempty"`

	// AdditionalVolumes represents additional VolumeSpec's that can be defined for Workbench
	AdditionalVolumes []product.VolumeSpec `json:"additionalVolumes,omitempty"`

	// NodeSelector that is applied universally to server and sessions
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations that are applied universally to server and sessions
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// SessionTolerations are tolerations applied only to session pods (not the main workbench server)
	SessionTolerations []corev1.Toleration `json:"sessionTolerations,omitempty"`

	CreateUsersAutomatically bool `json:"createUsersAutomatically,omitempty"`

	// AdminGroups specifies a list of groups that will have admin access to the Workbench administrative dashboard
	// These groups will be joined into a comma-delimited string for the admin-group configuration
	// If not specified, defaults to ["workbench-admin"]
	AdminGroups []string `json:"adminGroups,omitempty"`

	// AdminSuperuserGroups specifies a list of groups that will have superuser access to the Workbench administrative dashboard
	// These groups will be joined into a comma-delimited string for the admin-superuser-group configuration
	// If not specified, no superuser groups will be configured
	AdminSuperuserGroups []string `json:"adminSuperuserGroups,omitempty"`

	AddEnv map[string]string `json:"addEnv,omitempty"`

	Auth AuthSpec `json:"auth,omitempty"`

	Image string `json:"image,omitempty"`

	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	DefaultSessionImage string   `json:"defaultSessionImage,omitempty"`
	ExtraSessionImages  []string `json:"extraSessionImages,omitempty"`

	// SessionInitContainerImageName specifies the init container image name for Workbench sessions
	SessionInitContainerImageName string `json:"sessionInitContainerImageName,omitempty"`

	// SessionInitContainerImageTag specifies the init container image tag for Workbench sessions
	SessionInitContainerImageTag string `json:"sessionInitContainerImageTag,omitempty"`

	Replicas int `json:"replicas,omitempty"`

	// ExperimentalFeatures allows enabling miscellaneous experimental features for workbench
	ExperimentalFeatures *InternalWorkbenchExperimentalFeatures `json:"experimentalFeatures,omitempty"`

	VsCodeExtensions []string `json:"vsCodeExtensions,omitempty"`

	VsCodeUserSettings map[string]*apiextensionsv1.JSON `json:"vsCodeUserSettings,omitempty"`

	PositronSettings PositronConfig `json:"positronConfig,omitempty"`

	VSCodeSettings VSCodeConfig `json:"vsCodeConfig,omitempty"`

	ApiSettings ApiSettingsConfig `json:"apiSettings,omitempty"`

	// +kubebuilder:default=workbench
	DomainPrefix string `json:"domainPrefix,omitempty"`

	// Workbench Auth/Login Landing Page Customization HTML
	AuthLoginPageHtml string `json:"authLoginPageHtml,omitempty"`

	// JupyterConfig contains Jupyter configuration for Workbench
	JupyterConfig *WorkbenchJupyterConfig `json:"jupyterConfig,omitempty"`
}

type InternalWorkbenchExperimentalFeatures struct {
	EnableManagedCredentialJobs bool `json:"enableManagedCredentialJobs,omitempty"`

	// NonRoot enables "maximally rootless" operation (as much as workbench can allow at present)
	NonRoot bool `json:"nonRoot,omitempty"`

	// SessionServiceAccountName is the name of the service account to use for ALL session pods
	SessionServiceAccountName string `json:"sessionServiceAccountName,omitempty"`

	// SessionEnvVars is a map of environment variables to set in the session pods
	SessionEnvVars []corev1.EnvVar `json:"sessionEnvVars,omitempty"`

	// SessionImagePullPolicy is the image pull policy for session pods
	SessionImagePullPolicy corev1.PullPolicy `json:"sessionImagePullPolicy,omitempty"`

	// PrivilegedSessions toggles whether Workbench sessions should be started in "privileged" mode. This is useful
	// if trying to run "docker in docker" for example. Eventually we will work to make "docker in docker" an
	// unprivileged operation as well!
	PrivilegedSessions bool `json:"privilegedSessions,omitempty"`

	// DatabricksForceEnabled enforces that the Databricks pane should be enabled, even if there are no sets of
	// managed credentials in use
	DatabricksForceEnabled bool `json:"databricksForceEnabled,omitempty"`

	// DsnSecret is a key for the default SiteSecretType to embed in sessions as a DSN (/etc/odbc.ini)
	DsnSecret string `json:"dsnSecret,omitempty"`

	// WwwThreadPoolSize is an advanced configuration for the scalability of the Workbench web server. Defaults to 16
	WwwThreadPoolSize *int `json:"wwwThreadPoolSize,omitempty"`

	FirstProjectTemplatePath string `json:"firstProjectTemplatePath,omitempty"`

	LauncherSessionsProxyTimeoutSeconds *int `json:"launcherSessionsProxyTimeoutSecs,omitempty"`

	// VsCodeExtensionsDir is a path to a shared path where extensions (if any) will be sourced from
	VsCodeExtensionsDir string `json:"vsCodeExtensionsDir,omitempty"`

	// ResourceProfiles for use by Workbench. If not provided, a default will be used
	ResourceProfiles map[string]*WorkbenchLauncherKubnernetesResourcesConfigSection `json:"resourceProfiles,omitempty"`

	// CpuRequestRatio defines the ratio of CPU requests to limits for session pods
	// Value must be a decimal number between 0 and 1 (e.g., "0.6" means requests are 60% of limits)
	// Defaults to "0.6" if not specified
	// +kubebuilder:validation:Pattern=`^(0(\.[0-9]+)?|1(\.0+)?)$`
	// +kubebuilder:default="0.6"
	CpuRequestRatio string `json:"cpuRequestRatio,omitempty"`

	// MemoryRequestRatio defines the ratio of memory requests to limits for session pods
	// Value must be a decimal number between 0 and 1 (e.g., "0.8" means requests are 80% of limits)
	// Defaults to "0.8" if not specified
	// +kubebuilder:validation:Pattern=`^(0(\.[0-9]+)?|1(\.0+)?)$`
	// +kubebuilder:default="0.8"
	MemoryRequestRatio string `json:"memoryRequestRatio,omitempty"`

	// SessionSaveActionDefault determines whether .Rdata files should be saved automatically. Default is "no"
	SessionSaveActionDefault SessionSaveAction `json:"sessionSaveActionDefault,omitempty"`

	VsCodePath string `json:"vsCodePath,omitempty"`

	// LauncherEnvPath allows customization of the PATH environment variable for launcher sessions
	LauncherEnvPath string `json:"launcherEnvPath,omitempty"`

	// ChronicleSidecarProductApiKeyEnabled assumes the api key for this product has been added to a secret and
	// injects the secret as an environment variable to the sidecar
	ChronicleSidecarProductApiKeyEnabled bool `json:"chronicleSidecarProductApiKeyEnabled,omitempty"`

	// ForceAdminUiEnabled forces the configuration manager UI to be enabled even when Workbench has it disabled
	// by default when running on Kubernetes
	ForceAdminUiEnabled bool `json:"forceAdminUiEnabled,omitempty"`
}

type InternalChronicleSpec struct {
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	Image string `json:"image,omitempty"`

	AddEnv map[string]string `json:"addEnv,omitempty"`

	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	S3Bucket string `json:"s3Bucket,omitempty"`

	AgentImage string `json:"agentImage,omitempty"`
}

type InternalKeycloakSpec struct {
	Enabled         bool              `json:"enabled,omitempty"`
	Image           string            `json:"image,omitempty"`
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	NodeSelector    map[string]string `json:"nodeSelector,omitempty"`
}

type SnowflakeConfig struct {
	AccountId string `json:"accountId,omitempty"`
	ClientId  string `json:"clientId,omitempty"`
}

type DatabricksConfig struct {
	Name     string `json:"name,omitempty"`
	Url      string `json:"url,omitempty"`
	ClientId string `json:"clientId,omitempty"`
	TenantId string `json:"tenantId,omitempty"`
}

type PositronConfig struct {
	Enabled                      int                              `json:"enabled,omitempty"`
	Exe                          string                           `json:"exe,omitempty"`
	Args                         string                           `json:"args,omitempty"`
	DefaultSessionContainerImage string                           `json:"defaultSessionContainerImage,omitempty"`
	SessionContainerImages       []string                         `json:"sessionContainerImages,omitempty"`
	PositronSessionPath          string                           `json:"positronSessionPath,omitempty"`
	SessionNoProfile             int                              `json:"sessionNoProfile,omitempty"`
	UserDataDir                  string                           `json:"userDataDir,omitempty"`
	AllowFileDownloads           int                              `json:"allowFileDownloads,omitempty"`
	AllowFileUploads             int                              `json:"allowFileUploads,omitempty"`
	SessionTimeoutKillHours      int                              `json:"sessionTimeoutKillHours,omitempty"`
	Extensions                   []string                         `json:"extensions,omitempty"`
	UserSettings                 map[string]*apiextensionsv1.JSON `json:"userSettings,omitempty"`
}

type VSCodeConfig struct {
	Enabled int    `json:"enabled,omitempty"`
	Exe     string `json:"exe,omitempty"`
	Args    string `json:"args,omitempty"`

	//+kubebuilder:default=1
	SessionTimeoutKillHours int `json:"sessionTimeoutKillHours,omitempty"`
}

type ApiSettingsConfig struct {
	WorkbenchApiEnabled           int `json:"workbenchApiEnabled,omitempty"`
	WorkbenchApiAdminEnabled      int `json:"workbenchApiAdminEnabled,omitempty"`
	WorkbenchApiSuperAdminEnabled int `json:"workbenchApiSuperAdminEnabled,omitempty"`
}

// SiteStatus defines the observed state of Site
type SiteStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+genclient
//+k8s:openapi-gen=true

// Site is the Schema for the sites API
type Site struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SiteSpec   `json:"spec,omitempty"`
	Status SiteStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:path=sites

// SiteList contains a list of Site
type SiteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Site `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Site{}, &SiteList{})
}

func (s *Site) GetSecretType() product.SiteSecretType {
	return s.Spec.Secret.Type
}

func (s *Site) OwnerReferencesForChildren() []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: s.APIVersion,
			Kind:       s.Kind,
			Name:       s.Name,
			UID:        s.UID,
		},
	}
}

// SelectorLabels are immutable!
func (s *Site) SelectorLabels() map[string]string {
	return map[string]string{
		ManagedByLabelKey:          ManagedByLabelValue,
		KubernetesNameLabelKey:     "site",
		KubernetesInstanceLabelKey: s.Name,
	}
}

func (s *Site) KubernetesLabels() map[string]string {
	return product.LabelMerge(s.SelectorLabels(), map[string]string{
		SiteLabelKey:      s.Name,
		ComponentLabelKey: "site",
	})
}
