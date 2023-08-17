{{/*
Define addon Helm charts image information.
*/}}
{{- define "kubeblocks.addonChartsImage" }}
chartsImage: {{ .Values.addonChartsImage.registry | default "docker.io" }}/{{ .Values.addonChartsImage.repository }}:{{ .Values.addonChartsImage.tag | default .Chart.AppVersion }}
chartsPathInImage: {{ .Values.addonChartsImage.chartsPath }}
{{- end }}

{{/*
Define addon helm localtion URL
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