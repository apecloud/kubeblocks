{{/*
Define the cluster name.
We truncate at 15 chars because KubeBlocks will concatenate the names of other resources with cluster name
*/}}
{{- define "cluster-libchart.clusterName" }}
{{- (.Release.Name | trunc 15 | trimSuffix "-") | default "mycluster" }}
{{- end }}

{{/*
Define cluster labels
*/}}
{{- define "cluster-libchart.clusterLabels" -}}
app.kubernetes.io/instance: {{ include "cluster-libchart.clusterName" . }}
{{- end }}