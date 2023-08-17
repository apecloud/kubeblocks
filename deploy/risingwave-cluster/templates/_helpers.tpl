{{/*
Expand the name of the chart.
*/}}
{{- define "risingwave-cluster.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "risingwave-cluster.fullname" -}}
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
{{- define "risingwave-cluster.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "risingwave-cluster.labels" -}}
helm.sh/chart: {{ include "risingwave-cluster.chart" . }}
{{ include "risingwave-cluster.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "risingwave-cluster.selectorLabels" -}}
app.kubernetes.io/name: {{ include "risingwave-cluster.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "clustername" -}}
{{ include "risingwave-cluster.fullname" .}}
{{- end}}

{{/*
Create the name of the service account to use
*/}}
{{- define "risingwave-cluster.serviceAccountName" -}}
{{- default .Values.risingwave.stateStore.s3.authentication.serviceAccountName .Values.serviceAccount.name }}
{{- end }}

{{/*
Create the hummock option
*/}}
{{- define "risingwave-cluster.options.hummock" }}
hummock+s3://{{ .Values.risingwave.stateStore.s3 }}
{{- end }}

{{/*
Create extra env
*/}}
{{- define "risingwawve-cluster.extra-envs" }}
{
"RW_STATE_STORE": "hummock+s3://{{ .Values.risingwave.stateStore.s3.bucket }}",
"AWS_REGION": "{{ .Values.risingwave.stateStore.s3.region }}",
{{- if eq .Values.risingwave.stateStore.s3.authentication.serviceAccountName "" }}
"AWS_ACCESS_KEY_ID": "{{ .Values.risingwave.stateStore.s3.authentication.accessKey }}",
"AWS_SECRET_ACCESS_KEY": "{{ .Values.risingwave.stateStore.s3.authentication.secretAccessKey }}",
{{- end }}
"RW_DATA_DIRECTORY": "{{ .Values.risingwave.stateStore.dataDirectory }}",
{{- if .Values.risingwave.stateStore.s3.endpoint }}
"RW_S3_ENDPOINT": "{{ .Values.risingwave.stateStore.s3.endpoint }}",
{{- end }}
{{- if .Values.risingwave.metaStore.etcd.authentication.enabled }}
"RW_ETCD_USERNAME": "{{ .Values.risingwave.metaStore.etcd.authentication.username }}",
"RW_ETCD_PASSWORD": "{{ .Values.risingwave.metaStore.etcd.authentication.password }}",
{{- end }}
"RW_ETCD_ENDPOINTS": "{{ .Values.risingwave.metaStore.etcd.endpoints }}",
"RW_ETCD_AUTH": "{{ .Values.risingwave.metaStore.etcd.authentication.enabled}}"
}
{{- end }}

{{/*
Create the hummock option
*/}}
{{- define "risingwave-cluster.annotations.extra-envs" }}
"kubeblocks.io/extra-env": {{ include "risingwawve-cluster.extra-envs" . | nospace  | quote }}
{{- end }}