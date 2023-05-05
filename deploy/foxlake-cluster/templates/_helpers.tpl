{{/*
Expand the name of the chart.
*/}}
{{- define "foxlake-cluster.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "foxlake-cluster.fullname" -}}
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
{{- define "foxlake-cluster.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "foxlake-cluster.labels" -}}
helm.sh/chart: {{ include "foxlake-cluster.chart" . }}
{{ include "foxlake-cluster.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "foxlake-cluster.selectorLabels" -}}
app.kubernetes.io/name: {{ include "foxlake-cluster.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "foxlake-cluster.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "foxlake-cluster.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "foxlake-cluster.deployEnv" -}}
{{- if contains "eks" .Capabilities.KubeVersion.GitVersion -}}
cloud
{{- else -}}
local
{{- end }}
{{- end }}

{{- define "post-job.env" -}}
{{- if eq (include "foxlake-cluster.deployEnv" .) "cloud" -}}
- name: S3_BUCKET_NAME
  value: {{ .Values.s3BucketName }}
- name: AWS_ACCESS_KEY_ID
  valueFrom:
    secretKeyRef:
      name: csi-s3-secret
      key:  accessKeyID 
- name: AWS_SECRET_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: csi-s3-secret
      key: secretAccessKey
- name: AWS_DEFAULT_REGION
  value: cn-northwest-1
{{- else -}}
- name: MINIO_BUCKET_NAME
  valueFrom:
    configMapKeyRef:
      name: minio-chart-kubeblocks-values
      key: bucketName
- name: MINIO_ACCESS_KEY_ID
  valueFrom:
    secretKeyRef:
      name: kb-addon-minio
      key: rootUser
- name: MINIO_SECRET_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: kb-addon-minio
      key: rootPassword
{{- end }}
{{- end }}

{{- define "foxlake-endpoint.env" -}}
- name: FOXLAKE_HOST
  valueFrom:
    secretKeyRef:
      name: {{ .Release.Name }}-conn-credential
      key: host
- name: FOXLAKE_PORT
  valueFrom:
    secretKeyRef:
      name: {{ .Release.Name }}-conn-credential
      key: port
{{- end }}