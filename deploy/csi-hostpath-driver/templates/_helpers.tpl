{{/*
Expand the name of the chart.
*/}}
{{- define "csi-hostpath-driver.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "csi-hostpath-driver.fullname" -}}
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
{{- define "csi-hostpath-driver.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "csi-hostpath-driver.labels" -}}
helm.sh/chart: {{ include "csi-hostpath-driver.chart" . }}
{{ include "csi-hostpath-driver.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "csi-hostpath-driver.selectorLabels" -}}
app.kubernetes.io/name: {{ include "csi-hostpath-driver.name" . }}
app.kubernetes.io/instance: hostpath.csi.k8s.io
app.kubernetes.io/part-of: csi-driver-host-path
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "csi-hostpath-driver.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "csi-hostpath-driver.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
