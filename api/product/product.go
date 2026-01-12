package product

import (
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/strings/slices"
)

type GetClusterDater interface {
	GetClusterDate() string
}

type GetAwsAccountIder interface {
	GetAwsAccountId() string
}

type WorkloadCompoundNamer interface {
	WorkloadCompoundName() string
	ComponentName() string
	ShortName() string
	SiteName() string
}

type WorkloadAccountProvider interface {
	GetClusterDater
	GetAwsAccountIder
	WorkloadCompoundNamer
}

type KubernetesLabelser interface {
	KubernetesLabels() map[string]string
	SelectorLabels() map[string]string
}

type OwnerProvider interface {
	OwnerReferencesForChildren() []metav1.OwnerReference
}

type KubernetesOwnerProvider interface {
	OwnerProvider
	KubernetesLabelser
}

type Product interface {
	GetClusterDater
	GetAwsAccountIder
	WorkloadCompoundNamer

	GetLicenseSpec() LicenseSpec
	GetLicenseConstants() LicenseConstants
	GetSecretType() SiteSecretType
	GetSecretVaultName() string
	SecretProviderClassName() string
	GetChronicleAgentImage() string
	GetChronicleUrl() string
}

type ProductAndOwnerProvider interface {
	Product
	KubernetesOwnerProvider
}

type NamerAndOwnerProvider interface {
	KubernetesOwnerProvider
	WorkloadCompoundNamer
}

func OwnerReferencesHaveChanged(old, new []metav1.OwnerReference) bool {
	var oldStrings, newStrings []string
	for _, o := range old {
		oldStrings = append(oldStrings, o.String())
	}
	for _, n := range new {
		newStrings = append(newStrings, n.String())
	}

	return !slices.Equal(oldStrings, newStrings)
}

func MustParseOctal(octal string) int32 {
	if output, err := strconv.ParseInt(octal, 8, 64); err != nil {
		panic(err)
	} else {
		return int32(output)
	}
}
