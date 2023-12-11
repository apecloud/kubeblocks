{{/*
Expand the name of the chart.
*/}}
{{- define "oceanbase.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "oceanbase.fullname" -}}
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
{{- define "oceanbase.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "oceanbase.labels" -}}
helm.sh/chart: {{ include "oceanbase.chart" . }}
{{ include "oceanbase.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "oceanbase.selectorLabels" -}}
app.kubernetes.io/name: {{ include "oceanbase.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "oceanbase.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "oceanbase.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}


{{/*
Create extra env
*/}}
{{- define "oceanbase-cluster.extra-envs" }}
{
{{- if .Values.tenant -}}
"TENANT_NAME": "{{ .Values.tenant.name | default "tenant1" }}",
"TENANT_CPU": "{{ .Values.tenant.max_cpu | default "2" }}",
"TENANT_MEMORY": "{{ print .Values.tenant.memory_size "G" | default "2G" }}",
"TENANT_DISK": "{{ print .Values.tenant.log_disk_size "G" | default "5G" }}",
{{- end -}}
"ZONE_COUNT": "{{ .Values.zoneCount | default "1" }}",
"OB_CLUSTERS_COUNT": "{{ .Values.obClusters | default "1" }}"
}
{{- end }}

{{/*
Create extra envs annotations
*/}}
{{- define "oceanbase-cluster.annotations.extra-envs" }}
"kubeblocks.io/extra-env": {{ include "oceanbase-cluster.extra-envs" . | nospace  | quote }}
{{- end }}
