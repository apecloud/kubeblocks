{{/*
Expand the name of the chart.
*/}}
{{- define "foxlake.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "foxlake.fullname" -}}
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
{{- define "foxlake.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "foxlake.labels" -}}
helm.sh/chart: {{ include "foxlake.chart" . }}
{{ include "foxlake.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "foxlake.selectorLabels" -}}
app.kubernetes.io/name: {{ include "foxlake.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "foxlake.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "foxlake.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
FoxLake MetaDB Service FQDN
*/}}
{{- define "foxlake.metadb.fqdn" -}}
{{- printf "$(KB_CLUSTER_NAME)-foxlake-metadb.$(KB_NAMESPACE).svc" }}
{{- end }}

{{/*
FoxLake Configration Environment Variables
*/}}
{{- define "foxlake.env" -}}
- name: FOXLAKE_ROOT_USER
  value: "foxlake_root"
- name: FOXLAKE_ROOT_PASSWORD
  valueFrom:
    secretKeyRef:
      name: $(CONN_CREDENTIAL_SECRET_NAME)
      key: password  
- name: rootPasswd
  valueFrom:
    secretKeyRef:
      name: $(CONN_CREDENTIAL_SECRET_NAME)
      key: password
- name: instanceId
  valueFrom:
    fieldRef:
      fieldPath: metadata.labels['app.kubernetes.io/instance']
- name: clusterName
  value: "devcluster"
- name: rpcPort
  value: "10030"
- name: serverPort
  value: "11288"
- name: managerPort
  value: "11289"
- name: metaDbUser
  value: "root"
- name: metaDbPasswd
  valueFrom:
    secretKeyRef:
      name: $(CONN_CREDENTIAL_SECRET_NAME)
      key: metaDbPasswd
- name: metaDbName
  value: "foxlake_meta_db"
- name: META_DB_PORT
  value: "3306"
- name: META_DB_HOST
  value: "{{ include "foxlake.metadb.fqdn" . }}"
- name: MY_POD_NAMESPACE
  value: $(KB_NAMESPACE)
- name: MY_POD_NAME
  value: $(KB_POD_NAME)
- name: MY_POD_IP
  value: $(KB_POD_IP)
- name: MY_POD_RPC_SERVICE_NAME
  value: "$(KB_CLUSTER_NAME)-foxlake-server"
- name: MPP_WORKER_CONTAINER_IMAGE
  value: {{ .Values.images.foxlake.repository }}:{{ .Values.images.foxlake.tag }}
{{- end}}