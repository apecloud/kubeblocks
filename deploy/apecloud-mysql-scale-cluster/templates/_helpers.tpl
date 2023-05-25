{{/*
Expand the name of the chart.
*/}}
{{- define "apecloud-mysql-cluster.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "apecloud-mysql-cluster.fullname" -}}
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
{{- define "apecloud-mysql-cluster.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "apecloud-mysql-cluster.labels" -}}
helm.sh/chart: {{ include "apecloud-mysql-cluster.chart" . }}
{{ include "apecloud-mysql-cluster.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "apecloud-mysql-cluster.selectorLabels" -}}
app.kubernetes.io/name: {{ include "apecloud-mysql-cluster.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "clustername" -}}
{{ include "apecloud-mysql-cluster.fullname" .}}
{{- end}}

{{/*
Create the name of the service account to use
*/}}
{{- define "apecloud-mysql-cluster.serviceAccountName" -}}
{{- default (printf "kb-%s" (include "clustername" .)) .Values.serviceAccount.name }}
{{- end }}
