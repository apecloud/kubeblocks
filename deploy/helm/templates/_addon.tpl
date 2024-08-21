{{/*
Define addon Helm charts image information.
*/}}
{{- define "kubeblocks.addonChartsImage" }}
{{- $addonImageRegistry := include "kubeblocks.imageRegistry" . }}
chartsImage: {{ .Values.addonChartsImage.registry | default $addonImageRegistry }}/{{ .Values.addonChartsImage.repository }}:{{ .Values.addonChartsImage.tag | default .Chart.AppVersion }}
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
chartLocationURL: {{ include "kubeblocks.addonChartLocationURLValue" . }}
{{- end }}

{{- define "kubeblocks.addonChartLocationURLValue" }}
{{- $fullChart := print .name "-" .version -}}
{{- $base := .values.addonChartLocationBase -}}
{{- if hasPrefix "oci://" $base -}}
{{ $base }}/{{ .name }}
{{- else if hasPrefix "https://github.com/apecloud/helm-charts/releases/download" $base -}}
{{ $base }}/{{ $fullChart }}/{{ $fullChart }}.tgz
{{- else -}}
{{ $base }}/{{ $fullChart }}.tgz
{{- end -}}
{{- end -}}

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
{{- $install := .Release.IsInstall }}
{{- $upgrade := (and .Release.IsUpgrade .Values.upgradeAddons) }}
{{- $existingAddon := lookup "extensions.kubeblocks.io/v1alpha1" "Addon" "" .name -}}
{{- if or $install (and $upgrade (not $existingAddon)) -}}
{{- include "kubeblocks.buildAddon" . }}
{{- else if and (not $upgrade) $existingAddon -}}
{{- $obj := fromYaml (toYaml $existingAddon) -}}
{{- $metadata := get $obj "metadata" -}}
{{- $metadata = unset $metadata "managedFields" -}}
{{- $metadata = unset $metadata "resourceVersion" -}}
{{- $obj = set $obj "metadata" $metadata -}}
{{ $obj | toYaml }}
{{- else if and $upgrade $existingAddon -}}
{{- $addonCR := include "kubeblocks.buildAddon" . -}}
{{- $addonObj := fromYaml $addonCR -}}
{{- $spec := get $addonObj "spec" -}}
{{- $installable := get (get $existingAddon "spec") "installable" }}
{{- if $installable -}}
{{- $spec = set $spec "installable" $installable -}}
{{- end -}}
{{- $install = get (get $existingAddon "spec") "install" -}}
{{- if $install -}}
{{- $spec = set $spec "install" $install -}}
{{- end -}}
{{- $addonObj = set $addonObj "spec" $spec -}}
{{ $addonObj | toYaml }}
{{- end -}}
{{- end -}}

{{- define "kubeblocks.addonHelmInstallOptions" }}
{{- if hasPrefix "oci://" .values.addonChartLocationBase }}
installOptions:
  version: {{ .version }}
{{- end }}
{{- end }}

{{- define "kubeblocks.buildAddon" }}
{{- $addonImageRegistry := include "kubeblocks.imageRegistry" . }}
{{- $cloudProvider := (include "kubeblocks.cloudProvider" .) }}
apiVersion: extensions.kubeblocks.io/v1alpha1
kind: Addon
metadata:
  name: {{ .name }}
  labels:
    {{- .selectorLabels | nindent 4 }}
    addon.kubeblocks.io/version: {{ .version }}
    addon.kubeblocks.io/name: {{ .name }}
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
    chartsImage: {{ .Values.addonChartsImage.registry | default $addonImageRegistry }}/{{ .Values.addonChartsImage.repository }}:{{ .Values.addonChartsImage.tag | default .Chart.AppVersion }}
    chartsPathInImage: {{ .Values.addonChartsImage.chartsPath }}
    {{- include "kubeblocks.addonHelmInstallOptions" ( dict "version" .version "values" .Values) | indent 4 }}
    {{- if and (eq .name "pulsar") (eq $cloudProvider "huaweiCloud") }}
    installValues:
      setValues:
        - cloudProvider=huaweiCloud
    {{- end }}
  defaultInstallValues:
  - enabled: true
  installable:
    autoInstall: {{ .autoInstall }}
{{- end }}