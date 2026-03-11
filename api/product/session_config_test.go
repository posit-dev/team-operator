package product

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func TestSessionConfig_GenerateSessionConfigTemplate(t *testing.T) {
	nothing := SessionConfig{}

	str, err := nothing.GenerateSessionConfigTemplate()
	require.Nil(t, err)

	minimal := SessionConfig{
		Pod: &PodConfig{
			ImagePullPolicy: "Always",
		},
	}
	str, err = minimal.GenerateSessionConfigTemplate()
	require.Nil(t, err)
	require.Contains(t, str, "\"imagePullPolicy\":\"Always\"")

	complex := SessionConfig{
		Pod: &PodConfig{
			Volumes: []v1.Volume{
				{
					Name: "volume",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				},
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "volume",
					MountPath: "/mnt/tmp",
				},
			},
			InitContainers: []v1.Container{
				{
					Name:            "init",
					Image:           "some-image",
					ImagePullPolicy: v1.PullIfNotPresent,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "volume",
							MountPath: "/mnt/tmp-init",
						},
					},
				},
			},
		},
	}

	str, err = complex.GenerateSessionConfigTemplate()
	require.Nil(t, err)
	require.Contains(t, str, "\"imagePullPolicy\":\"IfNotPresent\"")
	require.Contains(t, str, "\"name\":\"volume\"")
	require.Contains(t, str, "\"image\":\"some-image\"")
	require.Contains(t, str, "\"mountPath\":\"/mnt/tmp-init\"")
	require.Contains(t, str, "\"mountPath\":\"/mnt/tmp\"")
}

func TestSessionConfig_DynamicLabels(t *testing.T) {
	t.Run("direct mapping rule serializes correctly", func(t *testing.T) {
		config := SessionConfig{
			Pod: &PodConfig{
				DynamicLabels: []DynamicLabelRule{
					{
						Field:    "user",
						LabelKey: "session.posit.team/user",
					},
				},
			},
		}

		str, err := config.GenerateSessionConfigTemplate()
		require.Nil(t, err)
		require.Contains(t, str, "\"dynamicLabels\"")
		require.Contains(t, str, "\"field\":\"user\"")
		require.Contains(t, str, "\"labelKey\":\"session.posit.team/user\"")
	})

	t.Run("pattern extraction rule serializes correctly", func(t *testing.T) {
		config := SessionConfig{
			Pod: &PodConfig{
				DynamicLabels: []DynamicLabelRule{
					{
						Field:       "args",
						Match:       "--ext-[a-z]+",
						TrimPrefix:  "--ext-",
						LabelPrefix: "session.posit.team/ext.",
						LabelValue:  "enabled",
					},
				},
			},
		}

		str, err := config.GenerateSessionConfigTemplate()
		require.Nil(t, err)
		require.Contains(t, str, "\"dynamicLabels\"")
		require.Contains(t, str, "\"field\":\"args\"")
		require.Contains(t, str, "\"match\":\"--ext-[a-z]+\"")
		require.Contains(t, str, "\"trimPrefix\":\"--ext-\"")
		require.Contains(t, str, "\"labelPrefix\":\"session.posit.team/ext.\"")
		require.Contains(t, str, "\"labelValue\":\"enabled\"")
	})
}

func TestValidateDynamicLabelRules(t *testing.T) {
	t.Run("valid direct mapping", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "session.posit.team/user"},
		})
		require.Nil(t, err)
	})

	t.Run("valid regex mapping", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "--ext-[a-z]+", LabelPrefix: "session.posit.team/ext."},
		})
		require.Nil(t, err)
	})

	t.Run("rejects labelKey and match both set", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "foo", Match: "bar", LabelPrefix: "baz"},
		})
		require.ErrorContains(t, err, "mutually exclusive")
	})

	t.Run("rejects neither labelKey nor match", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user"},
		})
		require.ErrorContains(t, err, "one of labelKey or match is required")
	})

	t.Run("rejects match without labelPrefix", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "--ext-[a-z]+"},
		})
		require.ErrorContains(t, err, "labelPrefix is required")
	})

	t.Run("rejects invalid regex", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "(unclosed", LabelPrefix: "prefix."},
		})
		require.ErrorContains(t, err, "invalid regex")
	})

	t.Run("rejects labelPrefix with name segment >= 53 chars", func(t *testing.T) {
		longName := strings.Repeat("a", 53)
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: "example.com/" + longName},
		})
		require.ErrorContains(t, err, "labelPrefix name segment")
	})

	t.Run("accepts labelPrefix with name segment at 52 chars", func(t *testing.T) {
		name52 := strings.Repeat("a", 52)
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: "example.com/" + name52},
		})
		require.NoError(t, err)
	})

	t.Run("rejects labelPrefix with DNS prefix > 253 chars", func(t *testing.T) {
		longDNS := strings.Repeat("a", 254)
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: longDNS + "/ext."},
		})
		require.ErrorContains(t, err, "DNS prefix")
	})

	t.Run("rejects labelPrefix with empty DNS prefix", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: "/name"},
		})
		require.ErrorContains(t, err, "DNS prefix")
	})

	t.Run("accepts labelPrefix with DNS prefix at 253 chars", func(t *testing.T) {
		// Build a valid 253-char DNS prefix with segments ≤ 63 chars each:
		// 63 + "." + 63 + "." + 63 + "." + 61 = 253
		dns253 := strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61)
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: dns253 + "/ext."},
		})
		require.NoError(t, err)
	})

	t.Run("rejects labelPrefix with DNS segment > 63 chars", func(t *testing.T) {
		longSegment := strings.Repeat("a", 64) + ".example.com"
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: longSegment + "/ext."},
		})
		require.ErrorContains(t, err, "DNS label segment")
		require.ErrorContains(t, err, "exceeds 63 characters")
	})

	t.Run("rejects labelKey with DNS segment > 63 chars", func(t *testing.T) {
		longSegment := strings.Repeat("a", 64) + ".example.com"
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: longSegment + "/name"},
		})
		require.ErrorContains(t, err, "DNS label segment")
		require.ErrorContains(t, err, "exceeds 63 characters")
	})

	t.Run("rejects labelKey with invalid name characters", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "invalid key!"},
		})
		require.ErrorContains(t, err, "labelKey name segment must match")
	})

	t.Run("rejects labelKey with empty name after prefix", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "example.com/"},
		})
		require.ErrorContains(t, err, "labelKey name segment must be between 1 and 63")
	})

	t.Run("rejects labelKey with empty DNS prefix", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "/name"},
		})
		require.ErrorContains(t, err, "labelKey DNS prefix")
	})

	t.Run("rejects labelKey with multiple slashes", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "a/b/name"},
		})
		require.ErrorContains(t, err, "labelKey must contain at most one '/'")
	})

	t.Run("rejects labelKey with invalid DNS prefix", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "!!!!/name"},
		})
		require.ErrorContains(t, err, "valid DNS subdomain")
	})

	t.Run("rejects labelKey with spaces in DNS prefix", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "spaces here/name"},
		})
		require.ErrorContains(t, err, "valid DNS subdomain")
	})

	t.Run("rejects labelKey with uppercase DNS prefix", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "My-Org.Example.COM/name"},
		})
		require.ErrorContains(t, err, "valid DNS subdomain")
	})

	t.Run("rejects labelKey with DNS prefix > 253 chars", func(t *testing.T) {
		longDNS := strings.Repeat("a", 254)
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: longDNS + "/name"},
		})
		require.ErrorContains(t, err, "labelKey DNS prefix")
	})

	t.Run("rejects labelKey with name > 63 chars", func(t *testing.T) {
		longName := strings.Repeat("a", 64)
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: longName},
		})
		require.ErrorContains(t, err, "labelKey name segment must be between 1 and 63")
	})

	t.Run("accepts labelKey with DNS prefix", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "my-org.example.com/session-user"},
		})
		require.NoError(t, err)
	})

	t.Run("accepts safe regex patterns (RE2 prevents backtracking by design)", func(t *testing.T) {
		// Go's regexp package uses RE2 which doesn't support backreferences,
		// so patterns like (a+)+$ are safe. But invalid patterns still fail.
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: "prefix."},
		})
		require.Nil(t, err)
	})

	t.Run("rejects labelPrefix with invalid DNS prefix format", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: "UPPER-CASE/ext."},
		})
		require.ErrorContains(t, err, "valid DNS subdomain")
	})

	t.Run("rejects labelPrefix with spaces in DNS prefix", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: "has spaces/ext."},
		})
		require.ErrorContains(t, err, "valid DNS subdomain")
	})

	t.Run("rejects labelValue with invalid characters", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: "prefix.", LabelValue: "!!!"},
		})
		require.ErrorContains(t, err, "labelValue must be a valid Kubernetes label value")
	})

	t.Run("rejects labelValue exceeding 63 characters", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: "prefix.", LabelValue: "a234567890123456789012345678901234567890123456789012345678901234"},
		})
		require.ErrorContains(t, err, "labelValue must not exceed 63 characters")
	})

	t.Run("accepts valid labelValue", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: "prefix.", LabelValue: "enabled"},
		})
		require.NoError(t, err)
	})

	t.Run("rejects labelKey with reserved kubernetes.io prefix", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "kubernetes.io/name"},
		})
		require.ErrorContains(t, err, "reserved Kubernetes label prefix")
	})

	t.Run("rejects labelKey with reserved app.kubernetes.io prefix", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "app.kubernetes.io/name"},
		})
		require.ErrorContains(t, err, "reserved Kubernetes label prefix")
	})

	t.Run("rejects labelKey with reserved k8s.io prefix", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "k8s.io/name"},
		})
		require.ErrorContains(t, err, "reserved Kubernetes label prefix")
	})

	t.Run("rejects labelPrefix with reserved kubernetes.io prefix", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: "kubernetes.io/ext."},
		})
		require.ErrorContains(t, err, "reserved Kubernetes label prefix")
	})

	t.Run("rejects labelPrefix with reserved k8s.io prefix", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: "k8s.io/ext."},
		})
		require.ErrorContains(t, err, "reserved Kubernetes label prefix")
	})

	t.Run("rejects labelPrefix with invalid name prefix characters", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: "example.com/!!!"},
		})
		require.ErrorContains(t, err, "labelPrefix name segment must start with alphanumeric")
	})

	t.Run("rejects labelPrefix without slash and invalid name characters", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: "!!!"},
		})
		require.ErrorContains(t, err, "labelPrefix name segment must start with alphanumeric")
	})

	t.Run("accepts labelPrefix with valid name prefix characters", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: "example.com/ext."},
		})
		require.NoError(t, err)
	})

	t.Run("rejects labelValue set with labelKey in direct-mapping mode", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "session.posit.team/user", LabelValue: "static"},
		})
		require.ErrorContains(t, err, "labelValue must not be set with labelKey")
	})

	t.Run("rejects trimPrefix set with labelKey in direct-mapping mode", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "session.posit.team/user", TrimPrefix: "--ext-"},
		})
		require.ErrorContains(t, err, "trimPrefix must not be set with labelKey")
	})

	t.Run("rejects labelPrefix set with labelKey in direct-mapping mode", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "session.posit.team/user", LabelPrefix: "prefix."},
		})
		require.ErrorContains(t, err, "labelPrefix must not be set with labelKey")
	})

	t.Run("rejects duplicate labelKey across rules", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "posit.team/user"},
			{Field: "email", LabelKey: "posit.team/user"},
		})
		require.ErrorContains(t, err, "duplicate labelKey")
		require.ErrorContains(t, err, "dynamicLabels[1]")
	})

	t.Run("rejects duplicate labelPrefix across regex rules", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "--ext-[a-z]+", LabelPrefix: "session.posit.team/ext."},
			{Field: "args", Match: "--plugin-[a-z]+", LabelPrefix: "session.posit.team/ext."},
		})
		require.ErrorContains(t, err, "duplicate labelPrefix")
		require.ErrorContains(t, err, "dynamicLabels[1]")
	})

	t.Run("accepts different labelPrefix across regex rules", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "--ext-[a-z]+", LabelPrefix: "session.posit.team/ext."},
			{Field: "args", Match: "--plugin-[a-z]+", LabelPrefix: "session.posit.team/plugin."},
		})
		require.NoError(t, err)
	})

	t.Run("rejects labelPrefix with multiple slashes", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: "a/b/name."},
		})
		require.ErrorContains(t, err, "labelPrefix must contain at most one '/'")
	})

	t.Run("reports correct index for invalid rule in mixed slice", func(t *testing.T) {
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "user", LabelKey: "session.posit.team/user"},
			{Field: "args", Match: "(unclosed", LabelPrefix: "prefix."},
		})
		require.ErrorContains(t, err, "dynamicLabels[1]")
	})
}

func TestGenerateSessionConfigTemplate_DynamicLabels_Validation(t *testing.T) {
	t.Run("rejects invalid regex at generation time", func(t *testing.T) {
		config := SessionConfig{
			Pod: &PodConfig{
				DynamicLabels: []DynamicLabelRule{
					{Field: "args", Match: "(unclosed", LabelPrefix: "prefix."},
				},
			},
		}
		_, err := config.GenerateSessionConfigTemplate()
		require.ErrorContains(t, err, "invalid regex")
	})

	t.Run("rejects mutually exclusive fields at generation time", func(t *testing.T) {
		config := SessionConfig{
			Pod: &PodConfig{
				DynamicLabels: []DynamicLabelRule{
					{Field: "user", LabelKey: "foo", Match: "bar", LabelPrefix: "baz"},
				},
			},
		}
		_, err := config.GenerateSessionConfigTemplate()
		require.ErrorContains(t, err, "mutually exclusive")
	})
}

func TestSiteSessionVaultName(t *testing.T) {
	t.Skip("Need to create a TestProduct struct to test this behavior")
}
