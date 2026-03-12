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
{{- /* Convert .Job to a map so hasKey/index work regardless of whether the launcher passes a struct or map */ -}}
{{- $jobMap := .Job | toJson | mustFromJson }}
{{- $capStatus := dict }}
{{- $matchCache := dict }}
{{- $globalTotal := dict "n" 0 }}
{{- with $templateData.pod.dynamicLabels }}
{{- range $i, $rule := . }}
{{- if and (hasKey $jobMap $rule.field) $rule.match }}
{{- $val := index $jobMap $rule.field }}
{{- $str := (kindIs "slice" $val) | ternary ($val | join " ") ($val | toString) }}
{{- /* Cap raw matches at 500 to bound memory before dedup/per-rule cap (50) is applied. */ -}}
{{- $matches := regexFindAll $rule.match $str 500 }}
{{- /* Deduplicate matches so duplicates don't consume cap budget */ -}}
{{- $seen := dict }}
{{- $deduped := list }}
{{- range $m := $matches }}
{{- if not (hasKey $seen $m) }}{{- $_ := set $seen $m "1" }}{{- $deduped = append $deduped $m }}{{- end }}
{{- end }}
{{- $matches = $deduped }}
{{- if gt (len $matches) 50 }}{{- $_ := set $capStatus "reached" "true" }}{{- $matches = slice $matches 0 50 }}{{- end }}
{{- $newTotal := add (index $globalTotal "n") (len $matches) | int }}
{{- if gt $newTotal 200 }}{{- $allowed := sub 200 (index $globalTotal "n") | int }}{{- if gt $allowed 0 }}{{- $matches = slice $matches 0 $allowed }}{{- else }}{{- $matches = list }}{{- end }}{{- $_ := set $capStatus "reached" "true" }}{{- end }}
{{- $_ := set $globalTotal "n" (add (index $globalTotal "n") (len $matches) | int) }}
{{- $_ := set $matchCache (printf "%d" $i) $matches }}
{{- end }}
{{- end }}
{{- end }}
{{- if hasKey $capStatus "reached" }}
posit.team/dynamic-label-cap-reached: "true"
{{- end }}
{{- with $templateData.pod.dynamicLabels }}
{{- range $i, $rule := . }}
{{- if hasKey $jobMap $rule.field }}
{{- $val := index $jobMap $rule.field }}
{{- if $rule.labelKey }}
{{- $labelVal := $val | toString | regexReplaceAll "[^a-zA-Z0-9._-]" "_" | regexReplaceAll "_{2,}" "_" | trunc 63 | regexReplaceAll "[^a-zA-Z0-9]+$" "" | regexReplaceAll "^[^a-zA-Z0-9]+" "" }}
{{- if ne $labelVal "" }}
{{ $rule.labelKey }}: {{ $labelVal | quote }}
{{- end }}
{{- else if $rule.match }}
{{- $matches := index $matchCache (printf "%d" $i) }}
{{- $namePrefix := regexFind "[^/]*$" $rule.labelPrefix }}
{{- /* Go validation (ValidateDynamicLabelRules) enforces namePrefix < 53 chars, so $maxSuffix is always > 0. */ -}}
{{- $maxSuffix := int (sub 63 (len $namePrefix)) }}
{{- range $match := $matches }}
{{- $suffix := trimPrefix ($rule.trimPrefix | default "") $match | lower | regexReplaceAll "[^a-zA-Z0-9._-]" "_" | regexReplaceAll "_{2,}" "_" | trunc $maxSuffix | regexReplaceAll "[^a-zA-Z0-9]+$" "" | regexReplaceAll "^[^a-zA-Z0-9]+" "" }}
{{- $computedKey := printf "%s%s" $rule.labelPrefix $suffix }}
{{- /* must match reservedOperatorAnnotationKey in session_config.go */ -}}
{{- if and (ne $suffix "") (ne $computedKey "posit.team/dynamic-label-cap-reached") }}
{{ $computedKey }}: {{ $rule.labelValue | default "true" | quote }}
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
	// TODO: These Go tests exercise a mocked argument order, not the real Helm rendering
	// pipeline. Consider adding an integration test that renders through `helm template`
	// to validate the actual end-to-end rendering path.
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

// TestDynamicLabelsTemplate_DriftDetection verifies that the dynamicLabelsTemplate
// test constant has not drifted from the actual job.tpl. It checks that critical
// template snippets (cap values, sanitization regexes, guard conditions) appear in
// both the test constant and the embedded template file.
func TestDynamicLabelsTemplate_DriftDetection(t *testing.T) {
	actual := jobTpl // embedded via //go:embed 2.5.0/job.tpl in template_helpers.go

	// Critical snippets that must appear in both the test template and job.tpl.
	// If any snippet is missing from either, the test and production template have diverged.
	snippets := []struct {
		name    string
		snippet string
	}{
		{"per-rule cap", "slice $matches 0 50"},
		{"global cap", "sub 200"},
		{"raw match cap", "regexFindAll $rule.match $str 500"},
		{"sanitize non-alnum", `regexReplaceAll "[^a-zA-Z0-9._-]" "_"`},
		{"collapse underscores", `regexReplaceAll "_{2,}" "_"`},
		{"reserved key guard", `posit.team/dynamic-label-cap-reached`},
		{"dedup seen dict", "$seen := dict"},
		{"global total dict", `$globalTotal := dict "n" 0`},
	}

	for _, s := range snippets {
		t.Run(s.name, func(t *testing.T) {
			assert.Contains(t, dynamicLabelsTemplate, s.snippet,
				"snippet missing from test template constant — update dynamicLabelsTemplate to match job.tpl")
			assert.Contains(t, actual, s.snippet,
				"snippet missing from job.tpl — update job.tpl to match dynamicLabelsTemplate")
		})
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

	t.Run("collapses consecutive underscores in direct mapping", func(t *testing.T) {
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{"field": "user", "labelKey": "session.posit.team/user"},
				},
			},
		}
		jobData := map[string]any{"user": "foo@@bar"}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.Contains(t, out, `session.posit.team/user: "foo_bar"`)
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

	t.Run("renders direct mapping label from numeric field via toString", func(t *testing.T) {
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{"field": "port", "labelKey": "session.posit.team/port"},
				},
			},
		}
		jobData := map[string]any{"port": 8080}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.Contains(t, out, `session.posit.team/port: "8080"`)
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

	t.Run("deduplicates regex matches", func(t *testing.T) {
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
			"args": []any{"--ext-foo", "--ext-foo", "--ext-bar"},
		}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.Contains(t, out, `session.posit.team/ext.foo: "enabled"`)
		assert.Contains(t, out, `session.posit.team/ext.bar: "enabled"`)
		count := strings.Count(out, "session.posit.team/ext.")
		assert.Equal(t, 2, count, "duplicate matches should be deduplicated to 2 labels")
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
		assert.Contains(t, out, `posit.team/dynamic-label-cap-reached: "true"`)
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
		assert.NotContains(t, out, "posit.team/dynamic-label-cap-reached")
	})

	t.Run("extracts labels from regex match on numeric field", func(t *testing.T) {
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{
						"field":       "port",
						"match":       "[0-9]+",
						"labelPrefix": "session.posit.team/port.",
					},
				},
			},
		}
		jobData := map[string]any{"port": 8080}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.Contains(t, out, `session.posit.team/port.8080: "true"`)
	})

	t.Run("global cap limits total matches across all rules to 200", func(t *testing.T) {
		// Create 5 rules, each with 60 unique matches (300 total before caps).
		// Per-rule cap: 50 each → 250. Global cap: 200.
		rules := make([]map[string]any, 5)
		for r := 0; r < 5; r++ {
			rules[r] = map[string]any{
				"field":       fmt.Sprintf("field%d", r),
				"match":       fmt.Sprintf("r%d_[0-9]+", r),
				"labelPrefix": fmt.Sprintf("prefix/r%d.", r),
			}
		}
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": rules,
			},
		}
		jobData := map[string]any{}
		for r := 0; r < 5; r++ {
			vals := make([]any, 60)
			for i := range vals {
				vals[i] = fmt.Sprintf("r%d_%03d", r, i)
			}
			jobData[fmt.Sprintf("field%d", r)] = vals
		}

		out := renderDynamicLabels(t, templateData, jobData)
		assert.Contains(t, out, `posit.team/dynamic-label-cap-reached: "true"`)
		// Count total dynamic labels across all rules
		totalLabels := 0
		for r := 0; r < 5; r++ {
			totalLabels += strings.Count(out, fmt.Sprintf("prefix/r%d.", r))
		}
		assert.Equal(t, 200, totalLabels, "global cap should limit total matches to 200")
	})

	t.Run("explicit empty trimPrefix behaves the same as omitting it", func(t *testing.T) {
		jobData := map[string]any{
			"args": []any{"--ext-foo"},
		}

		explicitData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{{
					"field":       "args",
					"match":       "--ext-[a-z]+",
					"trimPrefix":  "",
					"labelPrefix": "session.posit.team/ext.",
				}},
			},
		}
		outExplicit := renderDynamicLabels(t, explicitData, jobData)

		omittedData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{{
					"field":       "args",
					"match":       "--ext-[a-z]+",
					"labelPrefix": "session.posit.team/ext.",
				}},
			},
		}
		outOmitted := renderDynamicLabels(t, omittedData, jobData)

		assert.Equal(t, outExplicit, outOmitted, "explicit empty trimPrefix should produce the same output as omitting it")
		assert.Contains(t, outExplicit, `session.posit.team/ext.`)
	})

	t.Run("runtime guard drops computed key matching reserved annotation", func(t *testing.T) {
		// Craft a labelPrefix + regex that produces the reserved key
		// "posit.team/dynamic-label-cap-reached" at runtime.
		templateData := map[string]any{
			"pod": map[string]any{
				"dynamicLabels": []map[string]any{
					{
						"field":       "args",
						"match":       ".+",
						"labelPrefix": "posit.team/",
					},
				},
			},
		}
		jobData := map[string]any{
			"args": []any{"dynamic-label-cap-reached"},
		}

		out := renderDynamicLabels(t, templateData, jobData)
		// The label should NOT appear because the runtime guard skips it.
		assert.NotContains(t, out, `posit.team/dynamic-label-cap-reached: "true"`)
		// Also verify no cap annotation was emitted (we're under cap limits).
		assert.NotContains(t, out, "posit.team/dynamic-label-cap-reached")
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
