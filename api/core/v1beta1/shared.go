package v1beta1

import (
	"github.com/posit-dev/team-operator/api/product"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VolumeSourceType string

const (
	VolumeSourceTypeNone        VolumeSourceType = ""
	VolumeSourceTypeFsxZfs      VolumeSourceType = "fsx-zfs"
	VolumeSourceTypeNfs         VolumeSourceType = "nfs"
	VolumeSourceTypeAzureNetApp VolumeSourceType = "azure-netapp"
)

type StorageClass string

const (
	StorageClassAzureNetApp StorageClass = "azure-netapp-files"
)

type VolumeSource struct {
	Type     VolumeSourceType `json:"type,omitempty"`
	VolumeId string           `json:"volumeId,omitempty"`
	DnsName  string           `json:"dnsName,omitempty"`
}

type AuthType string

const (
	AuthTypePassword     AuthType     = "password"
	AuthTypeOidc         AuthType     = "oidc"
	AuthTypeSaml         AuthType     = "saml"
	NetworkTrustFull     NetworkTrust = 100
	NetworkTrustSameSite NetworkTrust = 50
	NetworkTrustZero     NetworkTrust = 0
)

type NetworkTrust uint8

type AuthSpec struct {
	Type               AuthType `json:"type,omitempty"`
	ClientId           string   `json:"clientId,omitempty"`
	Issuer             string   `json:"issuer,omitempty"`
	Groups             bool     `json:"groups,omitempty"`
	UsernameClaim      string   `json:"usernameClaim,omitempty"`
	EmailClaim         string   `json:"emailClaim,omitempty"`
	UniqueIdClaim      string   `json:"uniqueIdClaim,omitempty"`
	GroupsClaim        string   `json:"groupsClaim,omitempty"`
	DisableGroupsClaim bool     `json:"disableGroupsClaim,omitempty"`
	SamlMetadataUrl    string   `json:"samlMetadataUrl,omitempty"`
	// SAML-specific attribute mappings (mutually exclusive with SamlIdPAttributeProfile)
	SamlIdPAttributeProfile  string   `json:"samlIdPAttributeProfile,omitempty"`
	SamlUsernameAttribute    string   `json:"samlUsernameAttribute,omitempty"`
	SamlFirstNameAttribute   string   `json:"samlFirstNameAttribute,omitempty"`
	SamlLastNameAttribute    string   `json:"samlLastNameAttribute,omitempty"`
	SamlEmailAttribute       string   `json:"samlEmailAttribute,omitempty"`
	Scopes                   []string `json:"scopes,omitempty"`
	ViewerRoleMapping        []string `json:"viewerRoleMapping,omitempty"`
	PublisherRoleMapping     []string `json:"publisherRoleMapping,omitempty"`
	AdministratorRoleMapping []string `json:"administratorRoleMapping,omitempty"`
}

type SecretConfig struct {
	VaultName string                 `json:"vaultName,omitempty"`
	Type      product.SiteSecretType `json:"type,omitempty"`
}

// ComponentSpecPodAntiAffinity generates a *corev1.PodAntiAffinity suitable for use in a
// given component's deployment template spec to inform kubernetes to place pod replicas
// on separate nodes when possible.
func ComponentSpecPodAntiAffinity(p product.KubernetesLabelser, namespace string) *corev1.PodAntiAffinity {
	return &corev1.PodAntiAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
			{
				Weight: 1,
				PodAffinityTerm: corev1.PodAffinityTerm{
					TopologyKey: "kubernetes.io/hostname",
					Namespaces:  []string{namespace},
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      KubernetesInstanceLabelKey,
								Operator: metav1.LabelSelectorOpIn,
								Values: []string{
									p.KubernetesLabels()[KubernetesInstanceLabelKey],
								},
							},
							{
								Key:      SiteLabelKey,
								Operator: metav1.LabelSelectorOpIn,
								Values: []string{
									p.KubernetesLabels()[SiteLabelKey],
								},
							},
						},
					},
				},
			},
		},
	}
}
