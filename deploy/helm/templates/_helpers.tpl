{{/*
Expand the name of the chart.
*/}}
{{- define "opendbaas-core.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "opendbaas-core.fullname" -}}
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
{{- define "opendbaas-core.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "opendbaas-core.labels" -}}
helm.sh/chart: {{ include "opendbaas-core.chart" . }}
{{ include "opendbaas-core.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "opendbaas-core.selectorLabels" -}}
app.kubernetes.io/name: {{ include "opendbaas-core.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "opendbaas-core.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "opendbaas-core.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the webhook service.
*/}}
{{- define "opendbaas-core.svcName" -}}
{{ include "opendbaas-core.fullname" . }}
{{- end }}

{{/*
Create the default PodDisruptionBudget to use.
*/}}
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

{{/*
Print KubeBlocks's logo.
*/}}
{{- define "_logo" -}}
{{ print "\033[36m" }}
{{ print " __    __          __                _______  __                   __                 " }}
{{ print "|  \\  /  \\        |  \\              |       \\|  \\                 |  \\                " }}
{{ print "| ▓▓ /  ▓▓__    __| ▓▓____   ______ | ▓▓▓▓▓▓▓\\ ▓▓ ______   _______| ▓▓   __  _______  " }}
{{ print "| ▓▓/  ▓▓|  \\  |  \\ ▓▓    \\ /      \\| ▓▓__/ ▓▓ ▓▓/      \\ /       \\ ▓▓  /  \\/       \\ " }}
{{ print "| ▓▓  ▓▓ | ▓▓  | ▓▓ ▓▓▓▓▓▓▓\\  ▓▓▓▓▓▓\\ ▓▓    ▓▓ ▓▓  ▓▓▓▓▓▓\\  ▓▓▓▓▓▓▓ ▓▓_/  ▓▓  ▓▓▓▓▓▓▓ " }}
{{ print "| ▓▓▓▓▓\\ | ▓▓  | ▓▓ ▓▓  | ▓▓ ▓▓    ▓▓ ▓▓▓▓▓▓▓\\ ▓▓ ▓▓  | ▓▓ ▓▓     | ▓▓   ▓▓ \\▓▓    \\  " }}
{{ print "| ▓▓ \\▓▓\\| ▓▓__/ ▓▓ ▓▓__/ ▓▓ ▓▓▓▓▓▓▓▓ ▓▓__/ ▓▓ ▓▓ ▓▓__/ ▓▓ ▓▓_____| ▓▓▓▓▓▓\\ _\\▓▓▓▓▓▓\\ " }}
{{ print "| ▓▓  \\▓▓\\\\▓▓    ▓▓ ▓▓    ▓▓\\▓▓     \\ ▓▓    ▓▓ ▓▓\\▓▓    ▓▓\\▓▓     \\ ▓▓  \\▓▓\\       ▓▓ " }}
{{ print " \\▓▓   \\▓▓ \\▓▓▓▓▓▓ \\▓▓▓▓▓▓▓  \\▓▓▓▓▓▓▓\\▓▓▓▓▓▓▓ \\▓▓ \\▓▓▓▓▓▓  \\▓▓▓▓▓▓▓\\▓▓   \\▓▓\\▓▓▓▓▓▓▓  " }}
{{ print "\033[0m" }}
{{- end }}

{{/*
Print line divider.
*/}}
{{- define "_divider" -}}
{{ print "--------------------------------------------------------------------------------" }}
{{- end }}

{{/*
Print the supplied value in yellow.
*/}}
{{- define "_fmt.yellow" -}}
{{ print "\033[0;33m" . "\033[0m" }}
{{- end }}

{{/*
Print the supplied value in blue.
*/}}
{{- define "_fmt.blue" -}}
{{ print "\033[36m" . "\033[0m" }}
{{- end }}



                                                                                     
                                                                                     