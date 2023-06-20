{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "apecloud-mysql-cluster.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "apecloud-mysql-cluster.labels" -}}
helm.sh/chart: {{ include "apecloud-mysql-cluster.chart" . }}
{{ include "cluster-libchart.clusterLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Define the cluster componnets with proxy
*/}}
{{- define "apecloud-mysql-cluster.proxyComponents" }}
- name: etcd
  componentDefRef: etcd # ref clusterdefinition componentDefs.name
  replicas: 1
- name: vtctld
  componentDefRef: vtctld # ref clusterdefinition componentDefs.name
  replicas: 1
- name: vtconsensus
  componentDefRef: vtconsensus # ref clusterdefinition componentDefs.name
  replicas: 1
- name: vtgate
  componentDefRef: vtgate # ref clusterdefinition componentDefs.name
  replicas: 1
{{- end }}

{{/*
Define replica count
*/}}
{{- define "apecloud-mysql-cluster.replicaCount" -}}
{{- if eq .Values.mode "standalone" }}
replicas: 1
{{- else if eq .Values.mode "replication" }}
replicas: {{ max .Values.replicas 2 }}
{{- else }}
replicas: {{ max .Values.replicas 3 }}
{{- end }}
{{- end -}}