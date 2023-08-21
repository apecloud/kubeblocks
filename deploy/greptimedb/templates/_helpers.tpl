{{/*
Expand the name of the chart.
*/}}
{{- define "greptimedb.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "greptimedb.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "greptimedb.labels" -}}
helm.sh/chart: {{ include "greptimedb.chart" . }}
{{ include "greptimedb.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "greptimedb.selectorLabels" -}}
app.kubernetes.io/name: {{ include "greptimedb.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
