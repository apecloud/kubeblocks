{{/*
Expand the name of the chart.
*/}}
{{- define "flink-cluster.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "flink-cluster.fullname" -}}
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
{{- define "flink-cluster.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "flink-cluster.labels" -}}
helm.sh/chart: {{ include "flink-cluster.chart" . }}
{{ include "flink-cluster.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "flink-cluster.selectorLabels" -}}
app.kubernetes.io/name: {{ include "flink-cluster.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the jobmanager deployment
*/}}
{{- define "flink-cluster.jobmanager.fullname" -}}
    {{ printf "%s-jobmanager" (include "flink-cluster.fullname" .)  | trunc 63 | trimSuffix "-" }}
{{- end -}}

{{/*
Create the name of the taskmanager deployment
*/}}
{{- define "flink-cluster.taskmanager.fullname" -}}
    {{ printf "%s-taskmanager" (include "flink-cluster.fullname" .)  | trunc 63 | trimSuffix "-" }}
{{- end -}}


{{/*
Create the name of the service account to use for the taskmanager
*/}}
{{- define "flink-cluster.taskmanager.serviceAccountName" -}}
{{- if .Values.taskmanager.serviceAccount.create -}}
    {{ default (include "flink-cluster.taskmanager.fullname" .) .Values.taskmanager.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.taskmanager.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{/*
Create the name of the service account to use for the jobmanager
*/}}
{{- define "flink-cluster.jobmanager.serviceAccountName" -}}
{{- if .Values.jobmanager.serviceAccount.create -}}
    {{ default (include "flink-cluster.jobmanager.fullname" .) .Values.jobmanager.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.jobmanager.serviceAccount.name }}
{{- end -}}
{{- end -}}