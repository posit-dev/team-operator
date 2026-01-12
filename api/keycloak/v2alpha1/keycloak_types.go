package v2alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KeycloakSpec is the specification for the Keycloak object
type KeycloakSpec struct {
	Db          *KeycloakDbSpec          `json:"db,omitempty"`
	Http        *KeycloakHttpSpec        `json:"http,omitempty"`
	Hostname    *KeycloakHostnameSpec    `json:"hostname,omitempty"`
	Features    *KeycloakFeaturesSpec    `json:"features,omitempty"`
	Transaction *KeycloakTransactionSpec `json:"transaction,omitempty"`
	Unsupported *KeycloakUnsupportedSpec `json:"unsupported,omitempty"`
	Ingress     *KeycloakIngressSpec     `json:"ingress,omitempty"`
	Instances   int                      `json:"instances,omitempty"`
	Image       string                   `json:"image,omitempty"`
}

type KeycloakDbSpec struct {
	Vendor          string              `json:"vendor,omitempty"`
	UsernameSecret  *KeycloakSecretSpec `json:"usernameSecret,omitempty"`
	PasswordSecret  *KeycloakSecretSpec `json:"passwordSecret,omitempty"`
	Host            string              `json:"host,omitempty"`
	Database        string              `json:"database,omitempty"`
	Port            int                 `json:"port,omitempty"`
	Schema          string              `json:"schema,omitempty"`
	PoolInitialSize int                 `json:"poolInitialSize,omitempty"`
	PoolMinSize     int                 `json:"poolMinSize,omitempty"`
	PoolMaxSize     int                 `json:"poolMaxSize,omitempty"`
}

type KeycloakSecretSpec struct {
	Name string `json:"name,omitempty"`
	Key  string `json:"key,omitempty"`
}

type KeycloakHttpSpec struct {
	HttpEnabled bool   `json:"httpEnabled,omitempty"`
	HttpPort    int    `json:"httpPort,omitempty"`
	HttpsPort   int    `json:"httpsPort,omitempty"`
	TlsSecret   string `json:"tlsSecret,omitempty"`
}

type KeycloakIngressSpec struct {
	Annotations map[string]string `json:"annotations,omitempty"`
	ClassName   string            `json:"className,omitempty"`
	Enabled     bool              `json:"enabled,omitempty"`
}

type KeycloakHostnameSpec struct {
	Hostname          string `json:"hostname,omitempty"`
	Admin             string `json:"admin,omitempty"`
	Strict            bool   `json:"strict,omitempty"`
	StrictBackchannel bool   `json:"strictBackchannel,omitempty"`
}

type KeycloakFeaturesSpec struct {
	Enabled  []string `json:"enabled,omitempty"`
	Disabled []string `json:"disabled,omitempty"`
}

type KeycloakTransactionSpec struct {
	XaEnabled bool `json:"xaEnabled,omitempty"`
}

type KeycloakUnsupportedSpec struct {
	PodTemplate *v1.PodTemplateSpec `json:"podTemplate,omitempty"`
}

type KeycloakStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+genclient
//+k8s:openapi-gen=true

// Keycloak is the Schema for the Keycloak API
type Keycloak struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KeycloakSpec   `json:"spec,omitempty"`
	Status KeycloakStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:path=keycloaks
//+kubebuilder:skip

type KeycloakList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Keycloak `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Keycloak{}, &KeycloakList{})
}
