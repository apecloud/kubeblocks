{{/*
Expand the name of the chart.
*/}}
{{- define "mysql.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "mysql.fullname" -}}
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
{{- define "mysql.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "mysql.labels" -}}
helm.sh/chart: {{ include "mysql.chart" . }}
{{ include "mysql.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "mysql.selectorLabels" -}}
app.kubernetes.io/name: {{ include "mysql.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "mysql.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "mysql.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
apecloud-otel config
*/}}
{{- define "agamotto.config" -}}
extensions:
  memory_ballast:
    size_mib: 32

receivers:
  apecloudmysql:
    endpoint: ${env:ENDPOINT}
    username: ${env:MYSQL_USER}
    password: ${env:MYSQL_PASSWORD}
    allow_native_passwords: true
    database:
    collection_interval: 15s
    transport: tcp
  filelog/error:
    include: [/data/mysql/log/mysqld-error.log]
    include_file_name: false
    start_at: beginning
  filelog/slow:
    include: [/data/mysql/log/mysqld-slowquery.log]
    include_file_name: false
    start_at: beginning

processors:
  memory_limiter:
    limit_mib: 128
    spike_limit_mib: 32
    check_interval: 10s

exporters:
  prometheus:
    endpoint: 0.0.0.0:{{ .Values.metrics.service.port }}
    send_timestamps: false
    metric_expiration: 20s
    enable_open_metrics: false
    resource_to_telemetry_conversion:
      enabled: true
  apecloudfile/error:
    path: /var/log/kubeblocks/${env:KB_NAMESPACE}_${env:DB_TYPE}_${env:KB_CLUSTER_NAME}/${env:KB_POD_NAME}/error.log
    format: raw
    rotation:
      max_megabytes: 10
      max_days: 3
      max_backups: 1
      localtime: true
  apecloudfile/slow:
    path: /var/log/kubeblocks/${env:KB_NAMESPACE}_${env:DB_TYPE}_${env:KB_CLUSTER_NAME}/${env:KB_POD_NAME}/slow.log
    format: raw
    rotation:
      max_megabytes: 10
      max_days: 3
      max_backups: 1
      localtime: true

service:
  telemetry:
    logs:
      level: info
  extensions: [ memory_ballast ]
  pipelines:
    metrics:
      receivers: [ apecloudmysql ]
      processors: [ memory_limiter ]
      exporters: [ prometheus ]
    logs/error:
      receivers: [filelog/error]
      exporters: [apecloudfile/error]
    logs/slow:
      receivers: [filelog/slow]
      exporters: [apecloudfile/slow]
{{- end }}

{{/*
apecloud-otel config for proxy
*/}}
{{- define "proxy-monitor.config" -}}
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: 'agamotto'
          scrape_interval: 15s
          static_configs:
            - targets: ['127.0.0.1:15100']
service:
  pipelines:
    metrics:
      receivers: [ apecloudmysql, prometheus ]
{{- end }}