package product

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
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
	Annotations map[string]string `json:"annotations,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	// DynamicLabels defines rules for generating pod labels from runtime session data.
	// Requires template version 2.5.0 or later; ignored by older templates.
	// Currently only supported for Workbench sessions (exposed via Site CRD's workbench.sessionConfig).
	// +kubebuilder:validation:MaxItems=20
	DynamicLabels            []DynamicLabelRule            `json:"dynamicLabels,omitempty"`
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

// DynamicLabelRule defines a rule for generating pod labels from runtime session data.
// Each rule references a field from the .Job template object and either maps it directly
// to a label (using labelKey) or extracts multiple labels via regex (using match).
// +kubebuilder:object:generate=true
// +kubebuilder:validation:XValidation:rule="!(has(self.labelKey) && has(self.match))",message="labelKey and match are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="has(self.labelKey) || has(self.match)",message="one of labelKey or match is required"
// +kubebuilder:validation:XValidation:rule="!has(self.match) || has(self.labelPrefix)",message="labelPrefix is required when match is set"
type DynamicLabelRule struct {
	// Field is the name of a top-level .Job template field to read.
	// Common fields include "user", "args", "name", "image", "host", "exe", "command".
	// Any .Job field is addressable — this relies on CRD write access being a privileged
	// operation. If the field does not exist at runtime, the rule is silently skipped.
	// Field values may appear as pod labels visible to anyone with pod read access.
	// +kubebuilder:validation:MinLength=1
	Field string `json:"field"`
	// LabelKey is the label key for direct single-value mapping.
	// Mutually exclusive with match/labelPrefix.
	// Field values are sanitized for use as label values: non-alphanumeric characters
	// (except . - _) are replaced with underscores, then truncated to 63 characters with
	// leading/trailing non-alphanumeric characters stripped. Case is preserved in label
	// values (unlike regex mode suffixes, which are lowercased since they form label keys).
	// MaxLength = 253 (optional DNS prefix) + 1 (/) + 63 (name) = 317
	// +kubebuilder:validation:MaxLength=317
	LabelKey string `json:"labelKey,omitempty"`
	// Match is a regex pattern applied to the field value. Each match produces a label.
	// For array fields (like "args"), elements are joined with spaces before matching.
	// At runtime, at most 50 matches are applied per rule and 200 across all rules;
	// excess matches are dropped and a posit.team/dynamic-label-cap-reached annotation
	// is set on the pod. Matched suffixes are lowercased for use in label keys.
	// Mutually exclusive with labelKey.
	// +kubebuilder:validation:MaxLength=256
	Match string `json:"match,omitempty"`
	// TrimPrefix is stripped from each regex match before forming the label key suffix.
	// +kubebuilder:validation:MaxLength=256
	TrimPrefix string `json:"trimPrefix,omitempty"`
	// LabelPrefix is prepended to the cleaned match to form the label key.
	// Required when match is set.
	// MaxLength = 253 (DNS subdomain) + 1 (/) + 52 (name prefix, must be < 53 to leave ≥11 chars for suffix)
	// +kubebuilder:validation:MaxLength=306
	LabelPrefix string `json:"labelPrefix,omitempty"`
	// LabelValue is the static value for all matched labels. Defaults to "true".
	// +kubebuilder:validation:MaxLength=63
	LabelValue string `json:"labelValue,omitempty"`
}

// labelNameRegex validates the name segment of a Kubernetes label key.
var labelNameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$`)

// labelValueRegex validates a Kubernetes label value (same character rules as
// label name segments: alphanumeric start/end, interior allows . - _).
var labelValueRegex = labelNameRegex

// dnsSubdomainRegex validates a DNS subdomain per RFC 1123.
var dnsSubdomainRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)

// labelNamePrefixRegex validates the name prefix portion of a label key prefix.
// Trailing -, _, or . are allowed because the suffix (appended at runtime) always
// starts with an alphanumeric character, producing a valid final label name.
var labelNamePrefixRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// validateDNSSegmentLengths checks that each dot-separated segment of a DNS
// subdomain is at most 63 characters, per RFC 1123.
func validateDNSSegmentLengths(prefix string) error {
	for _, seg := range strings.Split(prefix, ".") {
		if len(seg) > 63 {
			return fmt.Errorf("DNS label segment %q exceeds 63 characters", seg)
		}
	}
	return nil
}

// ValidateDynamicLabelRules validates a slice of DynamicLabelRule, checking for
// regex compilation errors and mutual exclusivity of labelKey vs match/labelPrefix.
// NOTE: This validation runs at reconciliation time (via GenerateSessionConfigTemplate), not at
// admission time. The CRD XValidation markers only enforce structural rules. A validating webhook
// would be needed to surface these errors (e.g., invalid regex) at CRD write time.
func ValidateDynamicLabelRules(rules []DynamicLabelRule) error {
	seenKeys := map[string]bool{}
	seenPrefixes := map[string]bool{}
	// Collect direct-mapping labelKeys for cross-mode collision check.
	directKeys := map[string]int{}
	for i, rule := range rules {
		if rule.LabelKey != "" {
			directKeys[rule.LabelKey] = i
		}
	}
	for i, rule := range rules {
		// Check structural validity first so users see fundamental errors before
		// duplicate/collision messages that depend on the mode being unambiguous.
		if rule.LabelKey != "" && rule.Match != "" {
			return fmt.Errorf("dynamicLabels[%d]: labelKey and match are mutually exclusive", i)
		}
		if rule.LabelKey == "" && rule.Match == "" {
			return fmt.Errorf("dynamicLabels[%d]: one of labelKey or match is required", i)
		}
		if rule.Match != "" && rule.LabelPrefix == "" {
			return fmt.Errorf("dynamicLabels[%d]: labelPrefix is required when match is set", i)
		}
		if rule.LabelKey != "" {
			if seenKeys[rule.LabelKey] {
				return fmt.Errorf("dynamicLabels[%d]: duplicate labelKey %q", i, rule.LabelKey)
			}
			seenKeys[rule.LabelKey] = true
		}
		if rule.Match != "" && rule.LabelPrefix != "" {
			if seenPrefixes[rule.LabelPrefix] {
				return fmt.Errorf("dynamicLabels[%d]: duplicate labelPrefix %q across regex rules (overlapping matches would produce duplicate label keys)", i, rule.LabelPrefix)
			}
			seenPrefixes[rule.LabelPrefix] = true
			// Check for potential collision: a direct-mapping labelKey that starts
			// with this regex rule's labelPrefix could be overwritten at runtime.
			for key, keyIdx := range directKeys {
				if strings.HasPrefix(key, rule.LabelPrefix) {
					return fmt.Errorf("dynamicLabels[%d]: labelPrefix %q could collide with direct-mapping labelKey %q in rule %d (a regex match could produce the same label key)", i, rule.LabelPrefix, key, keyIdx)
				}
			}
		}
		if rule.LabelValue != "" {
			if rule.LabelKey != "" {
				return fmt.Errorf("dynamicLabels[%d]: labelValue must not be set with labelKey (the field value is used as the label value in direct-mapping mode)", i)
			}
			if len(rule.LabelValue) > 63 {
				return fmt.Errorf("dynamicLabels[%d]: labelValue must not exceed 63 characters", i)
			}
			if !labelValueRegex.MatchString(rule.LabelValue) {
				return fmt.Errorf("dynamicLabels[%d]: labelValue must be a valid Kubernetes label value (alphanumeric, -, _, . characters)", i)
			}
		}
		if rule.LabelKey != "" && rule.TrimPrefix != "" {
			return fmt.Errorf("dynamicLabels[%d]: trimPrefix must not be set with labelKey (trimPrefix only applies to regex match mode)", i)
		}
		if rule.LabelKey != "" && rule.LabelPrefix != "" {
			return fmt.Errorf("dynamicLabels[%d]: labelPrefix must not be set with labelKey (labelPrefix only applies to regex match mode)", i)
		}
		if rule.LabelKey != "" {
			if strings.Count(rule.LabelKey, "/") > 1 {
				return fmt.Errorf("dynamicLabels[%d]: labelKey must contain at most one '/'", i)
			}
			name := rule.LabelKey
			if idx := strings.Index(rule.LabelKey, "/"); idx >= 0 {
				prefix := rule.LabelKey[:idx]
				if len(prefix) == 0 {
					return fmt.Errorf("dynamicLabels[%d]: labelKey DNS prefix (before '/') must not be empty", i)
				}
				if len(prefix) > 253 {
					return fmt.Errorf("dynamicLabels[%d]: labelKey DNS prefix (before '/') must not exceed 253 characters", i)
				}
				if !dnsSubdomainRegex.MatchString(prefix) {
					return fmt.Errorf("dynamicLabels[%d]: labelKey DNS prefix must be a valid DNS subdomain (RFC 1123)", i)
				}
				if err := validateDNSSegmentLengths(prefix); err != nil {
					return fmt.Errorf("dynamicLabels[%d]: labelKey DNS prefix: %w", i, err)
				}
				if prefix == "kubernetes.io" || strings.HasSuffix(prefix, ".kubernetes.io") ||
					prefix == "k8s.io" || strings.HasSuffix(prefix, ".k8s.io") {
					return fmt.Errorf("dynamicLabels[%d]: labelKey must not use reserved Kubernetes label prefix %q", i, prefix)
				}
				name = rule.LabelKey[idx+1:]
			}
			if len(name) == 0 || len(name) > 63 {
				return fmt.Errorf("dynamicLabels[%d]: labelKey name segment must be between 1 and 63 characters", i)
			}
			if !labelNameRegex.MatchString(name) {
				return fmt.Errorf("dynamicLabels[%d]: labelKey name segment must match [a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?", i)
			}
		}
		if rule.Match != "" {
			if _, err := regexp.Compile(rule.Match); err != nil {
				return fmt.Errorf("dynamicLabels[%d]: invalid regex in match: %w", i, err)
			}
			// Validate labelPrefix conforms to Kubernetes label key structure:
			//   [dns-prefix/]name-prefix
			// - dns-prefix (before '/') must be ≤ 253 chars
			// - name-prefix (after '/', or entire string) must be < 63 chars
			//   to leave room for at least one suffix character
			if strings.Count(rule.LabelPrefix, "/") > 1 {
				return fmt.Errorf("dynamicLabels[%d]: labelPrefix must contain at most one '/'", i)
			}
			namePrefix := rule.LabelPrefix
			if idx := strings.Index(rule.LabelPrefix, "/"); idx >= 0 {
				dnsPrefix := rule.LabelPrefix[:idx]
				if len(dnsPrefix) == 0 {
					return fmt.Errorf("dynamicLabels[%d]: labelPrefix DNS prefix (before '/') must not be empty", i)
				}
				if len(dnsPrefix) > 253 {
					return fmt.Errorf("dynamicLabels[%d]: labelPrefix DNS prefix (before '/') must not exceed 253 characters", i)
				}
				if !dnsSubdomainRegex.MatchString(dnsPrefix) {
					return fmt.Errorf("dynamicLabels[%d]: labelPrefix DNS prefix must be a valid DNS subdomain (RFC 1123)", i)
				}
				if err := validateDNSSegmentLengths(dnsPrefix); err != nil {
					return fmt.Errorf("dynamicLabels[%d]: labelPrefix DNS prefix: %w", i, err)
				}
				if dnsPrefix == "kubernetes.io" || strings.HasSuffix(dnsPrefix, ".kubernetes.io") ||
					dnsPrefix == "k8s.io" || strings.HasSuffix(dnsPrefix, ".k8s.io") {
					return fmt.Errorf("dynamicLabels[%d]: labelPrefix must not use reserved Kubernetes label prefix %q", i, dnsPrefix)
				}
				namePrefix = rule.LabelPrefix[idx+1:]
			}
			if len(namePrefix) >= 53 {
				return fmt.Errorf("dynamicLabels[%d]: labelPrefix name segment (after '/') must be shorter than 53 characters to leave room for suffix", i)
			}
			if len(namePrefix) > 0 && !labelNamePrefixRegex.MatchString(namePrefix) {
				return fmt.Errorf("dynamicLabels[%d]: labelPrefix name segment must start with alphanumeric and contain only [a-zA-Z0-9._-]", i)
			}
		}
	}
	return nil
}

type wrapperTemplateData struct {
	Name  string         `json:"name"`
	Value *SessionConfig `json:"value"`
}

func (s *SessionConfig) GenerateSessionConfigTemplate() (string, error) {
	if s.Pod != nil && len(s.Pod.DynamicLabels) > 0 {
		if err := ValidateDynamicLabelRules(s.Pod.DynamicLabels); err != nil {
			return "", err
		}
	}

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
