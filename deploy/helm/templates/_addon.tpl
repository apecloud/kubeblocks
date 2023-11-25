{{/*
Define addon Helm charts image information.
*/}}
{{- define "kubeblocks.addonChartsImage" }}
chartsImage: {{ .Values.addonChartsImage.registry | default "docker.io" }}/{{ .Values.addonChartsImage.repository }}:{{ .Values.addonChartsImage.tag | default .Chart.AppVersion }}
chartsPathInImage: {{ .Values.addonChartsImage.chartsPath }}
{{- end }}

{{/*
Define addon helm location URL
Usage:
{{- include "kubeblocks.addonChartLocationURL" ( dict "name" "CHART-NAME" "version" "VERSION" "values" .Values) }}
Example:
{{- include "kubeblocks.addonChartLocationURL" ( dict "name" "apecloud-mysql" "version" "0.5.0" "values" .Values) }}
*/}}
{{- define "kubeblocks.addonChartLocationURL" }}
{{- $fullChart := print .name "-" .version }}
{{- $base := .values.addonChartLocationBase }}
{{- if hasPrefix "oci://" $base }}
chartLocationURL: {{ $base }}/{{ .name }}
{{- else if hasPrefix "https://github.com/apecloud/helm-charts/releases/download" $base }}
chartLocationURL: {{ $base }}/{{ $fullChart }}/{{ $fullChart }}.tgz
{{- else }}
chartLocationURL: {{ $base }}/{{ $fullChart }}.tgz
{{- end }}
{{- end }}

{{/*
Build add-on CR.
When upgrade KubeBlocks, if the add-on CR already exists, we will do not upgrade it if
cascadeUpgradeAddons is false, and use the existing add-on. Otherwise, we will upgrade it.

Parameters:
- name: name of the addon
- version: version of the addon
- model: model of the addon
- provider: provider of the addon
- descripton: description of the addon
- autoInstall: autoInstall of the addon
- kbVersion: KubeBlocks version that this addon is compatible with
*/}}
{{- define "kubeblocks.buildAddonCR" }}
{{- $upgrade:= or .Release.IsInstall (and .Release.IsUpgrade .Values.upgradeAddons) }}
{{- $existingAddon := lookup "extensions.kubeblocks.io/v1alpha1" "Addon" "" .name -}}
{{- if and (not $upgrade) $existingAddon -}}
{{- $obj := fromYaml (toYaml $existingAddon) -}}
{{- $metadata := get $obj "metadata" -}}
{{- $metadata = unset $metadata "managedFields" -}}
{{- $metadata = unset $metadata "resourceVersion" -}}
{{- $obj = set $obj "metadata" $metadata -}}
{{ $obj | toYaml }}
{{- else -}}
apiVersion: extensions.kubeblocks.io/v1alpha1
kind: Addon
metadata:
  name: {{ .name }}
  labels:
    app.kubernetes.io/name: {{ .name }}
    app.kubernetes.io/version: {{ .version }}
    addon.kubeblocks.io/provider: {{ .provider }}
    addon.kubeblocks.io/model: {{ .model }}
  annotations:
    addon.kubeblocks.io/kubeblocks-version: {{ .kbVersion | squote }}
  {{- if .Values.keepAddons }}
    helm.sh/resource-policy: keep
  {{- end }}
spec:
  description: {{ .description | squote }}
  type: Helm
  helm:
    {{- include "kubeblocks.addonChartLocationURL" ( dict "name" .name "version" .version "values" .Values) | indent 4 }}
    chartsImage: {{ .Values.addonChartsImage.registry | default "docker.io" }}/{{ .Values.addonChartsImage.repository }}:{{ .Values.addonChartsImage.tag | default .Chart.AppVersion }}
    chartsPathInImage: {{ .Values.addonChartsImage.chartsPath }}
    installOptions:
      {{- if hasPrefix "oci://" .Values.addonChartLocationBase }}
      version: {{ .version }}
      {{- end }}
  defaultInstallValues:
  - enabled: true
  installable:
    autoInstall: {{ .autoInstall }}
{{- end -}}
{{- end -}}