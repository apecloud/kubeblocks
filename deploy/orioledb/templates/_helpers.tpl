{{/*
Expand the name of the chart.
*/}}
{{- define "orioledb.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "orioledb.fullname" -}}
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
{{- define "orioledb.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "orioledb.labels" -}}
helm.sh/chart: {{ include "orioledb.chart" . }}
{{ include "orioledb.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "orioledb.selectorLabels" -}}
app.kubernetes.io/name: {{ include "orioledb.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "orioledb.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "orioledb.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}


{{/*
    custom config below
*/}}

{{/*
Return true if a configmap object should be created for PostgreSQL primary with the configuration
*/}}
{{- define "postgresql.primary.createConfigmap" -}}
{{- if and (or .Values.primary.configuration .Values.primary.pgHbaConfiguration) (not .Values.primary.existingConfigmap) }}
    {{- true -}}
{{- else -}}
{{- end -}}
{{- end -}}

{{/*
Return PostgreSQL service port
*/}}
{{- define "postgresql.service.port" -}}
{{- .Values.primary.service.ports.postgresql -}}
{{- end -}}

{{/*
Return the name for a custom database to create
*/}}
{{- define "postgresql.database" -}}
{{- .Values.auth.database -}}
{{- end -}}

{{/*
Get the password key.
*/}}
{{/* TODO: use $(RANDOM_PASSWD) instead */}}
{{- define "postgresql.postgresPassword" -}}
{{- if or (.Release.IsInstall) (not (lookup "apps.kubeblocks.io/v1alpha1" "ClusterDefinition" "" "postgresql")) -}}
{{ .Values.auth.postgresPassword | default (randAlphaNum 10) }}
{{- else -}}
{{ index (lookup "apps.kubeblocks.io/v1alpha1" "ClusterDefinition" "" "postgresql").spec.connectionCredential "password"}}
{{- end }}
{{- end }}

