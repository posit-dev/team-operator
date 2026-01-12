{{- define "rstudio-library.templates.skeleton" -}}
{{- printf "{{- define \"%s\" }}" .name -}}
{{- .value | toYaml | nindent 0 }}
{{ printf "{{- end }}" -}}
{{- end }}

{{- define "rstudio-library.templates.dataOutput" -}}
{{- printf "{{- define \"%s\" }}" .name -}}
{{- .value | toJson | nindent 0 }}
{{ printf "{{- end }}" -}}
{{- end }}

{{- define "rstudio-library.templates.dataOutputPretty" -}}
{{- printf "{{- define \"%s\" }}" .name -}}
{{- .value | toPrettyJson | nindent 0 }}
{{ printf "{{- end }}" -}}
{{- end }}
