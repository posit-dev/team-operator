package templates

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dynamicLabelsTemplate is the dynamic labels block extracted from job.tpl for isolated testing.
// This tests the actual Sprig template pipeline used in production.
const dynamicLabelsTemplate = `
{{- $templateDataJSON := include "rstudio-library.templates.data" nil -}}
{{- $templateData := $templateDataJSON | mustFromJson -}}
{{- $capStatus := dict }}
{{- $matchCache := dict }}
{{- with $templateData.pod.dynamicLabels }}
{{- range $i, $rule := . }}
{{- if and (hasKey $.Job $rule.field) $rule.match }}
{{- $val := index $.Job $rule.field }}
{{- $str := (kindIs "slice" $val) | ternary ($val | join " ") ($val | toString) }}
{{- $matches := regexFindAll $rule.match $str -1 }}
{{- /* Deduplicate matches so duplicates don't consume cap budget */ -}}
{{- $seen := dict }}
{{- $deduped := list }}
{{- range $m := $matches }}
{{- if not (hasKey $seen $m) }}{{- $_ := set $seen $m "1" }}{{- $deduped = append $deduped $m }}{{- end }}
{{- end }}
{{- $matches = $deduped }}
{{- if gt (len $matches) 50 }}{{- $_ := set $capStatus "reached" "true" }}{{- $matches = slice $matches 0 50 }}{{- end }}
{{- $_ := set $matchCache (printf "%d" $i) $matches }}
{{- end }}
{{- end }}
{{- end }}
{{- if hasKey $capStatus "reached" }}
posit.team/label-cap-reached: "true"
{{- end }}
{{- with $templateData.pod.dynamicLabels }}
{{- range $i, $rule := . }}
{{- if hasKey $.Job $rule.field }}
{{- $val := index $.Job $rule.field }}
{{- if $rule.labelKey }}
{{- $labelVal := $val | toString | regexReplaceAll "[^a-zA-Z0-9._-]" "_" | trunc 63 | regexReplaceAll "[^a-zA-Z0-9]+$" "" | regexReplaceAll "^[^a-zA-Z0-9]+" "" }}
{{- if ne $labelVal "" }}
{{ $rule.labelKey }}: {{ $labelVal | quote }}
{{- end }}
{{- else if $rule.match }}
{{- $matches := index $matchCache (printf "%d" $i) }}
{{- $namePrefix := regexFind "[^/]*$" $rule.labelPrefix }}
{{- $maxSuffix := int (sub 63 (len $namePrefix)) }}
{{- range $match := $matches }}
{{- $suffix := trimPrefix ($rule.trimPrefix | default "") $match | lower | regexReplaceAll "[^a-zA-Z0-9._-]" "_" | regexReplaceAll "[_]{2,}" "_" | trunc $maxSuffix | regexReplaceAll "[^a-zA-Z0-9]+$" "" | regexReplaceAll "^[^a-zA-Z0-9]+" "" }}
{{- if ne $suffix "" }}
{{ printf "%s%s" $rule.labelPrefix $suffix }}: {{ $rule.labelValue | default "true" | quote }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}`

// renderDynamicLabels renders the dynamic labels template block with the given
// session config (templateData) and Job data. Uses Helm-compatible regexReplaceAll
// argument order (regex, repl, s) where the piped value is the source string.
func renderDynamicLabels(t *testing.T, templateData map[string]any, jobData map[string]any) string {
	t.Helper()

	templateDataJSON, err := json.Marshal(templateData)
	require.NoError(t, err)

	mockDataDefine := `{{- define "rstudio-library.templates.data" -}}` + string(templateDataJSON) + `{{- end -}}`

	tmpl := template.New("gotpl")
	f := TemplateFuncMap(tmpl)
	f = AddOnFuncMap(tmpl, f)
	// Override regexReplaceAll to match Helm's pipeline-friendly argument order.
	// Sprig: regexReplaceAll(regex, s, repl) — piped value becomes repl.
	// Helm:  regexReplaceAll(regex, repl, s) — piped value becomes s (source).
	// NOTE: If upgrading Helm/Sprig, verify that the production argument order still
	// matches this mock — otherwise tests will pass against a stale signature.
	f["regexReplaceAll"] = func(regex string, repl string, s string) string {
		r := regexp.MustCompile(regex)
		return r.ReplaceAllString(s, repl)
	}
	tmpl.Funcs(f)

	_, err = tmpl.Parse(mockDataDefine + "\n" + dynamicLabelsTemplate)
	require.NoError(t, err)

	data := map[string]any{
		"Job": jobData,
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	require.NoError(t, err)

	return buf.String()
}

// TestCanary_SprigRegexReplaceAllOrder verifies that Sprig's regexReplaceAll
// still uses (regex, s, repl) order, which differs from Helm's (regex, repl, s).
// Our mock in renderDynamicLabels overrides to Helm's order. If Sprig ever changes
// to match Helm, this canary will fire and the mock override can be removed.
func TestCanary_SprigRegexReplaceAllOrder(t *testing.T) {
	tmpl := template.New("canary")
	f := TemplateFuncMap(tmpl)
	f = AddOnFuncMap(tmpl, f)

	prodFn := f["regexReplaceAll"]
	fn, ok := prodFn.(func(string, string, string) string)
	require.True(t, ok, "regexReplaceAll should be func(string, string, string) string")

	// With Sprig order (regex, s, repl): fn("h", "world", "hello") → replace "h" in "world" → "world" (no match)
	// With Helm  order (regex, repl, s): fn("h", "world", "hello") → replace "h" in "hello" → "worldello"
	result := fn("h", "world", "hello")
	if result == "worldello" {
		t.Fatalf("Sprig regexReplaceAll now uses Helm's argument order — remove the mock override in renderDynamicLabels")
	}
}

func TestJobTemplate_DynamicLabels_DirectMapping(t *testing.T) {
	t.Run("renders direct mapping label from string field", func(t *testing.T) {
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{"field": "user", "labelKey": "session.posit.team/user"},
				},
			},
		}
		jobData := map[string]any{"user": "alice"}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.Contains(t, out, `session.posit.team/user: "alice"`)
	})

	t.Run("sanitizes special characters in label value", func(t *testing.T) {
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{"field": "user", "labelKey": "session.posit.team/user"},
				},
			},
		}
		jobData := map[string]any{"user": "alice smith@org"}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.Contains(t, out, `session.posit.team/user: "alice_smith_org"`)
	})

	t.Run("truncates long label values to 63 chars", func(t *testing.T) {
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{"field": "user", "labelKey": "session.posit.team/user"},
				},
			},
		}
		longUser := strings.Repeat("a", 100)
		jobData := map[string]any{"user": longUser}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.Contains(t, out, `session.posit.team/user: "`+strings.Repeat("a", 63)+`"`)
		assert.NotContains(t, out, strings.Repeat("a", 64))
	})

	t.Run("skips label when field is missing", func(t *testing.T) {
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{"field": "nonexistent", "labelKey": "session.posit.team/missing"},
				},
			},
		}
		jobData := map[string]any{"user": "alice"}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.NotContains(t, out, "session.posit.team/missing")
	})

	t.Run("skips label when field value is empty string", func(t *testing.T) {
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{"field": "user", "labelKey": "session.posit.team/user"},
				},
			},
		}
		jobData := map[string]any{"user": ""}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.NotContains(t, out, "session.posit.team/user")
	})

	t.Run("skips label when value is only special characters", func(t *testing.T) {
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{"field": "user", "labelKey": "session.posit.team/user"},
				},
			},
		}
		jobData := map[string]any{"user": "!!!"}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.NotContains(t, out, "session.posit.team/user")
	})
}

func TestJobTemplate_DynamicLabels_RegexMapping(t *testing.T) {
	t.Run("extracts labels from regex matches on array field", func(t *testing.T) {
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{
						"field":       "args",
						"match":       "--ext-[a-z]+",
						"trimPrefix":  "--ext-",
						"labelPrefix": "session.posit.team/ext.",
						"labelValue":  "enabled",
					},
				},
			},
		}
		jobData := map[string]any{
			"args": []any{"--ext-foo", "--ext-bar", "--other"},
		}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.Contains(t, out, `session.posit.team/ext.foo: "enabled"`)
		assert.Contains(t, out, `session.posit.team/ext.bar: "enabled"`)
		assert.NotContains(t, out, "other")
	})

	t.Run("extracts labels from regex matches on string field", func(t *testing.T) {
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{
						"field":       "user",
						"match":       "[a-z]+",
						"labelPrefix": "session.posit.team/part.",
					},
				},
			},
		}
		jobData := map[string]any{"user": "alice"}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.Contains(t, out, `session.posit.team/part.alice: "true"`)
	})

	t.Run("strips leading non-alphanumeric from suffix", func(t *testing.T) {
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{
						"field":       "args",
						"match":       "--[a-z]+",
						"labelPrefix": "prefix/ext.",
					},
				},
			},
		}
		jobData := map[string]any{
			"args": []any{"--foo"},
		}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.Contains(t, out, `prefix/ext.foo: "true"`)
	})

	t.Run("truncates suffix to fit within 63-char label name limit", func(t *testing.T) {
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{
						"field":       "user",
						"match":       ".+",
						"labelPrefix": "session.posit.team/ext.",
					},
				},
			},
		}
		// "ext." is 4 chars in name segment, so maxSuffix = 63 - 4 = 59
		longUser := strings.Repeat("a", 100)
		jobData := map[string]any{"user": longUser}

		out := renderDynamicLabels(t, templateData, jobData)
		expectedLabel := "session.posit.team/ext." + strings.Repeat("a", 59)
		assert.Contains(t, out, expectedLabel+`: "true"`)
	})

	t.Run("sanitizes special characters in suffix for label key", func(t *testing.T) {
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{
						"field":       "args",
						"match":       "--ext-[^ ]+",
						"trimPrefix":  "--ext-",
						"labelPrefix": "session.posit.team/ext.",
					},
				},
			},
		}
		jobData := map[string]any{
			"args": []any{"--ext-foo@bar"},
		}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.Contains(t, out, `session.posit.team/ext.foo_bar: "true"`)
		assert.NotContains(t, out, "foo@bar")
	})

	t.Run("caps matches at 50 (test-only annotation verifies cap)", func(t *testing.T) {
		args := make([]any, 60)
		for i := range args {
			args[i] = fmt.Sprintf("ext%03d", i) // unique values: "ext000", "ext001", ...
		}
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{
						"field":       "args",
						"match":       "ext[0-9]+",
						"labelPrefix": "prefix/ext.",
					},
				},
			},
		}
		jobData := map[string]any{"args": args}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.Contains(t, out, `posit.team/label-cap-reached: "true"`)
		// Count label lines (each unique match produces one "prefix/ext." label)
		count := strings.Count(out, "prefix/ext.")
		assert.Equal(t, 50, count, "should cap at 50 matches")
	})

	t.Run("does not set cap annotation when under 50 matches", func(t *testing.T) {
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{
						"field":       "args",
						"match":       "--ext-[a-z]+",
						"trimPrefix":  "--ext-",
						"labelPrefix": "prefix/ext.",
					},
				},
			},
		}
		jobData := map[string]any{
			"args": []any{"--ext-foo", "--ext-bar"},
		}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.NotContains(t, out, "posit.team/label-cap-reached")
	})

	t.Run("skips empty suffix after sanitization", func(t *testing.T) {
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{
						"field":       "user",
						"match":       ".+",
						"labelPrefix": "prefix/ext.",
					},
				},
			},
		}
		jobData := map[string]any{"user": "---"}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.NotContains(t, out, "prefix/ext.")
	})
}
