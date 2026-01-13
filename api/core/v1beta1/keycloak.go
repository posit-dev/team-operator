package v1beta1

import (
	"fmt"
	"strings"

	"github.com/posit-dev/team-operator/api/product"
	"github.com/rstudio/goex/ptr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

type Keycloak struct {
	Site *Site `json:"site,omitempty"`
}

func (k *Keycloak) WorkloadCompoundName() string {
	return k.Site.Spec.WorkloadCompoundName
}

func (k *Keycloak) ComponentName() string {
	return fmt.Sprintf("%s-keycloak", k.Site.Name)
}

func (k *Keycloak) ShortName() string {
	return "key"
}

func (k *Keycloak) SiteName() string {
	return k.Site.Name
}

func (k *Keycloak) GetAwsAccountId() string {
	return k.Site.Spec.AwsAccountId
}

func (k *Keycloak) GetClusterDate() string {
	return k.Site.Spec.ClusterDate
}

func (k *Keycloak) DatabaseName() string {
	return strings.ReplaceAll(k.ComponentName(), "-", "_")
}

func (k *Keycloak) SecretName() string {
	if k.Site.GetSecretType() == product.SiteSecretKubernetes {
		return k.Site.Spec.Secret.VaultName
	}
	return k.Site.Name + "-keycloak-db-login"
}

func (k *Keycloak) SecretProviderClassName() string {
	return k.Site.Name + "-keycloak"
}

func (k *Keycloak) SecretProviderClass(request controllerruntime.Request) (*v1.SecretProviderClass, error) {
	// TODO: see if this can be handled by a secretVolumeFactory
	return product.GetSecretProviderClassForAllSecrets(
		k.Site, k.SecretProviderClassName(),
		request.Namespace, k.Site.Spec.Secret.VaultName,
		map[string]string{
			"keycloak-db-user":     "keycloak-db-user",
			"keycloak-db-password": "keycloak-db-password",
		},
		map[string]map[string]string{
			k.SecretName(): {
				"keycloak-db-user":     "keycloak-db-user",
				"keycloak-db-password": "keycloak-db-password",
			},
		},
	)
}

func (k *Keycloak) MiddlewareForwardName() string {
	return fmt.Sprintf("%s-keycloak-forward", k.Site.Name)
}

func (k *Keycloak) SpcConsumerDeploymentName() string {
	return k.Site.Name + "-keycloak-spc-consumer"
}

func (k *Keycloak) SecretVolumeFactory() *product.SecretVolumeFactory {

	vols := map[string]*product.VolumeDef{}
	csiEntries := map[string]*product.CSIDef{}
	csiAllSecrets := map[string]string{}
	csiKubernetesSecrets := map[string]map[string]string{}

	if k.Site.GetSecretType() == product.SiteSecretAws {
		// we do not create a volume here because the volume factory should take care of it...
		csiEntries["keycloak-csi-volume"] = &product.CSIDef{
			Driver:   "secrets-store.csi.k8s.io",
			ReadOnly: ptr.To(true),
			VolumeAttributes: map[string]string{
				"secretProviderClass": k.SecretProviderClassName(),
			},
		}
	} else {
		// some other type of secret... no volume factory required
	}

	return &product.SecretVolumeFactory{
		Vols:                 vols,
		Env:                  []corev1.EnvVar{},
		CsiEntries:           csiEntries,
		CsiAllSecrets:        csiAllSecrets,
		CsiKubernetesSecrets: csiKubernetesSecrets,
	}
}

func (k *Keycloak) SpcConsumerDeployment(request controllerruntime.Request) (*appsv1.Deployment, error) {
	volumeFactory := k.SecretVolumeFactory()
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            k.SpcConsumerDeploymentName(),
			Namespace:       request.Namespace,
			Labels:          k.Site.KubernetesLabels(),
			OwnerReferences: k.Site.OwnerReferencesForChildren(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: k.Site.SelectorLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: k.Site.KubernetesLabels(),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: k.ComponentName(),
					Containers: []corev1.Container{
						{
							Name:            "dummy",
							Image:           "busybox",
							ImagePullPolicy: corev1.PullAlways,
							Command:         []string{"sleep"},
							Args:            []string{"infinity"},
							Env:             product.ConcatLists(volumeFactory.EnvVars()),
							SecurityContext: &corev1.SecurityContext{
								// a dummy user ID to ensure we do not run as root...
								RunAsUser:                ptr.To(int64(1000)),
								RunAsNonRoot:             ptr.To(true),
								AllowPrivilegeEscalation: ptr.To(false),
								Privileged:               ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							VolumeMounts: product.ConcatLists(volumeFactory.VolumeMounts()),
						},
					},
					Volumes: product.ConcatLists(volumeFactory.Volumes()),
				},
			},
			Strategy:                appsv1.DeploymentStrategy{},
			MinReadySeconds:         0,
			RevisionHistoryLimit:    nil,
			Paused:                  false,
			ProgressDeadlineSeconds: nil,
		},
	}, nil
}
