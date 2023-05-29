{{/*
Expand the name of the chart.
*/}}
{{- define "delphic.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "delphic.fullname" -}}
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
{{- define "delphic.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "delphic.labels" -}}
helm.sh/chart: {{ include "delphic.chart" . }}
{{ include "delphic.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "delphic.selectorLabels" -}}
app.kubernetes.io/name: {{ include "delphic.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "delphic.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "delphic.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "delphic.common.envs" }}
- name: REDIS_URL
  value: redis://{{ .Release.Name }}-{{ index .Values "redis-cluster" "nameOverride" }}-redis:6379
- name: MODEL_NAME
  value: text-davinci-003
- name: MAX_TOKENS
  value: "512"
- name: USE_DOCKER
  value: "yes"
- name: POSTGRES_HOST
  valueFrom:
    secretKeyRef:
      name: {{ .Release.Name }}-{{ .Values.pgcluster.nameOverride }}-conn-credential
      key: host
- name: POSTGRES_PORT
  valueFrom:
    secretKeyRef:
      name: {{ .Release.Name }}-{{ .Values.pgcluster.nameOverride }}-conn-credential
      key: port
- name: POSTGRES_USER
  valueFrom:
    secretKeyRef:
      name: {{ .Release.Name }}-{{ .Values.pgcluster.nameOverride }}-conn-credential
      key: username
- name: POSTGRES_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ .Release.Name }}-{{ .Values.pgcluster.nameOverride }}-conn-credential
      key: password
- name: POSTGRES_DB
  value: delphic
{{- end }}
