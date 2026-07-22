// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC
//+k8s:openapi-gen=true

package v1beta1

import (
	"fmt"

	"github.com/posit-dev/team-operator/api/product"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChronicleSpec defines the desired state of Chronicle
type ChronicleSpec struct {
	// Suspended indicates Chronicle should not run serving resources (StatefulSet, Service)
	// but should preserve configuration. Set by the Site controller.
	// +optional
	Suspended *bool `json:"suspended,omitempty"`

	Config ChronicleConfig `json:"config,omitempty"`

	// ImagePullSecrets is a set of image pull secrets to use for all image pulls. These names / secrets
	// must already exist in the namespace in question.
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`

	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Resources sets the chronicle-server container's resource requests/limits.
	// If unset, product defaults are applied. Setting this replaces the defaults entirely.
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// AddEnv adds arbitrary environment variables to the container env
	AddEnv map[string]string `json:"addEnv,omitempty"`

	Image string `json:"image,omitempty"`

	// AwsAccountId is the account Id for this AWS Account. It is used to create EKS-to-IAM annotations
	AwsAccountId string `json:"awsAccountId,omitempty"`

	// ClusterDate is the date id (YYYYmmdd) for the cluster. It is used to create EKS-to-IAM annotations
	ClusterDate string `json:"clusterDate,omitempty"`

	// WorkloadCompoundName is the name for the workload
	WorkloadCompoundName string `json:"workloadCompoundName,omitempty"`
}

// ChronicleStatus defines the observed state of Chronicle
type ChronicleStatus struct {
	CommonProductStatus `json:",inline"`
	// +optional
	Ready bool `json:"ready"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName={pcr,chr},path=chronicles
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=='Ready')].status`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.version`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +genclient
// +k8s:openapi-gen=true

// Chronicle is the Schema for the chronicles API
type Chronicle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChronicleSpec   `json:"spec,omitempty"`
	Status ChronicleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ChronicleList contains a list of Chronicle
type ChronicleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Chronicle `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Chronicle{}, &ChronicleList{})
}

func (c *Chronicle) ComponentName() string {
	return fmt.Sprintf("%s-chronicle", c.Name)
}

func (c *Chronicle) SiteName() string {
	return c.Name
}

func (c *Chronicle) WorkloadCompoundName() string {
	return c.Spec.WorkloadCompoundName
}

func (c *Chronicle) ShortName() string {
	return "chr"
}

func (c *Chronicle) GetLicenseSpec() product.LicenseSpec {
	return product.LicenseSpec{}
}

func (c *Chronicle) GetLicenseConstants() product.LicenseConstants {
	return product.LicenseConstants{}
}

func (c *Chronicle) SelectorLabels() map[string]string {
	return map[string]string{
		ManagedByLabelKey:          ManagedByLabelValue,
		KubernetesNameLabelKey:     "chronicle",
		KubernetesInstanceLabelKey: c.ComponentName(),
	}
}

func (c *Chronicle) KubernetesLabels() map[string]string {
	return product.LabelMerge(c.SelectorLabels(), map[string]string{
		SiteLabelKey:      c.SiteName(),
		ComponentLabelKey: "chronicle",
	})
}

func (c *Chronicle) OwnerReferencesForChildren() []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: c.APIVersion,
			Kind:       c.Kind,
			Name:       c.Name,
			UID:        c.UID,
		},
	}
}

func (c *Chronicle) GetAwsAccountId() string {
	return c.Spec.AwsAccountId
}

func (c *Chronicle) GetClusterDate() string {
	return c.Spec.ClusterDate
}

func (c *Chronicle) GetSecretType() product.SiteSecretType {
	return product.SiteSecretNone
}

func (c *Chronicle) GetSecretVaultName() string {
	return ""
}

func (c *Chronicle) SecretProviderClassName() string {
	return ""
}

func (c *Chronicle) GetChronicleUrl() string {
	return fmt.Sprintf("%s-chronicle.%s.svc.cluster.local", c.Name, c.Namespace)
}

func (c *Chronicle) GetChronicleAgentImage() string {
	// TODO: make an interface that fits chronicle better...?
	//   - does chronicle care what version the agents are?
	return fmt.Sprintf("ghcr.io/rstudio/chronicle:2023.10.4")
}
