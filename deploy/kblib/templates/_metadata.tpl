{{/*
Define the cluster name.
We truncate at 15 chars because KubeBlocks will concatenate the names of other resources with cluster name
*/}}
{{- define "kblib.clusterName" }}
{{- $name := .Release.Name }}
{{- if not (regexMatch "^[a-z]([-a-z0-9]*[a-z0-9])?$" $name) }}
{{ fail (printf "Release name %q is invalid. It must match the regex %q." $name "^[a-z]([-a-z0-9]*[a-z0-9])?$") }}
{{- end }}
{{- if gt (len $name) 15 }}
{{ fail (printf "Release name %q is invalid, must be no more than 15 characters" $name) }}
{{- end }}
{{- $name }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "kblib.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Define cluster labels
*/}}
{{- define "kblib.clusterLabels" -}}
helm.sh/chart: {{ include "kblib.chart" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/instance: {{ include "kblib.clusterName" . }}
{{- end }}