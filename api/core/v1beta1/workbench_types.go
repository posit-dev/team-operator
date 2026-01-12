// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC
//+k8s:openapi-gen=true

package v1beta1

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/rstudio/goex/ptr"
	"github.com/posit-dev/team-operator/api/product"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MaxLoginPageHtmlSize is the maximum allowed size for the login page HTML content (64KB)
const MaxLoginPageHtmlSize = 64 * 1024

// WorkbenchSpec defines the desired state of Workbench
type WorkbenchSpec struct {
	License       product.LicenseSpec    `json:"license,omitempty"`
	Config        WorkbenchConfig        `json:"config,omitempty"`
	SecretConfig  WorkbenchSecretConfig  `json:"secretConfig,omitempty"`
	SessionConfig *product.SessionConfig `json:"sessionConfig,omitempty"`
	Volume        *product.VolumeSpec    `json:"volume,omitempty"`
	SecretType    product.SiteSecretType `json:"secretType,omitempty"`
	Auth          AuthSpec               `json:"auth,omitempty"`

	Url       string `json:"url,omitempty"`
	ParentUrl string `json:"parentUrl,omitempty"`

	// NonRoot is a flag that enables rootless execution for workbench (or as much as is currently possible...)
	NonRoot bool `json:"nonRoot,omitempty"`

	DatabaseConfig PostgresDatabaseConfig `json:"databaseConfig,omitempty"`

	// IngressClass is the ingress class to be used when creating ingress routes
	IngressClass string `json:"ingressClass,omitempty"`

	// IngressAnnotations is a set of annotations to be applied to all ingress routes
	IngressAnnotations map[string]string `json:"ingressAnnotations,omitempty"`

	// ImagePullSecrets is a set of image pull secrets to use for all image pulls. These names / secrets
	// must already exist in the namespace in question.
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`

	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// AddEnv adds arbitrary environment variables to the container env
	AddEnv map[string]string `json:"addEnv,omitempty"`

	OffHostExecution bool `json:"offHostExecution,omitempty"`

	Image string `json:"image,omitempty"`

	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Sleep puts the service to sleep... so you can debug a crash looping container / etc. It is an ugly escape hatch,
	// but can also be useful on occasion
	Sleep bool `json:"sleep,omitempty"`

	Snowflake SnowflakeConfig `json:"snowflake,omitempty"`

	// AwsAccountId is the account Id for this AWS Account. It is used to create EKS-to-IAM annotations
	AwsAccountId string `json:"awsAccountId,omitempty"`

	// ClusterDate is the date id (YYYYmmdd) for the cluster. It is used to create EKS-to-IAM annotations
	ClusterDate string `json:"clusterDate,omitempty"`

	// WorkloadCompoundName is the name for the workload
	WorkloadCompoundName string `json:"workloadCompoundName,omitempty"`

	// ChronicleAgentImage is the image used for the Chronicle Agent
	ChronicleAgentImage string `json:"chronicleImage,omitempty"`

	// AdditionalVolumes represents additional VolumeSpec's that can be defined
	AdditionalVolumes []product.VolumeSpec `json:"additionalVolumes,omitempty"`

	// Secret configures the secret management for this Workbench
	Secret SecretConfig `json:"secret,omitempty"`

	// WorkloadSecret configures the managed persistent secret for the entire workload account
	WorkloadSecret SecretConfig `json:"workloadSecret,omitempty"`

	// MainDatabaseCredentialSecret configures the secret used for storing the main database credentials
	MainDatabaseCredentialSecret SecretConfig `json:"mainDatabaseCredentialSecret,omitempty"`

	Replicas int `json:"replicas,omitempty"`

	// DsnSecret is the name of the secret that contains the DSN to include with all Workbench sessions
	DsnSecret string `json:"dsnSecret,omitempty"`

	// ChronicleSidecarProductApiKeyEnabled assumes the api key for this product has been added to a secret and
	// injects the secret as an environment variable to the sidecar. **EXPERIMENTAL**
	ChronicleSidecarProductApiKeyEnabled bool `json:"chronicleSidecarProductApiKeyEnabled,omitempty"`

	// AuthLoginPageHtml is the custom HTML content to be displayed on the Workbench login page.
	// This content will be mounted at /etc/rstudio/login.html in the Workbench pod.
	// The HTML content must be valid and complete HTML and less than 65,536 bytes (64KB) in size.
	// Empty or whitespace-only content will be ignored.
	// See: https://docs.posit.co/ide/server-pro/admin/authenticating_users/customizing_signin.html
	AuthLoginPageHtml string `json:"authLoginPageHtml,omitempty"`
}

// TODO: Validation should require Volume definition for off-host-execution...

// WorkbenchStatus defines the observed state of Workbench
type WorkbenchStatus struct {
	Ready        bool                   `json:"ready"`
	KeySecretRef corev1.SecretReference `json:"keySecretRef,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName={wb,wbs},path=workbenches,singular=workbench
//+genclient
//+k8s:openapi-gen=true

// Workbench is the Schema for the workbenches API
type Workbench struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkbenchSpec   `json:"spec,omitempty"`
	Status WorkbenchStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WorkbenchList contains a list of Workbench
type WorkbenchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workbench `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Workbench{}, &WorkbenchList{})
}

func (w *Workbench) OwnerReferencesForChildren() []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: w.APIVersion,
			Kind:       w.Kind,
			Name:       w.Name,
			UID:        w.UID,
		},
	}
}

func (w *Workbench) ComponentName() string {
	return fmt.Sprintf("%s-workbench", w.Name)
}

func (w *Workbench) SiteName() string {
	return w.Name
}

func (w *Workbench) SupervisorConfigmapName() string {
	return fmt.Sprintf("%s-supervisor", w.ComponentName())
}

func (w *Workbench) LoginConfigmapName() string {
	return fmt.Sprintf("%s-login", w.ComponentName())
}

// AuthLoginPageHtmlConfigmapName returns the name of the ConfigMap containing
// custom HTML content for the Workbench login page
func (w *Workbench) AuthLoginPageHtmlConfigmapName() string {
	return fmt.Sprintf("%s-auth-login-page-html", w.ComponentName())
}

func (w *Workbench) WorkloadCompoundName() string {
	return w.Spec.WorkloadCompoundName
}

func (w *Workbench) GetChronicleUrl() string {
	return fmt.Sprintf("http://%s-chronicle.%s.svc.cluster.local", w.Name, w.Namespace)
}

func (w *Workbench) GetChronicleAgentImage() string {
	return w.Spec.ChronicleAgentImage
}

func (w *Workbench) ServiceUrl(proto string, ns string) string {
	return fmt.Sprintf("%s%s.%s.svc.cluster.local:80", proto, w.ComponentName(), ns)
}

func (w *Workbench) KeySecretName() string {
	return fmt.Sprintf("%s-key", w.ComponentName())
}

func (w *Workbench) SessionConfigMapName() string {
	return fmt.Sprintf("%s-session", w.ComponentName())
}

func (w *Workbench) SessionServiceAccountName() string {
	return fmt.Sprintf("%s-session", w.ComponentName())
}

func (w *Workbench) SelectorLabels() map[string]string {
	return map[string]string{
		ManagedByLabelKey:          ManagedByLabelValue,
		KubernetesNameLabelKey:     "workbench",
		KubernetesInstanceLabelKey: w.ComponentName(),
	}
}

func (w *Workbench) KubernetesLabels() map[string]string {
	return product.LabelMerge(w.SelectorLabels(), map[string]string{
		SiteLabelKey:      w.Name,
		ComponentLabelKey: "workbench",
	})
}

func (w *Workbench) SecretProviderClassName() string {
	return fmt.Sprintf("%s-secrets", w.ComponentName())
}

func (w *Workbench) secretProviderClassVolumeSource() *corev1.VolumeSource {
	return &corev1.VolumeSource{
		CSI: &corev1.CSIVolumeSource{
			Driver:   "secrets-store.csi.k8s.io",
			ReadOnly: ptr.To(true),
			FSType:   nil,
			VolumeAttributes: map[string]string{
				"secretProviderClass": w.SecretProviderClassName(),
			},
			NodePublishSecretRef: nil,
		},
	}
}

func (w *Workbench) GetLicenseSpec() product.LicenseSpec {
	return w.Spec.License
}

func (w *Workbench) GetLicenseConstants() product.LicenseConstants {
	return product.LicenseConstants{
		Key:           "RSW_LICENSE",
		FilePathKey:   "RSW_LICENSE_FILE_PATH",
		FilePath:      "/etc/rstudio-server/license.lic",
		LicenseUrlKey: "RSW_LICENSE_FILE_URL",
	}
}

func (w *Workbench) DsnSecret() string {
	return w.Spec.DsnSecret
}

func (w *Workbench) SessionConfig() *product.SessionConfig {
	return w.Spec.SessionConfig
}

func (w *Workbench) ShortName() string {
	return "dev"
}
func (w *Workbench) GetClusterDate() string {
	return w.Spec.ClusterDate
}

func (w *Workbench) GetSecretType() product.SiteSecretType {
	return w.Spec.Secret.Type
}

func (w *Workbench) GetSecretVaultName() string {
	return w.Spec.Secret.VaultName
}

func (w *Workbench) GetAwsAccountId() string {
	return w.Spec.AwsAccountId
}

func (w *Workbench) CreateSessionVolumeFactory(cfg *WorkbenchConfig) *product.VolumeFactory {
	vols := map[string]*product.VolumeDef{}

	// NOTE: this is duplicated from the Server VolumeFactory
	// TODO: a way to consolidate the Session and Server volumes
	for _, v := range w.Spec.AdditionalVolumes {
		vols["addl-"+v.PvcName] = &product.VolumeDef{
			Name: v.PvcName,
			Source: &corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: v.PvcName,
					ReadOnly:  v.ReadOnly,
				},
			},
			Mounts: []*product.VolumeMountDef{
				{MountPath: v.MountPath},
			},
		}
	}

	vols["home-dir-volume"] = w.homeDirVolumeDef()
	vols["server-shared-storage-volume"] = w.serverSharedStorageVolumeDef()
	vols["etc-cover-volume"] = w.etcCoverVolumeDef()
	vols["login-volume"] = w.loginVolumeDef()

	sessionMounts := []*product.VolumeMountDef{
		{MountPath: "/mnt/session/rstudio/", ReadOnly: true},
	}

	if cfg.WorkbenchSessionNewlineConfig.VsCodeExtensionsConf != nil {
		sessionMounts = append(sessionMounts, &product.VolumeMountDef{
			MountPath: "/etc/rstudio/vscode.extensions.conf",
			SubPath:   "vscode.extensions.conf",
			ReadOnly:  true,
		})
	}

	if cfg.WorkbenchSessionNewlineConfig.PositronExtensionsConf != nil {
		sessionMounts = append(sessionMounts, &product.VolumeMountDef{
			MountPath: "/etc/rstudio/positron.extensions.conf",
			SubPath:   "positron.extensions.conf",
			ReadOnly:  true,
		})
	}

	vols["session-config-volume"] = &product.VolumeDef{
		Source: &corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: w.SessionConfigMapName(),
				},
				Items:       nil,
				DefaultMode: nil,
				Optional:    nil,
			},
		},
		Mounts: sessionMounts,
	}

	if w.Spec.DsnSecret != "" {
		switch w.Spec.SecretType {
		case product.SiteSecretAws:
			vols["dsn-volume"] = &product.VolumeDef{
				Source: w.secretProviderClassVolumeSource(),
				Mounts: []*product.VolumeMountDef{
					{MountPath: "/etc/odbc.ini", SubPath: "odbc.ini", ReadOnly: true},
				},
			}
		case product.SiteSecretKubernetes:
			vols["dsn-volume"] = &product.VolumeDef{
				Source: &corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: w.Spec.Secret.VaultName,
						Items: []corev1.KeyToPath{
							{Key: w.Spec.DsnSecret, Path: "odbc.ini"},
						},
						DefaultMode: ptr.To(product.MustParseOctal("0655")),
					},
				},
				Mounts: []*product.VolumeMountDef{
					{MountPath: "/etc/odbc.ini", SubPath: "odbc.ini", ReadOnly: true},
				},
			}
		}
	}
	return &product.VolumeFactory{
		Vols: vols,
	}
}

// InitializeNonRootSupervisorConfig modifies the provided config object in place. If w.Spec.NonRoot,
// then "maximally non-root" supervisor config that is supported by the current product version will be used
//
// NOTE: there are some sanity checks for duplication / etc. to avoid conflicts with a provided config
func (w *Workbench) InitializeNonRootSupervisorConfig(ctx context.Context, config *WorkbenchConfig) error {
	// "usually" do nothing...
	if !w.Spec.NonRoot {
		return nil
	}
	// else... if nonRoot is configured, we customize startup!

	l := getLoggerWithFallback(ctx, "workbench").WithValues("workbench-config", "supervisord-config-init")
	// check for conflicts in keys already provided... this would be bad... (i.e. start launcher twice / etc.)
	for key, obj := range config.SupervisordIniConfig.Programs {
		if key == "launcher.conf" || key == "rstudio-launcher.conf" || strings.Contains(key, "launcher") {
			// bail out! there is already a launcher
			l.Error(errors.New("duplicate launcher supervisord config file found"), "skipping custom NonRoot startup config", "key", key)
			return nil
		}
		for prog, _ := range obj {
			if prog == "rstudio-launcher" || prog == "launcher" || strings.Contains(prog, "launcher") {
				// bail out! there is already a launcher
				l.Error(errors.New("duplicate launcher program found"), "skipping custom NonRoot startup config", "key", key)
				return nil
			}
		}

	}

	// no problems found... go ahead with config modification!
	if config.SupervisordIniConfig.Programs == nil {
		config.SupervisordIniConfig.Programs = map[string]map[string]*SupervisordProgramConfig{}
	}
	config.SupervisordIniConfig.Programs["launcher.conf"] = map[string]*SupervisordProgramConfig{
		"rstudio-launcher": {
			User:                  "root",
			Command:               "/usr/lib/rstudio-server/bin/rstudio-launcher",
			AutoRestart:           false,
			NumProcs:              1,
			StdOutLogFile:         "/dev/stdout",
			StdOutLogFileMaxBytes: 0,
			StdErrLogFile:         "/dev/stderr",
			StdErrLogFileMaxBytes: 0,
			Environment:           `XDG_CONFIG_DIRS="/mnt/config/rstudio:/mnt/config:/mnt/secure:/mnt/secure-config:/mnt/session"`,
		},
	}
	return nil
}
func (w *Workbench) supervisorVolumeDef() *product.VolumeDef {
	return &product.VolumeDef{
		Source: &corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: w.SupervisorConfigmapName(),
				},
				DefaultMode: ptr.To(product.MustParseOctal("0644")),
			},
		},
		Mounts: []*product.VolumeMountDef{
			{MountPath: "/startup/custom/"},
		},
	}
}

func (w *Workbench) supervisorCoverVolumeDef() *product.VolumeDef {
	return &product.VolumeDef{
		Source: &corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
		Mounts: []*product.VolumeMountDef{
			// today we cover just the launcher.conf directory...
			{MountPath: "/startup/launcher"},
			// future paths for complete control:
			// /startup/base
			// /startup/user-provisioning
			// /startup/custom is used above
			// ... or we can overwrite the whole /etc/supervisor/supervisord.conf file
		},
	}
}

func (w *Workbench) homeDirVolumeDef() *product.VolumeDef {
	return &product.VolumeDef{
		Source: &corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: w.ComponentName(),
				ReadOnly:  false,
			},
		},
		Mounts: []*product.VolumeMountDef{
			{MountPath: "/home"},
		},
	}
}

func (w *Workbench) serverSharedStorageVolumeDef() *product.VolumeDef {
	return &product.VolumeDef{
		Source: &corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: fmt.Sprintf("%s-shared-storage", w.ComponentName()),
				ReadOnly:  false,
			},
		},
		Mounts: []*product.VolumeMountDef{
			{MountPath: "/mnt/shared-storage"},
		},
	}
}

func (w *Workbench) etcCoverVolumeDef() *product.VolumeDef {
	return &product.VolumeDef{
		Source: &corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
		Mounts: []*product.VolumeMountDef{
			{MountPath: "/etc/rstudio"},
		},
	}
}

func (w *Workbench) loginVolumeDef() *product.VolumeDef {
	return &product.VolumeDef{
		Source: &corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: w.LoginConfigmapName(),
				},
				DefaultMode: ptr.To(product.MustParseOctal("0644")),
			},
		},
		Mounts: []*product.VolumeMountDef{
			{
				MountPath: "/etc/login.defs",
				SubPath:   "login.defs",
				ReadOnly:  true,
			},
			{
				MountPath: "/etc/profile.d/99-ptd.sh",
				SubPath:   "99-ptd.sh",
				ReadOnly:  true,
			},
			{
				MountPath: "/etc/pam.d/common-session",
				SubPath:   "common-session",
				ReadOnly:  true,
			},
		},
	}
}

// AuthLoginPageHtmlVolumeDef returns a VolumeDef for mounting the custom login page HTML
// at /etc/rstudio/login.html in the Workbench pod
func (w *Workbench) AuthLoginPageHtmlVolumeDef() *product.VolumeDef {
	return &product.VolumeDef{
		Source: &corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: w.AuthLoginPageHtmlConfigmapName(),
				},
				DefaultMode: ptr.To(product.MustParseOctal("0644")),
			},
		},
		Mounts: []*product.VolumeMountDef{
			{
				MountPath: "/etc/rstudio/login.html",
				SubPath:   "login.html",
				ReadOnly:  true,
			},
		},
	}
}

func (w *Workbench) CreateVolumeFactory(cfg *WorkbenchConfig) *product.VolumeFactory {
	configVolumeMounts := []*product.VolumeMountDef{
		{
			MountPath: "/mnt/config/rstudio/",
			ReadOnly:  true,
		},
	}
	// if configured... mount workbench_nss.conf
	if cfg.WorkbenchSessionIniConfig.WorkbenchNss != nil && cfg.WorkbenchSessionIniConfig.WorkbenchNss.ServerAddress != "" {
		configVolumeMounts = append(
			configVolumeMounts,
			&product.VolumeMountDef{
				MountPath: "/etc/rstudio/workbench_nss.conf",
				SubPath:   "workbench_nss.conf",
				ReadOnly:  true,
			},
		)
	}

	if cfg.WorkbenchSessionIniConfig.Positron != nil {
		configVolumeMounts = append(
			configVolumeMounts,
			&product.VolumeMountDef{
				MountPath: "/etc/rstudio/positron.conf",
				SubPath:   "positron.conf",
				ReadOnly:  true,
			},
		)
	}

	if cfg.WorkbenchSessionJsonConfig.PositronUserSettingsJson != nil {
		configVolumeMounts = append(
			configVolumeMounts,
			&product.VolumeMountDef{
				MountPath: "/etc/rstudio/positron-user-settings.json",
				SubPath:   "positron-user-settings.json",
				ReadOnly:  true,
			},
		)
	}

	if cfg.WorkbenchDcfConfig.LauncherEnv != nil {
		configVolumeMounts = append(
			configVolumeMounts,
			&product.VolumeMountDef{
				MountPath: "/etc/rstudio/launcher-env",
				SubPath:   "launcher-env",
				ReadOnly:  true,
			},
		)
	}

	vols := map[string]*product.VolumeDef{
		"etc-cover-volume": w.etcCoverVolumeDef(),
		"login-volume":     w.loginVolumeDef(),
		"config-volume": {
			Source: &corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: w.ComponentName(),
					},
					Items:       nil,
					DefaultMode: nil,
					Optional:    nil,
				},
			},
			Mounts: configVolumeMounts,
			Env: []corev1.EnvVar{
				{
					Name:  "XDG_CONFIG_DIRS",
					Value: "/mnt/config/rstudio:/mnt/config:/mnt/secure:/mnt/secure-config:/mnt/session",
				},
				{
					Name: "RSW_TESTUSER_PASSWD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: w.KeySecretName(),
							},
							Key: "testuser-password",
						},
					},
				},
			},
		},
	}

	// NOTE: cannot create volumes here or we risk creating a circular reference with the Reconciler...
	// TODO: maybe we push this whole factory down into the controller eventually
	for _, v := range w.Spec.AdditionalVolumes {
		vols["addl-"+v.PvcName] = &product.VolumeDef{
			Name: v.PvcName,
			Source: &corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: v.PvcName,
					ReadOnly:  v.ReadOnly,
				},
			},
			Mounts: []*product.VolumeMountDef{
				{MountPath: v.MountPath},
			},
		}
	}

	// supervisor volume (conditional)
	if w.Spec.NonRoot {
		vols["supervisor-volume"] = w.supervisorVolumeDef()
		vols["supervisor-cover-volume"] = w.supervisorCoverVolumeDef()
	}

	// home dir volume (conditional)
	if w.Spec.Volume != nil {
		vols["home-dir-volume"] = w.homeDirVolumeDef()
		vols["server-shared-storage-volume"] = w.serverSharedStorageVolumeDef()
	}

	// Auth login page HTML volume (conditional)
	if w.Spec.AuthLoginPageHtml != "" {
		vols["auth-login-page-html-volume"] = w.AuthLoginPageHtmlVolumeDef()
	}

	// Off-Host Execution Volume (conditional)
	if w.Spec.OffHostExecution {
		vols["template-config"] = &product.VolumeDef{
			Source: &corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: w.TemplateConfigMapName(),
					},
					Items:       nil,
					DefaultMode: ptr.To(product.MustParseOctal("0644")),
				},
			},
			Mounts: []*product.VolumeMountDef{
				{
					MountPath: "/var/lib/rstudio-launcher/Kubernetes/rstudio-library-templates-data.tpl",
					SubPath:   "rstudio-library-templates-data.tpl",
					ReadOnly:  true,
				},
				{
					MountPath: "/var/lib/rstudio-launcher/Kubernetes/job.tpl",
					SubPath:   "job.tpl",
					ReadOnly:  true,
				},
				{
					MountPath: "/var/lib/rstudio-launcher/Kubernetes/service.tpl",
					SubPath:   "service.tpl",
					ReadOnly:  true,
				},
			},
		}
	}

	return &product.VolumeFactory{
		Vols: vols,
	}
}

func (w *Workbench) CreateSecretVolumeFactory() *product.SecretVolumeFactory {

	vols := map[string]*product.VolumeDef{}

	// this will pivot based on secret type and license type
	vols["license-volume"] = product.LicenseVolumeDefsFromProduct(w)

	vols["license-emptydir"] = product.EmptyDirVolumeDef("/etc/rstudio-server")

	// secure-cookie-key and launcher secrets
	// TODO: should this use the CSI...?
	vols["key-volume"] = &product.VolumeDef{
		Source: &corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				// TODO: where does this secret come from...?
				SecretName: w.KeySecretName(),
				Items: []corev1.KeyToPath{
					{Key: "key", Path: "secure-cookie-key", Mode: ptr.To(product.MustParseOctal("0600"))},
					{Key: "launcher.pem", Path: "launcher.pem", Mode: ptr.To(product.MustParseOctal("0600"))},
					{Key: "launcher.pub", Path: "launcher.pub", Mode: ptr.To(product.MustParseOctal("0600"))},
				},
				DefaultMode: ptr.To(product.MustParseOctal("0600")),
			},
		},
		Mounts: []*product.VolumeMountDef{
			{MountPath: "/mnt/secure/rstudio/", ReadOnly: true},
		},
	}

	// generic secret / secure config
	vols["secret-config"] = &product.VolumeDef{
		Source: &corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  fmt.Sprintf("%s-config", w.ComponentName()),
				Items:       nil,
				DefaultMode: nil,
				Optional:    nil,
			},
		},
		Mounts: []*product.VolumeMountDef{
			{MountPath: "/mnt/secure-config/rstudio/", ReadOnly: true},
		},
	}

	// case-by-case volumes based on secret type...
	if w.GetSecretType() == product.SiteSecretAws {
		mountDefs := []*product.VolumeMountDef{}
		if w.Spec.Auth.Type == AuthTypeOidc {
			mountDefs = product.ConcatLists(
				mountDefs,
				[]*product.VolumeMountDef{
					{MountPath: "/etc/rstudio/admin_token", SubPath: "admin_token", ReadOnly: true},
					{MountPath: "/etc/rstudio/user_token", SubPath: "user_token", ReadOnly: true},
				},
			)
		}
		vols["secret-volume"] = &product.VolumeDef{
			Source: w.secretProviderClassVolumeSource(),
			Mounts: mountDefs,
		}

		secretName := fmt.Sprintf("%s-secret", w.ComponentName())
		if w.Spec.Snowflake.AccountId != "" && w.Spec.Snowflake.ClientId != "" {
			vols["snowflake-volume"] = &product.VolumeDef{
				Env: []corev1.EnvVar{
					{Name: "SNOWFLAKE_ACCOUNT", Value: w.Spec.Snowflake.AccountId},
					{Name: "SNOWFLAKE_CLIENT_ID", Value: w.Spec.Snowflake.ClientId},
					{Name: "SNOWFLAKE_CLIENT_SECRET", ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
							Key:                  "snowflake-client-secret",
						},
					}},
				},
			}
		}

		vols["db-secret-volume"] = &product.VolumeDef{
			Env: []corev1.EnvVar{
				{Name: "WORKBENCH_POSTGRES_PASSWORD", ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  "dev-db-password",
					},
				}},
			},
		}

		awsSecretFactory := &product.SecretVolumeFactory{
			Vols: vols,
			CsiEntries: map[string]*product.CSIDef{
				"secret-volume": {
					Driver:   "secrets-store.csi.k8s.io",
					ReadOnly: ptr.To(true),
					VolumeAttributes: map[string]string{
						"secretProviderClass": w.SecretProviderClassName(),
					},
				},
			},
		}

		return awsSecretFactory
	} else if w.GetSecretType() == product.SiteSecretKubernetes {
		keyToPaths := []corev1.KeyToPath{}
		mountDefs := []*product.VolumeMountDef{}
		if w.Spec.Auth.Type == AuthTypeOidc {
			// rename the keys from the secret to the usable filenames
			keyToPaths = product.ConcatLists(
				keyToPaths,
				[]corev1.KeyToPath{
					{Key: "dev-admin-token", Path: "admin_token"},
					{Key: "dev-user-token", Path: "user_token"},
				})
			mountDefs = product.ConcatLists(
				mountDefs,
				[]*product.VolumeMountDef{
					{MountPath: "/etc/rstudio/admin_token", SubPath: "admin_token", ReadOnly: true},
					{MountPath: "/etc/rstudio/user_token", SubPath: "user_token", ReadOnly: true},
				},
			)
		}
		if w.Spec.Snowflake.AccountId != "" && w.Spec.Snowflake.ClientId != "" {
			vols["snowflake-volume"] = &product.VolumeDef{
				Env: []corev1.EnvVar{
					{Name: "SNOWFLAKE_ACCOUNT", Value: w.Spec.Snowflake.AccountId},
					{Name: "SNOWFLAKE_CLIENT_ID", Value: w.Spec.Snowflake.ClientId},
					{Name: "SNOWFLAKE_CLIENT_SECRET", ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: w.Spec.Secret.VaultName},
							Key:                  "snowflake-client-secret",
						},
					}},
				},
			}
		}
		vols["secret-volume"] = &product.VolumeDef{
			Source: &corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: w.Spec.Secret.VaultName,
					Items:      keyToPaths,
				},
			},
			Mounts: mountDefs,
		}

		vols["db-secret-volume"] = &product.VolumeDef{
			Env: []corev1.EnvVar{
				{Name: "WORKBENCH_POSTGRES_PASSWORD", ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: w.Spec.Secret.VaultName},
						Key:                  "dev-db-password",
					},
				}},
			},
		}

		k8sSecretFactory := &product.SecretVolumeFactory{
			Vols: vols,
		}
		return k8sSecretFactory
	}

	// do nothing...
	return &product.SecretVolumeFactory{}
}

func (w *Workbench) TemplateConfigMapName() string {
	return fmt.Sprintf("%s-templates", w.ComponentName())
}

func (w *Workbench) defaultSessionLabels() map[string]string {
	return map[string]string{
		SiteLabelKey:      w.Name,
		ComponentLabelKey: "workbench-session",
	}
}

func (w *Workbench) createDefaultSessionConfig(vf *product.VolumeFactory) *product.SessionConfig {
	// IMPORTANT NOTE: if you change defaults here... they may not always be applied.
	//   The defaults are "merged" in manually in the SessionConfigTemplateData method below
	return &product.SessionConfig{
		Service: &product.ServiceConfig{
			Type:        "ClusterIP",
			Annotations: nil,
			Labels:      w.defaultSessionLabels(),
		},
		Pod: &product.PodConfig{
			Annotations:        nil,
			Labels:             w.defaultSessionLabels(),
			ServiceAccountName: w.SessionServiceAccountName(),
			Volumes: product.ConcatLists(
				vf.Volumes(),
				[]corev1.Volume{},
			),
			VolumeMounts: product.ConcatLists(
				vf.VolumeMounts(),
				[]corev1.VolumeMount{},
			),
			Env: product.ConcatLists(
				vf.EnvVars(),
				[]corev1.EnvVar{},
			),
			ImagePullPolicy:          "",
			ImagePullSecrets:         nil,
			InitContainers:           nil,
			ExtraContainers:          nil,
			ContainerSecurityContext: corev1.SecurityContext{},
			DefaultSecurityContext:   corev1.SecurityContext{},
			SecurityContext:          corev1.SecurityContext{},
			Tolerations:              nil,
			Affinity:                 nil,
			NodeSelector:             nil,
			PriorityClassName:        "",
			Command:                  nil,
		},
		Job: &product.JobConfig{
			Labels: w.defaultSessionLabels(),
		},
	}
}

func (w *Workbench) SessionConfigTemplateData(l logr.Logger, cfg *WorkbenchConfig) string {
	// TODO: we could also probably combine this method with the Connect analog
	sessionVolumes := w.CreateSessionVolumeFactory(cfg)
	defaultSessionConfig := w.createDefaultSessionConfig(sessionVolumes)
	sess := &product.SessionConfig{}
	if w.Spec.SessionConfig != nil {
		sess = w.Spec.SessionConfig

		// merge in the defaults manually...

		// Service Type
		if sess.Service == nil {
			sess.Service = defaultSessionConfig.Service
		} else {
			if sess.Service.Type == "" {
				sess.Service.Type = defaultSessionConfig.Service.Type
			}
			sess.Service.Labels = product.LabelMerge(sess.Service.Labels, defaultSessionConfig.Service.Labels)
		}

		// Pod Volumes
		if sess.Pod == nil {
			sess.Pod = defaultSessionConfig.Pod
		} else {
			// fill in volumes
			// TODO: yes there can be a naming conflict... we are ignoring that possibility for now
			sess.Pod.Volumes = product.ConcatLists(
				sess.Pod.Volumes,
				defaultSessionConfig.Pod.Volumes,
			)
			sess.Pod.VolumeMounts = product.ConcatLists(
				sess.Pod.VolumeMounts,
				defaultSessionConfig.Pod.VolumeMounts,
			)

			// and labels
			sess.Pod.Labels = product.LabelMerge(sess.Pod.Labels, defaultSessionConfig.Pod.Labels)
		}

		// Job Config (empty)
		if sess.Job == nil {
			sess.Job = defaultSessionConfig.Job
		} else {
			sess.Job.Labels = product.LabelMerge(sess.Job.Labels, defaultSessionConfig.Job.Labels)
		}
	} else {
		// use the default
		sess = defaultSessionConfig
	}

	if str, err := sess.GenerateSessionConfigTemplate(); err != nil {
		l.Error(err, "Error generating session config template")
		return ""
	} else {
		return str
	}
}
