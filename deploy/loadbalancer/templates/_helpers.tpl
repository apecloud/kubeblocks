{{/*
Expand the name of the chart.
*/}}
{{- define "loadbalancer.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "loadbalancer.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "loadbalancer.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "loadbalancer.labels" -}}
helm.sh/chart: {{ include "loadbalancer.chart" . }}
{{ include "loadbalancer.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "loadbalancer.selectorLabels" -}}
app.kubernetes.io/name: {{ include "loadbalancer.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "loadbalancer.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "loadbalancer.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/* Create the default PodDisruptionBudget to use */}}
{{- define "podDisruptionBudget.spec" -}}
{{- if and .Values.podDisruptionBudget.minAvailable .Values.podDisruptionBudget.maxUnavailable }}
{{- fail "Cannot set both .Values.podDisruptionBudget.minAvailable and .Values.podDisruptionBudget.maxUnavailable" -}}
{{- end }}
{{- if not .Values.podDisruptionBudget.maxUnavailable }}
minAvailable: {{ default 1 .Values.podDisruptionBudget.minAvailable }}
{{- end }}
{{- if .Values.podDisruptionBudget.maxUnavailable }}
maxUnavailable: {{ .Values.podDisruptionBudget.maxUnavailable }}
{{- end }}
{{- end }}

{{- define "loadbalancer.nodeLabels" -}}
{{- if .Values.nodeLabelList }}
{{- join "," .Values.nodeLabelList }}
{{- end }}
{{- end }}

{{- define "loadbalancer.endpointLabels" -}}
{{- if .Values.endpointLabels }}
{{- join "," .Values.endpointLabelList }}
{{- end }}
{{- end }}
