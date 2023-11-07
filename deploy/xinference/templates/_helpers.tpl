{{/*
Expand the name of the chart.
*/}}
{{- define "xinference.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "xinference.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "xinference.labels" -}}
helm.sh/chart: {{ include "xinference.chart" . }}
{{ include "xinference.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "xinference.selectorLabels" -}}
app.kubernetes.io/name: {{ include "xinference.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
