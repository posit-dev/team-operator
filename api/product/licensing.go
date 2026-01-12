package product

import (
	"fmt"

	"github.com/rstudio/goex/ptr"
	v1 "k8s.io/api/core/v1"
)

type LicenseConstants struct {
	Key           string
	FilePathKey   string
	FilePath      string
	LicenseUrlKey string
}

type LicenseSpec struct {
	Type LicenseType `json:"type,omitempty"`

	Key string `json:"key,omitempty"`

	ExistingSecretName string `json:"existingSecretName,omitempty"`

	ExistingSecretKey string `json:"existingSecretKey,omitempty"`
}

type LicenseType string

const (
	LicenseTypeNone LicenseType = ""
	LicenseTypeKey  LicenseType = "KEY"
	LicenseTypeFile LicenseType = "FILE"
)

func LicenseVolumeDefsFromProduct(p Product) *VolumeDef {
	vol := &VolumeDef{}

	secretNameOrDefault := p.ComponentName() + "-license"
	if p.GetLicenseSpec().ExistingSecretName != "" {
		secretNameOrDefault = p.GetLicenseSpec().ExistingSecretName
	}

	// the key to look up the particular secret value within the Secret. Default to license.lic
	secretKeyOrDefault := "license.lic"
	if p.GetLicenseSpec().ExistingSecretKey != "" {
		secretKeyOrDefault = p.GetLicenseSpec().ExistingSecretKey
	}

	licType := p.GetLicenseSpec().Type
	if licType != LicenseTypeFile && licType != LicenseTypeNone {
		return vol
	}

	env := []v1.EnvVar{}

	switch licType {
	case LicenseTypeFile, LicenseTypeNone:
		env = append(env,
			v1.EnvVar{
				Name:  p.GetLicenseConstants().FilePathKey,
				Value: p.GetLicenseConstants().FilePath,
			},
		)
	case LicenseTypeKey:
		env = append(env, v1.EnvVar{
			Name:  p.GetLicenseConstants().Key,
			Value: p.GetLicenseSpec().Key,
		})
	}

	if p.GetSecretType() == SiteSecretAws {
		// add a license volume...
		vol = &VolumeDef{
			Source: &v1.VolumeSource{
				CSI: &v1.CSIVolumeSource{
					Driver:   "secrets-store.csi.k8s.io",
					ReadOnly: ptr.To(true),
					FSType:   nil,
					VolumeAttributes: map[string]string{
						"secretProviderClass": p.SecretProviderClassName(),
					},
					NodePublishSecretRef: nil,
				},
			},
			Mounts: []*VolumeMountDef{
				{
					MountPath: p.GetLicenseConstants().FilePath,
					SubPath:   fmt.Sprintf("%s.lic", p.ShortName()),
					ReadOnly:  true,
				},
			},
			Env: env,
		}
	} else if p.GetSecretType() == SiteSecretKubernetes {
		// TODO: we do not base64 decode the license file here, but we do above...
		//     Behavior is a bit ambiguous because k8s secrets are base64 encoded / auto-decoded anyways...
		//     Should we use the base64 shim here too for consistency?

		// add a license volume from kubernetes
		vol = &VolumeDef{
			Source: &v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName:  secretNameOrDefault,
					DefaultMode: nil,
					Optional:    nil,
				},
			},
			Mounts: []*VolumeMountDef{
				{
					MountPath: p.GetLicenseConstants().FilePath,
					SubPath:   secretKeyOrDefault,
					ReadOnly:  true,
				},
			},
			Env: env,
		}
	}
	return vol
}
