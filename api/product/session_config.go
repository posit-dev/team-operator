package product

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/posit-dev/team-operator/api/templates"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/ptr"
	v1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

// SessionConfig houses all session configuration
// +kubebuilder:object:generate=true
type SessionConfig struct {
	Service *ServiceConfig `json:"service,omitempty"`
	Pod     *PodConfig     `json:"pod,omitempty"`
	Job     *JobConfig     `json:"job,omitempty"`
}

// ServiceConfig is the configuration for session service definition
// +kubebuilder:object:generate=true
type ServiceConfig struct {
	Type        string            `json:"type,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// PodConfig is the configuration for session pods
// +kubebuilder:object:generate=true
type PodConfig struct {
	Annotations              map[string]string             `json:"annotations,omitempty"`
	Labels                   map[string]string             `json:"labels,omitempty"`
	ServiceAccountName       string                        `json:"serviceAccountName,omitempty"`
	Volumes                  []corev1.Volume               `json:"volumes,omitempty"`
	VolumeMounts             []corev1.VolumeMount          `json:"volumeMounts,omitempty"`
	Env                      []corev1.EnvVar               `json:"env,omitempty"`
	ImagePullPolicy          corev1.PullPolicy             `json:"imagePullPolicy,omitempty"`
	ImagePullSecrets         []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	InitContainers           []corev1.Container            `json:"initContainers,omitempty"`
	ExtraContainers          []corev1.Container            `json:"extraContainers,omitempty"`
	ContainerSecurityContext corev1.SecurityContext        `json:"containerSecurityContext,omitempty"`
	DefaultSecurityContext   corev1.SecurityContext        `json:"defaultSecurityContext,omitempty"`
	SecurityContext          corev1.SecurityContext        `json:"securityContext,omitempty"`
	Tolerations              []corev1.Toleration           `json:"tolerations,omitempty"`
	Affinity                 *corev1.Affinity              `json:"affinity,omitempty"`
	// TODO: if we use corev1.NodeSelector, an empty array NodeSelectorTerm is being written as a node selector...
	NodeSelector      map[string]string `json:"nodeSelector,omitempty"`
	PriorityClassName string            `json:"priorityClassName,omitempty"`
	Command           []string          `json:"command,omitempty"`
}

// JobConfig is the configuration for session jobs
// +kubebuilder:object:generate=true
type JobConfig struct {
	Annotations map[string]string `json:"annotations,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type wrapperTemplateData struct {
	Name  string         `json:"name"`
	Value *SessionConfig `json:"value"`
}

func (s *SessionConfig) GenerateSessionConfigTemplate() (string, error) {
	// build wrapper struct
	w := wrapperTemplateData{
		Name:  "rstudio-library.templates.data",
		Value: s,
	}

	// remove struct information by serializing to JSON
	jsonBuffer, err := json.Marshal(w)
	if err != nil {
		return "", err
	}

	mapData := map[string]any{}

	if err := json.Unmarshal(jsonBuffer, &mapData); err != nil {
		return "", err
	}

	return templates.RenderTemplateDataOutput(mapData)
}

type SessionConfigProvider interface {
	SessionConfig() *SessionConfig
}

type SessionAndOwnerProvidingProduct interface {
	Product
	KubernetesOwnerProvider
	SessionConfig() *SessionConfig
	DsnSecret() string
}

func SiteSessionSecretName(p Product) string {
	return fmt.Sprintf("%s-site-session", p.ComponentName())
}

func SiteSessionVaultName(p Product) string {
	return fmt.Sprintf("%s-%s.sessions.posit.team", p.WorkloadCompoundName(), p.SiteName())
}

func SessionSecretProviderClassVolumeSource(p Product) *corev1.VolumeSource {
	return &corev1.VolumeSource{
		CSI: &corev1.CSIVolumeSource{
			Driver:   "secrets-store.csi.k8s.io",
			ReadOnly: ptr.To(true),
			FSType:   nil,
			VolumeAttributes: map[string]string{
				"secretProviderClass": SiteSessionSecretName(p),
			},
			NodePublishSecretRef: nil,
		},
	}
}

const secretPrefix = "secret://"

// SiteSessionSecretProviderClass creates a SecretProviderClass for the site-session secret. It finds keys
// in _session scoped_ environment variables and the DsnSecret (if any). If it finds nothing relevant, it
// will return a nil pointer
func SiteSessionSecretProviderClass(ctx context.Context, p SessionAndOwnerProvidingProduct) (*v1.SecretProviderClass, error) {
	l := LoggerFromContext(ctx)
	if p.SessionConfig() == nil || p.SessionConfig().Pod == nil {
		return nil, nil
	}
	necessaryKeys := []string{}
	for _, env := range p.SessionConfig().Pod.Env {
		if env.Value == "" || !strings.HasPrefix(env.Value, secretPrefix) {
			continue
		}
		if secretUrl, err := url.Parse(env.Value); err != nil {
			l.Info("Problem parsing secret URL", "value", env.Value)
			continue
		} else {
			if secretUrl.Host == "site-session" {
				// this is one that we need!
				necessaryKeys = append(necessaryKeys, strings.TrimPrefix(secretUrl.Path, "/"))
			}
		}
	}
	if len(necessaryKeys) == 0 && p.DsnSecret() == "" {
		return nil, nil
	}
	keys := map[string]string{}
	for _, v := range necessaryKeys {
		keys[v] = v
	}
	if p.DsnSecret() != "" {
		keys["odbc.ini"] = p.DsnSecret()
	}
	kubernetesKeys := map[string]map[string]string{
		SiteSessionSecretName(p): keys,
	}
	return GetSecretProviderClassForAllSecrets(
		p,
		SiteSessionSecretName(p),
		PositTeamNamespace,
		SiteSessionVaultName(p),
		keys,
		kubernetesKeys,
	)
}

// ConfigureDsn modifies `factory` in place, adding volume(s) if necessary based on the Product's DSN secret (if any)
func ConfigureDsn(p SessionAndOwnerProvidingProduct, factory *MultiContainerVolumeFactory) {
	if p.DsnSecret() == "" {
		return
	}
	switch p.GetSecretType() {
	case SiteSecretAws:
		factory.Vols["dsn-volume"] = &VolumeDef{
			Source: SessionSecretProviderClassVolumeSource(p),
			Mounts: []*VolumeMountDef{
				{MountPath: "/etc/odbc.ini", SubPath: "odbc.ini", ReadOnly: true},
			},
		}
	case SiteSecretKubernetes:
		factory.Vols["dsn-volume"] = &VolumeDef{
			Source: &corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: p.GetSecretVaultName(),
					Items: []corev1.KeyToPath{
						{Key: p.DsnSecret(), Path: "odbc.ini"},
					},
					DefaultMode: ptr.To(MustParseOctal("0655")),
				},
			},
			Mounts: []*VolumeMountDef{
				{MountPath: "/etc/odbc.ini", SubPath: "odbc.ini", ReadOnly: true},
			},
		}
	}
}

// ParseSessionEnvVarSecrets loops through environment variables in the Product's session configuration and
// determines whether to replace them with a secret mount. If so, it modifies `factory` in place to add the
// necessary volume(s) and mounts.
func ParseSessionEnvVarSecrets(ctx context.Context, p SessionAndOwnerProvidingProduct, factory *MultiContainerVolumeFactory) {
	l := LoggerFromContext(ctx)
	// modify env vars if needed...
	if p.GetSecretType() == SiteSecretAws && p.SessionConfig() != nil && p.SessionConfig().Pod != nil {
		needSessionCsi := false
		for _, env := range p.SessionConfig().Pod.Env {
			targetEnv := &corev1.EnvVar{}
			// if the value is a secret:// then we need to replace it with a secret mount
			if env.Value == "" && env.ValueFrom == nil {
				l.Info("Got empty env var", "name", env.Name)
				continue
			}
			if !strings.HasPrefix(strings.TrimSpace(env.Value), secretPrefix) {
				l.V(10).Info("Not a secret URL", "value", env.Value, "name", env.Name)
				factory.Env = append(factory.Env, env)
				continue
			}
			if secretUrl, err := url.Parse(env.Value); err != nil {
				l.Info("Problem parsing secret URL", "name", env.Name, "value", env.Value)
				// so we keep the env var as-is
				factory.Env = append(factory.Env, env)
				continue
			} else {
				switch secretUrl.Host {
				case "site-session":
					key := strings.TrimPrefix(secretUrl.Path, "/")
					targetEnv.Name = env.Name
					targetEnv.Value = ""
					targetEnv.ValueFrom = &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: SiteSessionSecretName(p),
							},
							Key: key,
						},
					}
					factory.Env = append(factory.Env, *targetEnv)

					needSessionCsi = true
					if factory.CsiAllSecrets == nil {
						factory.CsiAllSecrets = map[string]string{}
					}
					factory.CsiAllSecrets[key] = key
					if factory.CsiKubernetesSecrets == nil {
						factory.CsiKubernetesSecrets = map[string]map[string]string{}
					}
					factory.CsiKubernetesSecrets["site-session"] = factory.CsiAllSecrets
				default:
					l.Info("Invalid secret type. Should be one of (site-session)", "type", secretUrl.Host, "name", env.Name, "value", env.Value)
					// we keep the env var as-is
					factory.Env = append(factory.Env, env)
				}
			}
		}
		if needSessionCsi {
			if factory.CsiEntries == nil {
				factory.CsiEntries = map[string]*CSIDef{}
			}
			factory.CsiEntries["session-csi"] = &CSIDef{
				Driver:   "secrets-store.csi.k8s.io",
				ReadOnly: ptr.To(true),
				VolumeAttributes: map[string]string{
					"secretProviderClass": SiteSessionSecretName(p),
				},
				DummyVolumeMount: []*VolumeMountDef{
					{MountPath: "/mnt/all-secrets", ReadOnly: true},
				},
			}
		}
	} else {
		// TODO: need to handle Kubernetes secrets...
	}
}
