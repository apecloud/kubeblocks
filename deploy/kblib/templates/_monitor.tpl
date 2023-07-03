{{/*
Define monitor
*/}}
{{- define "kblib.componentMonitor" }}
{{- if eq (int .Values.extra.monitoringInterval) 0 }}
monitor: false
{{- else }}
monitor: true
{{- end }}
{{- end }}