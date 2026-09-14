package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeIniContent(t *testing.T) {
	for _, tc := range []struct {
		name       string
		generated  string
		additional string
		want       string
	}{
		{
			name:       "same section merges into one header",
			generated:  "\n[*]\nlog-level=debug\nlog-message-format=pretty\n",
			additional: "[*]\nlog-message-format=json\n",
			want:       "\n[*]\nlog-level=debug\nlog-message-format=json\n",
		},
		{
			name:       "new section is appended after a blank line",
			generated:  "\n[*]\nlog-level=debug\n",
			additional: "[rserver]\nlog-level=warn\n",
			want:       "\n[*]\nlog-level=debug\n\n[rserver]\nlog-level=warn\n",
		},
		{
			name:       "flat file appends unknown keys",
			generated:  "admin-enabled=1\n",
			additional: "custom-option=value\n",
			want:       "admin-enabled=1\ncustom-option=value\n",
		},
		{
			name:       "section names are matched ignoring surrounding space",
			generated:  "\n[*]\nlog-level=debug\n",
			additional: "[ * ]\nlog-level=warn\n",
			want:       "\n[*]\nlog-level=warn\n",
		},
		{
			// The override keeps the additionalConfigs line as written,
			// spacing included, rather than reformatting it.
			name:       "keys are matched ignoring surrounding space",
			generated:  "log-level=debug\n",
			additional: " log-level = warn \n",
			want:       " log-level = warn \n",
		},
		{
			name:       "repeats of a section absent from the generated file also collapse",
			generated:  "\n[*]\nlog-level=debug\n",
			additional: "[rserver]\nlog-level=warn\n[rserver]\nlogger-type=stderr\n",
			want:       "\n[*]\nlog-level=debug\n\n[rserver]\nlog-level=warn\nlogger-type=stderr\n",
		},
		{
			name:       "a commented header still matches the same section",
			generated:  "\n[*]\nlog-level=debug\n",
			additional: "[*] ; the global section\nlog-level=warn\n",
			want:       "\n[*]\nlog-level=warn\n",
		},
		{
			// "[]" names no section, so it stays content ahead of the first
			// header instead of being dropped and matching the preamble.
			name:       "an empty header is content, not a section",
			generated:  "\n[*]\nlog-level=debug\n",
			additional: "[]\nlog-level=warn\n",
			want:       "[]\nlog-level=warn\n\n[*]\nlog-level=debug\n",
		},
		{
			name:       "comments are never treated as keys",
			generated:  "\n[*]\n# generated\nlog-level=debug\n",
			additional: "[*]\n# override\nlog-level=warn\n",
			want:       "\n[*]\n# generated\nlog-level=warn\n# override\n",
		},
		{
			name:       "keys are appended ahead of trailing blank lines",
			generated:  "\n[*]\nlog-level=debug\n\n[rserver]\nlog-level=warn\n",
			additional: "[*]\nlogger-type=stderr\n",
			want:       "\n[*]\nlog-level=debug\nlogger-type=stderr\n\n[rserver]\nlog-level=warn\n",
		},
		{
			name:       "keys land in the matching section not the first one",
			generated:  "\n[*]\nlog-level=debug\n\n[rserver]\nlog-level=warn\n",
			additional: "[rserver]\nlog-level=error\n",
			want:       "\n[*]\nlog-level=debug\n\n[rserver]\nlog-level=error\n",
		},
		{
			name:       "top-level keys do not leak into a section",
			generated:  "load-balancing-enabled=1\n",
			additional: "www-port=8787\n[server]\nthread-pool-size=4\n",
			want:       "load-balancing-enabled=1\nwww-port=8787\n\n[server]\nthread-pool-size=4\n",
		},
		{
			name:       "empty additional content leaves the file untouched",
			generated:  "\n[*]\nlog-level=debug\n",
			additional: "",
			want:       "\n[*]\nlog-level=debug\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, mergeIniContent(tc.generated, tc.additional))
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
