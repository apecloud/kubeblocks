{{- define "mariadb-cluster.replicas" }}
{{- if eq .Values.mode "standalone" }}
{{- 1 }}
{{- end -}}
{{- end -}}