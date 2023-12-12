{{/*
Allow the release namespace to be overridden for multi-namespace deployments in combined charts
*/}}
{{- define "bytebase.namespace" -}}
  {{- if .Values.namespaceOverride -}}
    {{- .Values.namespaceOverride -}}
  {{- else -}}
    {{- .Release.Namespace -}}
  {{- end -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "bytebase.labels" -}}
{{ include "bytebase.selectorLabels" . }}
app.kubernetes.io/version: {{ .Values.bytebase.version}}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "bytebase.selectorLabels" -}}
app: bytebase
{{- end }}


{{- define "bytebase.imageRegistry" }}
{{- if not .Values.images. }}
{{- "infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com" }}
{{- else }}
{{- .Values.images.registry }}
{{- end}}
{{- end}}