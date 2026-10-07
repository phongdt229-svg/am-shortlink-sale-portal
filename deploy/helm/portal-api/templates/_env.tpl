{{- define "portal-api.env" -}}
{{- range $k, $v := .Values.env }}
- name: {{ $k }}
  value: {{ $v | quote }}
{{- end }}
{{- range $k, $v := .Values.secretFiles }}
- name: {{ $k }}
  value: {{ $v | quote }}
{{- end }}
{{- end -}}
