// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2025 Posit Software, PBC
//+k8s:openapi-gen=true

package v1beta1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FlightdeckSpec defines the desired state of Flightdeck
type FlightdeckSpec struct {
	// SiteName is the name of the Site that owns this Flightdeck instance
	SiteName string `json:"siteName,omitempty"`

	// Image is the container image for Flightdeck
	Image string `json:"image,omitempty"`

	// ImagePullPolicy controls when the kubelet pulls the image
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Port is the port that the container will listen on
	// +kubebuilder:default=8080
	Port int32 `json:"port,omitempty"`

	// Replicas is the number of Flightdeck pods to run
	// +kubebuilder:default=1
	Replicas int `json:"replicas,omitempty"`

	// FeatureEnabler controls which features are enabled in Flightdeck
	FeatureEnabler FeatureEnablerConfig `json:"featureEnabler,omitempty"`

	// Domain is the domain name for Flightdeck ingress
	Domain string `json:"domain,omitempty"`

	// IngressClass is the ingress class to use
	IngressClass string `json:"ingressClass,omitempty"`

	// IngressAnnotations are annotations to apply to the ingress
	IngressAnnotations map[string]string `json:"ingressAnnotations,omitempty"`

	// ImagePullSecrets are references to secrets for pulling images
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`

	// AwsAccountId is the AWS account ID (used for EKS-to-IAM annotations)
	AwsAccountId string `json:"awsAccountId,omitempty"`

	// ClusterDate is the cluster date ID (used for EKS-to-IAM annotations)
	ClusterDate string `json:"clusterDate,omitempty"`

	// WorkloadCompoundName is the workload name
	WorkloadCompoundName string `json:"workloadCompoundName,omitempty"`

	// LogLevel sets the logging verbosity (debug, info, warn, error)
	// +kubebuilder:default="info"
	LogLevel string `json:"logLevel,omitempty"`

	// LogFormat sets the log output format (text, json)
	// +kubebuilder:default="text"
	LogFormat string `json:"logFormat,omitempty"`
}

// FlightdeckStatus defines the observed state of Flightdeck
type FlightdeckStatus struct {
	// Ready indicates whether the Flightdeck deployment is ready
	Ready bool `json:"ready"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+genclient
//+k8s:openapi-gen=true

// Flightdeck is the Schema for the flightdecks API
type Flightdeck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FlightdeckSpec   `json:"spec,omitempty"`
	Status FlightdeckStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:path=flightdecks

// FlightdeckList contains a list of Flightdeck
type FlightdeckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Flightdeck `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Flightdeck{}, &FlightdeckList{})
}

// SelectorLabels returns the selector labels for Flightdeck resources
func (f *Flightdeck) SelectorLabels() map[string]string {
	return map[string]string{
		ManagedByLabelKey:          ManagedByLabelValue,
		KubernetesNameLabelKey:     "flightdeck",
		KubernetesInstanceLabelKey: f.ComponentName(),
	}
}

// KubernetesLabels returns the full set of labels for Flightdeck resources
func (f *Flightdeck) KubernetesLabels() map[string]string {
	return map[string]string{
		ManagedByLabelKey:          ManagedByLabelValue,
		KubernetesNameLabelKey:     "flightdeck",
		KubernetesInstanceLabelKey: f.ComponentName(),
		SiteLabelKey:               f.Spec.SiteName,
		ComponentLabelKey:          "flightdeck",
	}
}

// ComponentName returns the component name for this Flightdeck instance
func (f *Flightdeck) ComponentName() string {
	return fmt.Sprintf("%s-flightdeck", f.Name)
}
