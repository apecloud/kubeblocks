{{/*
Define replica count.
standalone mode: 1
raftGroup mode: 3 or more
*/}}
{{- define "tdengine-cluster.replicaCount" -}}
{{- if eq .Values.mode "standalone" }}
replicas: 1
{{- else }}
replicas: {{ max .Values.replicas 3 }}
{{- end }}
{{- end -}}