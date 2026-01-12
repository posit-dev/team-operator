// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

// +k8s:openapi-gen=true

package v1beta1

import (
	"context"
	"fmt"

	"github.com/rstudio/goex/ptr"
	"github.com/posit-dev/team-operator/api/product"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

// ConnectSpec defines the desired state of Connect
type ConnectSpec struct {
	License       product.LicenseSpec    `json:"license,omitempty"`
	Config        ConnectConfig          `json:"config,omitempty"`
	SessionConfig *product.SessionConfig `json:"sessionConfig,omitempty"`
	Volume        *product.VolumeSpec    `json:"volume,omitempty"`
	SecretType    product.SiteSecretType `json:"secretType,omitempty"`
	Auth          AuthSpec               `json:"auth,omitempty"`

	Url string `json:"url,omitempty"`

	DatabaseConfig PostgresDatabaseConfig `json:"databaseConfig,omitempty"`

	// IngressClass is the ingress class to be used when creating ingress routes
	IngressClass string `json:"ingressClass,omitempty"`

	// IngressAnnotations is a set of annotations to be applied to all ingress routes
	IngressAnnotations map[string]string `json:"ingressAnnotations,omitempty"`

	// ImagePullSecrets is a set of image pull secrets to use for all image pulls. These names / secrets
	// must already exist in the namespace in question.
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`

	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// AddEnv adds arbitrary environment variables to the container env
	AddEnv map[string]string `json:"addEnv,omitempty"`

	OffHostExecution bool `json:"offHostExecution,omitempty"`

	Image string `json:"image,omitempty"`

	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Sleep puts the service to sleep... so you can debug a crash looping container / etc. It is an ugly escape hatch,
	// but can also be useful on occasion
	Sleep bool `json:"sleep,omitempty"`

	SessionImage string `json:"sessionImage,omitempty"`

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

	// Secret configures the secret management for this Connect
	Secret SecretConfig `json:"secret,omitempty"`

	// WorkloadSecret configures the managed persistent secret for the entire workload account
	WorkloadSecret SecretConfig `json:"workloadSecret,omitempty"`

	// MainDatabaseCredentialSecret configures the secret used for storing the main database credentials
	MainDatabaseCredentialSecret SecretConfig `json:"mainDatabaseCredentialSecret,omitempty"`

	// Debug sets whether to enable debug settings. This setting overrides specific "Config.Logging" sections globally
	Debug bool `json:"debug,omitempty"`

	Replicas int `json:"replicas,omitempty"`

	// DsnSecret is the name of the secret that contains the DSN to include with all Connect sessions
	DsnSecret string `json:"dsnSecret,omitempty"`

	// ChronicleSidecarProductApiKeyEnabled assumes the api key for this product has been added to a secret and
	// injects the secret as an environment variable to the sidecar. **EXPERIMENTAL**
	ChronicleSidecarProductApiKeyEnabled bool `json:"chronicleSidecarProductApiKeyEnabled,omitempty"`
}

// TODO: Validation should require Volume definition for off-host-execution...

// TODO: if config changes, then we should roll over the deployment!

// ConnectStatus defines the observed state of Connect
type ConnectStatus struct {
	KeySecretRef corev1.SecretReference `json:"keySecretRef,omitempty"`
	Ready        bool                   `json:"ready"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName={con,cons},path=connects
//+genclient
//+k8s:openapi-gen=true

// Connect is the Schema for the connects API
type Connect struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConnectSpec   `json:"spec,omitempty"`
	Status ConnectStatus `json:"status,omitempty"`
}

// ConnectList contains a list of Connect
// +kubebuilder:object:root=true
type ConnectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Connect `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Connect{}, &ConnectList{})
}

func (c *Connect) ComponentName() string {
	return fmt.Sprintf("%s-connect", c.Name)
}

func (c *Connect) SiteName() string { return c.Name }

func (c *Connect) WorkloadCompoundName() string {
	return c.Spec.WorkloadCompoundName
}

func (c *Connect) GetChronicleUrl() string {
	return fmt.Sprintf("http://%s-chronicle.%s.svc.cluster.local", c.Name, c.Namespace)
}

func (c *Connect) GetChronicleAgentImage() string {
	return c.Spec.ChronicleAgentImage
}

func (c *Connect) KeySecretName() string {
	return fmt.Sprintf("%s-key", c.ComponentName())
}

// SelectorLabels are immutable!
func (c *Connect) SelectorLabels() map[string]string {
	return map[string]string{
		ManagedByLabelKey:          ManagedByLabelValue,
		KubernetesNameLabelKey:     "connect",
		KubernetesInstanceLabelKey: c.ComponentName(),
	}
}

func (c *Connect) KubernetesLabels() map[string]string {
	return product.LabelMerge(c.SelectorLabels(), map[string]string{
		SiteLabelKey:      c.Name,
		ComponentLabelKey: "connect",
	})
}

func (c *Connect) DsnSecret() string {
	return c.Spec.DsnSecret
}

func (c *Connect) SessionConfig() *product.SessionConfig {
	return c.Spec.SessionConfig
}

func (c *Connect) SessionServiceAccountName() string {
	return fmt.Sprintf("%s-session", c.ComponentName())
}

func (c *Connect) GetLicenseSpec() product.LicenseSpec {
	return c.Spec.License
}

func (c *Connect) GetLicenseConstants() product.LicenseConstants {
	return product.LicenseConstants{
		Key:           "RSC_LICENSE",
		FilePathKey:   "RSC_LICENSE_FILE_PATH",
		FilePath:      "/etc/rstudio-connect/license.lic",
		LicenseUrlKey: "RSC_LICENSE_FILE_URL",
	}
}

func (c *Connect) ShortName() string {
	return "pub"
}

func (c *Connect) GetSecretType() product.SiteSecretType {
	return c.Spec.Secret.Type
}

func (c *Connect) GetSecretVaultName() string {
	return c.Spec.Secret.VaultName
}

func (c *Connect) GetClusterDate() string {
	return c.Spec.ClusterDate
}

func (c *Connect) GetAwsAccountId() string {
	return c.Spec.AwsAccountId
}

func (c *Connect) OwnerReferencesForChildren() []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: c.APIVersion,
			Kind:       c.Kind,
			Name:       c.Name,
			UID:        c.UID,
		},
	}
}

func (c *Connect) DefaultRuntimeYAML() (string, error) {
	def := product.ConnectRuntimeDefinition{
		Images: []product.RuntimeYAMLImageEntry{},
	}

	for _, img := range []product.ConnectRuntimeImageDefinition{
		{
			PyVersion:     "3.12.4",
			RVersion:      "4.4.1",
			OSVersion:     "ubuntu2204",
			QuartoVersion: "1.4.557",
			Repo:          "ghcr.io/rstudio/content-pro",
		},
		{
			PyVersion:     "3.11.3",
			RVersion:      "4.2.2",
			OSVersion:     "ubuntu2204",
			QuartoVersion: "1.3.340",
			Repo:          "ghcr.io/rstudio/content-pro",
		},
	} {

		imgEntry, err := img.GenerateImageEntry()
		if err != nil {
			return "", err
		}

		def.Images = append(def.Images, imgEntry)
	}

	return def.BuildDefaultRuntimeYAML()
}

func (c *Connect) SecretProviderClassName() string {
	return fmt.Sprintf("%s-secrets", c.ComponentName())
}

func (c *Connect) secretProviderClassVolumeSource() *corev1.VolumeSource {
	return &corev1.VolumeSource{
		CSI: &corev1.CSIVolumeSource{
			Driver:   "secrets-store.csi.k8s.io",
			ReadOnly: ptr.To(true),
			FSType:   nil,
			VolumeAttributes: map[string]string{
				"secretProviderClass": c.SecretProviderClassName(),
			},
			NodePublishSecretRef: nil,
		},
	}
}

func (c *Connect) DataDirVolumeDef() *product.VolumeDef {
	return &product.VolumeDef{
		Source: &corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: c.ComponentName(),
				ReadOnly:  false,
			},
		},
		Mounts: []*product.VolumeMountDef{
			{MountPath: "/var/lib/rstudio-connect"},
		},
	}
}

func (c *Connect) OffHostExecutionVolumeDef() *product.VolumeDef {
	return &product.VolumeDef{
		Source: &corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: c.TemplateConfigMapName(),
				},
				Items:       nil,
				DefaultMode: ptr.To(product.MustParseOctal("0644")),
			},
		},
		Mounts: []*product.VolumeMountDef{
			{
				MountPath: "/var/lib/rstudio-connect-launcher/Kubernetes/rstudio-library-templates-data.tpl",
				SubPath:   "rstudio-library-templates-data.tpl",
				ReadOnly:  true,
			},
			{
				MountPath: "/var/lib/rstudio-connect-launcher/Kubernetes/job.tpl",
				SubPath:   "job.tpl",
				ReadOnly:  true,
			},
			{
				MountPath: "/var/lib/rstudio-connect-launcher/Kubernetes/service.tpl",
				SubPath:   "service.tpl",
				ReadOnly:  true,
			},
		},
	}
}

func (c *Connect) CreateVolumeFactory(cfg *ConnectConfig) *product.VolumeFactory {
	vols := map[string]*product.VolumeDef{}
	configVolumeMounts := []*product.VolumeMountDef{
		{MountPath: "/etc/rstudio-connect/rstudio-connect.gcfg", SubPath: "rstudio-connect.gcfg", ReadOnly: true},
	}

	// NOTE: cannot create volumes here or we risk creating a circular reference with the Reconciler...
	// TODO: maybe we push this whole factory down into the controller eventually
	for _, v := range c.Spec.AdditionalVolumes {
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
	if c.Spec.OffHostExecution {
		// mount some config files from the "main" configuration
		configVolumeMounts = product.ConcatLists(configVolumeMounts,
			[]*product.VolumeMountDef{
				{MountPath: "/etc/rstudio-connect/runtime.yaml", SubPath: "runtime.yaml", ReadOnly: true},
				{MountPath: "/etc/rstudio-connect/launcher/launcher.kubernetes.profiles.conf", SubPath: "launcher.kubernetes.profiles.conf", ReadOnly: true},
			})

		// also define the template volume
		vols["template-config"] = c.OffHostExecutionVolumeDef()
	}
	vols["config-volume"] = &product.VolumeDef{
		Source: &corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: c.ComponentName(),
				},
				Items:       nil,
				DefaultMode: nil,
				Optional:    nil,
			},
		},
		Mounts: configVolumeMounts,
	}

	if c.Spec.Volume != nil {
		vols["data-dir-volume"] = c.DataDirVolumeDef()
	}
	return &product.VolumeFactory{
		Vols: vols,
	}
}

func (c *Connect) CreateSecretVolumeFactory(cfg *ConnectConfig) *product.SecretVolumeFactory {
	vols := map[string]*product.VolumeDef{}

	// this will pivot based on the secret type and license type
	vols["license-volume"] = product.LicenseVolumeDefsFromProduct(c)

	csiEntries := map[string]*product.CSIDef{}

	if c.GetSecretType() == product.SiteSecretAws {
		// TODO: where does this key come from...?
		secretName := fmt.Sprintf("%s-secret-key", c.ComponentName())

		vols["key-volume"] = &product.VolumeDef{
			Source: &corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
					Items: []corev1.KeyToPath{
						{Key: "secret.key", Path: "secret.key"},
					},
					DefaultMode: ptr.To(product.MustParseOctal("0600")),
				},
			},
			Mounts: []*product.VolumeMountDef{
				{MountPath: "/var/lib/rstudio-connect/db/secret.key", SubPath: "secret.key", ReadOnly: true},
			},
			Env: []corev1.EnvVar{
				// this dummy env var ensures that the secret above gets provisioned by the secret CSI
				{
					Name: "DUMMY_SECRET_KEY",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
							Key:                  "secret.key",
						},
					},
				},
			},
		}

		if cfg.Server != nil && cfg.Server.EmailProvider == "SMTP" {
			smtpSecret := fmt.Sprintf("%s-smtp", c.ComponentName())
			vols["smtp-volume"] = &product.VolumeDef{
				Env: []corev1.EnvVar{
					{
						Name: "CONNECT_SMTP_PASSWORD",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: smtpSecret},
								Key:                  "smtp-password",
							},
						},
					},
					{
						Name: "CONNECT_SMTP_USER",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: smtpSecret},
								Key:                  "smtp-user",
							},
						},
					},
					{
						Name: "CONNECT_SMTP_HOST",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: smtpSecret},
								Key:                  "smtp-host",
							},
						},
					},
					{
						Name: "CONNECT_SMTP_PORT",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: smtpSecret},
								Key:                  "smtp-port",
							},
						},
					},
				},
			}

			// NOTE: the csiEntry/ies are added below...
		}

		dbSecretName := fmt.Sprintf("%s-db", c.ComponentName())
		vols["db-volume"] = &product.VolumeDef{
			Env: []corev1.EnvVar{
				{
					Name: "CONNECT_POSTGRES_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: dbSecretName},
							Key:                  "password",
						},
					},
				},
				{
					Name: "CONNECT_POSTGRES_INSTRUMENTATIONPASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: dbSecretName},
							Key:                  "password",
						},
					},
				},
			},
		}

		if c.Spec.Auth.Type == AuthTypeOidc {
			vols["client-secret-volume"] = &product.VolumeDef{
				Source: c.secretProviderClassVolumeSource(),
				Mounts: []*product.VolumeMountDef{
					{MountPath: "/etc/rstudio-connect/client-secret", SubPath: "client-secret", ReadOnly: true},
				},
			}
		}

		// TODO: an easier way to provision this...?
		//  how to know if "required"...? Are we saying it is required by putting it here?
		csiEntries["secret-csi-volume"] = &product.CSIDef{
			Driver:   "secrets-store.csi.k8s.io",
			ReadOnly: ptr.To(true),
			VolumeAttributes: map[string]string{
				"secretProviderClass": c.SecretProviderClassName(),
			},
		}

	} else if c.GetSecretType() == product.SiteSecretKubernetes {
		vols["key-volume"] = &product.VolumeDef{
			Source: &corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					// TODO: do we need a default value here?
					SecretName: c.Spec.Secret.VaultName,
					Items: []corev1.KeyToPath{
						{Key: "pub-secret-key", Path: "secret.key"},
					},
					DefaultMode: ptr.To(product.MustParseOctal("0600")),
				},
			},
			Mounts: []*product.VolumeMountDef{
				{MountPath: "/var/lib/rstudio-connect/db/secret.key", SubPath: "secret.key", ReadOnly: true},
			},
		}

		vols["db-volume"] = &product.VolumeDef{
			Env: []corev1.EnvVar{
				{
					Name: "CONNECT_POSTGRES_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							// TODO: do we need a default value here?
							LocalObjectReference: corev1.LocalObjectReference{Name: c.Spec.Secret.VaultName},
							Key:                  "pub-db-password",
						},
					},
				},
				{
					Name: "CONNECT_POSTGRES_INSTRUMENTATIONPASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							// TODO: do we need a default value here?
							LocalObjectReference: corev1.LocalObjectReference{Name: c.Spec.Secret.VaultName},
							Key:                  "pub-db-password",
						},
					},
				},
			},
		}

		if c.Spec.Auth.Type == AuthTypeOidc {
			vols["client-secret-volume"] = &product.VolumeDef{
				Source: &corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: c.GetSecretVaultName(),
						Items: []corev1.KeyToPath{
							{
								Key:  "pub-client-secret",
								Path: "client-secret",
							},
						},
					},
				},
				Mounts: []*product.VolumeMountDef{
					{MountPath: "/etc/rstudio-connect/client-secret", SubPath: "client-secret", ReadOnly: true},
				},
			}
		}
	} else {
		// uh oh... some other type of secret...?
	}

	return &product.SecretVolumeFactory{
		Vols:       vols,
		Env:        nil,
		CsiEntries: csiEntries,
	}
}

func (c *Connect) ConfigVolume() []corev1.Volume {
	vols := []corev1.Volume{
		{
			Name: "config-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: c.ComponentName(),
					},
					Items:       nil,
					DefaultMode: nil,
					Optional:    nil,
				},
			},
		},
	}
	if c.GetSecretType() == product.SiteSecretKubernetes {
		vols = append(vols,
			corev1.Volume{
				Name: "key-volume",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: c.Spec.Secret.VaultName,
						Items: []corev1.KeyToPath{
							{Key: "pub-secret-key", Path: "secret.key"},
						},
						DefaultMode: ptr.To(product.MustParseOctal("0600")),
					},
				},
			},
		)
	} else if c.GetSecretType() == product.SiteSecretAws {
		vols = append(vols,
			corev1.Volume{
				Name: "key-volume",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: fmt.Sprintf("%s-secret-key", c.ComponentName()),
						Items: []corev1.KeyToPath{
							{Key: "secret.key", Path: "secret.key"},
						},
						DefaultMode: ptr.To(product.MustParseOctal("0600")),
					},
				},
			},
		)
	} else {
		// uh oh... this is an unhandled case / secret type!
	}
	return vols
}

func (c *Connect) TokenVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "kube-api-access-volume",
			ReadOnly:  true,
			MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
		},
	}
}

func (c *Connect) TokenVolumes() []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "kube-api-access-volume",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
								ExpirationSeconds: ptr.To(int64(3600)),
								Path:              "token",
							},
						},
						{
							ConfigMap: &corev1.ConfigMapProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "kube-root-ca.crt",
								},
								Items: []corev1.KeyToPath{
									{
										Key:  "ca.crt",
										Path: "ca.crt",
									},
								},
							},
						},
						{
							DownwardAPI: &corev1.DownwardAPIProjection{
								Items: []corev1.DownwardAPIVolumeFile{
									{
										Path: "namespace",
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.namespace",
										},
									},
								},
							},
						},
					},
					DefaultMode: ptr.To(product.MustParseOctal("0644")),
				},
			},
		},
	}
}

func (c *Connect) TemplateConfigMapName() string {
	return fmt.Sprintf("%s-templates", c.ComponentName())
}

func (c *Connect) defaultSessionLabels() map[string]string {
	return map[string]string{
		SiteLabelKey:      c.Name,
		ComponentLabelKey: "connect-session",
	}
}

func (c *Connect) createDefaultSessionConfig(vf *product.MultiContainerVolumeFactory) *product.SessionConfig {
	// IMPORTANT NOTE: if you change defaults here... they may not always be applied.
	//   The defaults are "merged" in manually in the SessionConfigTemplateData method below
	return &product.SessionConfig{
		Service: &product.ServiceConfig{
			Type:   "ClusterIP",
			Labels: c.defaultSessionLabels(),
		},
		Pod: &product.PodConfig{
			Annotations:        nil,
			Labels:             c.defaultSessionLabels(),
			ServiceAccountName: c.SessionServiceAccountName(),
			Volumes: product.ConcatLists(
				[]corev1.Volume{},
				vf.Volumes(),
			),
			VolumeMounts: product.ConcatLists(
				[]corev1.VolumeMount{},
				vf.VolumeMounts(),
			),
			Env:              vf.EnvVars(),
			ImagePullPolicy:  "",
			ImagePullSecrets: nil,
			InitContainers: product.ConcatLists(
				[]corev1.Container{},
				vf.InitContainers(),
			),
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
			Labels: c.defaultSessionLabels(),
		},
	}
}

func (c *Connect) SiteSessionSecretProviderClass(ctx context.Context) (*v1.SecretProviderClass, error) {
	return product.SiteSessionSecretProviderClass(ctx, c)
}

func (c *Connect) SessionConfigTemplateData(ctx context.Context) string {
	l := product.LoggerFromContext(ctx)
	// TODO: we could also probably combine this method with the Workbench analog

	vFactory := c.CreateSessionVolumeFactory(ctx)
	defaultSessionConfig := c.createDefaultSessionConfig(vFactory)

	sess := &product.SessionConfig{}
	if c.Spec.SessionConfig != nil {
		sess = c.Spec.SessionConfig

		// merge in the defaults manually

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
			// TODO: yes there can be a naming conflict... we are ignoring that possibility
			sess.Pod.Volumes = product.ConcatLists(
				sess.Pod.Volumes,
				defaultSessionConfig.Pod.Volumes,
			)
			sess.Pod.VolumeMounts = product.ConcatLists(
				sess.Pod.VolumeMounts,
				defaultSessionConfig.Pod.VolumeMounts,
			)

			// and init containers
			sess.Pod.InitContainers = product.ConcatLists(
				sess.Pod.InitContainers,
				defaultSessionConfig.Pod.InitContainers,
			)
			// and labels
			sess.Pod.Labels = product.LabelMerge(sess.Pod.Labels, defaultSessionConfig.Pod.Labels)
		}

		// Job Config (empty)
		if sess.Job == nil {
			sess.Job = defaultSessionConfig.Job
		} else {
			// fill in labels
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

func (c *Connect) CreateSessionVolumeFactory(ctx context.Context) *product.MultiContainerVolumeFactory {
	img := c.Spec.SessionImage
	if img == "" {
		// set a default for now...
		// TODO: make this better!! determine version from somewhere?
		img = "ghcr.io/rstudio/rstudio-connect-content-init:ubuntu2204-2024.06.0"
	}

	mcv := &product.MultiContainerVolumeFactory{
		SecretVolumeFactory: product.SecretVolumeFactory{
			Vols:                 map[string]*product.VolumeDef{},
			Env:                  []corev1.EnvVar{},
			CsiEntries:           map[string]*product.CSIDef{},
			CsiAllSecrets:        map[string]string{},
			CsiKubernetesSecrets: map[string]map[string]string{},
		},
		InitContainerDefs: map[string]*product.ContainerDef{
			"init-runtime": {
				Image:           img,
				ImagePullPolicy: corev1.PullAlways,
				Mounts: map[string]*product.VolumeMountDef{
					"init-volume": {
						MountPath: "/mnt/rstudio-connect-runtime",
						ReadOnly:  false,
					},
				},
				SecurityContext: &corev1.SecurityContext{},
			},
		},
	}

	mcv.Vols["init-volume"] = &product.VolumeDef{
		Source: &corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
		Mounts: []*product.VolumeMountDef{
			{MountPath: "/opt/rstudio-connect/"},
		},
	}

	// NOTE: this is copied wholesale from the ConnectVolumeFactory... we should refactor this...
	// TODO: find a way to consolidate this duplication with the ConnectVolumeFactory
	for _, v := range c.Spec.AdditionalVolumes {
		mcv.Vols["addl-"+v.PvcName] = &product.VolumeDef{
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

	product.ParseSessionEnvVarSecrets(ctx, c, mcv)
	product.ConfigureDsn(c, mcv)

	return mcv
}
