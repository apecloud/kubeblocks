{{/*
Expand the name of the chart.
*/}}
{{- define "kubeblocks.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "kubeblocks.fullname" -}}
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
{{- define "kubeblocks.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kubeblocks.labels" -}}
helm.sh/chart: {{ include "kubeblocks.chart" . }}
{{ include "kubeblocks.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kubeblocks.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kubeblocks.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "kubeblocks.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "kubeblocks.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the addon installer name of the service account to use
*/}}
{{- define "kubeblocks.addonSAName" -}}
{{- printf "%s-%s" (include "kubeblocks.serviceAccountName" .) "addon-installer" }}
{{- end }}

{{/*
Create the name of the webhook service.
*/}}
{{- define "kubeblocks.svcName" -}}
{{ include "kubeblocks.fullname" . }}
{{- end }}

{{/*
matchLabels
*/}}
{{- define "kubeblocks.matchLabels" -}}
{{ template "kubeblocks.selectorLabels" . }}
{{- end -}}

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


{{/*
Allow the release namespace to be overridden for multi-namespace deployments in combined charts
*/}}
{{- define "kubeblocks.namespace" -}}
  {{- if .Values.namespaceOverride -}}
    {{- .Values.namespaceOverride -}}
  {{- else -}}
    {{- .Release.Namespace -}}
  {{- end -}}
{{- end -}}




{{/*
Use the prometheus namespace override for multi-namespace deployments in combined charts
*/}}
{{- define "kubeblocks.prometheus.namespace" -}}
  {{- if .Values.prometheus.namespaceOverride -}}
    {{- .Values.prometheus.namespaceOverride -}}
  {{- else -}}
    {{- .Release.Namespace -}}
  {{- end -}}
{{- end -}}

{{/*
Use the grafana namespace override for multi-namespace deployments in combined charts
*/}}
{{- define "kubeblocks.grafana.namespace" -}}
  {{- if .Values.grafana.namespaceOverride -}}
    {{- .Values.grafana.namespaceOverride -}}
  {{- else -}}
    {{- .Release.Namespace -}}
  {{- end -}}
{{- end -}}

{{/*
Create a fully qualified Prometheus server name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "kubeblocks.prometheus.server.fullname" -}}
{{- if .Values.prometheus.server.fullnameOverride -}}
{{- .Values.prometheus.server.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Values.prometheus.nameOverride "prometheus" -}}
{{- if contains $name .Release.Name -}}
{{- printf "%s-%s" .Release.Name .Values.prometheus.server.name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s-%s" .Release.Name $name .Values.prometheus.server.name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create a fully qualified alertmanager name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "kubeblocks.prometheus.alertmanager.fullname" -}}
{{- if .Values.prometheus.alertmanager.fullnameOverride -}}
{{- .Values.prometheus.alertmanager.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Values.prometheus.nameOverride "prometheus" -}}
{{- if contains $name .Release.Name -}}
{{- printf "%s-%s" .Release.Name .Values.prometheus.alertmanager.name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s-%s" .Release.Name $name .Values.prometheus.alertmanager.name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "kubeblocks.grafana.fullname" -}}
{{- if .Values.grafana.fullnameOverride -}}
{{- .Values.grafana.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Values.grafana.nameOverride "grafana" -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Specify KubeBlocks Operator deployment with priorityClassName=system-cluster-critical, if deployed to "kube-system"
namespace and unspecified priorityClassName.
*/}}
{{- define "kubeblocks.priorityClassName" -}}
{{- if .Values.priorityClassName -}}
{{- .Values.priorityClassName }}
{{- else if ( eq ( include "kubeblocks.namespace" . ) "kube-system" ) -}}
{{- "system-cluster-critical" -}}
{{- else -}}
{{- "" -}}
{{- end -}}
{{- end -}}

{{/*
Get addon controller enabled attributes.
*/}}
{{- define "kubeblocks.addonControllerEnabled" -}}
{{- if and .Values.addonController .Values.addonController.enabled }}
{{- true }}
{{- else }}
{{- false }}
{{- end }}
{{- end }}

{{/*
Define addon prometheus name
*/}}
{{- define "addon.prometheus.name" -}}
{{- print "prometheus" }}
{{- end }}

{{/*
Define addon alertmanager-webhook-adaptor name
*/}}
{{- define "addon.alertmanager-webhook-adaptor.name" -}}
{{- print "alertmanager-webhook-adaptor" }}
{{- end }}

{{/*
Define addon loki name
*/}}
{{- define "addon.loki.name" -}}
{{- print "loki" }}
{{- end }}

{{/*
Define addon agamotto name
*/}}
{{- define "addon.agamotto.name" -}}
{{- print "agamotto" }}
{{- end }}