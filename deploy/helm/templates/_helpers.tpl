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
Get cloud provider, now support aws, gcp, aliyun and tencentCloud.
TODO: For azure, we should get provider from node.Spec.ProviderID
*/}}
{{- define "kubeblocks.cloudProvider" }}
{{- $kubeVersion := .Capabilities.KubeVersion.GitVersion }}
{{- $validProviders := .Values.validProviders}}
{{- $provider := .Values.provider }}
{{- $valid := false }}
{{- range $validProviders }}
    {{- if eq . $provider }}
        {{- $valid = true }}
    {{- end }}
{{- end }}
{{- if contains "-eks" $kubeVersion }}
{{- "aws" -}}
{{- else if contains "-gke" $kubeVersion }}
{{- "gcp" -}}
{{- else if contains "-aliyun" $kubeVersion }}
{{- "aliyun" -}}
{{- else if contains "-tke" $kubeVersion }}
{{- "tencentCloud" -}}
{{- else if contains "-aks" $kubeVersion }}
{{- "azure" -}}
{{- else if $valid }}
{{- $provider }}
{{- else}}
{{- $invalidProvider := join ", " .Values.validProviders }}
{{- $errorMessage := printf "Warning: Your provider is invalid. Please use one of the following: %s" $invalidProvider | trimSuffix ", " }}
{{- fail $errorMessage}}
{{- end }}
{{- end }}


{{/*
Define default storage class name, if cloud provider is known, specify a default storage class name.
*/}}
{{- define "kubeblocks.defaultStorageClass" }}
{{- $cloudProvider := (include "kubeblocks.cloudProvider" .) }}
{{- if and .Values.storageClass .Values.storageClass.name }}
{{- .Values.storageClass.name }}
{{- else if $cloudProvider }}
{{- "kb-default-sc"  }}
{{- else }}
{{- "" }}
{{- end }}
{{- end }}


{{- define "kubeblocks.imageRegistry" }}
{{- if not .Values.image.registry }}
{{- "apecloud-registry.cn-zhangjiakou.cr.aliyuncs.com" }}
{{- else }}
{{- .Values.image.registry }}
{{- end }}
{{- end }}

{{/*
Define the replica count for kubeblocks.
*/}}
{{- define "kubeblocks.replicaCount" }}
{{- if and .Values.webhooks.conversionEnabled .Release.IsInstall }}
{{- print 0 }}
{{- else }}
{{- .Values.replicaCount }}
{{- end }}
{{- end }}

{{- define "kubeblocks.i18nResourcesName" -}}
{{ include "kubeblocks.fullname" . }}-i18n-resources
{{- end }}
