// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2025 Posit Software, PBC

//+k8s:openapi-gen=true

package v1beta1

import (
	"fmt"

	"github.com/rstudio/goex/ptr"
	"github.com/posit-dev/team-operator/api/product"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PackageManagerSpec defines the desired state of PackageManager
type PackageManagerSpec struct {
	License    product.LicenseSpec    `json:"license,omitempty"`
	Config     *PackageManagerConfig  `json:"config,omitempty"`
	Volume     *product.VolumeSpec    `json:"volume,omitempty"`
	SecretType product.SiteSecretType `json:"secretType,omitempty"`

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

	Image string `json:"image,omitempty"`

	ImagePullPolicy v1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Sleep puts the service to sleep... so you can debug a crash looping container / etc. It is an ugly escape hatch,
	// but can also be useful on occasion
	Sleep bool `json:"sleep,omitempty"`

	// AwsAccountId is the account Id for this AWS Account. It is used to create EKS-to-IAM annotations
	AwsAccountId string `json:"awsAccountId,omitempty"`

	// WorkloadCompoundName is the name for the workload
	WorkloadCompoundName string `json:"workloadCompoundName,omitempty"`

	// ClusterDate is the date id (YYYYmmdd) for the cluster. It is used to create EKS-to-IAM annotations
	ClusterDate string `json:"clusterDate,omitempty"`

	// ChronicleAgentImage is the image used for the Chronicle Agent
	ChronicleAgentImage string `json:"chronicleImage,omitempty"`

	// Secret configures the secret management for this PackageManager
	Secret SecretConfig `json:"secret,omitempty"`

	// WorkloadSecret configures the managed persistent secret for the entire workload account
	WorkloadSecret SecretConfig `json:"workloadSecret,omitempty"`

	// MainDatabaseCredentialSecret configures the secret used for storing the main database credentials
	MainDatabaseCredentialSecret SecretConfig `json:"mainDatabaseCredentialSecret,omitempty"`

	Replicas int `json:"replicas,omitempty"`

	// GitSSHKeys defines SSH key configurations for Git authentication
	// This is used for mounting SSH keys but not included in the .gcfg file
	// +optional
	GitSSHKeys []SSHKeyConfig `json:"gitSSHKeys,omitempty"`

	// AzureFiles configures Azure Files integration for persistent storage
	// +optional
	AzureFiles *AzureFilesConfig `json:"azureFiles,omitempty"`
}

// PackageManagerStatus defines the observed state of PackageManager
type PackageManagerStatus struct {
	KeySecretRef v1.SecretReference `json:"keySecretRef,omitempty"`
	Ready        bool               `json:"ready"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName={pm,pms},path=packagemanagers
//+genclient
//+k8s:openapi-gen=true

// PackageManager is the Schema for the packagemanagers API
type PackageManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PackageManagerSpec   `json:"spec,omitempty"`
	Status PackageManagerStatus `json:"status,omitempty"`
}

// PackageManagerList contains a list of PackageManager
// +kubebuilder:object:root=true
type PackageManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PackageManager `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PackageManager{}, &PackageManagerList{})
}

func (pm *PackageManager) SecretProviderClassName() string {
	return fmt.Sprintf("%s-secrets", pm.ComponentName())
}

func (pm *PackageManager) GetLicenseConstants() product.LicenseConstants {
	return product.LicenseConstants{
		Key:           "RSPM_LICENSE",
		FilePathKey:   "RSPM_LICENSE_FILE_PATH",
		FilePath:      "/etc/rstudio-pm/license/license.lic",
		LicenseUrlKey: "RSPM_LICENSE_FILE_URL",
	}
}

func (pm *PackageManager) GetLicenseSpec() product.LicenseSpec {
	return pm.Spec.License
}

func (pm *PackageManager) ShortName() string {
	return "pkg"
}

func (pm *PackageManager) GetSecretType() product.SiteSecretType {
	return pm.Spec.Secret.Type
}

func (pm *PackageManager) GetSecretVaultName() string {
	return pm.Spec.Secret.VaultName
}

func (pm *PackageManager) GetClusterDate() string {
	return pm.Spec.ClusterDate
}

func (pm *PackageManager) GetAwsAccountId() string {
	return pm.Spec.AwsAccountId
}

func (pm *PackageManager) ComponentName() string {
	return fmt.Sprintf("%s-packagemanager", pm.Name)
}

func (pm *PackageManager) SiteName() string {
	return pm.Name
}

func (pm *PackageManager) WorkloadCompoundName() string {
	return pm.Spec.WorkloadCompoundName
}

func (pm *PackageManager) GetChronicleUrl() string {
	return fmt.Sprintf("%s-chronicle.%s.svc.cluster.local", pm.Name, pm.Namespace)
}

func (pm *PackageManager) GetChronicleAgentImage() string {
	return pm.Spec.ChronicleAgentImage
}

// SelectorLabels are immutable!
func (pm *PackageManager) SelectorLabels() map[string]string {
	return map[string]string{
		ManagedByLabelKey:          ManagedByLabelValue,
		KubernetesNameLabelKey:     "package-manager",
		KubernetesInstanceLabelKey: pm.ComponentName(),
	}
}

func (pm *PackageManager) KubernetesLabels() map[string]string {
	return product.LabelMerge(pm.SelectorLabels(), map[string]string{
		SiteLabelKey:      pm.SiteName(),
		ComponentLabelKey: "package-manager",
	})
}

func (pm *PackageManager) KeySecretName() string {
	return fmt.Sprintf("%s-key", pm.ComponentName())
}

func (pm *PackageManager) CreateSecretVolumeFactory() *product.SecretVolumeFactory {
	vols := map[string]*product.VolumeDef{}

	vols["license-volume"] = product.LicenseVolumeDefsFromProduct(pm)

	// TODO: this is only needed for File licenses... but we do it universally for now
	vols["license-volume-emptydir"] = &product.VolumeDef{
		Source: &v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
		Mounts: []*product.VolumeMountDef{
			{MountPath: "/etc/rstudio-pm/license", ReadOnly: false},
		},
	}

	csiEntries := map[string]*product.CSIDef{}

	switch pm.GetSecretType() {
	case product.SiteSecretAws:
		dbSecretName := fmt.Sprintf("%s-db", pm.ComponentName())

		vols["key-volume"] = &product.VolumeDef{
			Env: []v1.EnvVar{
				{
					Name: "PACKAGEMANAGER_SECRET_KEY",
					ValueFrom: &v1.EnvVarSource{
						SecretKeyRef: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: pm.KeySecretName()},
							Key:                  "key",
						},
					},
				},
			},
		}

		vols["db-volume"] = &product.VolumeDef{
			Env: []v1.EnvVar{
				{
					Name: "PACKAGEMANAGER_POSTGRES_PASSWORD",
					ValueFrom: &v1.EnvVarSource{
						SecretKeyRef: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: dbSecretName},
							Key:                  "password",
						},
					},
				},
				{
					Name: "PACKAGEMANAGER_POSTGRES_USAGEDATAPASSWORD",
					ValueFrom: &v1.EnvVarSource{
						SecretKeyRef: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: dbSecretName},
							Key:                  "password",
						},
					},
				},
			},
		}

		// TODO: an easier way to provision this...?
		//   how to know if "required"...? Are we saying it is required by putting it here?
		csiEntries["secret-csi-volume"] = &product.CSIDef{
			Driver:   "secrets-store.csi.k8s.io",
			ReadOnly: ptr.To(true),
			VolumeAttributes: map[string]string{
				"secretProviderClass": pm.SecretProviderClassName(),
			},
		}

		// Add SSH CSI volume if SSH keys are configured
		if len(pm.Spec.GitSSHKeys) > 0 {
			hasAwsSSHKeys := false
			for _, sshKey := range pm.Spec.GitSSHKeys {
				if sshKey.SecretRef.Source == "aws-secrets-manager" {
					hasAwsSSHKeys = true
					break
				}
			}

			if hasAwsSSHKeys {
				// Define the CSI configuration for SSH secrets
				// This will be used by the individual SSH key volumes below
				csiEntries["ssh-secrets-volume"] = &product.CSIDef{
					Driver:   "secrets-store.csi.k8s.io",
					ReadOnly: ptr.To(true),
					VolumeAttributes: map[string]string{
						"secretProviderClass": fmt.Sprintf("%s-ssh-secrets", pm.ComponentName()),
					},
				}

				// Create individual CSI volumes for each SSH key
				// Each volume uses the same SecretProviderClass but mounts only its specific key via SubPath
				// This approach:
				// 1. Marks the CSI entry as "complete" (preventing default /mnt/dummy mount)
				// 2. Ensures only the specific SSH key file is visible (not the entire vault)
				// 3. Maintains isolation between different SSH keys
				for _, sshKey := range pm.Spec.GitSSHKeys {
					if sshKey.SecretRef.Source == "aws-secrets-manager" {
						volName := fmt.Sprintf("ssh-key-%s", sshKey.Name)

						vols[volName] = &product.VolumeDef{
							Source: &v1.VolumeSource{
								CSI: &v1.CSIVolumeSource{
									Driver:   "secrets-store.csi.k8s.io",
									ReadOnly: ptr.To(true),
									VolumeAttributes: map[string]string{
										"secretProviderClass": fmt.Sprintf("%s-ssh-secrets", pm.ComponentName()),
									},
								},
							},
							Mounts: []*product.VolumeMountDef{
								{
									MountPath: fmt.Sprintf("/mnt/ssh-keys/%s", sshKey.Name),
									SubPath:   sshKey.Name, // Mount only this specific SSH key file
									ReadOnly:  true,
								},
							},
						}
					}
				}
			}
		}

	case product.SiteSecretKubernetes:
		vols["key-volume"] = &product.VolumeDef{
			Env: []v1.EnvVar{
				{
					Name: "PACKAGEMANAGER_SECRET_KEY",
					ValueFrom: &v1.EnvVarSource{
						SecretKeyRef: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: pm.Spec.Secret.VaultName},
							Key:                  "pkg-secret-key",
						},
					},
				},
			},
		}
		vols["db-volume"] = &product.VolumeDef{
			Env: []v1.EnvVar{
				{
					Name: "PACKAGEMANAGER_POSTGRES_PASSWORD",
					ValueFrom: &v1.EnvVarSource{
						SecretKeyRef: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: pm.Spec.Secret.VaultName},
							Key:                  "pkg-db-password",
						},
					},
				},
				{
					Name: "PACKAGEMANAGER_POSTGRES_USAGEDATAPASSWORD",
					ValueFrom: &v1.EnvVarSource{
						SecretKeyRef: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: pm.Spec.Secret.VaultName},
							Key:                  "pkg-db-password",
						},
					},
				},
			},
		}

	default:
		// uh oh... some other type of secret...?
	}

	// SSH keys are now included in the main CSI volume, not separate ones

	return &product.SecretVolumeFactory{
		Vols:       vols,
		Env:        nil,
		CsiEntries: csiEntries,
	}
}

func (pm *PackageManager) OwnerReferencesForChildren() []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: pm.APIVersion,
			Kind:       pm.Kind,
			Name:       pm.Name,
			UID:        pm.UID,
		},
	}
}
