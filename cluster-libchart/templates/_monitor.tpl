{{/*
Define monitor
*/}}
{{- define "cluster-libchart.componentMonitor" }}
{{- if eq (int .Values.monitoringInterval) 0 }}
monitor: false
{{- else }}
monitor: true
{{- end }}
{{- end }}