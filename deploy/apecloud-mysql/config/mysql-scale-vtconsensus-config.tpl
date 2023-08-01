[vtconsensus]
refresh_interval=1s
scan_repair_timeout=1s

{{ block "logsBlock" . }}
{{- if hasKey $.component "enabledLogs" }}
enable_logs=true
{{- end }}
{{ end }}