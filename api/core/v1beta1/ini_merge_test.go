package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeIniContent(t *testing.T) {
	for _, tc := range []struct {
		name      string
		generated string
		additonal string
		want      string
	}{
		{
			name:      "same section merges into one header",
			generated: "\n[*]\nlog-level=debug\nlog-message-format=pretty\n",
			additonal: "[*]\nlog-message-format=json\n",
			want:      "\n[*]\nlog-level=debug\nlog-message-format=json\n",
		},
		{
			name:      "new section is appended after a blank line",
			generated: "\n[*]\nlog-level=debug\n",
			additonal: "[rserver]\nlog-level=warn\n",
			want:      "\n[*]\nlog-level=debug\n\n[rserver]\nlog-level=warn\n",
		},
		{
			name:      "flat file appends unknown keys",
			generated: "admin-enabled=1\n",
			additonal: "custom-option=value\n",
			want:      "admin-enabled=1\ncustom-option=value\n",
		},
		{
			name:      "section names are matched ignoring surrounding space",
			generated: "\n[*]\nlog-level=debug\n",
			additonal: "[ * ]\nlog-level=warn\n",
			want:      "\n[*]\nlog-level=warn\n",
		},
		{
			name:      "keys are matched ignoring surrounding space",
			generated: "log-level=debug\n",
			additonal: " log-level = warn \n",
			want:      " log-level = warn \n",
		},
		{
			name:      "comments are never treated as keys",
			generated: "\n[*]\n# generated\nlog-level=debug\n",
			additonal: "[*]\n# override\nlog-level=warn\n",
			want:      "\n[*]\n# generated\nlog-level=warn\n# override\n",
		},
		{
			name:      "keys are appended ahead of trailing blank lines",
			generated: "\n[*]\nlog-level=debug\n\n[rserver]\nlog-level=warn\n",
			additonal: "[*]\nlogger-type=stderr\n",
			want:      "\n[*]\nlog-level=debug\nlogger-type=stderr\n\n[rserver]\nlog-level=warn\n",
		},
		{
			name:      "keys land in the matching section not the first one",
			generated: "\n[*]\nlog-level=debug\n\n[rserver]\nlog-level=warn\n",
			additonal: "[rserver]\nlog-level=error\n",
			want:      "\n[*]\nlog-level=debug\n\n[rserver]\nlog-level=error\n",
		},
		{
			name:      "top-level keys do not leak into a section",
			generated: "load-balancing-enabled=1\n",
			additonal: "www-port=8787\n[server]\nthread-pool-size=4\n",
			want:      "load-balancing-enabled=1\nwww-port=8787\n\n[server]\nthread-pool-size=4\n",
		},
		{
			name:      "empty additional content leaves the file untouched",
			generated: "\n[*]\nlog-level=debug\n",
			additonal: "",
			want:      "\n[*]\nlog-level=debug\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, mergeIniContent(tc.generated, tc.additonal))
		})
	}
}

func TestMergeIniContent_RoundTripsWithoutAdditions(t *testing.T) {
	// The merge rewrites the whole file, so an unchanged file must come back
	// byte-identical or every reconcile churns the ConfigMap and restarts pods.
	for _, generated := range []string{
		"admin-enabled=1\nadmin-group=workbench-admin\n",
		"\n[*]\nlog-level=debug\nlogger-type=stderr\n",
		"\n[default]\nname=small\ncpus=1\n\n[large]\nname=large\ncpus=8\n",
		"\n",
	} {
		require.Equal(t, generated, mergeIniContent(generated, ""))
	}
}

func TestMergeAdditionalConfigs_UngeneratedFileIsVerbatim(t *testing.T) {
	configMap := map[string]string{}
	mergeAdditionalConfigs(configMap, map[string]string{
		"custom.sh": "#!/bin/bash\nif [ -n \"$X\" ]; then\n  echo hi\nfi\n",
	})

	require.Equal(t, "#!/bin/bash\nif [ -n \"$X\" ]; then\n  echo hi\nfi\n", configMap["custom.sh"])
}
