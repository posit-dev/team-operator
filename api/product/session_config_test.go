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
		dns253 := strings.Repeat("a", 253)
		err := ValidateDynamicLabelRules([]DynamicLabelRule{
			{Field: "args", Match: "[a-z]+", LabelPrefix: dns253 + "/ext."},
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
